// Package upload provides file upload session management.
package upload

import (
	"context"
	"net/http"
	"net/url"
)

type doer interface {
	Do(ctx context.Context, method, path string, body, out any) error
}

// Service wraps all upload endpoints.
type Service struct{ doer doer }

func New(d doer) *Service { return &Service{doer: d} }

// ─── Types ────────────────────────────────────────────────────────────────────

// InitiateResponse is returned by Initiate.
type InitiateResponse struct {
	UploadID     string `json:"upload_id"`
	PresignedURL string `json:"presigned_url"` // PUT this URL directly to R2
	AssetKey     string `json:"asset_key"`
	ExpiresAt    string `json:"expires_at"`
}

// ProgressResponse is returned by GetProgress.
type ProgressResponse struct {
	UploadID    string `json:"upload_id"`
	Status      string `json:"status"` // "initiated" | "uploading" | "validating" | "processing" | "completed" | "failed"
	FileName    string `json:"file_name"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
	ExpiresAt   string `json:"expires_at"`
}

// CompleteResponse is returned by Complete.
type CompleteResponse struct {
	AssetID    string `json:"asset_id"`
	UploadID   string `json:"upload_id"`
	StorageKey string `json:"storage_key"`
	Status     string `json:"status"`
}

// MediaAsset is a completed upload record.
type MediaAsset struct {
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

// AssetListResult is the paginated response from ListMyAssets.
type AssetListResult struct {
	Assets []MediaAsset `json:"assets"`
	Total  int          `json:"total"`
	Limit  int          `json:"limit"`
	Offset int          `json:"offset"`
}

// InitiateParams is the request body for Initiate.
type InitiateParams struct {
	FileName    string `json:"file_name"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
}

// ─── Methods ──────────────────────────────────────────────────────────────────

// Initiate requests a presigned R2 upload URL.
// The client should then PUT the file bytes directly to PresignedURL.
//
//	resp, err := client.Upload.Initiate(ctx, sdk.InitiateParams{
//	    FileName: "feature.mp4", ContentType: "video/mp4", SizeBytes: fileSize,
//	})
//	// then: http.NewRequest("PUT", resp.PresignedURL, fileReader)
func (s *Service) Initiate(ctx context.Context, p InitiateParams) (*InitiateResponse, error) {
	var out InitiateResponse
	return &out, s.doer.Do(ctx, http.MethodPost, "/api/v1/uploads/initiate", p, &out)
}

// GetProgress returns the current upload session status.
func (s *Service) GetProgress(ctx context.Context, uploadID string) (*ProgressResponse, error) {
	var out ProgressResponse
	return &out, s.doer.Do(ctx, http.MethodGet,
		"/api/v1/uploads/"+url.PathEscape(uploadID), nil, &out)
}

// Complete confirms the file has fully landed in R2 and triggers the
// upload pipeline Temporal workflow.
func (s *Service) Complete(ctx context.Context, uploadID string) (*CompleteResponse, error) {
	var out CompleteResponse
	return &out, s.doer.Do(ctx, http.MethodPost,
		"/api/v1/uploads/"+url.PathEscape(uploadID)+"/complete", nil, &out)
}

// Abort cancels an in-progress upload and deletes any partial R2 object.
func (s *Service) Abort(ctx context.Context, uploadID, reason string) error {
	body := map[string]string{}
	if reason != "" {
		body["reason"] = reason
	}
	return s.doer.Do(ctx, http.MethodDelete,
		"/api/v1/uploads/"+url.PathEscape(uploadID), body, nil)
}

// GetAsset returns a single media asset by ID.
func (s *Service) GetAsset(ctx context.Context, assetID string) (*MediaAsset, error) {
	var out MediaAsset
	return &out, s.doer.Do(ctx, http.MethodGet,
		"/api/v1/me/assets/"+url.PathEscape(assetID), nil, &out)
}

// ListMyAssets returns all media assets belonging to the caller.
func (s *Service) ListMyAssets(ctx context.Context, limit, offset int) (*AssetListResult, error) {
	v := url.Values{}
	if limit > 0 {
		v.Set("limit", "")
	}
	if offset > 0 {
		v.Set("offset", "")
	}
	var out AssetListResult
	return &out, s.doer.Do(ctx, http.MethodGet,
		"/api/v1/me/assets?"+v.Encode(), nil, &out)
}
