package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"

	apierror "github.com/Ab-dex/view-aura/internal/platform/error"
	r2pkg "github.com/Ab-dex/view-aura/internal/platform/r2"
	"github.com/Ab-dex/view-aura/internal/platform/resilience"
)

// ResilientR2Client wraps *r2pkg.Client with two-tier degradation:
//
//	Tier 1  R2 presigned URL — client uploads directly to Cloudflare
//	Tier 2  Local temp path  — client uploads to the monolith's /tmp dir
//	        (size-limited; only for small uploads during R2 outages)
//	Tier 3  Hard 503         — never silently discard user data
//
// R2 is the only dependency in this app that does NOT fall back to a log file
// because we cannot afford to lose uploaded bytes.  If both R2 and local temp
// are unavailable we return an unambiguous error to the client immediately.
//
// The local temp directory is capped at cfg.Resilience.R2TempMaxBytes.
// Uploads that would exceed the cap are rejected with a clean 503.
type ResilientR2Client struct {
	primary      *r2pkg.Client
	breaker      *resilience.InProcessBreaker
	tempDir      string
	tempMaxBytes int64
	tempUsed     atomic.Int64 // approximate bytes currently in tempDir
	baseURL      string       // URL the monolith serves temp files from
}

// NewResilientR2Client builds the wrapper.
//
//   - primary:     the real R2 client (may be nil if R2 is not configured)
//   - tempDir:     local temp upload directory (from config)
//   - tempMaxBytes: cap on total bytes in tempDir
//   - baseURL:     public URL the monolith uses to serve temp uploads,
//     e.g. "http://api.viewaura.com/internal/temp-uploads"
func NewResilientR2Client(
	primary *r2pkg.Client,
	tempDir string,
	tempMaxBytes int64,
	baseURL string,
) *ResilientR2Client {
	if tempDir == "" {
		tempDir = "/tmp/viewaura-uploads"
	}
	if tempMaxBytes == 0 {
		tempMaxBytes = 500 * 1024 * 1024 // 500 MB default
	}
	c := &ResilientR2Client{
		primary:      primary,
		breaker:      resilience.NewInProcessBreaker("r2", 3, 60*time.Second),
		tempDir:      tempDir,
		tempMaxBytes: tempMaxBytes,
		baseURL:      baseURL,
	}
	// Approximate current disk usage at startup — non-fatal if it fails.
	c.tempUsed.Store(c.measureTempDir())
	return c
}

// PresignPutObject returns a presigned upload URL.
// On R2 failure it falls back to a local temp path URL.
// On both failures it returns apierror.ServiceUnavailable.
func (c *ResilientR2Client) PresignPutObject(ctx context.Context, key, contentType string) (string, error) {
	// ── Tier 1: R2 presigned URL ──────────────────────────────────────────────
	if c.primary != nil && c.breaker.Allow() {
		url, err := c.primary.PresignPutObject(ctx, key, contentType)
		if err == nil {
			c.breaker.RecordSuccess()
			return url, nil
		}
		c.breaker.RecordFailure()
		log.Warn().Err(err).Str("key", key).
			Msg("resilient_r2: presign failed — falling back to local temp path")
	} else if c.breaker.IsOpen() {
		log.Warn().Str("key", key).
			Msg("resilient_r2: R2 circuit open — falling back to local temp path")
	}

	// ── Tier 2: local temp path ───────────────────────────────────────────────
	tempURL, err := c.localTempURL(key, contentType)
	if err == nil {
		log.Warn().Str("key", key).
			Msg("resilient_r2: using local temp upload path (R2 unavailable)")
		return tempURL, nil
	}

	// ── Tier 3: hard error — never silently lose uploads ─────────────────────
	log.Error().Err(err).Str("key", key).
		Msg("resilient_r2: all upload paths unavailable — returning 503")

	return "", apierror.New(503, "STORAGE_UNAVAILABLE",
		"Upload storage is temporarily unavailable. Please try again in a few minutes.")
}

// PresignGetObject delegates to the real client with no fallback —
// a failed read presign is not a data-loss scenario.
func (c *ResilientR2Client) PresignGetObject(ctx context.Context, key string) (string, error) {
	if c.primary == nil {
		return "", apierror.New(503, "STORAGE_UNAVAILABLE", "Object storage is not configured.")
	}
	if c.breaker.Allow() {
		url, err := c.primary.PresignGetObject(ctx, key)
		if err == nil {
			c.breaker.RecordSuccess()
			return url, nil
		}
		c.breaker.RecordFailure()
	}
	return "", apierror.New(503, "STORAGE_UNAVAILABLE",
		"Download storage is temporarily unavailable. Please try again shortly.")
}

// DeleteObject delegates to the real client; non-fatal on failure — the
// object will be cleaned up by the nightly orphan-sweep job.
func (c *ResilientR2Client) DeleteObject(ctx context.Context, key string) error {
	if c.primary == nil {
		return nil
	}
	if c.breaker.Allow() {
		err := c.primary.DeleteObject(ctx, key)
		if err == nil {
			c.breaker.RecordSuccess()
			return nil
		}
		c.breaker.RecordFailure()
		log.Warn().Err(err).Str("key", key).
			Msg("resilient_r2: DeleteObject failed — object may be orphaned")
	}
	// Non-fatal — the nightly sweep will clean up.
	return nil
}

// CopyObject delegates to the real client with no silent fallback.
func (c *ResilientR2Client) CopyObject(ctx context.Context, srcKey, dstKey string) error {
	if c.primary == nil {
		return apierror.New(503, "STORAGE_UNAVAILABLE", "Object storage is not configured.")
	}
	if c.breaker.Allow() {
		err := c.primary.CopyObject(ctx, srcKey, dstKey)
		if err == nil {
			c.breaker.RecordSuccess()
			return nil
		}
		c.breaker.RecordFailure()
	}
	return apierror.New(503, "STORAGE_UNAVAILABLE",
		"Storage copy operation temporarily unavailable.")
}

// ─── Local temp path helpers ──────────────────────────────────────────────────

// localTempURL creates a temp directory slot and returns the monolith-served
// URL the client should POST to.  Returns an error when the cap is exceeded.
func (c *ResilientR2Client) localTempURL(key, _ string) (string, error) {
	if c.baseURL == "" {
		return "", fmt.Errorf("resilient_r2: temp base URL not configured")
	}

	// Rough cap check — not byte-accurate but prevents runaway disk usage.
	// The exact file size is unknown until after the upload.
	if c.tempUsed.Load() >= c.tempMaxBytes {
		return "", fmt.Errorf("resilient_r2: temp upload directory full (%d bytes used)", c.tempUsed.Load())
	}

	// Create the subdirectory structure under tempDir.
	safePath := filepath.Join(c.tempDir, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(safePath), 0o750); err != nil {
		return "", fmt.Errorf("resilient_r2: mkdir temp: %w", err)
	}

	// The URL is the monolith's internal upload endpoint which accepts a PUT
	// with the same key and writes to the local path.
	// Format: {baseURL}/{key}
	return fmt.Sprintf("%s/%s", c.baseURL, key), nil
}

// measureTempDir returns an approximate total size of files in tempDir.
// Non-fatal — returns 0 on any error.
func (c *ResilientR2Client) measureTempDir() int64 {
	var total int64
	_ = filepath.Walk(c.tempDir, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total
}

// RecordTempUploadSize is called by the temp upload handler after a file lands
// to keep the approximate usage counter current.
func (c *ResilientR2Client) RecordTempUploadSize(bytes int64) {
	c.tempUsed.Add(bytes)
}
