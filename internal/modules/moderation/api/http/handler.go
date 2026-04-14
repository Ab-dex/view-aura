package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Ab-dex/view-aura/internal/modules/moderation/domain"
	"github.com/Ab-dex/view-aura/internal/modules/moderation/service"
	apierror "github.com/Ab-dex/view-aura/internal/platform/error"
	"github.com/Ab-dex/view-aura/internal/platform/logger"
)

type ModerationHandler struct{ svc service.ModerationService }

func NewModerationHandler(svc service.ModerationService) *ModerationHandler {
	return &ModerationHandler{svc: svc}
}

// RegisterAdminRoutes registers routes accessible to moderators and admins.
//
//	GET    /moderation/queue              — list pending cases
//	GET    /moderation/:id               — get case detail
//	POST   /moderation/:id/lock          — acquire lock before deciding
//	DELETE /moderation/:id/lock          — release lock
//	POST   /moderation/:id/approve       — approve content
//	POST   /moderation/:id/reject        — reject content
//	GET    /moderation/:id/appeal        — get appeal for a case
//	POST   /moderation/:id/appeal/decide — decide an appeal (senior moderator)
func (h *ModerationHandler) RegisterAdminRoutes(r gin.IRouter) {
	r.GET("/moderation/queue", h.ListQueue)
	r.GET("/moderation/:id", h.GetCase)
	r.POST("/moderation/:id/lock", h.AcquireLock)
	r.DELETE("/moderation/:id/lock", h.ReleaseLock)
	r.POST("/moderation/:id/approve", h.Approve)
	r.POST("/moderation/:id/reject", h.Reject)
	r.GET("/moderation/:id/appeal", h.GetAppeal)
	r.POST("/moderation/:id/appeal/decide", h.DecideAppeal)
}

// RegisterProtectedRoutes registers user-facing moderation endpoints.
//
//	GET  /me/moderation         — list my content under moderation
//	POST /me/moderation/:id/appeal — submit an appeal
func (h *ModerationHandler) RegisterProtectedRoutes(r gin.IRouter) {
	r.GET("/me/moderation", h.MyCases)
	r.POST("/me/moderation/:id/appeal", h.SubmitAppeal)
}

// ─── DTOs ─────────────────────────────────────────────────────────────────────

type decideRequest struct {
	Reason string `json:"reason"`
}

type appealRequest struct {
	Statement string `json:"statement" binding:"required"`
}

type decideAppealRequest struct {
	Outcome string `json:"outcome" binding:"required"` // "upheld" | "overturned"
}

// ─── Response types ───────────────────────────────────────────────────────────

type caseResponse struct {
	ID               string             `json:"id"`
	ContentType      string             `json:"content_type"`
	ContentID        string             `json:"content_id"`
	OwnerID          string             `json:"owner_id"`
	Status           string             `json:"status"`
	TriggerReason    string             `json:"trigger_reason"`
	AISignals        []string           `json:"ai_signals,omitempty"`
	AIConfidence     map[string]float64 `json:"ai_confidence,omitempty"`
	AIRecommendation string             `json:"ai_recommendation,omitempty"`
	Decision         *string            `json:"decision,omitempty"`
	DecidedBy        string             `json:"decided_by,omitempty"`
	Reason           string             `json:"reason,omitempty"`
	DecidedAt        *string            `json:"decided_at,omitempty"`
	LockedBy         string             `json:"locked_by,omitempty"`
	CreatedAt        string             `json:"created_at"`
}

type appealResponse struct {
	ID          string  `json:"id"`
	CaseID      string  `json:"case_id"`
	AppellantID string  `json:"appellant_id"`
	Statement   string  `json:"statement"`
	Outcome     string  `json:"outcome,omitempty"`
	DecidedBy   string  `json:"decided_by,omitempty"`
	DecidedAt   *string `json:"decided_at,omitempty"`
	CreatedAt   string  `json:"created_at"`
}

// ─── Admin handlers ───────────────────────────────────────────────────────────

func (h *ModerationHandler) ListQueue(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	filter := domain.CaseFilter{Limit: limit, Offset: offset}
	if s := c.Query("status"); s != "" {
		st := domain.CaseStatus(s)
		filter.Status = &st
	}
	if ct := c.Query("content_type"); ct != "" {
		t := domain.ContentType(ct)
		filter.ContentType = &t
	}

	cases, total, err := h.svc.ListPendingCases(c.Request.Context(), filter)
	if err != nil {
		respondError(c, err)
		return
	}
	out := make([]caseResponse, 0, len(cases))
	for _, cs := range cases {
		out = append(out, toCaseResponse(cs))
	}
	c.JSON(http.StatusOK, gin.H{"cases": out, "total": total, "limit": limit, "offset": offset})
}

func (h *ModerationHandler) GetCase(c *gin.Context) {
	cs, err := h.svc.GetCase(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toCaseResponse(cs))
}

