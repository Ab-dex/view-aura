package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Ab-dex/view-aura/internal/modules/social/domain"
	"github.com/Ab-dex/view-aura/internal/modules/social/service"
	apierror "github.com/Ab-dex/view-aura/internal/platform/error"
	"github.com/Ab-dex/view-aura/internal/platform/logger"
)

type SocialHandler struct {
	svc service.SocialService
}

func NewSocialHandler(svc service.SocialService) *SocialHandler {
	return &SocialHandler{svc: svc}
}

// Public routes (no auth):
//
//	GET /users/:user_id/followers
//	GET /users/:user_id/following
//	GET /users/:user_id/follow-stats
//	GET /users/:user_id/activity
//	GET /movies/:movie_id/threads
//	GET /threads/:id
//	GET /threads/:id/posts
//	GET /posts/:id/replies
//	GET /challenges
//	GET /challenges/:id
//	GET /challenges/:id/leaderboard
//
// Protected routes (auth required):
//
//	POST   /users/:user_id/follow
//	DELETE /users/:user_id/follow
//	GET    /users/:user_id/follow-status  (am I following them?)
//	GET    /me/feed
//	POST   /movies/:movie_id/threads
//	DELETE /threads/:id
//	POST   /threads/:id/posts
//	DELETE /posts/:id
//	POST   /posts/:id/like
//	DELETE /posts/:id/like
//	POST   /challenges
//	DELETE /challenges/:id
//	POST   /challenges/:id/join
//	DELETE /challenges/:id/join
//	POST   /challenges/:id/progress/:movie_id
//	GET    /challenges/:id/me

func (h *SocialHandler) RegisterPublicRoutes(r gin.IRouter) {
	r.GET("/users/:user_id/followers", h.ListFollowers)
	r.GET("/users/:user_id/following", h.ListFollowing)
	r.GET("/users/:user_id/follow-stats", h.FollowStats)
	r.GET("/users/:user_id/activity", h.UserActivity)
	r.GET("/movies/:movie_id/threads", h.ListThreads)
	r.GET("/threads/:id", h.GetThread)
	r.GET("/threads/:id/posts", h.ListPosts)
	r.GET("/posts/:id/replies", h.ListReplies)
	r.GET("/challenges", h.ListChallenges)
	r.GET("/challenges/:id", h.GetChallenge)
	r.GET("/challenges/:id/leaderboard", h.Leaderboard)
}

func (h *SocialHandler) RegisterProtectedRoutes(r gin.IRouter) {
	r.POST("/users/:user_id/follow", h.Follow)
	r.DELETE("/users/:user_id/follow", h.Unfollow)
	r.GET("/users/:user_id/follow-status", h.FollowStatus)
	r.GET("/me/feed", h.Feed)
	r.POST("/movies/:movie_id/threads", h.CreateThread)
	r.DELETE("/threads/:id", h.DeleteThread)
	r.POST("/threads/:id/posts", h.CreatePost)
	r.DELETE("/posts/:id", h.DeletePost)
	r.POST("/posts/:id/like", h.LikePost)
	r.DELETE("/posts/:id/like", h.UnlikePost)
	r.POST("/challenges", h.CreateChallenge)
	r.DELETE("/challenges/:id", h.DeleteChallenge)
	r.POST("/challenges/:id/join", h.JoinChallenge)
	r.DELETE("/challenges/:id/join", h.LeaveChallenge)
	r.POST("/challenges/:id/progress/:movie_id", h.MarkMovieWatched)
	r.GET("/challenges/:id/me", h.MyProgress)
}

// ─── DTOs ─────────────────────────────────────────────────────────────────────

type createThreadRequest struct {
	Title              string `json:"title"               binding:"required"`
	SceneTimestampSecs *int   `json:"scene_timestamp_secs"`
	IsSpoiler          bool   `json:"is_spoiler"`
}

type createPostRequest struct {
	Body     string  `json:"body"      binding:"required"`
	ParentID *string `json:"parent_id"`
}

