/**
 * ViewAura A/B Testing Edge Worker
 *
 * Route: viewaura.com/* (runs on every request)
 *
 * Assignment strategy:
 *   - Authenticated users: keyed by user_id (extracted from JWT sub claim
 *     WITHOUT validating the signature — we only need the stable ID, not
 *     the auth guarantee, which Kong handles separately).
 *   - Unauthenticated users: keyed by session cookie (__va_sid).
 *     The cookie is created on first visit and persists for 30 days.
 *
 * Stickiness:
 *   Assignment is stored in KV with a 30-day TTL.
 *   The same user always receives the same variant for a given experiment,
 *   even across devices if they are authenticated.
 *
 * The worker:
 *   1. Reads or creates the stable user/session key.
 *   2. Looks up or computes variant assignments for all active experiments.
 *   3. Injects a single X-AB-Experiments header into the proxied request:
 *      X-AB-Experiments: home_feed_layout=b,recommendation_algo=a
 *   4. The Go API reads this header and serves the appropriate variant.
 *
 * Variant assignment determinism:
 *   For a given key + experiment_id, the same variant is always assigned
 *   by hashing (key + experiment_id) and comparing against the weightA
 *   threshold. This is deterministic without KV — KV is only used to
 *   persist the assignment for analytics correlation.
 */

import { EXPERIMENTS, type Experiment } from "../shared/types";

interface Env {
  AB_ASSIGNMENTS:       KVNamespace;
  ENVIRONMENT:          string;
  ASSIGNMENT_TTL_SECS:  string;
  SESSION_COOKIE:       string;
  SESSION_COOKIE_MAXAGE: string;
  DEBUG_HEADERS:        string;
}

type Variant = "a" | "b" | "control";

interface AssignmentMap {
  [experimentId: string]: Variant;
}

export default {
  async fetch(request: Request, env: Env, ctx: ExecutionContext): Promise<Response> {
    // ── Derive stable identity key ────────────────────────────────────────────
    const { key: identityKey, sessionCookie } = resolveIdentity(request, env);

    // ── Compute / retrieve variant assignments ────────────────────────────────
    const activeExperiments = EXPERIMENTS.filter(e => e.active);
    const assignments       = await getOrCreateAssignments(
      env, identityKey, activeExperiments, ctx
    );

    // ── Build the experiment header string ────────────────────────────────────
    // Format: "experiment_id=variant,experiment_id=variant"
    const experimentHeader = Object.entries(assignments)
      .map(([id, variant]) => `${id}=${variant}`)
      .join(",");

    // ── Proxy request with injected header ───────────────────────────────────
    const requestHeaders = new Headers(request.headers);
    requestHeaders.set("X-AB-Experiments", experimentHeader);
    requestHeaders.set("X-AB-Identity",    identityKey.slice(0, 16) + "…");  // truncated for privacy

    const proxiedRequest = new Request(request, { headers: requestHeaders });
    const response       = await fetch(proxiedRequest);

    // ── Build response ────────────────────────────────────────────────────────
    const responseHeaders = new Headers(response.headers);

    // Set session cookie on first visit (unauthenticated users)
    if (sessionCookie.isNew) {
      const maxAge = env.SESSION_COOKIE_MAXAGE;
      responseHeaders.append(
        "Set-Cookie",
        `${env.SESSION_COOKIE}=${sessionCookie.value}; Max-Age=${maxAge}; Path=/; SameSite=Lax; Secure`
      );
    }

    // Debug headers (development only)
    if (env.DEBUG_HEADERS === "true") {
      responseHeaders.set("X-AB-Experiments-Debug", experimentHeader);
    }

    return new Response(response.body, {
      status:  response.status,
      headers: responseHeaders,
    });
  },
};

// ─── Identity resolution ──────────────────────────────────────────────────────

interface IdentityResult {
  key:           string;
  sessionCookie: { value: string; isNew: boolean };
}

