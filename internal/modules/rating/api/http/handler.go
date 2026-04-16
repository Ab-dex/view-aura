package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Ab-dex/view-aura/internal/modules/rating/domain"
	"github.com/Ab-dex/view-aura/internal/modules/rating/service"
	apierror "github.com/Ab-dex/view-aura/internal/platform/error"
	"github.com/Ab-dex/view-aura/internal/platform/logger"
)

type RatingHandler struct {
	svc service.RatingService
}

func NewRatingHandler(svc service.RatingService) *RatingHandler {
	return &RatingHandler{svc: svc}
}

// RegisterRoutes attaches rating routes.
//
//	PUT  /movies/:movie_id/rating        — upsert own rating
//	DELETE /movies/:movie_id/rating      — delete own rating
//	GET  /movies/:movie_id/rating        — get own rating for this movie
//	GET  /movies/:movie_id/ratings       — list all ratings (public)
//	GET  /movies/:movie_id/ratings/stats — aggregated stats (public)
//	GET  /users/:user_id/ratings         — user's rating history (public)
func (h *RatingHandler) RegisterPublicMovieRatingRoutes(r gin.IRouter) {
	r.GET("", h.ListByMovie)
	r.GET("/stats", h.GetStats)
}

func (h *RatingHandler) RegisterPublicUserRatingRoutes(r gin.IRouter) {
	r.GET("/users/:user_id/ratings", h.ListByUser)
}

// func (h *RatingHandler) RegisterPublicRoutes(r gin.IRouter) {
// 	r.GET("/movies/:movie_id/ratings", h.ListByMovie)
// 	r.GET("/movies/:movie_id/ratings/stats", h.GetStats)
// 	r.GET("/users/:user_id/ratings", h.ListByUser)
// }

func (h *RatingHandler) RegisterProtectedRoutes(r gin.IRouter) {
	r.PUT("", h.Upsert)
	r.DELETE("", h.Delete)
	r.GET("", h.GetMine)
}

// ─── DTOs ─────────────────────────────────────────────────────────────────────

type upsertRatingRequest struct {
	Overall        float64  `json:"overall"         binding:"required,min=1,max=5"`
	Acting         *float64 `json:"acting"`
	Direction      *float64 `json:"direction"`
	Writing        *float64 `json:"writing"`
	Cinematography *float64 `json:"cinematography"`
	Soundtrack     *float64 `json:"soundtrack"`
	Reaction       *string  `json:"reaction"`
}

type ratingResponse struct {
	ID             string   `json:"id"`
	MovieID        string   `json:"movie_id"`
	UserID         string   `json:"user_id"`
	Overall        float64  `json:"overall"`
	Acting         *float64 `json:"acting,omitempty"`
	Direction      *float64 `json:"direction,omitempty"`
	Writing        *float64 `json:"writing,omitempty"`
	Cinematography *float64 `json:"cinematography,omitempty"`
	Soundtrack     *float64 `json:"soundtrack,omitempty"`
	Reaction       *string  `json:"reaction,omitempty"`
	IsVerified     bool     `json:"is_verified"`
	CreatedAt      string   `json:"created_at"`
	UpdatedAt      string   `json:"updated_at"`
}

type statsResponse struct {
	MovieID           string      `json:"movie_id"`
	TotalRatings      int         `json:"total_ratings"`
	AvgOverall        float64     `json:"avg_overall"`
	AvgActing         *float64    `json:"avg_acting,omitempty"`
	AvgDirection      *float64    `json:"avg_direction,omitempty"`
	AvgWriting        *float64    `json:"avg_writing,omitempty"`
	AvgCinematography *float64    `json:"avg_cinematography,omitempty"`
	AvgSoundtrack     *float64    `json:"avg_soundtrack,omitempty"`
	Distribution      map[int]int `json:"distribution"`
}

// ─── Handlers ─────────────────────────────────────────────────────────────────

func (h *RatingHandler) Upsert(c *gin.Context) {
	var req upsertRatingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierror.Validation(err.Error(), nil))
		return
	}

	var reaction *domain.Reaction
	if req.Reaction != nil {
		r := domain.Reaction(*req.Reaction)
		reaction = &r
	}

	rt, err := h.svc.Upsert(c.Request.Context(), domain.UpsertRatingCmd{
		MovieID:        c.Param("movie_id"),
		UserID:         mustUserID(c),
		Overall:        req.Overall,
		Acting:         req.Acting,
		Direction:      req.Direction,
		Writing:        req.Writing,
		Cinematography: req.Cinematography,
		Soundtrack:     req.Soundtrack,
		Reaction:       reaction,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toRatingResponse(rt))
}

func (h *RatingHandler) Delete(c *gin.Context) {
	if err := h.svc.Delete(c.Request.Context(), mustUserID(c), c.Param("movie_id")); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *RatingHandler) GetMine(c *gin.Context) {
	rt, err := h.svc.GetByUser(c.Request.Context(), mustUserID(c), c.Param("movie_id"))
	if err != nil {
		respondError(c, err)
		return
	}
	if rt == nil {
		c.JSON(http.StatusOK, gin.H{"rating": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"rating": toRatingResponse(rt)})
}

func (h *RatingHandler) ListByMovie(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	ratings, total, err := h.svc.ListByMovie(c.Request.Context(), domain.RatingFilter{
		MovieID: c.Param("movie_id"),
		Limit:   limit,
		Offset:  offset,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"ratings": toRatingResponses(ratings),
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}

func (h *RatingHandler) ListByUser(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	ratings, total, err := h.svc.ListByUser(c.Request.Context(), domain.RatingFilter{
		UserID: c.Param("user_id"),
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"ratings": toRatingResponses(ratings),
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}

func (h *RatingHandler) GetStats(c *gin.Context) {
	stats, err := h.svc.GetStats(c.Request.Context(), c.Param("movie_id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, statsResponse{
		MovieID:           stats.MovieID,
		TotalRatings:      stats.TotalRatings,
		AvgOverall:        stats.AvgOverall,
		AvgActing:         stats.AvgActing,
		AvgDirection:      stats.AvgDirection,
		AvgWriting:        stats.AvgWriting,
		AvgCinematography: stats.AvgCinematography,
		AvgSoundtrack:     stats.AvgSoundtrack,
		Distribution:      stats.Distribution,
	})
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func mustUserID(c *gin.Context) string {
	v, _ := c.Get("user_id")
	return v.(string)
}

func toRatingResponse(rt *domain.Rating) ratingResponse {
	var reaction *string
	if rt.Reaction != nil {
		s := string(*rt.Reaction)
		reaction = &s
	}
	return ratingResponse{
		ID:             string(rt.ID),
		MovieID:        rt.MovieID,
		UserID:         rt.UserID,
		Overall:        rt.Overall,
		Acting:         rt.Acting,
		Direction:      rt.Direction,
		Writing:        rt.Writing,
		Cinematography: rt.Cinematography,
		Soundtrack:     rt.Soundtrack,
		Reaction:       reaction,
		IsVerified:     rt.IsVerified,
		CreatedAt:      rt.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      rt.UpdatedAt.Format(time.RFC3339),
	}
}

func toRatingResponses(rts []*domain.Rating) []ratingResponse {
	out := make([]ratingResponse, 0, len(rts))
	for _, rt := range rts {
		out = append(out, toRatingResponse(rt))
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
