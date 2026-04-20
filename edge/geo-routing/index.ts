/**
 * ViewAura Geo-Routing Edge Worker
 *
 * Route: viewaura.com/* (runs on every request)
 *
 * Routing strategy:
 *   1. Read CF-IPCountry header (set by Cloudflare for free).
 *   2. Map country → nearest origin region.
 *   3. Check KV for origin health (written by the scheduled health checker).
 *   4. If the preferred origin is unhealthy, fail over to the next nearest.
 *   5. Rewrite the request URL to the selected origin and proxy.
 *
 * The worker adds context headers to the proxied request:
 *   X-Origin-Region    — the selected region (for logging)
 *   X-Origin-Fallback  — "true" when the preferred origin was unhealthy
 *   X-User-Country     — ISO-3166 country code
 *
 * Health check (scheduled every minute via Cron Trigger):
 *   Each region's /health endpoint is polled. Response time > DEGRADED_THRESHOLD_MS
 *   or non-200 response marks the region as "unhealthy" in KV (TTL 90s).
 */

import { COUNTRY_TO_REGION, ORIGINS, OriginRegion } from "../shared/types";

interface Env {
  ORIGIN_HEALTH:          KVNamespace;
  ENVIRONMENT:            string;
  ORIGIN_US:              string;
  ORIGIN_EU:              string;
  ORIGIN_AP:              string;
  ORIGIN_FALLBACK:        string;
  HEALTH_PATH:            string;
  HEALTH_TIMEOUT_MS:      string;
  DEGRADED_THRESHOLD_MS:  string;
}

type HealthStatus = "healthy" | "unhealthy" | "degraded";

export default {
  // ── Request handler ─────────────────────────────────────────────────────────
  async fetch(request: Request, env: Env, ctx: ExecutionContext): Promise<Response> {
    const url     = new URL(request.url);
    const country = (request.cf?.country as string | undefined) ?? "US";

    // Select preferred region from country mapping
    const preferredRegion: OriginRegion = COUNTRY_TO_REGION[country] ?? "us-east-1";

    // Build region priority list: preferred → next nearest → fallback
    const regionPriority = buildRegionPriority(preferredRegion);

    // Select the first healthy region
    const { region, origin, isFallback } = await selectOrigin(
      env, regionPriority
    );

    // Rewrite URL to the selected origin
    const targetUrl  = new URL(url.pathname + url.search, origin);
    const proxied    = new Request(targetUrl.toString(), {
      method:  request.method,
      headers: new Headers({
        ...Object.fromEntries(request.headers),
        "X-Forwarded-Host":  url.host,
        "X-Forwarded-Proto": url.protocol.replace(":", ""),
        "X-Origin-Region":   region,
        "X-Origin-Fallback": String(isFallback),
        "X-User-Country":    country,
      }),
      body:    ["GET", "HEAD"].includes(request.method) ? null : request.body,
    });

    const response = await fetch(proxied);

    // Surface routing info in the response headers
    return new Response(response.body, {
      status:  response.status,
      headers: new Headers({
        ...Object.fromEntries(response.headers),
        "X-Origin-Region":  region,
        "X-User-Country":   country,
      }),
    });
  },

  // ── Scheduled health check (Cron Trigger) ───────────────────────────────────
  async scheduled(event: ScheduledEvent, env: Env, ctx: ExecutionContext): Promise<void> {
    const origins: [OriginRegion, string][] = [
      ["us-east-1",       env.ORIGIN_US],
      ["eu-west-1",       env.ORIGIN_EU],
      ["ap-southeast-1",  env.ORIGIN_AP],
    ];

    await Promise.all(origins.map(([region, origin]) =>
      checkAndUpdateHealth(region, origin, env)
    ));
  },
};

// ─── Origin selection ─────────────────────────────────────────────────────────

async function selectOrigin(
  env:      Env,
  priority: OriginRegion[],
): Promise<{ region: OriginRegion; origin: string; isFallback: boolean }> {
  for (let i = 0; i < priority.length; i++) {
    const region     = priority[i];
    const origin     = regionToOrigin(env, region);
    const health     = await env.ORIGIN_HEALTH.get<HealthStatus>(region, { type: "json" });
    const status     = health ?? "healthy";   // assume healthy on first deploy

    if (status === "healthy") {
      return { region, origin, isFallback: i > 0 };
    }
  }

  // All regions unhealthy — use the configured fallback
  return {
    region:     "us-east-1",
    origin:     env.ORIGIN_FALLBACK,
    isFallback: true,
  };
}

function buildRegionPriority(preferred: OriginRegion): OriginRegion[] {
  const all: OriginRegion[] = ["us-east-1", "eu-west-1", "ap-southeast-1"];
  // Put preferred first, then the rest
  return [preferred, ...all.filter(r => r !== preferred)];
}

function regionToOrigin(env: Env, region: OriginRegion): string {
  switch (region) {
    case "us-east-1":       return env.ORIGIN_US;
    case "eu-west-1":       return env.ORIGIN_EU;
    case "ap-southeast-1":  return env.ORIGIN_AP;
  }
}

// ─── Health checker ───────────────────────────────────────────────────────────

async function checkAndUpdateHealth(
  region: OriginRegion,
  origin: string,
  env:    Env,
): Promise<void> {
  const timeoutMs   = parseInt(env.HEALTH_TIMEOUT_MS, 10);
  const degradedMs  = parseInt(env.DEGRADED_THRESHOLD_MS, 10);
  const healthUrl   = `${origin}${env.HEALTH_PATH}`;

  const controller  = new AbortController();
  const timeout     = setTimeout(() => controller.abort(), timeoutMs);

  let status: HealthStatus = "unhealthy";
  try {
    const start = Date.now();
    const resp  = await fetch(healthUrl, {
      signal: controller.signal,
      cf: { cacheEverything: false },
    });
    const elapsed = Date.now() - start;

    if (resp.ok && elapsed < degradedMs) {
      status = "healthy";
    } else if (resp.ok) {
      status = "degraded";
    } else {
      status = "unhealthy";
    }

    console.log(`health check ${region}: ${status} (${elapsed}ms)`);
  } catch (err) {
    console.error(`health check ${region} failed:`, err);
    status = "unhealthy";
  } finally {
    clearTimeout(timeout);
  }

  // Store health status — TTL 90s ensures stale data auto-expires if the
  // scheduled worker stops running (e.g. deployment downtime).
  await env.ORIGIN_HEALTH.put(region, JSON.stringify(status), {
    expirationTtl: 90,
  });
}