package httpapi

// Follow godoc
//
//	@Summary     Follow a user
//	@Description Creates a follower → followee edge. Idempotent.
//	@Tags        Social
//	@Security    BearerAuth
//	@Param       user_id path string true "User UUID to follow"
//	@Success     204 "Following"
//	@Failure     400 {object} errorResponse "Cannot follow yourself"
//	@Failure     401 {object} errorResponse
//	@Router      /users/{user_id}/follow [post]
func (h *SocialHandler) swaggerFollow() {}

// Unfollow godoc
//
//	@Summary     Unfollow a user
//	@Description Removes the follow edge. Idempotent.
//	@Tags        Social
//	@Security    BearerAuth
//	@Param       user_id path string true "User UUID to unfollow"
//	@Success     204 "Unfollowed"
//	@Failure     401 {object} errorResponse
//	@Router      /users/{user_id}/follow [delete]
func (h *SocialHandler) swaggerUnfollow() {}

// FollowStats godoc
//
//	@Summary     Get follow stats
//	@Description Returns follower and following counts for any user.
//	@Tags        Social
//	@Produce     json
//	@Param       user_id path string true "User UUID"
//	@Success     200 {object} map[string]interface{} "follower_count and following_count"
//	@Router      /users/{user_id}/follow-stats [get]
func (h *SocialHandler) swaggerFollowStats() {}

// Feed godoc
//
//	@Summary     Get activity feed
//	@Description Returns a reverse-chronological feed of activities from users
//	             the authenticated user follows, plus their own activities.
//	             Served from Redis (ZSet) when warm; falls back to Postgres.
//	@Tags        Social
//	@Security    BearerAuth
//	@Produce     json
//	@Param       limit  query int false "Page size (max 100)" default(20)
//	@Param       offset query int false "Pagination offset" default(0)
//	@Success     200 {object} feedResponse
//	@Failure     401 {object} errorResponse
//	@Router      /me/feed [get]
func (h *SocialHandler) swaggerFeed() {}

// UserActivity godoc
//
//	@Summary     Get user activity
//	@Description Returns a user's public activity stream (ratings, reviews,
//	             watchlist additions, challenges joined).
//	@Tags        Social
//	@Produce     json
//	@Param       user_id path  string true  "User UUID"
//	@Param       limit   query int    false "Page size" default(20)
//	@Param       offset  query int    false "Pagination offset" default(0)
//	@Success     200 {object} feedResponse
//	@Router      /users/{user_id}/activity [get]
func (h *SocialHandler) swaggerUserActivity() {}

// ListThreads godoc
//
//	@Summary     List discussion threads for a movie
//	@Description Returns discussion threads scoped to a movie, newest-first.
//	             Spoiler threads are excluded by default.
//	@Tags        Social
//	@Produce     json
//	@Param       movie_id         path  string true  "Movie UUID"
//	@Param       include_spoilers query bool   false "Include spoiler threads" default(false)
//	@Param       limit            query int    false "Page size" default(20)
//	@Param       offset           query int    false "Pagination offset" default(0)
//	@Success     200 {object} threadListResponse
//	@Router      /movies/{movie_id}/threads [get]
func (h *SocialHandler) swaggerListThreads() {}

// CreateThread godoc
//
//	@Summary     Create discussion thread
//	@Description Creates a new thread scoped to a movie. Set `is_spoiler: true`
//	             for scene discussions. `scene_timestamp_secs` is optional and
//	             marks the specific moment being discussed.
//	@Tags        Social
//	@Security    BearerAuth
//	@Accept      json
//	@Produce     json
//	@Param       movie_id path string true "Movie UUID"
//	@Param       body body createThreadRequest true "Thread details"
//	@Success     201 {object} threadResponse
//	@Failure     400 {object} errorResponse
//	@Failure     401 {object} errorResponse
//	@Router      /movies/{movie_id}/threads [post]
func (h *SocialHandler) swaggerCreateThread() {}

// ListPosts godoc
//
//	@Summary     List posts in a thread
//	@Description Returns top-level posts in a thread (replies are fetched separately).
//	@Tags        Social
//	@Produce     json
//	@Param       id     path  string true  "Thread UUID"
//	@Param       limit  query int    false "Page size" default(20)
//	@Param       offset query int    false "Pagination offset" default(0)
//	@Success     200 {object} postListResponse
//	@Failure     404 {object} errorResponse
//	@Router      /threads/{id}/posts [get]
func (h *SocialHandler) swaggerListPosts() {}