function resolveIdentity(request: Request, env: Env): IdentityResult {
  // Try to extract user_id from JWT (without verifying signature)
  const authHeader = request.headers.get("Authorization") ?? "";
  if (authHeader.startsWith("Bearer ")) {
    const userId = extractJwtSubject(authHeader.slice(7));
    if (userId) {
      return {
        key:           `user:${userId}`,
        sessionCookie: { value: "", isNew: false },
      };
    }
  }

  // Fall back to session cookie
  const cookies    = parseCookies(request.headers.get("Cookie") ?? "");
  const cookieName = env.SESSION_COOKIE;
  let   sessionId  = cookies[cookieName];
  let   isNew      = false;

  if (!sessionId) {
    sessionId = generateSessionId();
    isNew     = true;
  }

  return {
    key:           `session:${sessionId}`,
    sessionCookie: { value: sessionId, isNew },
  };
}

/** Extract `sub` claim from a JWT without verifying the signature. */
function extractJwtSubject(token: string): string | null {
  try {
    const parts   = token.split(".");
    if (parts.length !== 3) return null;
    const payload = JSON.parse(atob(parts[1].replace(/-/g, "+").replace(/_/g, "/")));
    return typeof payload.sub === "string" ? payload.sub : null;
  } catch {
    return null;
  }
}

// ─── Assignment logic ─────────────────────────────────────────────────────────

async function getOrCreateAssignments(
  env:         Env,
  identityKey: string,
  experiments: Experiment[],
  ctx:         ExecutionContext,
): Promise<AssignmentMap> {
  const kvKey = `ab:${identityKey}`;
  const stored = await env.AB_ASSIGNMENTS.get<AssignmentMap>(kvKey, { type: "json" });

  let assignments: AssignmentMap = stored ?? {};
  let dirty = false;

  // Assign any experiments not yet in the stored map
  for (const experiment of experiments) {
    if (!(experiment.id in assignments)) {
      assignments[experiment.id] = computeVariant(identityKey, experiment);
      dirty = true;
    }
  }

  // Persist if changed
  if (dirty) {
    const ttl = parseInt(env.ASSIGNMENT_TTL_SECS, 10);
    ctx.waitUntil(
      env.AB_ASSIGNMENTS.put(kvKey, JSON.stringify(assignments), {
        expirationTtl: ttl,
      })
    );
  }

  return assignments;
}

/**
 * Deterministic variant assignment via FNV-1a hash.
 *
 * For a given identityKey + experimentId, always returns the same variant.
 * No randomness — the hash IS the assignment. This means:
 *   - No KV read required for assignment (KV is only for analytics lookups)
 *   - Assignment is consistent across pods, regions, and restarts
 */
function computeVariant(identityKey: string, experiment: Experiment): Variant {
  if (!experiment.active) return "control";

  const input     = `${identityKey}:${experiment.id}`;
  const hashValue = fnv1aHash(input);
  // Normalize to 0.0–1.0
  const normalized = (hashValue >>> 0) / 0xFFFFFFFF;

  return normalized < experiment.weightA ? "a" : "b";
}

/** FNV-1a 32-bit hash — fast, low collision rate, runs in Workers V8 sandbox */
function fnv1aHash(input: string): number {
  let hash = 0x811c9dc5;
  for (let i = 0; i < input.length; i++) {
    hash ^= input.charCodeAt(i);
    hash  = (hash * 0x01000193) >>> 0;   // unsigned 32-bit
  }
  return hash;
}

// ─── Utilities ────────────────────────────────────────────────────────────────

function parseCookies(cookieHeader: string): Record<string, string> {
  return Object.fromEntries(
    cookieHeader.split(";")
      .map(c => c.trim().split("="))
      .filter(p => p.length === 2)
      .map(([k, v]) => [k.trim(), v.trim()])
  );
}

function generateSessionId(): string {
  // crypto.randomUUID() is available in Workers runtime
  return crypto.randomUUID().replace(/-/g, "");
}