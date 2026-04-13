package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Ab-dex/view-aura/internal/modules/review/domain"
	apierror "github.com/Ab-dex/view-aura/internal/platform/error"
)

type rowScanner interface{ Scan(dest ...any) error }

// ─── Review repository ────────────────────────────────────────────────────────

type pgReviewRepository struct{ pool *pgxpool.Pool }

func NewReviewRepository(pool *pgxpool.Pool) ReviewRepository {
	return &pgReviewRepository{pool: pool}
}

var _ ReviewRepository = (*pgReviewRepository)(nil)

const reviewCols = `id, movie_id, user_id, type, body, video_url,
	is_spoiler, is_published,
	like_count, helpful_count, insightful_count, funny_count,
	credibility_score, created_at, updated_at`

func (r *pgReviewRepository) Create(ctx context.Context, rev *domain.Review) (*domain.Review, error) {
	const q = `
		INSERT INTO reviews (id, movie_id, user_id, type, body, video_url,
			is_spoiler, is_published,
			like_count, helpful_count, insightful_count, funny_count,
			credibility_score, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8, 0,0,0,0, 0,NOW(),NOW())
		RETURNING ` + reviewCols

	row := r.pool.QueryRow(ctx, q,
		rev.ID, rev.MovieID, rev.UserID, rev.Type, rev.Body, rev.VideoURL,
		rev.IsSpoiler, rev.IsPublished,
	)
	return scanReview(row)
}

func (r *pgReviewRepository) GetByID(ctx context.Context, id domain.ReviewID) (*domain.Review, error) {
	const q = `SELECT ` + reviewCols + ` FROM reviews WHERE id=$1`
	rev, err := scanReview(r.pool.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apierror.New(404, apierror.CodeNotFound, "review not found")
		}
		return nil, mapErr(err, "get review")
	}
	return rev, nil
}

func (r *pgReviewRepository) Update(ctx context.Context, rev *domain.Review) (*domain.Review, error) {
	const q = `
		UPDATE reviews SET body=$2, video_url=$3, is_spoiler=$4, is_published=$5, updated_at=NOW()
		WHERE id=$1 AND is_published=false
		RETURNING ` + reviewCols

	result, err := scanReview(r.pool.QueryRow(ctx, q,
		rev.ID, rev.Body, rev.VideoURL, rev.IsSpoiler, rev.IsPublished,
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apierror.New(404, apierror.CodeNotFound, "review not found or already published")
		}
		return nil, mapErr(err, "update review")
	}
	return result, nil
}

func (r *pgReviewRepository) Delete(ctx context.Context, id domain.ReviewID, userID string) error {
	const q = `DELETE FROM reviews WHERE id=$1 AND user_id=$2`
	ct, err := r.pool.Exec(ctx, q, id, userID)
	if err != nil {
		return mapErr(err, "delete review")
	}
	if ct.RowsAffected() == 0 {
		return apierror.New(404, apierror.CodeNotFound, "review not found")
	}
	return nil
}

func (r *pgReviewRepository) List(ctx context.Context, f domain.ReviewFilter) ([]*domain.Review, int, error) {
	var conds []string
	var args []any
	i := 1
	arg := func(v any) string {
		args = append(args, v)
		s := fmt.Sprintf("$%d", i)
		i++
		return s
	}

	conds = append(conds, "is_published = true")

	if f.MovieID != "" {
		conds = append(conds, fmt.Sprintf("movie_id = %s", arg(f.MovieID)))
	}
	if f.UserID != "" {
		conds = append(conds, fmt.Sprintf("user_id = %s", arg(f.UserID)))
	}
	if f.Type != nil {
		conds = append(conds, fmt.Sprintf("type = %s", arg(*f.Type)))
	}
	if !f.IncludeSpoilers {
		conds = append(conds, "is_spoiler = false")
	}

	where := "WHERE " + strings.Join(conds, " AND ")

	sortCol := "created_at"
	switch f.SortBy {
	case "helpful_count":
		sortCol = "helpful_count"
	case "credibility_score":
		sortCol = "credibility_score"
	}

	limit := f.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}

	var total int
	if err := r.pool.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM reviews %s", where), args...).Scan(&total); err != nil {
		return nil, 0, mapErr(err, "count reviews")
	}

	listQ := fmt.Sprintf(`SELECT `+reviewCols+` FROM reviews %s ORDER BY %s DESC LIMIT %s OFFSET %s`,
		where, sortCol, arg(limit), arg(offset))
	rows, err := r.pool.Query(ctx, listQ, args...)
	if err != nil {
		return nil, 0, mapErr(err, "list reviews")
	}
	defer rows.Close()

	var out []*domain.Review
	for rows.Next() {
		rev, err := scanReview(rows)
		if err != nil {
			return nil, 0, mapErr(err, "scan review")
		}
		out = append(out, rev)
	}
	return out, total, rows.Err()
}

