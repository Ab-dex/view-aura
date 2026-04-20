/**
 * Base REST client for the Go monolith.
 *
 * Features:
 *   - All requests carry the caller's Authorization header downstream
 *     so the Go API can enforce per-user permissions.
 *   - Circuit breaker via cockatiel: after 5 consecutive failures the
 *     circuit opens and the BFF returns a fallback value immediately
 *     instead of waiting for a timeout. This prevents cascading failures
 *     when a downstream service is slow.
 *   - Structured error logging.
 */
import {
  ConsecutiveBreaker,
  ExponentialBackoff,
  Policy,
  SamplingBreaker,
  handleAll,
  wrap,
} from "cockatiel";
import { logger } from "../logger.js";

export class ApiError extends Error {
  constructor(
    public readonly status: number,
    message: string,
    public readonly body?: unknown,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

// One circuit breaker per service — shared across all resolvers.
const makeBreaker = (label: string) =>
  Policy.wrap(
    // Retry up to 2 times with exponential backoff before opening the circuit.
    Policy.handleAll().retry().attempts(2).delay(new ExponentialBackoff()),
    // Open the circuit after 5 consecutive failures; half-open after 30s.
    Policy.handleAll().circuitBreaker(30_000, new ConsecutiveBreaker(5)),
  );

const apiBreaker   = makeBreaker("go-api");
const searchBreaker = makeBreaker("search-service");
const recBreaker   = makeBreaker("rec-service");

export class RestClient {
  constructor(
    private readonly baseUrl: string,
    private readonly breaker: ReturnType<typeof makeBreaker> = apiBreaker,
  ) {}

  async get<T>(path: string, token?: string, params?: Record<string, string>): Promise<T> {
    const url = new URL(path, this.baseUrl);
    if (params) {
      Object.entries(params).forEach(([k, v]) => url.searchParams.set(k, v));
    }
    return this.request<T>("GET", url.toString(), undefined, token);
  }

  async post<T>(path: string, body: unknown, token?: string): Promise<T> {
    return this.request<T>("POST", `${this.baseUrl}${path}`, body, token);
  }

  async put<T>(path: string, body: unknown, token?: string): Promise<T> {
    return this.request<T>("PUT", `${this.baseUrl}${path}`, body, token);
  }

  async patch<T>(path: string, body: unknown, token?: string): Promise<T> {
    return this.request<T>("PATCH", `${this.baseUrl}${path}`, body, token);
  }

  async delete<T>(path: string, token?: string): Promise<T> {
    return this.request<T>("DELETE", `${this.baseUrl}${path}`, undefined, token);
  }

  private async request<T>(
    method:  string,
    url:     string,
    body?:   unknown,
    token?:  string,
  ): Promise<T> {
    return this.breaker.execute(async () => {
      const headers: Record<string, string> = {
        "Content-Type":    "application/json",
        "X-Request-Source": "graphql-bff",
      };
      if (token) headers["Authorization"] = token;

      const res = await fetch(url, {
        method,
        headers,
        body: body != null ? JSON.stringify(body) : undefined,
      });

      if (!res.ok) {
        let errBody: unknown;
        try { errBody = await res.json(); } catch {}
        logger.warn({ url, status: res.status, body: errBody }, "upstream error");
        throw new ApiError(res.status, `Upstream ${res.status}`, errBody);
      }

      if (res.status === 204) return undefined as T;
      return res.json() as Promise<T>;
    });
  }
}

// ── Singleton clients (re-used across requests) ───────────────────────────────

const apiBaseUrl    = process.env.API_BASE_URL    ?? "http://localhost:8080";
const searchBaseUrl = process.env.SEARCH_SERVICE_URL ?? "http://localhost:9001";
const recBaseUrl    = process.env.REC_SERVICE_URL ?? "http://localhost:9101";

export const apiClient    = new RestClient(apiBaseUrl,    apiBreaker);
export const searchClient = new RestClient(searchBaseUrl, searchBreaker);
export const recClient    = new RestClient(recBaseUrl,    recBreaker);