func (h *ModerationHandler) AcquireLock(c *gin.Context) {
	acquired, err := h.svc.AcquireLock(c.Request.Context(), c.Param("id"), mustUserID(c))
	if err != nil {
		respondError(c, err)
		return
	}
	if !acquired {
		respondError(c, apierror.ErrModerationLocked)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *ModerationHandler) ReleaseLock(c *gin.Context) {
	if err := h.svc.ReleaseLock(c.Request.Context(), c.Param("id")); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *ModerationHandler) Approve(c *gin.Context) {
	var req decideRequest
	_ = c.ShouldBindJSON(&req)
	cs, err := h.svc.Decide(c.Request.Context(), domain.DecideCmd{
		CaseID:      c.Param("id"),
		ModeratorID: mustUserID(c),
		Decision:    domain.DecisionApproved,
		Reason:      req.Reason,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toCaseResponse(cs))
}

func (h *ModerationHandler) Reject(c *gin.Context) {
	var req decideRequest
	_ = c.ShouldBindJSON(&req)
	cs, err := h.svc.Decide(c.Request.Context(), domain.DecideCmd{
		CaseID:      c.Param("id"),
		ModeratorID: mustUserID(c),
		Decision:    domain.DecisionRejected,
		Reason:      req.Reason,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toCaseResponse(cs))
}

func (h *ModerationHandler) GetAppeal(c *gin.Context) {
	appeal, err := h.svc.GetAppeal(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	if appeal == nil {
		c.JSON(http.StatusOK, gin.H{"appeal": nil})
		return
	}
	c.JSON(http.StatusOK, toAppealResponse(appeal))
}

func (h *ModerationHandler) DecideAppeal(c *gin.Context) {
	var req decideAppealRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierror.Validation(err.Error(), nil))
		return
	}
	// Get the appeal ID from the case ID.
	appeal, err := h.svc.GetAppeal(c.Request.Context(), c.Param("id"))
	if err != nil || appeal == nil {
		respondError(c, apierror.NotFound("appeal not found"))
		return
	}
	updated, err := h.svc.DecideAppeal(c.Request.Context(), domain.DecideAppealCmd{
		AppealID:    appeal.ID,
		ModeratorID: mustUserID(c),
		Outcome:     req.Outcome,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toAppealResponse(updated))
}

// ─── User-facing handlers ─────────────────────────────────────────────────────

func (h *ModerationHandler) MyCases(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	cases, total, err := h.svc.GetMyCases(c.Request.Context(), mustUserID(c), limit, offset)
	if err != nil {
		respondError(c, err)
		return
	}
	out := make([]caseResponse, 0, len(cases))
	for _, cs := range cases {
		out = append(out, toCaseResponse(cs))
	}
	c.JSON(http.StatusOK, gin.H{"cases": out, "total": total})
}

func (h *ModerationHandler) SubmitAppeal(c *gin.Context) {
	var req appealRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierror.Validation(err.Error(), nil))
		return
	}
	appeal, err := h.svc.SubmitAppeal(c.Request.Context(), domain.SubmitAppealCmd{
		CaseID:      c.Param("id"),
		AppellantID: mustUserID(c),
		Statement:   req.Statement,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, toAppealResponse(appeal))
}

// ─── Mappers ──────────────────────────────────────────────────────────────────

func toCaseResponse(c *domain.ModerationCase) caseResponse {
	r := caseResponse{
		ID:               c.ID,
		ContentType:      string(c.ContentType),
		ContentID:        c.ContentID,
		OwnerID:          c.OwnerID,
		Status:           string(c.Status),
		TriggerReason:    c.TriggerReason,
		AISignals:        c.AISignals,
		AIConfidence:     c.AIConfidence,
		AIRecommendation: c.AIRecommendation,
		DecidedBy:        c.DecidedBy,
		Reason:           c.Reason,
		LockedBy:         c.LockedBy,
		CreatedAt:        c.CreatedAt.Format(time.RFC3339),
	}
	if c.Decision != nil {
		s := string(*c.Decision)
		r.Decision = &s
	}
	if c.DecidedAt != nil {
		s := c.DecidedAt.Format(time.RFC3339)
		r.DecidedAt = &s
	}
	return r
}

func toAppealResponse(a *domain.Appeal) appealResponse {
	r := appealResponse{
		ID:          a.ID,
		CaseID:      a.CaseID,
		AppellantID: a.AppellantID,
		Statement:   a.Statement,
		Outcome:     a.Outcome,
		DecidedBy:   a.DecidedBy,
		CreatedAt:   a.CreatedAt.Format(time.RFC3339),
	}
	if a.DecidedAt != nil {
		s := a.DecidedAt.Format(time.RFC3339)
		r.DecidedAt = &s
	}
	return r
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func mustUserID(c *gin.Context) string { v, _ := c.Get("user_id"); return v.(string) }

func respondError(c *gin.Context, err error) {
	log := logger.FromContext(c.Request.Context())
	ae, ok := apierror.As(err)
	if !ok {
		ae = apierror.Internal("unexpected error", err)
	}
	if ae.HTTPStatus >= 500 {
		log.Error().Err(err).Msg("moderation: internal error")
	}
	c.JSON(ae.HTTPStatus, gin.H{"error": gin.H{"code": ae.Code, "message": ae.Message}})
}
