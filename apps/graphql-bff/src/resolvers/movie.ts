/**
 * Movie type field resolvers.
 *
 * These resolvers fire only when a client explicitly requests the field.
 * Without DataLoader each request for a list of 20 movies with credits would
 * produce 20 serial HTTP calls. With DataLoader, all 20 movie IDs are batched
 * into a single call.
 */
import type { ViewAuraContext } from "../context/index.js";
import type { RawCredit, RawStreamingLink } from "../loaders/index.js";

interface MovieParent {
  id:      string;
  movieId?: string;
  __raw?:  Record<string, unknown>;
}

export const Movie = {
  async credits(parent: MovieParent, _: unknown, ctx: ViewAuraContext) {
    const raw = await ctx.loaders.credits.load(parent.id);
    return (raw ?? []).map((c: RawCredit) => ({
      person: {
        id:         c.person_id,
        name:       c.person_name,
        slug:       c.person_slug,
        profileUrl: c.profile_url,
      },
      role:         c.role,
      character:    c.character,
      billingOrder: c.billing_order,
    }));
  },

  async streamingLinks(parent: MovieParent, _: unknown, ctx: ViewAuraContext) {
    const raw = await ctx.loaders.streaming.load(parent.id);
    return (raw ?? []).map((l: RawStreamingLink) => ({
      id:         l.id,
      provider:   l.provider,
      linkUrl:    l.link_url,
      accessType: l.access_type,
      priceCents: l.price_cents,
      region:     l.region,
    }));
  },

  async myRating(parent: MovieParent, _: unknown, ctx: ViewAuraContext) {
    if (!ctx.user) return null;
    const raw = await ctx.loaders.myRating.load(parent.id);
    if (!raw) return null;
    return {
      id:             raw.id,
      movieId:        raw.movie_id,
      userId:         raw.user_id,
      overall:        raw.overall,
      acting:         raw.acting,
      direction:      raw.direction,
      writing:        raw.writing,
      cinematography: raw.cinematography,
      soundtrack:     raw.soundtrack,
      reaction:       raw.reaction,
      isVerified:     raw.is_verified,
      createdAt:      raw.created_at,
      updatedAt:      raw.updated_at,
    };
  },

  async myWatchlistEntry(parent: MovieParent, _: unknown, ctx: ViewAuraContext) {
    if (!ctx.user) return null;
    const raw = await ctx.loaders.myWatchlist.load(parent.id);
    if (!raw) return null;
    return {
      id:        raw.id,
      movieId:   raw.movie_id,
      status:    raw.status,
      progress:  raw.progress,
      addedAt:   raw.added_at,
      updatedAt: raw.updated_at,
    };
  },
};

/** WatchlistEntry.movie — load the full Movie object via DataLoader */
export const WatchlistEntry = {
  async movie(parent: { movieId: string }, _: unknown, ctx: ViewAuraContext) {
    const raw = await ctx.loaders.movie.load(parent.movieId);
    if (!raw) return null;
    const { mapMovie } = await import("./mappers.js");
    return mapMovie(raw);
  },
};

/** RecommendedMovie.movie — the rec service only returns IDs; hydrate via DataLoader */
export const RecommendedMovie = {
  async movie(parent: { movie: { id: string } }, _: unknown, ctx: ViewAuraContext) {
    const raw = await ctx.loaders.movie.load(parent.movie.id);
    if (!raw) return null;
    const { mapMovie } = await import("./mappers.js");
    return mapMovie(raw);
  },
};

/** FeedItem type resolvers — hydrate actor, movie, and review via DataLoader */
export const FeedItem = {
  async actor(parent: { actorId: string }, _: unknown, ctx: ViewAuraContext) {
    const raw = await ctx.loaders.user.load(parent.actorId);
    if (!raw) return null;
    const { mapUser } = await import("./mappers.js");
    return mapUser(raw);
  },

  async movie(parent: { movieId?: string }, _: unknown, ctx: ViewAuraContext) {
    if (!parent.movieId) return null;
    const raw = await ctx.loaders.movie.load(parent.movieId);
    if (!raw) return null;
    const { mapMovie } = await import("./mappers.js");
    return mapMovie(raw);
  },
};

/** Review.author — load from user DataLoader */
export const Review = {
  async author(parent: { authorId: string }, _: unknown, ctx: ViewAuraContext) {
    const raw = await ctx.loaders.user.load(parent.authorId);
    if (!raw) return { id: parent.authorId, displayName: "Unknown", role: "user", createdAt: "", followerCount: 0, followingCount: 0 };
    const { mapUser } = await import("./mappers.js");
    return mapUser(raw);
  },
};