func (r *pgReviewRepository) GetByUserAndMovie(ctx context.Context, userID, movieID string) (*domain.Review, error) {
	const q = `SELECT ` + reviewCols + ` FROM reviews WHERE user_id=$1 AND movie_id=$2 LIMIT 1`
	rev, err := scanReview(r.pool.QueryRow(ctx, q, userID, movieID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, mapErr(err, "get review by user and movie")
	}
	return rev, nil
}

func scanReview(row rowScanner) (*domain.Review, error) {
	var rev domain.Review
	err := row.Scan(
		&rev.ID, &rev.MovieID, &rev.UserID, &rev.Type, &rev.Body, &rev.VideoURL,
		&rev.IsSpoiler, &rev.IsPublished,
		&rev.LikeCount, &rev.HelpfulCount, &rev.InsightfulCount, &rev.FunnyCount,
		&rev.CredibilityScore, &rev.CreatedAt, &rev.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &rev, nil
}

// ─── Reaction repository ──────────────────────────────────────────────────────

type pgReviewReactionRepository struct{ pool *pgxpool.Pool }

func NewReviewReactionRepository(pool *pgxpool.Pool) ReviewReactionRepository {
	return &pgReviewReactionRepository{pool: pool}
}

var _ ReviewReactionRepository = (*pgReviewReactionRepository)(nil)

func (r *pgReviewReactionRepository) Upsert(ctx context.Context, rx *domain.ReviewReaction) (*domain.ReviewReaction, error) {
	const q = `
		INSERT INTO review_reactions (id, review_id, user_id, type, created_at)
		VALUES ($1,$2,$3,$4,NOW())
		ON CONFLICT (review_id, user_id) DO UPDATE SET type=EXCLUDED.type
		RETURNING id, review_id, user_id, type, created_at`

	var out domain.ReviewReaction
	err := r.pool.QueryRow(ctx, q, rx.ID, rx.ReviewID, rx.UserID, rx.Type).
		Scan(&out.ID, &out.ReviewID, &out.UserID, &out.Type, &out.CreatedAt)
	if err != nil {
		return nil, mapErr(err, "upsert review reaction")
	}
	return &out, nil
}

func (r *pgReviewReactionRepository) Delete(ctx context.Context, reviewID domain.ReviewID, userID string) error {
	const q = `DELETE FROM review_reactions WHERE review_id=$1 AND user_id=$2`
	_, err := r.pool.Exec(ctx, q, reviewID, userID)
	return mapErr(err, "delete review reaction")
}

func (r *pgReviewReactionRepository) GetByUserAndReview(ctx context.Context, userID string, reviewID domain.ReviewID) (*domain.ReviewReaction, error) {
	const q = `SELECT id, review_id, user_id, type, created_at FROM review_reactions WHERE user_id=$1 AND review_id=$2`
	var rx domain.ReviewReaction
	err := r.pool.QueryRow(ctx, q, userID, reviewID).
		Scan(&rx.ID, &rx.ReviewID, &rx.UserID, &rx.Type, &rx.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, mapErr(err, "get review reaction")
	}
	return &rx, nil
}

func (r *pgReviewReactionRepository) IncrementCounter(ctx context.Context, reviewID domain.ReviewID, t domain.ReviewReactionType, delta int) error {
	// Map reaction type to its column — static mapping, no dynamic SQL injection.
	col := map[domain.ReviewReactionType]string{
		domain.ReviewReactionLike:       "like_count",
		domain.ReviewReactionHelpful:    "helpful_count",
		domain.ReviewReactionInsightful: "insightful_count",
		domain.ReviewReactionFunny:      "funny_count",
	}[t]
	if col == "" {
		return apierror.Validation("unknown reaction type", nil)
	}
	// Column name comes from a static map — safe to interpolate.
	q := fmt.Sprintf(`UPDATE reviews SET %s = GREATEST(0, %s + $2), updated_at=NOW() WHERE id=$1`, col, col)
	_, err := r.pool.Exec(ctx, q, reviewID, delta)
	return mapErr(err, "increment reaction counter")
}

// ─── Report repository ────────────────────────────────────────────────────────

type pgReviewReportRepository struct{ pool *pgxpool.Pool }

func NewReviewReportRepository(pool *pgxpool.Pool) ReviewReportRepository {
	return &pgReviewReportRepository{pool: pool}
}

var _ ReviewReportRepository = (*pgReviewReportRepository)(nil)

func (r *pgReviewReportRepository) Create(ctx context.Context, report *domain.ReviewReport) (*domain.ReviewReport, error) {
	const q = `
		INSERT INTO review_reports (id, review_id, user_id, reason, note, created_at)
		VALUES ($1,$2,$3,$4,$5,NOW())
		RETURNING id, review_id, user_id, reason, note, created_at`

	var out domain.ReviewReport
	err := r.pool.QueryRow(ctx, q, report.ID, report.ReviewID, report.UserID, report.Reason, report.Note).
		Scan(&out.ID, &out.ReviewID, &out.UserID, &out.Reason, &out.Note, &out.CreatedAt)
	if err != nil {
		return nil, mapErr(err, "create review report")
	}
	return &out, nil
}

func (r *pgReviewReportRepository) HasUserReported(ctx context.Context, userID string, reviewID domain.ReviewID) (bool, error) {
	const q = `SELECT EXISTS(SELECT 1 FROM review_reports WHERE user_id=$1 AND review_id=$2)`
	var exists bool
	if err := r.pool.QueryRow(ctx, q, userID, reviewID).Scan(&exists); err != nil {
		return false, mapErr(err, "has user reported")
	}
	return exists, nil
}

// ─── Shared error mapper ──────────────────────────────────────────────────────

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
