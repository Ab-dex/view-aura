package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Ab-dex/view-aura/internal/modules/profile/domain"
	apierror "github.com/Ab-dex/view-aura/internal/platform/error"
)

const pgUniqueViolation = "23505"

// ─── Error mapping ────────────────────────────────────────────────────────────

func mapPgError(err error, op string) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
		return apierror.Conflict(fmt.Sprintf("%s: duplicate record", op)).WithCause(err)
	}
	return apierror.DatabaseError(fmt.Errorf("%s: %w", op, err))
}

type pgHouseholdProfileRepository struct {
	pool *pgxpool.Pool
}

func NewHouseholdProfileRepository(pool *pgxpool.Pool) HouseholdProfileRepository {
	return &pgHouseholdProfileRepository{pool: pool}
}

var _ HouseholdProfileRepository = (*pgHouseholdProfileRepository)(nil)

// ─── Create ───────────────────────────────────────────────────────────────────

func (r *pgHouseholdProfileRepository) Create(ctx context.Context, p *domain.Profile) (*domain.Profile, error) {
	// Enforce household cap in the DB layer (race-safe).
	const q = `
		INSERT INTO household_profiles
			(id, user_id, name, avatar_url, type, pin_hash, max_rating,
			 preferred_languages, is_default, sort_order)
		SELECT $1,$2,$3,$4,$5,$6,$7,$8,$9,$10
		WHERE (SELECT COUNT(*) FROM household_profiles WHERE user_id = $2) < $11
		RETURNING id, user_id, name, avatar_url, type, pin_hash, max_rating,
		          preferred_languages, is_default, sort_order, created_at, updated_at`

	var out domain.Profile
	err := r.pool.QueryRow(ctx, q,
		p.ID, p.UserID, p.Name, p.AvatarURL, p.Type, p.PINHash, p.MaxRating,
		p.PreferredLanguages, p.IsDefault, p.SortOrder,
		domain.MaxProfilesPerUser,
	).Scan(
		&out.ID, &out.UserID, &out.Name, &out.AvatarURL, &out.Type,
		&out.PINHash, &out.MaxRating, &out.PreferredLanguages,
		&out.IsDefault, &out.SortOrder, &out.CreatedAt, &out.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apierror.New(409, "PROFILE_LIMIT_REACHED",
				"maximum number of profiles reached for this account")
		}
		return nil, mapPgError(err, "create household profile")
	}
	return &out, nil
}

// ─── GetByID ──────────────────────────────────────────────────────────────────

func (r *pgHouseholdProfileRepository) GetByID(ctx context.Context, id domain.ProfileID) (*domain.Profile, error) {
	const q = `
		SELECT id, user_id, name, avatar_url, type, pin_hash, max_rating,
		       preferred_languages, is_default, sort_order, created_at, updated_at
		FROM household_profiles WHERE id = $1`

	var out domain.Profile
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&out.ID, &out.UserID, &out.Name, &out.AvatarURL, &out.Type,
		&out.PINHash, &out.MaxRating, &out.PreferredLanguages,
		&out.IsDefault, &out.SortOrder, &out.CreatedAt, &out.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apierror.ErrProfileNotFound
		}
		return nil, mapPgError(err, "get household profile")
	}
	return &out, nil
}

// ─── ListByUser ───────────────────────────────────────────────────────────────

func (r *pgHouseholdProfileRepository) ListByUser(ctx context.Context, userID domain.UserID) ([]*domain.Profile, error) {
	const q = `
		SELECT id, user_id, name, avatar_url, type, pin_hash, max_rating,
		       preferred_languages, is_default, sort_order, created_at, updated_at
		FROM household_profiles
		WHERE user_id = $1
		ORDER BY sort_order ASC, created_at ASC`

	rows, err := r.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, mapPgError(err, "list household profiles")
	}
	defer rows.Close()

	var out []*domain.Profile
	for rows.Next() {
		var p domain.Profile
		if err := rows.Scan(
			&p.ID, &p.UserID, &p.Name, &p.AvatarURL, &p.Type,
			&p.PINHash, &p.MaxRating, &p.PreferredLanguages,
			&p.IsDefault, &p.SortOrder, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, mapPgError(err, "scan household profile")
		}
		out = append(out, &p)
	}
	return out, rows.Err()
}

// ─── Update ───────────────────────────────────────────────────────────────────

func (r *pgHouseholdProfileRepository) Update(ctx context.Context, p *domain.Profile) (*domain.Profile, error) {
	const q = `
		UPDATE household_profiles
		SET name = $2, avatar_url = $3, pin_hash = $4, max_rating = $5,
		    preferred_languages = $6, sort_order = $7, updated_at = NOW()
		WHERE id = $1 AND user_id = $8
		RETURNING id, user_id, name, avatar_url, type, pin_hash, max_rating,
		          preferred_languages, is_default, sort_order, created_at, updated_at`

	var out domain.Profile
	err := r.pool.QueryRow(ctx, q,
		p.ID, p.Name, p.AvatarURL, p.PINHash, p.MaxRating,
		p.PreferredLanguages, p.SortOrder, p.UserID,
	).Scan(
		&out.ID, &out.UserID, &out.Name, &out.AvatarURL, &out.Type,
		&out.PINHash, &out.MaxRating, &out.PreferredLanguages,
		&out.IsDefault, &out.SortOrder, &out.CreatedAt, &out.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apierror.ErrProfileNotFound
		}
		return nil, mapPgError(err, "update household profile")
	}
	return &out, nil
}

// ─── SetDefault ───────────────────────────────────────────────────────────────

func (r *pgHouseholdProfileRepository) SetDefault(
	ctx context.Context, userID domain.UserID, profileID domain.ProfileID,
) error {
	// Two-step update in a transaction: clear all, then set one.
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return mapPgError(err, "begin set default")
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx,
		`UPDATE household_profiles SET is_default = FALSE WHERE user_id = $1`, userID,
	); err != nil {
		return mapPgError(err, "clear defaults")
	}

	ct, err := tx.Exec(ctx,
		`UPDATE household_profiles SET is_default = TRUE WHERE id = $1 AND user_id = $2`,
		profileID, userID,
	)
	if err != nil {
		return mapPgError(err, "set default")
	}
	if ct.RowsAffected() == 0 {
		return apierror.ErrProfileNotFound
	}

	return tx.Commit(ctx)
}

// ─── Delete ───────────────────────────────────────────────────────────────────

func (r *pgHouseholdProfileRepository) Delete(
	ctx context.Context, userID domain.UserID, profileID domain.ProfileID,
) error {
	ct, err := r.pool.Exec(ctx,
		`DELETE FROM household_profiles WHERE id = $1 AND user_id = $2`,
		profileID, userID,
	)
	if err != nil {
		return mapPgError(err, "delete household profile")
	}
	if ct.RowsAffected() == 0 {
		return apierror.ErrProfileNotFound
	}
	return nil
}

// ─── CountByUser ──────────────────────────────────────────────────────────────

func (r *pgHouseholdProfileRepository) CountByUser(ctx context.Context, userID domain.UserID) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM household_profiles WHERE user_id = $1`, userID,
	).Scan(&n)
	if err != nil {
		return 0, mapPgError(err, "count household profiles")
	}
	return n, nil
}
