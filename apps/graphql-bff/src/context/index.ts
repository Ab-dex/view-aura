import type { IncomingMessage } from "node:http";
import type DataLoader from "dataloader";
import { makeLoaders } from "../loaders/index.js";
import { logger } from "../logger.js";

export interface AuthUser {
  id:   string;
  role: string;
  name: string;
}

export interface ViewAuraContext {
  /** Raw Bearer token forwarded to the Go API on every downstream call. */
  token:     string | undefined;
  /** Parsed JWT claims if the request was authenticated. */
  user:      AuthUser | undefined;
  /** Per-request DataLoader instances. Never shared across requests. */
  loaders:   ReturnType<typeof makeLoaders>;
  /** Request-scoped logger with trace ID. */
  log:       typeof logger;
  /** Experiment assignments injected by the A/B edge worker. */
  experiments: Record<string, string>;
}

/**
 * Build the Apollo context for every HTTP and WS request.
 *
 * Token extraction: the Authorization header is passed through as-is to every
 * downstream Go REST call. The Go API gateway validates the JWT — the BFF
 * deliberately does NOT validate it, avoiding key management complexity and
 * keeping the BFF stateless.
 *
 * The token is also decoded (without verification) to extract the user ID for
 * DataLoader cache keys and for the `me` resolver.
 */
export function buildContext(req: IncomingMessage): ViewAuraContext {
  const authHeader = req.headers["authorization"] as string | undefined;
  const token      = authHeader?.startsWith("Bearer ") ? authHeader : undefined;
  const user       = token ? parseJwtPayload(token.slice(7)) : undefined;

  // A/B experiments injected by the edge worker
  const expHeader  = (req.headers["x-ab-experiments"] as string | undefined) ?? "";
  const experiments = Object.fromEntries(
    expHeader.split(",").filter(Boolean).map(e => e.split("=") as [string, string])
  );

  const traceId = (req.headers["x-request-id"] as string | undefined) ?? crypto.randomUUID();
  const reqLogger = logger.child({ traceId, userId: user?.id });

  return {
    token,
    user,
    loaders:     makeLoaders(token),
    log:         reqLogger,
    experiments,
  };
}

/** Decode JWT payload without verifying the signature (Go API handles auth). */
function parseJwtPayload(token: string): AuthUser | undefined {
  try {
    const parts   = token.split(".");
    if (parts.length !== 3) return undefined;
    const payload = JSON.parse(
      Buffer.from(parts[1].replace(/-/g, "+").replace(/_/g, "/"), "base64").toString()
    ) as { sub?: string; role?: string; name?: string };
    if (!payload.sub) return undefined;
    return { id: payload.sub, role: payload.role ?? "user", name: payload.name ?? "" };
  } catch {
    return undefined;
  }
}

/** Throw a standard unauthenticated error for protected resolvers. */
export function requireAuth(ctx: ViewAuraContext): asserts ctx is ViewAuraContext & { user: AuthUser; token: string } {
  if (!ctx.user || !ctx.token) {
    throw Object.assign(new Error("You must be logged in."), { extensions: { code: "UNAUTHENTICATED" } });
  }
}