package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/Ab-dex/view-aura/internal/modules/upload/domain"
	sharedcache "github.com/Ab-dex/view-aura/internal/platform/cache"
	apierror "github.com/Ab-dex/view-aura/internal/platform/error"
)

const sessionTTL = 2 * time.Hour

type redisSessionRepository struct{ client *sharedcache.Client }

func NewUploadSessionRepository(client *sharedcache.Client) UploadSessionRepository {
	return &redisSessionRepository{client: client}
}

func sessionKey(id string) string { return "upload:" + id + ":session" }

func (r *redisSessionRepository) Create(ctx context.Context, s *domain.UploadSession) (*domain.UploadSession, error) {
	b, err := json.Marshal(s)
	if err != nil {
		return nil, apierror.Internal("upload: marshal session", err)
	}
	// SetNX prevents overwriting an existing session with the same ID.
	set, err := r.client.SetNX(ctx, sessionKey(s.ID), b, sessionTTL).Result()
	if err != nil {
		return nil, apierror.New(500, apierror.CodeCache, "upload: store session").WithCause(err)
	}
	if !set {
		return nil, apierror.Conflict("upload session already exists")
	}
	return s, nil
}

func (r *redisSessionRepository) GetByID(ctx context.Context, id string) (*domain.UploadSession, error) {
	b, err := r.client.Get(ctx, sessionKey(id)).Bytes()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return nil, apierror.ErrUploadNotFound
		}
		return nil, apierror.New(500, apierror.CodeCache, "upload: get session").WithCause(err)
	}
	var s domain.UploadSession
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, apierror.Internal("upload: unmarshal session", err)
	}
	return &s, nil
}

func (r *redisSessionRepository) UpdateStatus(ctx context.Context, id string, status domain.UploadStatus) error {
	s, err := r.GetByID(ctx, id)
	if err != nil {
		return err
	}
	s.Status = status
	s.UpdatedAt = time.Now()

	b, err := json.Marshal(s)
	if err != nil {
		return apierror.Internal("upload: marshal session update", err)
	}
	// Preserve the remaining TTL — do not reset to full sessionTTL on every update.
	ttl, err := r.client.TTL(ctx, sessionKey(id)).Result()
	if err != nil || ttl <= 0 {
		ttl = sessionTTL
	}
	return r.client.Set(ctx, sessionKey(id), b, ttl).Err()
}

func (r *redisSessionRepository) Delete(ctx context.Context, id string) error {
	return fmt.Errorf("%w", r.client.Del(ctx, sessionKey(id)).Err())
}