// CreatePost godoc
//
//	@Summary     Post to thread
//	@Description Creates a post in a thread. Set `parent_id` to reply to an
//	             existing post.
//	@Tags        Social
//	@Security    BearerAuth
//	@Accept      json
//	@Produce     json
//	@Param       id   path string true "Thread UUID"
//	@Param       body body createPostRequest true "Post body"
//	@Success     201 {object} postResponse
//	@Failure     400 {object} errorResponse
//	@Failure     401 {object} errorResponse
//	@Router      /threads/{id}/posts [post]
func (h *SocialHandler) swaggerCreatePost() {}

// LikePost godoc
//
//	@Summary     Like a post
//	@Description Records a like on a discussion post. Idempotent.
//	@Tags        Social
//	@Security    BearerAuth
//	@Param       id path string true "Post UUID"
//	@Success     204 "Liked"
//	@Failure     401 {object} errorResponse
//	@Router      /posts/{id}/like [post]
func (h *SocialHandler) swaggerLikePost() {}

// CreateChallenge godoc
//
//	@Summary     Create challenge
//	@Description Creates a watch challenge. Types: watch_list (requires movie_ids),
//	             genre_blind (requires target_genre + target_count), custom.
//	             Public challenges appear in the discover feed.
//	@Tags        Social
//	@Security    BearerAuth
//	@Accept      json
//	@Produce     json
//	@Param       body body createChallengeRequest true "Challenge details"
//	@Success     201 {object} challengeResponse
//	@Failure     400 {object} errorResponse
//	@Failure     401 {object} errorResponse
//	@Router      /challenges [post]
func (h *SocialHandler) swaggerCreateChallenge() {}

// JoinChallenge godoc
//
//	@Summary     Join a challenge
//	@Description Enrolls the authenticated user in a challenge. Cannot join
//	             after the deadline has passed.
//	@Tags        Social
//	@Security    BearerAuth
//	@Param       id path string true "Challenge UUID"
//	@Success     204 "Joined"
//	@Failure     400 {object} errorResponse "Deadline passed"
//	@Failure     401 {object} errorResponse
//	@Router      /challenges/{id}/join [post]
func (h *SocialHandler) swaggerJoinChallenge() {}

// MarkMovieWatched godoc
//
//	@Summary     Mark movie watched in challenge
//	@Description Records a movie as watched toward the challenge goal.
//	             Automatically marks the challenge complete when all required
//	             movies are watched.
//	@Tags        Social
//	@Security    BearerAuth
//	@Param       id       path string true "Challenge UUID"
//	@Param       movie_id path string true "Movie UUID"
//	@Success     204 "Progress recorded"
//	@Failure     401 {object} errorResponse
//	@Failure     404 {object} errorResponse "Not a participant"
//	@Router      /challenges/{id}/progress/{movie_id} [post]
func (h *SocialHandler) swaggerMarkMovieWatched() {}

// Leaderboard godoc
//
//	@Summary     Challenge leaderboard
//	@Description Returns participants ordered by completion status and join date.
//	             Completed participants appear first.
//	@Tags        Social
//	@Produce     json
//	@Param       id     path  string true  "Challenge UUID"
//	@Param       limit  query int    false "Page size" default(20)
//	@Param       offset query int    false "Pagination offset" default(0)
//	@Success     200 {object} leaderboardResponse
//	@Router      /challenges/{id}/leaderboard [get]
func (h *SocialHandler) swaggerLeaderboard() {}

type feedResponse struct {
	Activities []activityResponse `json:"activities"`
	Total      int                `json:"total"`
	Limit      int                `json:"limit"`
	Offset     int                `json:"offset"`
}

type threadListResponse struct {
	Threads []threadResponse `json:"threads"`
	Total   int              `json:"total"`
	Limit   int              `json:"limit"`
	Offset  int              `json:"offset"`
}

type postListResponse struct {
	Posts  []postResponse `json:"posts"`
	Total  int            `json:"total"`
	Limit  int            `json:"limit"`
	Offset int            `json:"offset"`
}

type leaderboardResponse struct {
	Leaderboard []participantResponse `json:"leaderboard"`
	Total       int                   `json:"total"`
	Limit       int                   `json:"limit"`
	Offset      int                   `json:"offset"`
}

type errorResponse struct {
	Error struct {
		Code    string      `json:"code"`
		Message string      `json:"message"`
		Details interface{} `json:"details,omitempty"`
	} `json:"error"`
}
