package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Ab-dex/view-aura/internal/modules/moderation/domain"
	apierror "github.com/Ab-dex/view-aura/internal/platform/error"
)

// ─── Case repository ──────────────────────────────────────────────────────────

type pgCaseRepository struct{ pool *pgxpool.Pool }

func NewCaseRepository(pool *pgxpool.Pool) CaseRepository {
	return &pgCaseRepository{pool: pool}
}

const caseCols = `id, content_type, content_id, owner_id, submitted_by,
	trigger_reason, status, ai_signals, ai_confidence, ai_recommendation,
	decision, decided_by, reason, decided_at,
	locked_by, locked_until, created_at, updated_at`

func (r *pgCaseRepository) Create(ctx context.Context, c *domain.ModerationCase) (*domain.ModerationCase, error) {
	const q = `
		INSERT INTO moderation_cases (id, content_type, content_id, owner_id, submitted_by,
			trigger_reason, status, ai_signals, ai_confidence, ai_recommendation,
			decision, decided_by, reason, decided_at,
			locked_by, locked_until, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,NOW(),NOW())
		RETURNING ` + caseCols
	return scanCase(r.pool.QueryRow(ctx, q,
		c.ID, c.ContentType, c.ContentID, c.OwnerID, c.SubmittedBy,
		c.TriggerReason, c.Status, c.AISignals, c.AIConfidence, c.AIRecommendation,
		c.Decision, c.DecidedBy, c.Reason, c.DecidedAt,
		c.LockedBy, c.LockedUntil,
	))
}

func (r *pgCaseRepository) GetByID(ctx context.Context, id string) (*domain.ModerationCase, error) {
	c, err := scanCase(r.pool.QueryRow(ctx, `SELECT `+caseCols+` FROM moderation_cases WHERE id=$1`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apierror.ErrModerationCaseNotFound
		}
		return nil, fmt.Errorf("moderation: get case: %w", err)
	}
	return c, nil
}

func (r *pgCaseRepository) GetByContentID(ctx context.Context, contentType domain.ContentType, contentID string) (*domain.ModerationCase, error) {
	const q = `SELECT ` + caseCols + ` FROM moderation_cases
		WHERE content_type=$1 AND content_id=$2 AND status != 'decided'
		ORDER BY created_at DESC LIMIT 1`
	c, err := scanCase(r.pool.QueryRow(ctx, q, contentType, contentID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("moderation: get case by content: %w", err)
	}
	return c, nil
}

func (r *pgCaseRepository) List(ctx context.Context, f domain.CaseFilter) ([]*domain.ModerationCase, int, error) {
	var conds []string
	var args []any
	i := 1
	arg := func(v any) string {
		args = append(args, v)
		s := fmt.Sprintf("$%d", i)
		i++
		return s
	}

	if f.Status != nil {
		conds = append(conds, fmt.Sprintf("status = %s", arg(*f.Status)))
	}
	if f.ContentType != nil {
		conds = append(conds, fmt.Sprintf("content_type = %s", arg(*f.ContentType)))
	}

	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}

	var total int
	if err := r.pool.QueryRow(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM moderation_cases %s", where), args...,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("moderation: count cases: %w", err)
	}

	limit := 20
	if f.Limit > 0 {
		limit = f.Limit
	}
	rows, err := r.pool.Query(ctx,
		fmt.Sprintf("SELECT "+caseCols+" FROM moderation_cases %s ORDER BY created_at ASC LIMIT %s OFFSET %s",
			where, arg(limit), arg(f.Offset)),
		args...,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("moderation: list cases: %w", err)
	}
	defer rows.Close()

	var out []*domain.ModerationCase
	for rows.Next() {
		c, err := scanCase(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("moderation: scan case: %w", err)
		}
		out = append(out, c)
	}
	return out, total, rows.Err()
}

