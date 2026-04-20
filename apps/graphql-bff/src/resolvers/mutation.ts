import type { ViewAuraContext } from "../context/index.js";
import { requireAuth } from "../context/index.js";
import { apiClient } from "../datasources/api-client.js";
import { mapUser, mapRating, mapReview, mapWatchlistEntry } from "./mappers.js";

export const Mutation = {
  // ── Auth ────────────────────────────────────────────────────────────────────
  async register(_: unknown, { input }: { input: Record<string, unknown> }) {
    const result = await apiClient.post<{
      access_token: string; refresh_token: string; expires_in: number; user: unknown;
    }>("/api/v1/auth/register", input);
    return {
      accessToken:  result.access_token,
      refreshToken: result.refresh_token,
      expiresIn:    result.expires_in,
      user:         mapUser(result.user),
    };
  },

  async login(_: unknown, { input }: { input: Record<string, unknown> }) {
    const result = await apiClient.post<{
      access_token: string; refresh_token: string; expires_in: number; user: unknown;
    }>("/api/v1/auth/login", input);
    return {
      accessToken:  result.access_token,
      refreshToken: result.refresh_token,
      expiresIn:    result.expires_in,
      user:         mapUser(result.user),
    };
  },

  async logout(_: unknown, __: unknown, ctx: ViewAuraContext) {
    requireAuth(ctx);
    await apiClient.post<void>("/api/v1/auth/logout", {}, ctx.token);
    return true;
  },

  async refreshToken(_: unknown, { token }: { token: string }) {
    const result = await apiClient.post<{
      access_token: string; refresh_token: string; expires_in: number; user: unknown;
    }>("/api/v1/auth/refresh", { refresh_token: token });
    return {
      accessToken:  result.access_token,
      refreshToken: result.refresh_token,
      expiresIn:    result.expires_in,
      user:         mapUser(result.user),
    };
  },

  // ── Profile ─────────────────────────────────────────────────────────────────
  async updateProfile(_: unknown, { input }: { input: Record<string, unknown> }, ctx: ViewAuraContext) {
    requireAuth(ctx);
    const raw = await apiClient.patch<unknown>("/api/v1/me/profile", input, ctx.token);
    return mapUser(raw);
  },

  // ── Ratings ─────────────────────────────────────────────────────────────────
  async upsertRating(_: unknown, { input }: { input: Record<string, unknown> }, ctx: ViewAuraContext) {
    requireAuth(ctx);
    const { movieId, ...rest } = input;
    const raw = await apiClient.post<unknown>(
      `/api/v1/movies/${movieId as string}/ratings`,
      rest,
      ctx.token,
    );
    // Invalidate the DataLoader cache so subsequent reads see the new value
    ctx.loaders.myRating.clear(movieId as string);
    return mapRating(raw);
  },

  async deleteRating(_: unknown, { movieId }: { movieId: string }, ctx: ViewAuraContext) {
    requireAuth(ctx);
    await apiClient.delete<void>(`/api/v1/movies/${movieId}/ratings/mine`, ctx.token);
    ctx.loaders.myRating.clear(movieId);
    return true;
  },

  // ── Reviews ─────────────────────────────────────────────────────────────────
  async createReview(_: unknown, { input }: { input: Record<string, unknown> }, ctx: ViewAuraContext) {
    requireAuth(ctx);
    const { movieId, ...rest } = input;
    const raw = await apiClient.post<unknown>(
      `/api/v1/movies/${movieId as string}/reviews`,
      rest,
      ctx.token,
    );
    return mapReview(raw);
  },

  async deleteReview(_: unknown, { id }: { id: string }, ctx: ViewAuraContext) {
    requireAuth(ctx);
    await apiClient.delete<void>(`/api/v1/reviews/${id}`, ctx.token);
    return true;
  },

  async reactToReview(
    _: unknown,
    { input }: { input: { reviewId: string; type: string } },
    ctx: ViewAuraContext,
  ) {
    requireAuth(ctx);
    const raw = await apiClient.post<unknown>(
      `/api/v1/reviews/${input.reviewId}/reactions`,
      { type: input.type },
      ctx.token,
    );
    return mapReview(raw);
  },

  // ── Watchlist ────────────────────────────────────────────────────────────────
  async setWatchlistStatus(
    _: unknown,
    { input }: { input: { movieId: string; status: string; progress?: number } },
    ctx: ViewAuraContext,
  ) {
    requireAuth(ctx);
    const raw = await apiClient.put<unknown>(
      `/api/v1/watchlist/${input.movieId}`,
      { status: input.status, progress: input.progress ?? 0 },
      ctx.token,
    );
    ctx.loaders.myWatchlist.clear(input.movieId);
    return mapWatchlistEntry(raw);
  },

  async removeFromWatchlist(_: unknown, { movieId }: { movieId: string }, ctx: ViewAuraContext) {
    requireAuth(ctx);
    await apiClient.delete<void>(`/api/v1/watchlist/${movieId}`, ctx.token);
    ctx.loaders.myWatchlist.clear(movieId);
    return true;
  },

  async createCustomList(
    _: unknown,
    { input }: { input: Record<string, unknown> },
    ctx: ViewAuraContext,
  ) {
    requireAuth(ctx);
    return apiClient.post<unknown>("/api/v1/lists", input, ctx.token);
  },

  async addToCustomList(
    _: unknown,
    { listId, movieId }: { listId: string; movieId: string },
    ctx: ViewAuraContext,
  ) {
    requireAuth(ctx);
    return apiClient.post<unknown>(`/api/v1/lists/${listId}/movies`, { movie_id: movieId }, ctx.token);
  },

  async removeFromCustomList(
    _: unknown,
    { listId, movieId }: { listId: string; movieId: string },
    ctx: ViewAuraContext,
  ) {
    requireAuth(ctx);
    return apiClient.delete<unknown>(`/api/v1/lists/${listId}/movies/${movieId}`, ctx.token);
  },

  async deleteCustomList(_: unknown, { id }: { id: string }, ctx: ViewAuraContext) {
    requireAuth(ctx);
    await apiClient.delete<void>(`/api/v1/lists/${id}`, ctx.token);
    return true;
  },

  // ── Social ───────────────────────────────────────────────────────────────────
  async follow(_: unknown, { userId }: { userId: string }, ctx: ViewAuraContext) {
    requireAuth(ctx);
    await apiClient.post<void>(`/api/v1/users/${userId}/follow`, {}, ctx.token);
    ctx.loaders.user.clear(userId);
    return true;
  },

  async unfollow(_: unknown, { userId }: { userId: string }, ctx: ViewAuraContext) {
    requireAuth(ctx);
    await apiClient.delete<void>(`/api/v1/users/${userId}/follow`, ctx.token);
    ctx.loaders.user.clear(userId);
    return true;
  },

  // ── Notifications ────────────────────────────────────────────────────────────
  async markNotificationRead(_: unknown, { id }: { id: string }, ctx: ViewAuraContext) {
    requireAuth(ctx);
    await apiClient.patch<void>(`/api/v1/notifications/${id}/read`, {}, ctx.token);
    return true;
  },

  async markAllNotificationsRead(_: unknown, __: unknown, ctx: ViewAuraContext) {
    requireAuth(ctx);
    await apiClient.post<void>("/api/v1/notifications/read-all", {}, ctx.token);
    return true;
  },
};