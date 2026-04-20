/**
 * DataLoader instances for the GraphQL BFF.
 *
 * All loaders are created fresh per request (never shared) so that stale
 * data cannot leak between users, and per-user auth tokens are used for
 * every batch call.
 *
 * Loader strategy per type:
 *
 *   movieLoader      — batch GET /api/v1/movies?ids=a,b,c
 *                      Used by WatchlistEntry.movie, FeedItem.movie, etc.
 *
 *   creditsLoader    — batch GET /api/v1/movies/:id/credits (one per movie)
 *                      Used by Movie.credits
 *
 *   streamingLoader  — batch GET /api/v1/movies/:id/streaming (one per movie)
 *                      Used by Movie.streamingLinks
 *
 *   userLoader       — batch GET /api/v1/users?ids=a,b,c
 *                      Used by Review.author, FeedItem.actor, etc.
 *
 *   myRatingLoader   — batch GET /api/v1/ratings/mine?movie_ids=a,b,c
 *                      Used by Movie.myRating (viewer-specific)
 *
 *   myWatchlistLoader — batch GET /api/v1/watchlist/mine?movie_ids=a,b,c
 *                       Used by Movie.myWatchlistEntry (viewer-specific)
 */
import DataLoader from "dataloader";
import { apiClient } from "../datasources/api-client.js";

// ── Raw API shapes (minimal — resolvers map to GraphQL types) ─────────────────

export interface RawMovie {
  id: string;
  title: string;
  original_title?: string;
  slug: string;
  synopsis?: string;
  tagline?: string;
  release_date?: string;
  runtime_mins?: number;
  content_rating?: string;
  status: string;
  original_lang: string;
  genres: string[];
  countries: string[];
  poster_url?: string;
  backdrop_url?: string;
  trailer_url?: string;
  imdb_id?: string;
  tmdb_id?: number;
  avg_rating: number;
  rating_count: number;
  popularity_score: number;
}

export interface RawCredit {
  person_id: string;
  person_name: string;
  person_slug: string;
  profile_url?: string;
  role: string;
  character?: string;
  billing_order: number;
}

export interface RawStreamingLink {
  id: string;
  provider: string;
  link_url: string;
  access_type: string;
  price_cents?: number;
  region: string;
}

export interface RawUser {
  id: string;
  email?: string;
  username?: string;
  display_name: string;
  avatar_url?: string;
  bio?: string;
  role: string;
  created_at: string;
  follower_count: number;
  following_count: number;
  is_following?: boolean;
}

export interface RawRating {
  id: string;
  movie_id: string;
  user_id: string;
  overall: number;
  acting?: number;
  direction?: number;
  writing?: number;
  cinematography?: number;
  soundtrack?: number;
  reaction?: string;
  is_verified: boolean;
  created_at: string;
  updated_at: string;
}

export interface RawWatchlistEntry {
  id: string;
  movie_id: string;
  status: string;
  progress: number;
  added_at: string;
  updated_at: string;
}

// ── Loader factories ──────────────────────────────────────────────────────────

export function makeLoaders(token: string | undefined) {
  const auth = token;

  const movieLoader = new DataLoader<string, RawMovie | null>(
    async (ids) => {
      const result = await apiClient.get<{ movies: RawMovie[] }>(
        "/api/v1/movies/batch",
        auth,
        { ids: ids.join(",") },
      ).catch(() => ({ movies: [] as RawMovie[] }));

      const byId = Object.fromEntries(result.movies.map(m => [m.id, m]));
      return ids.map(id => byId[id] ?? null);
    },
    { maxBatchSize: 100, cache: true },
  );

  const creditsLoader = new DataLoader<string, RawCredit[]>(
    async (movieIds) => {
      // Credits are not batchable in a single call — fan out in parallel
      // but cap concurrency so we don't blast the Go API with 100 connections.
      const results = await Promise.all(
        movieIds.map(id =>
          apiClient.get<{ credits: RawCredit[] }>(`/api/v1/movies/${id}/credits`, auth)
            .then(r => r.credits)
            .catch(() => [] as RawCredit[])
        )
      );
      return results;
    },
    { maxBatchSize: 20, cache: true },
  );

  const streamingLoader = new DataLoader<string, RawStreamingLink[]>(
    async (movieIds) => {
      const results = await Promise.all(
        movieIds.map(id =>
          apiClient.get<{ links: RawStreamingLink[] }>(`/api/v1/movies/${id}/streaming`, auth)
            .then(r => r.links)
            .catch(() => [] as RawStreamingLink[])
        )
      );
      return results;
    },
    { maxBatchSize: 20, cache: true },
  );

  const userLoader = new DataLoader<string, RawUser | null>(
    async (ids) => {
      const result = await apiClient.get<{ users: RawUser[] }>(
        "/api/v1/users/batch",
        auth,
        { ids: ids.join(",") },
      ).catch(() => ({ users: [] as RawUser[] }));

      const byId = Object.fromEntries(result.users.map(u => [u.id, u]));
      return ids.map(id => byId[id] ?? null);
    },
    { maxBatchSize: 100, cache: true },
  );

  const myRatingLoader = new DataLoader<string, RawRating | null>(
    async (movieIds) => {
      if (!auth) return movieIds.map(() => null);
      const result = await apiClient.get<{ ratings: RawRating[] }>(
        "/api/v1/ratings/mine/batch",
        auth,
        { movie_ids: movieIds.join(",") },
      ).catch(() => ({ ratings: [] as RawRating[] }));

      const byMovieId = Object.fromEntries(result.ratings.map(r => [r.movie_id, r]));
      return movieIds.map(id => byMovieId[id] ?? null);
    },
    { maxBatchSize: 100, cache: true },
  );

  const myWatchlistLoader = new DataLoader<string, RawWatchlistEntry | null>(
    async (movieIds) => {
      if (!auth) return movieIds.map(() => null);
      const result = await apiClient.get<{ entries: RawWatchlistEntry[] }>(
        "/api/v1/watchlist/mine/batch",
        auth,
        { movie_ids: movieIds.join(",") },
      ).catch(() => ({ entries: [] as RawWatchlistEntry[] }));

      const byMovieId = Object.fromEntries(result.entries.map(e => [e.movie_id, e]));
      return movieIds.map(id => byMovieId[id] ?? null);
    },
    { maxBatchSize: 100, cache: true },
  );

  return {
    movie:       movieLoader,
    credits:     creditsLoader,
    streaming:   streamingLoader,
    user:        userLoader,
    myRating:    myRatingLoader,
    myWatchlist: myWatchlistLoader,
  };
}