type createChallengeRequest struct {
	Type        string   `json:"type"         binding:"required"`
	Title       string   `json:"title"        binding:"required"`
	Description string   `json:"description"`
	MovieIDs    []string `json:"movie_ids"`
	TargetGenre string   `json:"target_genre"`
	TargetCount int      `json:"target_count"`
	DeadlineAt  *string  `json:"deadline_at"` // RFC3339
	IsPublic    bool     `json:"is_public"`
}

// ─── Response types ───────────────────────────────────────────────────────────

type activityResponse struct {
	ID          string         `json:"id"`
	ActorID     string         `json:"actor_id"`
	Type        string         `json:"type"`
	SubjectType string         `json:"subject_type"`
	SubjectID   string         `json:"subject_id"`
	Payload     map[string]any `json:"payload,omitempty"`
	CreatedAt   string         `json:"created_at"`
}

type threadResponse struct {
	ID                 string `json:"id"`
	Scope              string `json:"scope"`
	ScopeID            string `json:"scope_id"`
	Title              string `json:"title"`
	SceneTimestampSecs *int   `json:"scene_timestamp_secs,omitempty"`
	IsSpoiler          bool   `json:"is_spoiler"`
	PostCount          int    `json:"post_count"`
	CreatedBy          string `json:"created_by"`
	CreatedAt          string `json:"created_at"`
}

type postResponse struct {
	ID        string  `json:"id"`
	ThreadID  string  `json:"thread_id"`
	AuthorID  string  `json:"author_id"`
	Body      string  `json:"body"`
	ParentID  *string `json:"parent_id,omitempty"`
	LikeCount int     `json:"like_count"`
	IsDeleted bool    `json:"is_deleted"`
	CreatedAt string  `json:"created_at"`
}

type challengeResponse struct {
	ID               string   `json:"id"`
	CreatedBy        string   `json:"created_by"`
	Type             string   `json:"type"`
	Title            string   `json:"title"`
	Description      string   `json:"description,omitempty"`
	MovieIDs         []string `json:"movie_ids,omitempty"`
	TargetGenre      string   `json:"target_genre,omitempty"`
	TargetCount      int      `json:"target_count,omitempty"`
	DeadlineAt       *string  `json:"deadline_at,omitempty"`
	IsPublic         bool     `json:"is_public"`
	ParticipantCount int      `json:"participant_count"`
	CreatedAt        string   `json:"created_at"`
}

type participantResponse struct {
	UserID            string   `json:"user_id"`
	CompletedMovieIDs []string `json:"completed_movie_ids"`
	IsCompleted       bool     `json:"is_completed"`
	JoinedAt          string   `json:"joined_at"`
	CompletedAt       *string  `json:"completed_at,omitempty"`
}

// ─── Follow handlers ──────────────────────────────────────────────────────────

func (h *SocialHandler) Follow(c *gin.Context) {
	if err := h.svc.Follow(c.Request.Context(), mustUserID(c), c.Param("user_id")); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *SocialHandler) Unfollow(c *gin.Context) {
	if err := h.svc.Unfollow(c.Request.Context(), mustUserID(c), c.Param("user_id")); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *SocialHandler) FollowStatus(c *gin.Context) {
	ok, err := h.svc.IsFollowing(c.Request.Context(), mustUserID(c), c.Param("user_id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"is_following": ok})
}

func (h *SocialHandler) ListFollowers(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	ids, total, err := h.svc.ListFollowers(c.Request.Context(), c.Param("user_id"), limit, offset)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"user_ids": orStringSlice(ids), "total": total, "limit": limit, "offset": offset})
}

func (h *SocialHandler) ListFollowing(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	ids, total, err := h.svc.ListFollowing(c.Request.Context(), c.Param("user_id"), limit, offset)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"user_ids": orStringSlice(ids), "total": total, "limit": limit, "offset": offset})
}

