package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Ab-dex/view-aura/internal/modules/rating/domain"
	apierror "github.com/Ab-dex/view-aura/internal/platform/error"
)

type pgRatingRepository struct{ pool *pgxpool.Pool }

func NewRatingRepository(pool *pgxpool.Pool) RatingRepository {
	return &pgRatingRepository{pool: pool}
}

var _ RatingRepository = (*pgRatingRepository)(nil)

const ratingCols = `id, movie_id, user_id,
	overall, acting, direction, writing, cinematography, soundtrack,
	reaction, is_verified, created_at, updated_at`

func (r *pgRatingRepository) Upsert(ctx context.Context, rt *domain.Rating) (*domain.Rating, error) {
	const q = `
		INSERT INTO ratings (id, movie_id, user_id,
			overall, acting, direction, writing, cinematography, soundtrack,
			reaction, is_verified, created_at, updated_at)
		VALUES ($1,$2,$3, $4,$5,$6,$7,$8,$9, $10,$11, NOW(),NOW())
		ON CONFLICT (user_id, movie_id) DO UPDATE SET
			overall         = EXCLUDED.overall,
			acting          = EXCLUDED.acting,
			direction       = EXCLUDED.direction,
			writing         = EXCLUDED.writing,
			cinematography  = EXCLUDED.cinematography,
			soundtrack      = EXCLUDED.soundtrack,
			reaction        = EXCLUDED.reaction,
			updated_at      = NOW()
		RETURNING ` + ratingCols

	row := r.pool.QueryRow(ctx, q,
		rt.ID, rt.MovieID, rt.UserID,
		rt.Overall, rt.Acting, rt.Direction, rt.Writing, rt.Cinematography, rt.Soundtrack,
		rt.Reaction, rt.IsVerified,
	)
	return scanRating(row)
}

func (r *pgRatingRepository) GetByUserAndMovie(ctx context.Context, userID, movieID string) (*domain.Rating, error) {
	const q = `SELECT ` + ratingCols + ` FROM ratings WHERE user_id=$1 AND movie_id=$2`
	rt, err := scanRating(r.pool.QueryRow(ctx, q, userID, movieID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apierror.New(404, apierror.CodeNotFound, "rating not found")
		}
		return nil, mapErr(err, "get rating")
	}
	return rt, nil
}

func (r *pgRatingRepository) Delete(ctx context.Context, userID, movieID string) error {
	const q = `DELETE FROM ratings WHERE user_id=$1 AND movie_id=$2`
	ct, err := r.pool.Exec(ctx, q, userID, movieID)
	if err != nil {
		return mapErr(err, "delete rating")
	}
	if ct.RowsAffected() == 0 {
		return apierror.New(404, apierror.CodeNotFound, "rating not found")
	}
	return nil
}

func (r *pgRatingRepository) ListByMovie(ctx context.Context, f domain.RatingFilter) ([]*domain.Rating, int, error) {
	return r.list(ctx, "movie_id", f.MovieID, f)
}

func (r *pgRatingRepository) ListByUser(ctx context.Context, f domain.RatingFilter) ([]*domain.Rating, int, error) {
	return r.list(ctx, "user_id", f.UserID, f)
}

func (r *pgRatingRepository) list(ctx context.Context, col, val string, f domain.RatingFilter) ([]*domain.Rating, int, error) {
	limit := f.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}

	countQ := fmt.Sprintf(`SELECT COUNT(*) FROM ratings WHERE %s=$1`, col)
	var total int
	if err := r.pool.QueryRow(ctx, countQ, val).Scan(&total); err != nil {
		return nil, 0, mapErr(err, "count ratings")
	}

	listQ := fmt.Sprintf(`SELECT `+ratingCols+` FROM ratings WHERE %s=$1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`, col)
	rows, err := r.pool.Query(ctx, listQ, val, limit, offset)
	if err != nil {
		return nil, 0, mapErr(err, "list ratings")
	}
	defer rows.Close()

	var out []*domain.Rating
	for rows.Next() {
		rt, err := scanRating(rows)
		if err != nil {
			return nil, 0, mapErr(err, "scan rating")
		}
		out = append(out, rt)
	}
	return out, total, rows.Err()
}

func (r *pgRatingRepository) GetStats(ctx context.Context, movieID string) (*domain.MovieRatingStats, error) {
	const q = `
		SELECT
			COUNT(*)                    AS total,
			AVG(overall)                AS avg_overall,
			AVG(acting)                 AS avg_acting,
			AVG(direction)              AS avg_direction,
			AVG(writing)                AS avg_writing,
			AVG(cinematography)         AS avg_cinematography,
			AVG(soundtrack)             AS avg_soundtrack
		FROM ratings WHERE movie_id = $1`

	stats := &domain.MovieRatingStats{MovieID: movieID}
	err := r.pool.QueryRow(ctx, q, movieID).Scan(
		&stats.TotalRatings,
		&stats.AvgOverall,
		&stats.AvgActing,
		&stats.AvgDirection,
		&stats.AvgWriting,
		&stats.AvgCinematography,
		&stats.AvgSoundtrack,
	)
	if err != nil {
		return nil, mapErr(err, "get rating stats")
	}

	// Distribution: count per integer bucket 1–5.
	const distQ = `
		SELECT ROUND(overall)::int AS bucket, COUNT(*) AS cnt
		FROM ratings WHERE movie_id=$1
		GROUP BY bucket ORDER BY bucket`

	rows, err := r.pool.Query(ctx, distQ, movieID)
	if err != nil {
		return nil, mapErr(err, "get rating distribution")
	}
	defer rows.Close()

	stats.Distribution = make(map[int]int, 5)
	for rows.Next() {
		var bucket, cnt int
		if err := rows.Scan(&bucket, &cnt); err != nil {
			return nil, mapErr(err, "scan distribution")
		}
		stats.Distribution[bucket] = cnt
	}
	return stats, rows.Err()
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

type rowScanner interface{ Scan(dest ...any) error }

func scanRating(row rowScanner) (*domain.Rating, error) {
	var rt domain.Rating
	err := row.Scan(
		&rt.ID, &rt.MovieID, &rt.UserID,
		&rt.Overall, &rt.Acting, &rt.Direction, &rt.Writing, &rt.Cinematography, &rt.Soundtrack,
		&rt.Reaction, &rt.IsVerified, &rt.CreatedAt, &rt.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &rt, nil
}

func mapErr(err error, op string) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return apierror.Conflict(fmt.Sprintf("%s: %s", op, pgErr.Detail)).WithCause(err)
	}
	return apierror.DatabaseError(fmt.Errorf("%s: %w", op, err))
}
