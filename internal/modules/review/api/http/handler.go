package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Ab-dex/view-aura/internal/modules/review/domain"
	"github.com/Ab-dex/view-aura/internal/modules/review/service"
	apierror "github.com/Ab-dex/view-aura/internal/platform/error"
	"github.com/Ab-dex/view-aura/internal/platform/logger"
)

type ReviewHandler struct {
	svc service.ReviewService
}

func NewReviewHandler(svc service.ReviewService) *ReviewHandler {
	return &ReviewHandler{svc: svc}
}

// Route layout (all under a shared router, movie_id from path):
//
//	GET  /movies/:movie_id/reviews              — list (public)
//	GET  /reviews/:id                           — single review (public)
//	GET  /users/:user_id/reviews                — user's review history (public)
//	POST /movies/:movie_id/reviews              — create (auth)
//	PATCH /reviews/:id                          — edit (auth, owner only)
//	DELETE /reviews/:id                         — delete (auth, owner only)
//	POST /reviews/:id/reactions                 — react (auth)
//	DELETE /reviews/:id/reactions               — un-react (auth)
//	GET  /reviews/:id/reactions/me              — my reaction (auth)
//	POST /reviews/:id/reports                   — report (auth)

func (h *ReviewHandler) RegisterPublicRoutes(r gin.IRouter) {
	r.GET("/movies/:movie_id/reviews", h.List)
	r.GET("/reviews/:id", h.GetByID)
	r.GET("/users/:user_id/reviews", h.ListByUser)
}

func (h *ReviewHandler) RegisterProtectedRoutes(r gin.IRouter) {
	r.POST("/movies/:movie_id/reviews", h.Create)
	r.PATCH("/reviews/:id", h.Update)
	r.DELETE("/reviews/:id", h.Delete)
	r.POST("/reviews/:id/reactions", h.React)
	r.DELETE("/reviews/:id/reactions", h.UnReact)
	r.GET("/reviews/:id/reactions/me", h.GetMyReaction)
	r.POST("/reviews/:id/reports", h.Report)
}

// ─── DTOs ─────────────────────────────────────────────────────────────────────

type createReviewRequest struct {
	Type      string `json:"type"       binding:"required"`
	Body      string `json:"body"`
	VideoURL  string `json:"video_url"`
	IsSpoiler bool   `json:"is_spoiler"`
}

type updateReviewRequest struct {
	Body      *string `json:"body"`
	VideoURL  *string `json:"video_url"`
	IsSpoiler *bool   `json:"is_spoiler"`
	Publish   *bool   `json:"publish"`
}

type reactRequest struct {
	Type string `json:"type" binding:"required"`
}

type reportRequest struct {
	Reason string `json:"reason" binding:"required"`
	Note   string `json:"note"`
}

type reviewResponse struct {
	ID               string  `json:"id"`
	MovieID          string  `json:"movie_id"`
	UserID           string  `json:"user_id"`
	Type             string  `json:"type"`
	Body             string  `json:"body,omitempty"`
	VideoURL         string  `json:"video_url,omitempty"`
	IsSpoiler        bool    `json:"is_spoiler"`
	LikeCount        int     `json:"like_count"`
	HelpfulCount     int     `json:"helpful_count"`
	InsightfulCount  int     `json:"insightful_count"`
	FunnyCount       int     `json:"funny_count"`
	CredibilityScore float64 `json:"credibility_score"`
	CreatedAt        string  `json:"created_at"`
	UpdatedAt        string  `json:"updated_at"`
}

// ─── Handlers ─────────────────────────────────────────────────────────────────

func (h *ReviewHandler) Create(c *gin.Context) {
	var req createReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierror.Validation(err.Error(), nil))
		return
	}

	rev, err := h.svc.Create(c.Request.Context(), domain.CreateReviewCmd{
		MovieID:   c.Param("movie_id"),
		UserID:    mustUserID(c),
		Type:      domain.ReviewType(req.Type),
		Body:      req.Body,
		VideoURL:  req.VideoURL,
		IsSpoiler: req.IsSpoiler,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, toReviewResponse(rev))
}