func (h *SocialHandler) FollowStats(c *gin.Context) {
	stats, err := h.svc.GetFollowStats(c.Request.Context(), c.Param("user_id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"user_id":         stats.UserID,
		"follower_count":  stats.FollowerCount,
		"following_count": stats.FollowingCount,
	})
}

// ─── Feed handlers ────────────────────────────────────────────────────────────

func (h *SocialHandler) Feed(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	page, err := h.svc.GetFeed(c.Request.Context(), domain.FeedFilter{
		UserID: mustUserID(c),
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"activities": toActivityResponses(page.Activities),
		"total":      page.Total,
		"limit":      page.Limit,
		"offset":     page.Offset,
	})
}

func (h *SocialHandler) UserActivity(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	page, err := h.svc.GetUserActivity(c.Request.Context(), domain.FeedFilter{
		ActorID: c.Param("user_id"),
		Limit:   limit,
		Offset:  offset,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"activities": toActivityResponses(page.Activities),
		"total":      page.Total,
		"limit":      page.Limit,
		"offset":     page.Offset,
	})
}

// ─── Discussion handlers ──────────────────────────────────────────────────────

func (h *SocialHandler) ListThreads(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	threads, total, err := h.svc.ListThreads(c.Request.Context(), domain.ThreadFilter{
		Scope:           domain.ThreadScopeMovie,
		ScopeID:         c.Param("movie_id"),
		IncludeSpoilers: c.Query("include_spoilers") == "true",
		Limit:           limit,
		Offset:          offset,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"threads": toThreadResponses(threads),
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}

func (h *SocialHandler) GetThread(c *gin.Context) {
	thread, err := h.svc.GetThread(c.Request.Context(), domain.ThreadID(c.Param("id")))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toThreadResponse(thread))
}

func (h *SocialHandler) CreateThread(c *gin.Context) {
	var req createThreadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierror.Validation(err.Error(), nil))
		return
	}

	thread, err := h.svc.CreateThread(c.Request.Context(), domain.CreateThreadCmd{
		Scope:              domain.ThreadScopeMovie,
		ScopeID:            c.Param("movie_id"),
		Title:              req.Title,
		SceneTimestampSecs: req.SceneTimestampSecs,
		IsSpoiler:          req.IsSpoiler,
		CreatedBy:          mustUserID(c),
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, toThreadResponse(thread))
}

func (h *SocialHandler) DeleteThread(c *gin.Context) {
	if err := h.svc.DeleteThread(c.Request.Context(), domain.ThreadID(c.Param("id")), mustUserID(c)); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *SocialHandler) ListPosts(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	posts, total, err := h.svc.ListPosts(c.Request.Context(), domain.PostFilter{
		ThreadID: domain.ThreadID(c.Param("id")),
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"posts":  toPostResponses(posts),
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

func (h *SocialHandler) CreatePost(c *gin.Context) {
	var req createPostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierror.Validation(err.Error(), nil))
		return
	}

	cmd := domain.CreatePostCmd{
		ThreadID: domain.ThreadID(c.Param("id")),
		AuthorID: mustUserID(c),
		Body:     req.Body,
	}
	if req.ParentID != nil {
		pid := domain.PostID(*req.ParentID)
		cmd.ParentID = &pid
	}

	post, err := h.svc.CreatePost(c.Request.Context(), cmd)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, toPostResponse(post))
}

func (h *SocialHandler) ListReplies(c *gin.Context) {
	replies, err := h.svc.ListReplies(c.Request.Context(), domain.PostID(c.Param("id")))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"replies": toPostResponses(replies)})
}

