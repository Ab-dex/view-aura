import type { ViewAuraContext } from "../context/index.js";
import { requireAuth } from "../context/index.js";
import { apiClient, recClient, searchClient } from "../datasources/api-client.js";
import {
  mapMovie, mapRating, mapReview, mapWatchlistEntry,
  mapUser, mapFeedItem, mapNotification, mapRecommendedMovie,
} from "./mappers.js";

export const Query = {
  // ── Home feed ───────────────────────────────────────────────────────────────
  async homeFeed(_: unknown, __: unknown, ctx: ViewAuraContext) {
    requireAuth(ctx);

    // Fan out three parallel calls — the 50ms p95 target depends on this
    const [userPrefs, trendingRes, recRes, feedRes] = await Promise.allSettled([
      apiClient.get<{ continue_watching: unknown[]; preferences: unknown }>(
        "/api/v1/me/home-context", ctx.token
      ),
      apiClient.get<{ movies: unknown[] }>("/api/v1/movies/trending?limit=20", ctx.token),
      recClient.post<{ movies: { movie_id: string; score: number; reason: string }[] }>(
        "/recommend",
        { user_id: ctx.user.id, limit: 10, context: "home_feed", exclude_watched: true }
      ),
      apiClient.get<{ items: unknown[] }>("/api/v1/social/feed?limit=5", ctx.token),
    ]);

    const continueWatching = userPrefs.status === "fulfilled"
      ? (userPrefs.value.continue_watching ?? []).map(mapWatchlistEntry)
      : [];

    const trending = trendingRes.status === "fulfilled"
      ? trendingRes.value.movies.map(mapMovie)
      : [];

    const recommended = recRes.status === "fulfilled"
      ? recRes.value.movies.map(m => ({ movie: { id: m.movie_id } as ReturnType<typeof mapMovie>, score: m.score, reason: m.reason }))
      : [];

    const friendActivity = feedRes.status === "fulfilled"
      ? feedRes.value.items.map(mapFeedItem)
      : [];

    return { continueWatching, trending, recommended, newReleases: [], friendActivity };
  },

  // ── Movies ──────────────────────────────────────────────────────────────────
  async movie(_: unknown, { id }: { id: string }, ctx: ViewAuraContext) {
    const raw = await ctx.loaders.movie.load(id);
    return raw ? mapMovie(raw) : null;
  },

  async movies(_: unknown, { input }: { input: Record<string, unknown> }, ctx: ViewAuraContext) {
    const params: Record<string, string> = {};
    for (const [k, v] of Object.entries(input)) {
      if (v != null) params[k] = String(v);
    }
    const result = await apiClient.get<{ movies: unknown[]; total: number; limit: number; offset: number }>(
      "/api/v1/movies", ctx.token, params
    );
    return {
      movies: result.movies.map(mapMovie),
      total:  result.total,
      limit:  result.limit,
      offset: result.offset,
    };
  },

  async searchMovies(_: unknown, { input }: { input: Record<string, unknown> }, ctx: ViewAuraContext) {
    const params: Record<string, string> = {};
    for (const [k, v] of Object.entries(input)) {
      if (v != null) params[k] = String(v);
    }
    const result = await searchClient.get<{ movies: unknown[]; total: number; processing_time_ms: number }>(
      "/api/v1/search", ctx.token, params
    );
    return {
      movies:           result.movies.map(mapMovie),
      total:            result.total,
      processingTimeMs: result.processing_time_ms,
    };
  },

  async trendingMovies(_: unknown, { limit = 20 }: { limit?: number }, ctx: ViewAuraContext) {
    const result = await apiClient.get<{ movies: unknown[] }>(
      "/api/v1/movies/trending", ctx.token, { limit: String(limit) }
    );
    return result.movies.map(mapMovie);
  },

  async recommendedMovies(
    _: unknown,
    { limit = 20, context = "home_feed" }: { limit?: number; context?: string },
    ctx: ViewAuraContext,
  ) {
    requireAuth(ctx);
    const result = await recClient.post<{ movies: { movie_id: string; score: number; reason: string }[] }>(
      "/recommend",
      { user_id: ctx.user.id, limit, context, exclude_watched: true },
    );
    return result.movies.map(mapRecommendedMovie);
  },

  // ── User ────────────────────────────────────────────────────────────────────
  async me(_: unknown, __: unknown, ctx: ViewAuraContext) {
    requireAuth(ctx);
    const raw = await apiClient.get<unknown>("/api/v1/me", ctx.token);
    return mapUser(raw);
  },

  async user(_: unknown, { id }: { id: string }, ctx: ViewAuraContext) {
    const raw = await ctx.loaders.user.load(id);
    return raw ? mapUser(raw) : null;
  },

  // ── Ratings ─────────────────────────────────────────────────────────────────
  async myRating(_: unknown, { movieId }: { movieId: string }, ctx: ViewAuraContext) {
    requireAuth(ctx);
    const raw = await ctx.loaders.myRating.load(movieId);
    return raw ? mapRating(raw) : null;
  },

  async movieRatings(
    _: unknown,
    { movieId, limit = 20, offset = 0 }: { movieId: string; limit?: number; offset?: number },
    ctx: ViewAuraContext,
  ) {
    const result = await apiClient.get<{ ratings: unknown[]; total: number }>(
      `/api/v1/movies/${movieId}/ratings`, ctx.token, { limit: String(limit), offset: String(offset) }
    );
    return { ratings: result.ratings.map(mapRating), total: result.total };
  },

  async movieReviews(
    _: unknown,
    { movieId, limit = 20, offset = 0, type }: { movieId: string; limit?: number; offset?: number; type?: string },
    ctx: ViewAuraContext,
  ) {
    const params: Record<string, string> = { limit: String(limit), offset: String(offset) };
    if (type) params.type = type;
    const result = await apiClient.get<{ reviews: unknown[]; total: number }>(
      `/api/v1/movies/${movieId}/reviews`, ctx.token, params
    );
    return { reviews: result.reviews.map(mapReview), total: result.total };
  },

  // ── Watchlist ───────────────────────────────────────────────────────────────
  async myWatchlist(
    _: unknown,
    { status, limit = 20, offset = 0 }: { status?: string; limit?: number; offset?: number },
    ctx: ViewAuraContext,
  ) {
    requireAuth(ctx);
    const params: Record<string, string> = { limit: String(limit), offset: String(offset) };
    if (status) params.status = status;
    const result = await apiClient.get<{ entries: unknown[]; total: number }>(
      "/api/v1/watchlist", ctx.token, params
    );
    return { entries: result.entries.map(mapWatchlistEntry), total: result.total };
  },

  async customList(_: unknown, { id }: { id: string }, ctx: ViewAuraContext) {
    const raw = await apiClient.get<unknown>(`/api/v1/lists/${id}`, ctx.token);
    return raw;
  },

  async myCustomLists(_: unknown, __: unknown, ctx: ViewAuraContext) {
    requireAuth(ctx);
    const result = await apiClient.get<{ lists: unknown[] }>("/api/v1/lists/mine", ctx.token);
    return result.lists;
  },

  // ── Social ──────────────────────────────────────────────────────────────────
  async myFeed(
    _: unknown,
    { limit = 20, offset = 0 }: { limit?: number; offset?: number },
    ctx: ViewAuraContext,
  ) {
    requireAuth(ctx);
    const result = await apiClient.get<{ items: unknown[]; total: number }>(
      "/api/v1/social/feed", ctx.token, { limit: String(limit), offset: String(offset) }
    );
    return { items: result.items.map(mapFeedItem), total: result.total };
  },

  async followers(
    _: unknown,
    { userId, limit = 20, offset = 0 }: { userId: string; limit?: number; offset?: number },
    ctx: ViewAuraContext,
  ) {
    const result = await apiClient.get<{ users: unknown[]; total: number }>(
      `/api/v1/users/${userId}/followers`, ctx.token, { limit: String(limit), offset: String(offset) }
    );
    return { users: result.users.map(mapUser), total: result.total };
  },

  async following(
    _: unknown,
    { userId, limit = 20, offset = 0 }: { userId: string; limit?: number; offset?: number },
    ctx: ViewAuraContext,
  ) {
    const result = await apiClient.get<{ users: unknown[]; total: number }>(
      `/api/v1/users/${userId}/following`, ctx.token, { limit: String(limit), offset: String(offset) }
    );
    return { users: result.users.map(mapUser), total: result.total };
  },

  // ── Notifications ────────────────────────────────────────────────────────────
  async myNotifications(
    _: unknown,
    { limit = 20, offset = 0 }: { limit?: number; offset?: number },
    ctx: ViewAuraContext,
  ) {
    requireAuth(ctx);
    const result = await apiClient.get<{ notifications: unknown[]; total: number; unread_count: number }>(
      "/api/v1/notifications", ctx.token, { limit: String(limit), offset: String(offset) }
    );
    return {
      notifications: result.notifications.map(mapNotification),
      total:         result.total,
      unreadCount:   result.unread_count,
    };
  },

  async unreadNotificationCount(_: unknown, __: unknown, ctx: ViewAuraContext) {
    requireAuth(ctx);
    const result = await apiClient.get<{ count: number }>("/api/v1/notifications/unread-count", ctx.token);
    return result.count;
  },
};