func (r *pgCaseRepository) Update(ctx context.Context, c *domain.ModerationCase) (*domain.ModerationCase, error) {
	const q = `
		UPDATE moderation_cases SET
			status=$2, ai_signals=$3, ai_confidence=$4, ai_recommendation=$5,
			decision=$6, decided_by=$7, reason=$8, decided_at=$9,
			locked_by=$10, locked_until=$11, updated_at=NOW()
		WHERE id=$1
		RETURNING ` + caseCols
	result, err := scanCase(r.pool.QueryRow(ctx, q,
		c.ID, c.Status, c.AISignals, c.AIConfidence, c.AIRecommendation,
		c.Decision, c.DecidedBy, c.Reason, c.DecidedAt,
		c.LockedBy, c.LockedUntil,
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apierror.ErrModerationCaseNotFound
		}
		return nil, fmt.Errorf("moderation: update case: %w", err)
	}
	return result, nil
}

func scanCase(row interface{ Scan(...any) error }) (*domain.ModerationCase, error) {
	var c domain.ModerationCase
	err := row.Scan(
		&c.ID, &c.ContentType, &c.ContentID, &c.OwnerID, &c.SubmittedBy,
		&c.TriggerReason, &c.Status, &c.AISignals, &c.AIConfidence, &c.AIRecommendation,
		&c.Decision, &c.DecidedBy, &c.Reason, &c.DecidedAt,
		&c.LockedBy, &c.LockedUntil, &c.CreatedAt, &c.UpdatedAt,
	)
	return &c, err
}

// ─── Appeal repository ────────────────────────────────────────────────────────

type pgAppealRepository struct{ pool *pgxpool.Pool }

func NewAppealRepository(pool *pgxpool.Pool) AppealRepository {
	return &pgAppealRepository{pool: pool}
}

const appealCols = `id, case_id, appellant_id, statement, outcome, decided_by, decided_at, created_at`

func (r *pgAppealRepository) Create(ctx context.Context, a *domain.Appeal) (*domain.Appeal, error) {
	const q = `
		INSERT INTO moderation_appeals (id, case_id, appellant_id, statement, outcome, decided_by, decided_at, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,NOW())
		RETURNING ` + appealCols
	return scanAppeal(r.pool.QueryRow(ctx, q,
		a.ID, a.CaseID, a.AppellantID, a.Statement, a.Outcome, a.DecidedBy, a.DecidedAt,
	))
}

func (r *pgAppealRepository) GetByID(ctx context.Context, id string) (*domain.Appeal, error) {
	a, err := scanAppeal(r.pool.QueryRow(ctx, `SELECT `+appealCols+` FROM moderation_appeals WHERE id=$1`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apierror.NotFound("appeal not found")
		}
		return nil, fmt.Errorf("moderation: get appeal: %w", err)
	}
	return a, nil
}

func (r *pgAppealRepository) GetByCaseID(ctx context.Context, caseID string) (*domain.Appeal, error) {
	a, err := scanAppeal(r.pool.QueryRow(ctx,
		`SELECT `+appealCols+` FROM moderation_appeals WHERE case_id=$1 ORDER BY created_at DESC LIMIT 1`,
		caseID,
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("moderation: get appeal by case: %w", err)
	}
	return a, nil
}

func (r *pgAppealRepository) Update(ctx context.Context, a *domain.Appeal) (*domain.Appeal, error) {
	const q = `
		UPDATE moderation_appeals SET outcome=$2, decided_by=$3, decided_at=$4
		WHERE id=$1
		RETURNING ` + appealCols
	result, err := scanAppeal(r.pool.QueryRow(ctx, q, a.ID, a.Outcome, a.DecidedBy, a.DecidedAt))
	if err != nil {
		return nil, fmt.Errorf("moderation: update appeal: %w", err)
	}
	return result, nil
}

func scanAppeal(row interface{ Scan(...any) error }) (*domain.Appeal, error) {
	var a domain.Appeal
	err := row.Scan(&a.ID, &a.CaseID, &a.AppellantID, &a.Statement,
		&a.Outcome, &a.DecidedBy, &a.DecidedAt, &a.CreatedAt)
	return &a, err
}