func (h *SocialHandler) DeletePost(c *gin.Context) {
	if err := h.svc.DeletePost(c.Request.Context(), domain.PostID(c.Param("id")), mustUserID(c)); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *SocialHandler) LikePost(c *gin.Context) {
	if err := h.svc.LikePost(c.Request.Context(), domain.PostID(c.Param("id")), mustUserID(c)); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *SocialHandler) UnlikePost(c *gin.Context) {
	if err := h.svc.UnlikePost(c.Request.Context(), domain.PostID(c.Param("id")), mustUserID(c)); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ─── Challenge handlers ───────────────────────────────────────────────────────

func (h *SocialHandler) ListChallenges(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	isPublic := true

	challenges, total, err := h.svc.ListChallenges(c.Request.Context(), domain.ChallengeFilter{
		IsPublic: &isPublic,
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"challenges": toChallengeResponses(challenges),
		"total":      total,
		"limit":      limit,
		"offset":     offset,
	})
}

func (h *SocialHandler) GetChallenge(c *gin.Context) {
	detail, err := h.svc.GetChallenge(c.Request.Context(), domain.ChallengeID(c.Param("id")))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toChallengeResponse(detail.Challenge))
}

func (h *SocialHandler) CreateChallenge(c *gin.Context) {
	var req createChallengeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierror.Validation(err.Error(), nil))
		return
	}

	cmd := domain.CreateChallengeCmd{
		CreatedBy:   mustUserID(c),
		Type:        domain.ChallengeType(req.Type),
		Title:       req.Title,
		Description: req.Description,
		MovieIDs:    req.MovieIDs,
		TargetGenre: req.TargetGenre,
		TargetCount: req.TargetCount,
		IsPublic:    req.IsPublic,
	}
	if req.DeadlineAt != nil {
		t, err := time.Parse(time.RFC3339, *req.DeadlineAt)
		if err != nil {
			respondError(c, apierror.Validation("deadline_at must be RFC3339", nil))
			return
		}
		cmd.DeadlineAt = &t
	}

	challenge, err := h.svc.CreateChallenge(c.Request.Context(), cmd)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, toChallengeResponse(challenge))
}

func (h *SocialHandler) DeleteChallenge(c *gin.Context) {
	if err := h.svc.DeleteChallenge(c.Request.Context(), domain.ChallengeID(c.Param("id")), mustUserID(c)); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *SocialHandler) JoinChallenge(c *gin.Context) {
	if err := h.svc.JoinChallenge(c.Request.Context(), domain.ChallengeID(c.Param("id")), mustUserID(c)); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *SocialHandler) LeaveChallenge(c *gin.Context) {
	if err := h.svc.LeaveChallenge(c.Request.Context(), domain.ChallengeID(c.Param("id")), mustUserID(c)); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *SocialHandler) MarkMovieWatched(c *gin.Context) {
	if err := h.svc.MarkMovieWatched(
		c.Request.Context(),
		domain.ChallengeID(c.Param("id")),
		mustUserID(c),
		c.Param("movie_id"),
	); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *SocialHandler) MyProgress(c *gin.Context) {
	p, err := h.svc.GetMyProgress(c.Request.Context(), domain.ChallengeID(c.Param("id")), mustUserID(c))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toParticipantResponse(p))
}

func (h *SocialHandler) Leaderboard(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	participants, total, err := h.svc.ListLeaderboard(c.Request.Context(), domain.ChallengeID(c.Param("id")), limit, offset)
	if err != nil {
		respondError(c, err)
		return
	}
	out := make([]participantResponse, 0, len(participants))
	for _, p := range participants {
		out = append(out, toParticipantResponse(p))
	}
	c.JSON(http.StatusOK, gin.H{"leaderboard": out, "total": total, "limit": limit, "offset": offset})
}

// ─── Response mappers ─────────────────────────────────────────────────────────

func toActivityResponse(a *domain.Activity) activityResponse {
	return activityResponse{
		ID:          string(a.ID),
		ActorID:     a.ActorID,
		Type:        string(a.Type),
		SubjectType: a.SubjectType,
		SubjectID:   a.SubjectID,
		Payload:     a.Payload,
		CreatedAt:   a.CreatedAt.Format(time.RFC3339),
	}
}

