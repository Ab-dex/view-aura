/**
 * Mapper functions from raw Go API JSON shapes to GraphQL resolver return types.
 *
 * Kept as thin identity-preserving transforms — no business logic here.
 * Each mapper accepts `unknown` so TypeScript forces us to cast rather than
 * silently accepting any shape, making API contract drift visible at compile time.
 */

type Raw = Record<string, unknown>;

function r(v: unknown): Raw {
  return (v ?? {}) as Raw;
}

export function mapMovie(raw: unknown) {
  const v = r(raw);
  return {
    id:              String(v.id ?? ""),
    title:           String(v.title ?? ""),
    originalTitle:   v.original_title != null ? String(v.original_title) : undefined,
    slug:            String(v.slug ?? ""),
    synopsis:        v.synopsis != null ? String(v.synopsis) : undefined,
    tagline:         v.tagline != null ? String(v.tagline) : undefined,
    releaseDate:     v.release_date != null ? String(v.release_date) : undefined,
    runtimeMins:     v.runtime_mins != null ? Number(v.runtime_mins) : undefined,
    contentRating:   v.content_rating != null ? String(v.content_rating) : undefined,
    status:          String(v.status ?? "unknown"),
    originalLang:    String(v.original_lang ?? "en"),
    genres:          Array.isArray(v.genres) ? v.genres.map(String) : [],
    countries:       Array.isArray(v.countries) ? v.countries.map(String) : [],
    posterUrl:       v.poster_url != null ? String(v.poster_url) : undefined,
    backdropUrl:     v.backdrop_url != null ? String(v.backdrop_url) : undefined,
    trailerUrl:      v.trailer_url != null ? String(v.trailer_url) : undefined,
    imdbId:          v.imdb_id != null ? String(v.imdb_id) : undefined,
    tmdbId:          v.tmdb_id != null ? Number(v.tmdb_id) : undefined,
    avgRating:       Number(v.avg_rating ?? 0),
    ratingCount:     Number(v.rating_count ?? 0),
    popularityScore: Number(v.popularity_score ?? 0),
    // Resolved by Movie type resolvers (DataLoader), not inline
    __raw: v,
  };
}

export function mapUser(raw: unknown) {
  const v = r(raw);
  return {
    id:             String(v.id ?? ""),
    email:          v.email != null ? String(v.email) : undefined,
    username:       v.username != null ? String(v.username) : undefined,
    displayName:    String(v.display_name ?? ""),
    avatarUrl:      v.avatar_url != null ? String(v.avatar_url) : undefined,
    bio:            v.bio != null ? String(v.bio) : undefined,
    role:           String(v.role ?? "user"),
    createdAt:      String(v.created_at ?? new Date().toISOString()),
    isFollowing:    v.is_following != null ? Boolean(v.is_following) : undefined,
    followerCount:  Number(v.follower_count ?? 0),
    followingCount: Number(v.following_count ?? 0),
  };
}

export function mapRating(raw: unknown) {
  const v = r(raw);
  return {
    id:             String(v.id ?? ""),
    movieId:        String(v.movie_id ?? ""),
    userId:         String(v.user_id ?? ""),
    overall:        Number(v.overall ?? 0),
    acting:         v.acting != null ? Number(v.acting) : undefined,
    direction:      v.direction != null ? Number(v.direction) : undefined,
    writing:        v.writing != null ? Number(v.writing) : undefined,
    cinematography: v.cinematography != null ? Number(v.cinematography) : undefined,
    soundtrack:     v.soundtrack != null ? Number(v.soundtrack) : undefined,
    reaction:       v.reaction != null ? String(v.reaction) : undefined,
    isVerified:     Boolean(v.is_verified),
    createdAt:      String(v.created_at ?? ""),
    updatedAt:      String(v.updated_at ?? ""),
  };
}

export function mapReview(raw: unknown) {
  const v = r(raw);
  return {
    id:               String(v.id ?? ""),
    movieId:          String(v.movie_id ?? ""),
    // author resolved by Review type resolver
    authorId:         String(v.user_id ?? ""),
    type:             String(v.type ?? "short"),
    body:             v.body != null ? String(v.body) : undefined,
    videoUrl:         v.video_url != null ? String(v.video_url) : undefined,
    isSpoiler:        Boolean(v.is_spoiler),
    isPublished:      Boolean(v.is_published ?? true),
    likeCount:        Number(v.like_count ?? 0),
    helpfulCount:     Number(v.helpful_count ?? 0),
    insightfulCount:  Number(v.insightful_count ?? 0),
    funnyCount:       Number(v.funny_count ?? 0),
    credibilityScore: Number(v.credibility_score ?? 0),
    createdAt:        String(v.created_at ?? ""),
    updatedAt:        String(v.updated_at ?? ""),
    myReaction:       v.my_reaction != null ? String(v.my_reaction) : undefined,
  };
}

export function mapWatchlistEntry(raw: unknown) {
  const v = r(raw);
  return {
    id:        String(v.id ?? ""),
    movieId:   String(v.movie_id ?? ""),
    status:    String(v.status ?? "to_watch"),
    progress:  Number(v.progress ?? 0),
    addedAt:   String(v.added_at ?? ""),
    updatedAt: String(v.updated_at ?? ""),
  };
}

export function mapFeedItem(raw: unknown) {
  const v = r(raw);
  return {
    id:        String(v.id ?? ""),
    type:      String(v.type ?? ""),
    actorId:   String(v.actor_id ?? ""),
    movieId:   v.movie_id != null ? String(v.movie_id) : undefined,
    reviewId:  v.review_id != null ? String(v.review_id) : undefined,
    ratingId:  v.rating_id != null ? String(v.rating_id) : undefined,
    createdAt: String(v.created_at ?? ""),
  };
}

export function mapNotification(raw: unknown) {
  const v = r(raw);
  return {
    id:        String(v.id ?? ""),
    type:      String(v.type ?? ""),
    title:     String(v.title ?? ""),
    body:      String(v.body ?? ""),
    data:      v.data ?? null,
    channel:   String(v.channel ?? "in_app"),
    isRead:    Boolean(v.is_read),
    createdAt: String(v.created_at ?? ""),
  };
}

export function mapRecommendedMovie(raw: { movie_id: string; score: number; reason: string }) {
  return {
    movie:  { id: raw.movie_id } as ReturnType<typeof mapMovie>,
    score:  raw.score,
    reason: raw.reason,
  };
}