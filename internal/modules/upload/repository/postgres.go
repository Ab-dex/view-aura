package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Ab-dex/view-aura/internal/modules/upload/domain"
	apierror "github.com/Ab-dex/view-aura/internal/platform/error"
)

type pgMediaAssetRepository struct{ pool *pgxpool.Pool }

func NewMediaAssetRepository(pool *pgxpool.Pool) MediaAssetRepository {
	return &pgMediaAssetRepository{pool: pool}
}

const assetCols = `id, upload_id, user_id, movie_id, storage_key,
	content_type, size_bytes, status, source_info, created_at, updated_at`

func (r *pgMediaAssetRepository) Create(ctx context.Context, a *domain.MediaAsset) (*domain.MediaAsset, error) {
	src, _ := json.Marshal(a.SourceInfo)
	const q = `
		INSERT INTO media_assets (id, upload_id, user_id, movie_id, storage_key,
			content_type, size_bytes, status, source_info, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,NOW(),NOW())
		RETURNING ` + assetCols
	return scanAsset(r.pool.QueryRow(ctx, q,
		a.ID, a.UploadID, a.UserID, nilIfEmpty(a.MovieID), a.StorageKey,
		a.ContentType, a.SizeBytes, a.Status, src,
	))
}

func (r *pgMediaAssetRepository) GetByID(ctx context.Context, id string) (*domain.MediaAsset, error) {
	a, err := scanAsset(r.pool.QueryRow(ctx, `SELECT `+assetCols+` FROM media_assets WHERE id=$1`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apierror.NotFound("media asset not found")
		}
		return nil, fmt.Errorf("upload: get asset: %w", err)
	}
	return a, nil
}

func (r *pgMediaAssetRepository) GetByUploadID(ctx context.Context, uploadID string) (*domain.MediaAsset, error) {
	a, err := scanAsset(r.pool.QueryRow(ctx,
		`SELECT `+assetCols+` FROM media_assets WHERE upload_id=$1 ORDER BY created_at DESC LIMIT 1`,
		uploadID,
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("upload: get asset by upload_id: %w", err)
	}
	return a, nil
}

func (r *pgMediaAssetRepository) UpdateStatus(ctx context.Context, id, status string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE media_assets SET status=$2, updated_at=NOW() WHERE id=$1`,
		id, status,
	)
	return err
}

func (r *pgMediaAssetRepository) LinkMovie(ctx context.Context, assetID, movieID string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE media_assets SET movie_id=$2, updated_at=NOW() WHERE id=$1`,
		assetID, movieID,
	)
	return err
}

func (r *pgMediaAssetRepository) ListByUser(ctx context.Context, userID string, limit, offset int) ([]*domain.MediaAsset, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM media_assets WHERE user_id=$1`, userID,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("upload: count assets: %w", err)
	}

	rows, err := r.pool.Query(ctx,
		`SELECT `+assetCols+` FROM media_assets WHERE user_id=$1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		userID, limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("upload: list assets: %w", err)
	}
	defer rows.Close()

	var out []*domain.MediaAsset
	for rows.Next() {
		a, err := scanAsset(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("upload: scan asset: %w", err)
		}
		out = append(out, a)
	}
	return out, total, rows.Err()
}

func scanAsset(row interface{ Scan(...any) error }) (*domain.MediaAsset, error) {
	var a domain.MediaAsset
	var rawSrc []byte
	err := row.Scan(
		&a.ID, &a.UploadID, &a.UserID, &a.MovieID, &a.StorageKey,
		&a.ContentType, &a.SizeBytes, &a.Status, &rawSrc,
		&a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if len(rawSrc) > 0 {
		_ = json.Unmarshal(rawSrc, &a.SourceInfo)
	}
	return &a, nil
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