func toActivityResponses(acts []*domain.Activity) []activityResponse {
	out := make([]activityResponse, 0, len(acts))
	for _, a := range acts {
		out = append(out, toActivityResponse(a))
	}
	return out
}

func toThreadResponse(t *domain.Thread) threadResponse {
	return threadResponse{
		ID:                 string(t.ID),
		Scope:              string(t.Scope),
		ScopeID:            t.ScopeID,
		Title:              t.Title,
		SceneTimestampSecs: t.SceneTimestampSecs,
		IsSpoiler:          t.IsSpoiler,
		PostCount:          t.PostCount,
		CreatedBy:          t.CreatedBy,
		CreatedAt:          t.CreatedAt.Format(time.RFC3339),
	}
}

func toThreadResponses(threads []*domain.Thread) []threadResponse {
	out := make([]threadResponse, 0, len(threads))
	for _, t := range threads {
		out = append(out, toThreadResponse(t))
	}
	return out
}

func toPostResponse(p *domain.Post) postResponse {
	resp := postResponse{
		ID:        string(p.ID),
		ThreadID:  string(p.ThreadID),
		AuthorID:  p.AuthorID,
		Body:      p.Body,
		LikeCount: p.LikeCount,
		IsDeleted: p.IsDeleted,
		CreatedAt: p.CreatedAt.Format(time.RFC3339),
	}
	if p.ParentID != nil {
		s := string(*p.ParentID)
		resp.ParentID = &s
	}
	return resp
}

func toPostResponses(posts []*domain.Post) []postResponse {
	out := make([]postResponse, 0, len(posts))
	for _, p := range posts {
		out = append(out, toPostResponse(p))
	}
	return out
}

func toChallengeResponse(c *domain.Challenge) challengeResponse {
	resp := challengeResponse{
		ID:               string(c.ID),
		CreatedBy:        c.CreatedBy,
		Type:             string(c.Type),
		Title:            c.Title,
		Description:      c.Description,
		MovieIDs:         orStringSlice(c.MovieIDs),
		TargetGenre:      c.TargetGenre,
		TargetCount:      c.TargetCount,
		IsPublic:         c.IsPublic,
		ParticipantCount: c.ParticipantCount,
		CreatedAt:        c.CreatedAt.Format(time.RFC3339),
	}
	if c.DeadlineAt != nil {
		s := c.DeadlineAt.Format(time.RFC3339)
		resp.DeadlineAt = &s
	}
	return resp
}

func toChallengeResponses(challenges []*domain.Challenge) []challengeResponse {
	out := make([]challengeResponse, 0, len(challenges))
	for _, c := range challenges {
		out = append(out, toChallengeResponse(c))
	}
	return out
}

func toParticipantResponse(p *domain.ChallengeParticipant) participantResponse {
	resp := participantResponse{
		UserID:            p.UserID,
		CompletedMovieIDs: orStringSlice(p.CompletedMovieIDs),
		IsCompleted:       p.IsCompleted,
		JoinedAt:          p.JoinedAt.Format(time.RFC3339),
	}
	if p.CompletedAt != nil {
		s := p.CompletedAt.Format(time.RFC3339)
		resp.CompletedAt = &s
	}
	return resp
}

// ─── Shared helpers ───────────────────────────────────────────────────────────

func mustUserID(c *gin.Context) string {
	v, _ := c.Get("user_id")
	return v.(string)
}

func orStringSlice(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func respondError(c *gin.Context, err error) {
	log := logger.FromContext(c.Request.Context())
	ae, ok := apierror.As(err)
	if !ok {
		ae = apierror.Internal("unexpected error", err)
	}
	if ae.HTTPStatus >= 500 {
		log.Error().Err(err).Str("code", string(ae.Code)).Msg("internal error")
	}
	c.JSON(ae.HTTPStatus, gin.H{"error": gin.H{
		"code": ae.Code, "message": ae.Message, "details": ae.Details,
	}})
}
