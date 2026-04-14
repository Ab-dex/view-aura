package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Ab-dex/view-aura/internal/modules/quota/service"
	apierror "github.com/Ab-dex/view-aura/internal/platform/error"
	"github.com/Ab-dex/view-aura/internal/platform/logger"
)

type QuotaHandler struct{ svc service.QuotaService }

func NewQuotaHandler(svc service.QuotaService) *QuotaHandler {
	return &QuotaHandler{svc: svc}
}

// RegisterProtectedRoutes registers all quota endpoints (all require auth).
//
//	GET  /me/quota       — full usage snapshot (storage %, daily %, tier, limits)
//	GET  /me/quota/tier  — just the tier name and limits
func (h *QuotaHandler) RegisterProtectedRoutes(r gin.IRouter) {
	r.GET("/me/quota", h.GetStatus)
	r.GET("/me/quota/tier", h.GetTier)
}

func (h *QuotaHandler) GetStatus(c *gin.Context) {
	status, err := h.svc.GetStatus(c.Request.Context(), mustUserID(c))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"tier": string(status.Tier),
		"storage": gin.H{
			"used_bytes": status.StorageUsedBytes,
			"max_bytes":  status.Limits.MaxStorageBytes,
			"used_pct":   status.StorageUsedPct,
		},
		"daily_uploads": gin.H{
			"used":     status.DailyUploadsUsed,
			"max":      status.Limits.MaxDailyUploads,
			"used_pct": status.DailyUsedPct,
		},
		"limits": gin.H{
			"max_file_size_bytes": status.Limits.MaxFileSizeBytes,
			"transcode_speed":     status.Limits.TranscodeSpeed,
			"retention_days":      status.Limits.RetentionDays,
		},
		"as_of": status.AsOf,
	})
}

func (h *QuotaHandler) GetTier(c *gin.Context) {
	status, err := h.svc.GetStatus(c.Request.Context(), mustUserID(c))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"tier":   string(status.Tier),
		"limits": status.Limits,
	})
}

func mustUserID(c *gin.Context) string { v, _ := c.Get("user_id"); return v.(string) }

func respondError(c *gin.Context, err error) {
	log := logger.FromContext(c.Request.Context())
	ae, ok := apierror.As(err)
	if !ok {
		ae = apierror.Internal("unexpected error", err)
	}
	if ae.HTTPStatus >= 500 {
		log.Error().Err(err).Msg("quota: internal error")
	}
	c.JSON(ae.HTTPStatus, gin.H{"error": gin.H{"code": ae.Code, "message": ae.Message}})
}