func (h *ReviewHandler) Update(c *gin.Context) {
	var req updateReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierror.Validation(err.Error(), nil))
		return
	}

	rev, err := h.svc.Update(c.Request.Context(), domain.UpdateReviewCmd{
		ID:        domain.ReviewID(c.Param("id")),
		UserID:    mustUserID(c),
		Body:      req.Body,
		VideoURL:  req.VideoURL,
		IsSpoiler: req.IsSpoiler,
		Publish:   req.Publish,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toReviewResponse(rev))
}

func (h *ReviewHandler) Delete(c *gin.Context) {
	if err := h.svc.Delete(c.Request.Context(), domain.ReviewID(c.Param("id")), mustUserID(c)); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *ReviewHandler) GetByID(c *gin.Context) {
	rev, err := h.svc.GetByID(c.Request.Context(), domain.ReviewID(c.Param("id")))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toReviewResponse(rev))
}

func (h *ReviewHandler) List(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	filter := domain.ReviewFilter{
		MovieID:         c.Param("movie_id"),
		SortBy:          c.DefaultQuery("sort_by", "created_at"),
		IncludeSpoilers: c.Query("include_spoilers") == "true",
		Limit:           limit,
		Offset:          offset,
	}
	if t := c.Query("type"); t != "" {
		rt := domain.ReviewType(t)
		filter.Type = &rt
	}

	reviews, total, err := h.svc.List(c.Request.Context(), filter)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"reviews": toReviewResponses(reviews),
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}

func (h *ReviewHandler) ListByUser(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	reviews, total, err := h.svc.List(c.Request.Context(), domain.ReviewFilter{
		UserID: c.Param("user_id"),
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"reviews": toReviewResponses(reviews),
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}

func (h *ReviewHandler) React(c *gin.Context) {
	var req reactRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierror.Validation(err.Error(), nil))
		return
	}
	if err := h.svc.React(c.Request.Context(), domain.ReactToReviewCmd{
		ReviewID: domain.ReviewID(c.Param("id")),
		UserID:   mustUserID(c),
		Type:     domain.ReviewReactionType(req.Type),
	}); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *ReviewHandler) UnReact(c *gin.Context) {
	if err := h.svc.UnReact(c.Request.Context(), domain.ReviewID(c.Param("id")), mustUserID(c)); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *ReviewHandler) GetMyReaction(c *gin.Context) {
	rx, err := h.svc.GetMyReaction(c.Request.Context(), mustUserID(c), domain.ReviewID(c.Param("id")))
	if err != nil {
		respondError(c, err)
		return
	}
	if rx == nil {
		c.JSON(http.StatusOK, gin.H{"reaction": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"reaction": gin.H{
		"id":         string(rx.ID),
		"type":       string(rx.Type),
		"created_at": rx.CreatedAt.Format(time.RFC3339),
	}})
}

func (h *ReviewHandler) Report(c *gin.Context) {
	var req reportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierror.Validation(err.Error(), nil))
		return
	}
	if err := h.svc.Report(c.Request.Context(), domain.ReportReviewCmd{
		ReviewID: domain.ReviewID(c.Param("id")),
		UserID:   mustUserID(c),
		Reason:   domain.ReportReason(req.Reason),
		Note:     req.Note,
	}); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func mustUserID(c *gin.Context) string {
	v, _ := c.Get("user_id")
	return v.(string)
}

func toReviewResponse(rev *domain.Review) reviewResponse {
	return reviewResponse{
		ID:               string(rev.ID),
		MovieID:          rev.MovieID,
		UserID:           rev.UserID,
		Type:             string(rev.Type),
		Body:             rev.Body,
		VideoURL:         rev.VideoURL,
		IsSpoiler:        rev.IsSpoiler,
		LikeCount:        rev.LikeCount,
		HelpfulCount:     rev.HelpfulCount,
		InsightfulCount:  rev.InsightfulCount,
		FunnyCount:       rev.FunnyCount,
		CredibilityScore: rev.CredibilityScore,
		CreatedAt:        rev.CreatedAt.Format(time.RFC3339),
		UpdatedAt:        rev.UpdatedAt.Format(time.RFC3339),
	}
}

func toReviewResponses(reviews []*domain.Review) []reviewResponse {
	out := make([]reviewResponse, 0, len(reviews))
	for _, r := range reviews {
		out = append(out, toReviewResponse(r))
	}
	return out
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
