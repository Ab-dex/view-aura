package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Ab-dex/view-aura/internal/modules/upload/domain"
	"github.com/Ab-dex/view-aura/internal/modules/upload/service"
	apierror "github.com/Ab-dex/view-aura/internal/platform/error"
	"github.com/Ab-dex/view-aura/internal/platform/logger"
)

type UploadHandler struct{ svc service.UploadService }

func NewUploadHandler(svc service.UploadService) *UploadHandler {
	return &UploadHandler{svc: svc}
}

// RegisterProtectedRoutes registers all upload endpoints (all require auth).
//
//	POST   /uploads/initiate            — quota check, presigned URL, create session
//	GET    /uploads/:id                 — poll session status / progress
//	POST   /uploads/:id/complete        — confirm file landed, trigger pipeline
//	DELETE /uploads/:id                 — abort and clean up
//	GET    /me/assets                   — list caller's media assets
//	GET    /me/assets/:asset_id         — get a single asset
func (h *UploadHandler) RegisterProtectedRoutes(r gin.IRouter) {
	r.POST("/uploads/initiate", h.Initiate)
	r.GET("/uploads/:id", h.GetProgress)
	r.POST("/uploads/:id/complete", h.Complete)
	r.DELETE("/uploads/:id", h.Abort)
	r.GET("/me/assets", h.ListMyAssets)
	r.GET("/me/assets/:asset_id", h.GetAsset)
}

// ─── DTOs ─────────────────────────────────────────────────────────────────────

type initiateRequest struct {
	FileName    string `json:"file_name"    binding:"required"`
	ContentType string `json:"content_type" binding:"required"`
	SizeBytes   int64  `json:"size_bytes"   binding:"required,min=1"`
}

type abortRequest struct {
	Reason string `json:"reason"` // optional; defaults to "user_cancelled"
}

// ─── Response types ───────────────────────────────────────────────────────────

type initiateResponse struct {
	UploadID     string `json:"upload_id"`
	PresignedURL string `json:"presigned_url"`
	AssetKey     string `json:"asset_key"`
	ExpiresAt    string `json:"expires_at"`
}

type progressResponse struct {
	UploadID    string `json:"upload_id"`
	Status      string `json:"status"`
	FileName    string `json:"file_name"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
	ExpiresAt   string `json:"expires_at"`
}

type completeResponse struct {
	AssetID    string `json:"asset_id"`
	UploadID   string `json:"upload_id"`
	StorageKey string `json:"storage_key"`
	Status     string `json:"status"`
}

type assetResponse struct {
	ID          string         `json:"id"`
	UploadID    string         `json:"upload_id"`
	UserID      string         `json:"user_id"`
	MovieID     string         `json:"movie_id,omitempty"`
	StorageKey  string         `json:"storage_key"`
	ContentType string         `json:"content_type"`
	SizeBytes   int64          `json:"size_bytes"`
	Status      string         `json:"status"`
	SourceInfo  map[string]any `json:"source_info,omitempty"`
	CreatedAt   string         `json:"created_at"`
	UpdatedAt   string         `json:"updated_at"`
}

// ─── Handlers ─────────────────────────────────────────────────────────────────

func (h *UploadHandler) Initiate(c *gin.Context) {
	var req initiateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, apierror.Validation(err.Error(), nil))
		return
	}

	session, err := h.svc.Initiate(c.Request.Context(), domain.InitiateUploadCmd{
		UserID:      mustUserID(c),
		FileName:    req.FileName,
		ContentType: req.ContentType,
		SizeBytes:   req.SizeBytes,
	})
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusCreated, initiateResponse{
		UploadID:     session.ID,
		PresignedURL: session.PresignedURL,
		AssetKey:     session.AssetKey,
		ExpiresAt:    session.ExpiresAt.Format(time.RFC3339),
	})
}

func (h *UploadHandler) GetProgress(c *gin.Context) {
	session, err := h.svc.GetProgress(c.Request.Context(), c.Param("id"), mustUserID(c))
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, progressResponse{
		UploadID:    session.ID,
		Status:      string(session.Status),
		FileName:    session.FileName,
		ContentType: session.ContentType,
		SizeBytes:   session.SizeBytes,
		ExpiresAt:   session.ExpiresAt.Format(time.RFC3339),
	})
}

func (h *UploadHandler) Complete(c *gin.Context) {
	asset, err := h.svc.Complete(c.Request.Context(), domain.CompleteUploadCmd{
		UploadID: c.Param("id"),
		UserID:   mustUserID(c),
	})
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, completeResponse{
		AssetID:    asset.ID,
		UploadID:   asset.UploadID,
		StorageKey: asset.StorageKey,
		Status:     asset.Status,
	})
}

func (h *UploadHandler) Abort(c *gin.Context) {
	var req abortRequest
	_ = c.ShouldBindJSON(&req) // optional body
	reason := req.Reason
	if reason == "" {
		reason = "user_cancelled"
	}

	if err := h.svc.Abort(c.Request.Context(), domain.AbortUploadCmd{
		UploadID: c.Param("id"),
		UserID:   mustUserID(c),
		Reason:   reason,
	}); err != nil {
		respondError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *UploadHandler) GetAsset(c *gin.Context) {
	asset, err := h.svc.GetAsset(c.Request.Context(), c.Param("asset_id"), mustUserID(c))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toAssetResponse(asset))
}

func (h *UploadHandler) ListMyAssets(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	assets, total, err := h.svc.ListMyAssets(c.Request.Context(), mustUserID(c), limit, offset)
	if err != nil {
		respondError(c, err)
		return
	}

	out := make([]assetResponse, 0, len(assets))
	for _, a := range assets {
		out = append(out, toAssetResponse(a))
	}
	c.JSON(http.StatusOK, gin.H{
		"assets": out,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// ─── Mappers ──────────────────────────────────────────────────────────────────

func toAssetResponse(a *domain.MediaAsset) assetResponse {
	return assetResponse{
		ID:          a.ID,
		UploadID:    a.UploadID,
		UserID:      a.UserID,
		MovieID:     a.MovieID,
		StorageKey:  a.StorageKey,
		ContentType: a.ContentType,
		SizeBytes:   a.SizeBytes,
		Status:      a.Status,
		SourceInfo:  a.SourceInfo,
		CreatedAt:   a.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   a.UpdatedAt.Format(time.RFC3339),
	}
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
		log.Error().Err(err).Str("code", string(ae.Code)).Msg("upload: internal error")
	}
	c.JSON(ae.HTTPStatus, gin.H{"error": gin.H{
		"code":    ae.Code,
		"message": ae.Message,
		"details": ae.Details,
	}})
}
