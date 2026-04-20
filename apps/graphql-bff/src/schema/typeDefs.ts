import { gql } from "graphql-tag";

export const typeDefs = gql`
  # ─── Scalars ────────────────────────────────────────────────────────────────
  scalar DateTime
  scalar JSON

  # ─── Root types ─────────────────────────────────────────────────────────────
  type Query {
    # ── Home feed ──────────────────────────────────────────────────────────────
    homeFeed: HomeFeed!

    # ── Movies ─────────────────────────────────────────────────────────────────
    movie(id: ID!):                    Movie
    movies(input: MovieFilterInput!):  MovieConnection!
    searchMovies(input: SearchInput!): SearchResult!
    trendingMovies(limit: Int):        [Movie!]!
    recommendedMovies(
      limit:   Int
      context: String
    ):                                 [RecommendedMovie!]!

    # ── User ───────────────────────────────────────────────────────────────────
    me:                                User
    user(id: ID!):                     User

    # ── Ratings & reviews ──────────────────────────────────────────────────────
    myRating(movieId: ID!):            Rating
    movieRatings(
      movieId: ID!
      limit:   Int
      offset:  Int
    ):                                 RatingConnection!
    movieReviews(
      movieId:  ID!
      limit:    Int
      offset:   Int
      type:     ReviewType
    ):                                 ReviewConnection!

    # ── Watchlist ──────────────────────────────────────────────────────────────
    myWatchlist(
      status: WatchlistStatus
      limit:  Int
      offset: Int
    ):                                 WatchlistConnection!
    customList(id: ID!):               CustomList
    myCustomLists:                     [CustomList!]!

    # ── Social ─────────────────────────────────────────────────────────────────
    myFeed(limit: Int, offset: Int):   FeedConnection!
    followers(userId: ID!, limit: Int, offset: Int): UserConnection!
    following(userId: ID!, limit: Int, offset: Int): UserConnection!

    # ── Notifications ──────────────────────────────────────────────────────────
    myNotifications(
      limit:  Int
      offset: Int
    ):                                 NotificationConnection!
    unreadNotificationCount:           Int!
  }

  type Mutation {
    # ── Auth ───────────────────────────────────────────────────────────────────
    register(input: RegisterInput!):   AuthPayload!
    login(input: LoginInput!):         AuthPayload!
    logout:                            Boolean!
    refreshToken(token: String!):      AuthPayload!

    # ── Profile ────────────────────────────────────────────────────────────────
    updateProfile(input: UpdateProfileInput!): User!

    # ── Ratings ────────────────────────────────────────────────────────────────
    upsertRating(input: RatingInput!): Rating!
    deleteRating(movieId: ID!):        Boolean!

    # ── Reviews ────────────────────────────────────────────────────────────────
    createReview(input: ReviewInput!): Review!
    deleteReview(id: ID!):             Boolean!
    reactToReview(input: ReviewReactionInput!): Review!

    # ── Watchlist ──────────────────────────────────────────────────────────────
    setWatchlistStatus(input: WatchlistInput!):    WatchlistEntry!
    removeFromWatchlist(movieId: ID!):             Boolean!
    createCustomList(input: CustomListInput!):     CustomList!
    addToCustomList(listId: ID!, movieId: ID!):    CustomList!
    removeFromCustomList(listId: ID!, movieId: ID!): CustomList!
    deleteCustomList(id: ID!):                     Boolean!

    # ── Social ─────────────────────────────────────────────────────────────────
    follow(userId: ID!):               Boolean!
    unfollow(userId: ID!):             Boolean!

    # ── Notifications ──────────────────────────────────────────────────────────
    markNotificationRead(id: ID!):     Boolean!
    markAllNotificationsRead:          Boolean!
  }

  type Subscription {
    # Real-time activity feed updates for the authenticated user
    feedUpdated:                FeedItem!
    # Real-time notification delivery
    notificationReceived:       Notification!
  }

  # ─── Home feed ───────────────────────────────────────────────────────────────
  type HomeFeed {
    continueWatching:  [WatchlistEntry!]!
    trending:          [Movie!]!
    recommended:       [RecommendedMovie!]!
    newReleases:       [Movie!]!
    friendActivity:    [FeedItem!]!
  }

  # ─── Movie ───────────────────────────────────────────────────────────────────
  type Movie {
    id:              ID!
    title:           String!
    originalTitle:   String
    slug:            String!
    synopsis:        String
    tagline:         String
    releaseDate:     String
    runtimeMins:     Int
    contentRating:   String
    status:          String!
    originalLang:    String!
    genres:          [String!]!
    countries:       [String!]!
    posterUrl:       String
    backdropUrl:     String
    trailerUrl:      String
    imdbId:          String
    tmdbId:          Int
    avgRating:       Float!
    ratingCount:     Int!
    popularityScore: Float!
    # Resolved fields (DataLoader-backed)
    credits:         [Credit!]!
    streamingLinks:  [StreamingLink!]!
    # Viewer-specific fields (requires auth)
    myRating:        Rating
    myWatchlistEntry: WatchlistEntry
  }

  type Credit {
    person:       Person!
    role:         String!
    character:    String
    billingOrder: Int!
  }

  type Person {
    id:         ID!
    name:       String!
    slug:       String!
    profileUrl: String
    bio:        String
  }

  type StreamingLink {
    id:         ID!
    provider:   String!
    linkUrl:    String!
    accessType: String!
    priceCents: Int
    region:     String!
  }

  type MovieConnection {
    movies:     [Movie!]!
    total:      Int!
    limit:      Int!
    offset:     Int!
  }

  input MovieFilterInput {
    genres:        [String!]
    countries:     [String!]
    minRating:     Float
    yearFrom:      Int
    yearTo:        Int
    contentRating: String
    limit:         Int
    offset:        Int
    sortBy:        String
    sortDir:       String
  }

  # ─── Search ───────────────────────────────────────────────────────────────────
  type SearchResult {
    movies:           [Movie!]!
    total:            Int!
    processingTimeMs: Int!
  }

  input SearchInput {
    q:             String!
    genres:        [String!]
    minRating:     Float
    yearFrom:      Int
    yearTo:        Int
    provider:      String
    limit:         Int
    offset:        Int
  }

  type RecommendedMovie {
    movie:  Movie!
    score:  Float!
    reason: String!
  }

  # ─── User ────────────────────────────────────────────────────────────────────
  type User {
    id:           ID!
    email:        String
    username:     String
    displayName:  String!
    avatarUrl:    String
    bio:          String
    role:         String!
    createdAt:    DateTime!
    # Viewer-specific (requires auth)
    isFollowing:  Boolean
    followerCount: Int!
    followingCount: Int!
  }

  type UserConnection {
    users:  [User!]!
    total:  Int!
  }

  type AuthPayload {
    accessToken:  String!
    refreshToken: String!
    expiresIn:    Int!
    user:         User!
  }

  input RegisterInput {
    email:       String!
    password:    String!
    displayName: String!
    username:    String
  }

  input LoginInput {
    email:    String!
    password: String!
  }

  input UpdateProfileInput {
    displayName: String
    username:    String
    bio:         String
    avatarUrl:   String
  }

  # ─── Ratings ─────────────────────────────────────────────────────────────────
  type Rating {
    id:             ID!
    movieId:        ID!
    userId:         ID!
    overall:        Float!
    acting:         Float
    direction:      Float
    writing:        Float
    cinematography: Float
    soundtrack:     Float
    reaction:       String
    isVerified:     Boolean!
    createdAt:      DateTime!
    updatedAt:      DateTime!
  }

  type RatingConnection {
    ratings: [Rating!]!
    total:   Int!
  }

  input RatingInput {
    movieId:        ID!
    overall:        Float!
    acting:         Float
    direction:      Float
    writing:        Float
    cinematography: Float
    soundtrack:     Float
    reaction:       String
  }

  # ─── Reviews ─────────────────────────────────────────────────────────────────
  enum ReviewType {
    short
    long_form
    spoiler
    video
  }

  type Review {
    id:              ID!
    movieId:         ID!
    author:          User!
    type:            ReviewType!
    body:            String
    videoUrl:        String
    isSpoiler:       Boolean!
    isPublished:     Boolean!
    likeCount:       Int!
    helpfulCount:    Int!
    insightfulCount: Int!
    funnyCount:      Int!
    credibilityScore: Float!
    createdAt:       DateTime!
    updatedAt:       DateTime!
    myReaction:      String
  }

  type ReviewConnection {
    reviews: [Review!]!
    total:   Int!
  }

  input ReviewInput {
    movieId:   ID!
    type:      ReviewType!
    body:      String
    videoUrl:  String
    isSpoiler: Boolean
  }

  input ReviewReactionInput {
    reviewId: ID!
    type:     String!
  }

  # ─── Watchlist ────────────────────────────────────────────────────────────────
  enum WatchlistStatus {
    to_watch
    watching
    watched
    dropped
  }

  type WatchlistEntry {
    id:        ID!
    movie:     Movie!
    status:    WatchlistStatus!
    progress:  Int!
    addedAt:   DateTime!
    updatedAt: DateTime!
  }

  type WatchlistConnection {
    entries: [WatchlistEntry!]!
    total:   Int!
  }

  input WatchlistInput {
    movieId:  ID!
    status:   WatchlistStatus!
    progress: Int
  }

  type CustomList {
    id:          ID!
    name:        String!
    description: String
    isPublic:    Boolean!
    coverUrl:    String
    movies:      [Movie!]!
    owner:       User!
    createdAt:   DateTime!
  }

  input CustomListInput {
    name:        String!
    description: String
    isPublic:    Boolean
  }

  # ─── Social ───────────────────────────────────────────────────────────────────
  type FeedItem {
    id:        ID!
    type:      String!
    actor:     User!
    movie:     Movie
    review:    Review
    rating:    Rating
    createdAt: DateTime!
  }

  type FeedConnection {
    items: [FeedItem!]!
    total: Int!
  }

  # ─── Notifications ────────────────────────────────────────────────────────────
  type Notification {
    id:        ID!
    type:      String!
    title:     String!
    body:      String!
    data:      JSON
    channel:   String!
    isRead:    Boolean!
    createdAt: DateTime!
  }

  type NotificationConnection {
    notifications: [Notification!]!
    total:         Int!
    unreadCount:   Int!
  }
`;