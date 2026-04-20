/**
 * ViewAura Bot Detection Edge Worker
 *
 * Route: viewaura.com/* (runs on every request)
 *
 * Checks (in order, short-circuits on first block):
 *
 *   1. Exempt paths  — webhooks and health checks bypass all checks.
 *
 *   2. Cloudflare Bot Score (cf.botManagement.score)
 *      Cloudflare assigns every request a score 1–99.
 *      Score < threshold → treat as bot.
 *      Requires Cloudflare Bot Management add-on.
 *      Gracefully degrades to heuristics when unavailable.
 *
 *   3. Known bad ASN check
 *      Blocks requests from datacenter ASNs known for scraping.
 *
 *   4. User-Agent heuristics
 *      Blocks headless browsers (empty UA, HeadlessChrome, python-requests, curl).
 *
 *   5. Per-IP sliding window rate limit (KV-backed)
 *      Unauthenticated: UNAUTH_RATE_LIMIT rpm
 *      Authenticated (Bearer token present): AUTH_RATE_LIMIT rpm
 *
 * Detected bots receive 429 (rate limit) or 403 (hard block).
 * The worker PASSES good requests through with additional context headers:
 *   X-Bot-Score   — raw CF bot score (for logging)
 *   X-Client-IP   — normalised client IP forwarded to origin
 *   X-Rate-Limit-Remaining — remaining requests in the current window
 */

import { BLOCKED_ASNS } from "../shared/types";

interface Env {
  RATE_LIMIT:           KVNamespace;
  ENVIRONMENT:          string;
  UNAUTH_RATE_LIMIT:    string;
  AUTH_RATE_LIMIT:      string;
  RATE_WINDOW_SECS:     string;
  BOT_SCORE_THRESHOLD:  string;
  CHALLENGE_BORDERLINE: string;
  EXEMPT_PATHS:         string;
}

// Cloudflare extends the Request type with bot management data
declare global {
  interface IncomingRequestCfProperties {
    botManagement?: {
      score:              number;
      verifiedBot:        boolean;
      staticResource:     boolean;
      ja3Hash:            string;
    };
    asn?:     number;
    country?: string;
  }
}

