package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Ab-dex/view-aura/internal/modules/notification/domain"
	"github.com/Ab-dex/view-aura/internal/modules/notification/service"
	apierror "github.com/Ab-dex/view-aura/internal/platform/error"
	"github.com/Ab-dex/view-aura/internal/platform/logger"
)

type NotificationHandler struct {
	svc service.NotificationService
}

func NewNotificationHandler(svc service.NotificationService) *NotificationHandler {
	return &NotificationHandler{svc: svc}
}

// All routes are protected (auth required).
//
//	GET    /me/notifications               — inbox (paginated, filter ?unread=true)
//	GET    /me/notifications/unread-count  — badge count
//	PATCH  /me/notifications/:id/read      — mark one read
//	PATCH  /me/notifications/read-all      — mark all read
//	DELETE /me/notifications/:id           — delete one
//
//	GET    /me/notification-preferences            — get all preferences
//	PUT    /me/notification-preferences            — bulk update
//	PATCH  /me/notification-preferences/:type/:channel — update single pref
//
//	POST   /me/push-tokens                 — register device token
//	DELETE /me/push-tokens                 — unregister device token

func (h *NotificationHandler) RegisterProtectedRoutes(r gin.IRouter) {
	r.GET("/me/notifications", h.List)
	r.GET("/me/notifications/unread-count", h.UnreadCount)
	r.PATCH("/me/notifications/:id/read", h.MarkRead)
	r.PATCH("/me/notifications/read-all", h.MarkAllRead)
	r.DELETE("/me/notifications/:id", h.Delete)

	r.GET("/me/notification-preferences", h.GetPreferences)
	r.PUT("/me/notification-preferences", h.BulkSetPreferences)
	r.PATCH("/me/notification-preferences/:type/:channel", h.SetPreference)

	r.POST("/me/push-tokens", h.RegisterPushToken)
	r.DELETE("/me/push-tokens", h.UnregisterPushToken)
}

// ─── DTOs ─────────────────────────────────────────────────────────────────────

type preferenceUpdate struct {
	Type    string `json:"type"    binding:"required"`
	Channel string `json:"channel" binding:"required"`
	Enabled bool   `json:"enabled"`
}

type bulkPreferenceRequest struct {
	Preferences []preferenceUpdate `json:"preferences" binding:"required"`
}

type pushTokenRequest struct {
	Token    string `json:"token"    binding:"required"`
	Platform string `json:"platform" binding:"required"`
}

// ─── Response types ───────────────────────────────────────────────────────────

type notificationResponse struct {
	ID            string         `json:"id"`
	Type          string         `json:"type"`
	Title         string         `json:"title"`
	Body          string         `json:"body"`
	Payload       map[string]any `json:"payload,omitempty"`
	ReferenceType string         `json:"reference_type,omitempty"`
	ReferenceID   string         `json:"reference_id,omitempty"`
	IsRead        bool           `json:"is_read"`
	CreatedAt     string         `json:"created_at"`
	ReadAt        *string        `json:"read_at,omitempty"`
}

type preferenceResponse struct {
	Type    string `json:"type"`
	Channel string `json:"channel"`
	Enabled bool   `json:"enabled"`
}

// ─── Handlers — inbox ─────────────────────────────────────────────────────────

func (h *NotificationHandler) List(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	filter := domain.NotificationFilter{
		UserID: mustUserID(c),
		Limit:  limit,
		Offset: offset,
	}
	if v := c.Query("unread"); v == "true" {
		b := false // is_read = false
		filter.IsRead = &b
	}

	notifs, total, err := h.svc.List(c.Request.Context(), filter)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"notifications": toNotifResponses(notifs),
		"total":         total,
		"limit":         limit,
		"offset":        offset,
	})
}

func (h *NotificationHandler) UnreadCount(c *gin.Context) {
	count, err := h.svc.UnreadCount(c.Request.Context(), mustUserID(c))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"unread_count": count})
}

func (h *NotificationHandler) MarkRead(c *gin.Context) {
	if err := h.svc.MarkRead(
		c.Request.Context(),
		domain.NotificationID(c.Param("id")),
		mustUserID(c),
	); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *NotificationHandler) MarkAllRead(c *gin.Context) {
	if err := h.svc.MarkAllRead(c.Request.Context(), mustUserID(c)); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *NotificationHandler) Delete(c *gin.Context) {
	if err := h.svc.Delete(
		c.Request.Context(),
		domain.NotificationID(c.Param("id")),
		mustUserID(c),
	); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ─── Handlers — preferences ───────────────────────────────────────────────────

func (h *NotificationHandler) GetPreferences(c *gin.Context) {
	prefs, err := h.svc.GetPreferences(c.Request.Context(), mustUserID(c))
	if err != nil {
		respondError(c, err)
		return
	}
	out := make([]preferenceResponse, 0, len(prefs))
	for _, p := range prefs {
		out = append(out, preferenceResponse{
			Type:    string(p.Type),
			Channel: string(p.Channel),
			Enabled: p.Enabled,
		})
	}
	c.JSON(http.StatusOK, gin.H{"preferences": out})
}

func (h *NotificationHandler) BulkSetPreferences(c *gin.Context) {
	var req bulkPreferenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierror.Validation(err.Error(), nil))
		return
	}

	userID := mustUserID(c)
	for _, p := range req.Preferences {
		if err := h.svc.SetPreference(
			c.Request.Context(),
			userID,
			domain.NotificationType(p.Type),
			domain.Channel(p.Channel),
			p.Enabled,
		); err != nil {
			respondError(c, err)
			return
		}
	}
	c.Status(http.StatusNoContent)
}

func (h *NotificationHandler) SetPreference(c *gin.Context) {
	// Body contains only the "enabled" boolean — type and channel come from the path.
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierror.Validation(err.Error(), nil))
		return
	}

	if err := h.svc.SetPreference(
		c.Request.Context(),
		mustUserID(c),
		domain.NotificationType(c.Param("type")),
		domain.Channel(c.Param("channel")),
		req.Enabled,
	); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ─── Handlers — push tokens ───────────────────────────────────────────────────

func (h *NotificationHandler) RegisterPushToken(c *gin.Context) {
	var req pushTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierror.Validation(err.Error(), nil))
		return
	}

	if err := h.svc.RegisterPushToken(c.Request.Context(), mustUserID(c), req.Token, req.Platform); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *NotificationHandler) UnregisterPushToken(c *gin.Context) {
	var req struct {
		Token string `json:"token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierror.Validation(err.Error(), nil))
		return
	}

	if err := h.svc.UnregisterPushToken(c.Request.Context(), mustUserID(c), req.Token); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ─── Response mappers ─────────────────────────────────────────────────────────

func toNotifResponse(n *domain.Notification) notificationResponse {
	resp := notificationResponse{
		ID:            string(n.ID),
		Type:          string(n.Type),
		Title:         n.Title,
		Body:          n.Body,
		Payload:       n.Payload,
		ReferenceType: n.ReferenceType,
		ReferenceID:   n.ReferenceID,
		IsRead:        n.IsRead,
		CreatedAt:     n.CreatedAt.Format(time.RFC3339),
	}
	if n.ReadAt != nil {
		s := n.ReadAt.Format(time.RFC3339)
		resp.ReadAt = &s
	}
	return resp
}

func toNotifResponses(notifs []*domain.Notification) []notificationResponse {
	out := make([]notificationResponse, 0, len(notifs))
	for _, n := range notifs {
		out = append(out, toNotifResponse(n))
	}
	return out
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func mustUserID(c *gin.Context) string {
	v, _ := c.Get("user_id")
	return v.(string)
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
