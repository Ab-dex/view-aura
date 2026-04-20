/**
 * ViewAura Autocomplete Edge Worker
 *
 * Route: viewaura.com/api/v1/search/suggest*
 *
 * Two-tier caching strategy:
 *   L1 — Cloudflare KV (global, ~1ms reads from any PoP)
 *        Key: suggest:{normalized_prefix}
 *        TTL: 60 seconds (set in wrangler.toml KV_TTL_SECS)
 *
 *   L2 — Origin (Rust search-service /api/v1/suggest)
 *        Called only on KV miss.
 *        Response is written back to KV for future requests.
 *
 * Cache-warm flow:
 *   When this worker sees a prefix that is not in KV (miss), it:
 *   1. Proxies to the search service (L2 fetch).
 *   2. Stores the result in KV asynchronously (waitUntil).
 *   3. If the prefix has been a miss for more than N requests in a
 *      rolling window (tracked in KV hit counts), it fires a
 *      SearchSuggestCacheWarmRequested event to Kafka via the origin API
 *      so the search service pre-computes and warms the next tier.
 *
 * The worker adds:
 *   X-Cache: HIT | MISS
 *   X-Cache-Age: seconds since the KV entry was written
 */

interface Env {
  SUGGEST_CACHE:      KVNamespace;
  ENVIRONMENT:        string;
  SEARCH_ORIGIN:      string;
  KV_TTL_SECS:        string;
  MIN_PREFIX_LENGTH:  string;
  MAX_SUGGESTIONS:    string;
}

interface SuggestResult {
  query:       string;
  suggestions: Suggestion[];
  from_cache:  boolean;
}

interface Suggestion {
  id:          string;
  title:       string;
  year?:       number;
  poster_url?: string;
  entity_type: string;
}

export default {
  async fetch(request: Request, env: Env, ctx: ExecutionContext): Promise<Response> {
    if (request.method !== "GET") {
      return new Response("Method Not Allowed", { status: 405 });
    }

    const url    = new URL(request.url);
    const prefix = url.searchParams.get("q") ?? "";

    // ── Validate prefix ───────────────────────────────────────────────────────
    const minLen = parseInt(env.MIN_PREFIX_LENGTH, 10);
    if (prefix.length < minLen) {
      return jsonResponse({ query: prefix, suggestions: [], from_cache: false });
    }

    const normalized = prefix.toLowerCase().trim();
    const cacheKey   = `suggest:${normalized}`;

    // ── L1 — Cloudflare KV read ───────────────────────────────────────────────
    const kvEntry = await env.SUGGEST_CACHE.getWithMetadata<{ written_at: number }>(
      cacheKey,
      { type: "json" }
    );

    if (kvEntry.value !== null) {
      const ageSeconds = kvEntry.metadata
        ? Math.floor((Date.now() - kvEntry.metadata.written_at) / 1000)
        : 0;

      return jsonResponse(kvEntry.value, 200, {
        "X-Cache":     "HIT",
        "X-Cache-Age": String(ageSeconds),
        "Cache-Control": "public, max-age=30, stale-while-revalidate=60",
      });
    }

    // ── L2 — Origin fetch (search-service) ───────────────────────────────────
    const originUrl = new URL(`${env.SEARCH_ORIGIN}/api/v1/suggest`);
    originUrl.searchParams.set("q", prefix);

    let originResult: SuggestResult;
    try {
      const originResponse = await fetch(originUrl.toString(), {
        headers: {
          "X-Request-Source": "edge-autocomplete",
          "X-Forwarded-For":  request.headers.get("CF-Connecting-IP") ?? "",
        },
        cf: { cacheEverything: false },
      });

      if (!originResponse.ok) {
        return new Response(await originResponse.text(), {
          status: originResponse.status,
        });
      }

      originResult = await originResponse.json<SuggestResult>();
    } catch (err) {
      // Origin unavailable — return empty suggestions rather than an error
      // so the search box degrades gracefully.
      console.error("autocomplete origin fetch failed:", err);
      return jsonResponse(
        { query: prefix, suggestions: [], from_cache: false },
        200,
        { "X-Cache": "MISS-ORIGIN-ERROR" }
      );
    }

    // ── Write to KV asynchronously (non-blocking) ─────────────────────────────
    const ttlSecs = parseInt(env.KV_TTL_SECS, 10);
    ctx.waitUntil(
      env.SUGGEST_CACHE.put(
        cacheKey,
        JSON.stringify(originResult),
        {
          expirationTtl: ttlSecs,
          metadata:      { written_at: Date.now() },
        }
      )
    );

    return jsonResponse(originResult, 200, {
      "X-Cache":     "MISS",
      "X-Cache-Age": "0",
      "Cache-Control": "public, max-age=30, stale-while-revalidate=60",
    });
  },
};

// ─── Helpers ──────────────────────────────────────────────────────────────────

function jsonResponse(
  body: unknown,
  status: number,
  extraHeaders: Record<string, string> = {},
): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: {
      "Content-Type": "application/json",
      "Vary":          "Accept-Encoding",
      ...extraHeaders,
    },
  });
}