export default {
  async fetch(request: Request, env: Env, ctx: ExecutionContext): Promise<Response> {
    const url       = new URL(request.url);
    const clientIP  = request.headers.get("CF-Connecting-IP") ?? "0.0.0.0";
    const isAuth    = request.headers.get("Authorization")?.startsWith("Bearer ") ?? false;

    // ── 1. Exempt paths (webhooks, health) ───────────────────────────────────
    const exemptPrefixes = env.EXEMPT_PATHS.split(",").map(p => p.trim()).filter(Boolean);
    if (exemptPrefixes.some(p => url.pathname.startsWith(p))) {
      return fetch(request);
    }

    // ── 2. Cloudflare bot score ───────────────────────────────────────────────
    const cf            = request.cf;
    const botScore      = cf?.botManagement?.score ?? 50;   // default 50 = unknown
    const isVerifiedBot = cf?.botManagement?.verifiedBot ?? false;
    const threshold     = parseInt(env.BOT_SCORE_THRESHOLD, 10);

    // Always allow verified bots (Googlebot, Bingbot, etc.)
    if (!isVerifiedBot && botScore < threshold) {
      const isChallenge = env.CHALLENGE_BORDERLINE === "true" && botScore >= threshold * 0.5;
      if (isChallenge) {
        // Return a Cloudflare challenge page (requires Bot Management plan)
        return new Response("Challenge required", { status: 429, headers: { "Retry-After": "5" } });
      }
      return new Response(
        JSON.stringify({ error: { code: "BOT_BLOCKED", message: "Automated requests are not allowed." } }),
        { status: 403, headers: { "Content-Type": "application/json" } }
      );
    }

    // ── 3. ASN block ──────────────────────────────────────────────────────────
    const asn = cf?.asn ? `AS${cf.asn}` : "";
    if (asn && BLOCKED_ASNS.has(asn)) {
      return new Response(
        JSON.stringify({ error: { code: "ASN_BLOCKED", message: "Requests from this network are blocked." } }),
        { status: 403, headers: { "Content-Type": "application/json" } }
      );
    }

    // ── 4. User-Agent heuristics ──────────────────────────────────────────────
    const ua = request.headers.get("User-Agent") ?? "";
    if (isSuspiciousUA(ua)) {
      return new Response(
        JSON.stringify({ error: { code: "UA_BLOCKED", message: "Suspicious user agent." } }),
        { status: 403, headers: { "Content-Type": "application/json" } }
      );
    }

    // ── 5. Sliding window rate limit ─────────────────────────────────────────
    const limit      = parseInt(isAuth ? env.AUTH_RATE_LIMIT : env.UNAUTH_RATE_LIMIT, 10);
    const windowSecs = parseInt(env.RATE_WINDOW_SECS, 10);
    const rlKey      = `rl:${clientIP}`;

    const { allowed, remaining } = await checkRateLimit(env.RATE_LIMIT, rlKey, limit, windowSecs, ctx);
    if (!allowed) {
      return new Response(
        JSON.stringify({ error: { code: "RATE_LIMITED", message: "Too many requests. Please slow down." } }),
        {
          status: 429,
          headers: {
            "Content-Type":  "application/json",
            "Retry-After":   String(windowSecs),
            "X-RateLimit-Limit":     String(limit),
            "X-RateLimit-Remaining": "0",
            "X-RateLimit-Reset":     String(Math.floor(Date.now() / 1000) + windowSecs),
          },
        }
      );
    }

    // ── Pass through with context headers ────────────────────────────────────
    const modifiedRequest = new Request(request, {
      headers: new Headers({
        ...Object.fromEntries(request.headers),
        "X-Bot-Score":              String(botScore),
        "X-Client-IP":              clientIP,
        "X-Rate-Limit-Remaining":   String(remaining),
        "X-Verified-Bot":           String(isVerifiedBot),
      }),
    });

    return fetch(modifiedRequest);
  },
};

// ─── Helpers ──────────────────────────────────────────────────────────────────

/** UA patterns that strongly indicate non-human traffic */
const SUSPICIOUS_UA_PATTERNS = [
  /^$/,                           // empty UA
  /HeadlessChrome/i,
  /PhantomJS/i,
  /Selenium/i,
  /python-requests/i,
  /Go-http-client/i,
  /^curl\//,
  /libwww-perl/i,
  /scrapy/i,
  /^Java\//,
];

function isSuspiciousUA(ua: string): boolean {
  return SUSPICIOUS_UA_PATTERNS.some(p => p.test(ua));
}

/**
 * Sliding window rate limiter backed by Cloudflare KV.
 *
 * KV stores a JSON counter: { count: number, window_start: number }
 * On each request:
 *   - If window has expired: reset counter.
 *   - If within window: increment counter.
 *   - If count > limit: deny.
 *
 * KV consistency note: Workers KV is eventually consistent across PoPs.
 * This means the rate limit is approximate — a burst of requests from
 * geographically distributed clients may momentarily exceed the limit.
 * For a strict per-IP limit, use Cloudflare's built-in rate limiting rules.
 */
async function checkRateLimit(
  kv:         KVNamespace,
  key:        string,
  limit:      number,
  windowSecs: number,
  ctx:        ExecutionContext,
): Promise<{ allowed: boolean; remaining: number }> {
  const now       = Math.floor(Date.now() / 1000);
  const raw       = await kv.get<{ count: number; window_start: number }>(key, { type: "json" });

  let count = 1;
  let windowStart = now;

  if (raw && now - raw.window_start < windowSecs) {
    // Within current window
    count       = raw.count + 1;
    windowStart = raw.window_start;
  }

  const allowed   = count <= limit;
  const remaining = Math.max(0, limit - count);

  // Write back asynchronously — don't block the response
  ctx.waitUntil(
    kv.put(key, JSON.stringify({ count, window_start: windowStart }), {
      expirationTtl: windowSecs * 2,   // keep key for 2 windows to handle slow requests
    })
  );

  return { allowed, remaining };
}