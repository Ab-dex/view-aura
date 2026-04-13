package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Ab-dex/view-aura/internal/modules/user/domain"
	apierror "github.com/Ab-dex/view-aura/internal/platform/error"
)

type pgAuthProviderRepository struct {
	pool *pgxpool.Pool
}

// NewAuthProviderRepository constructs a Postgres AuthProviderRepository.
func NewAuthProviderRepository(pool *pgxpool.Pool) AuthProviderRepository {
	return &pgAuthProviderRepository{pool: pool}
}

// Link inserts a new provider row, or updates the tokens + last_used_at if the
// (user_id, provider, provider_id) triple already exists.
func (r *pgAuthProviderRepository) Link(ctx context.Context, p *domain.LinkedAuthProvider) error {
	const q = `
		INSERT INTO user_auth_providers (
			id, user_id, provider, provider_id,
			access_token, refresh_token, linked_at, last_used_at
		) VALUES ($1, $2, $3, $4, $5, $6, NOW(), $7)
		ON CONFLICT (user_id, provider) DO UPDATE SET
			provider_id   = EXCLUDED.provider_id,
			access_token  = EXCLUDED.access_token,
			refresh_token = EXCLUDED.refresh_token,
			last_used_at  = EXCLUDED.last_used_at`

	_, err := r.pool.Exec(ctx, q,
		p.ID, p.UserID, p.Provider, p.ProviderID,
		p.AccessToken, p.RefreshToken, p.LastUsedAt,
	)
	return mapPgError(err, "link auth provider")
}

// Unlink removes the provider row for the given user + provider combination.
func (r *pgAuthProviderRepository) Unlink(ctx context.Context, userID domain.UserID, provider domain.AuthProvider) error {
	const q = `DELETE FROM user_auth_providers WHERE user_id = $1 AND provider = $2`
	ct, err := r.pool.Exec(ctx, q, userID, provider)
	if err != nil {
		return mapPgError(err, "unlink auth provider")
	}
	if ct.RowsAffected() == 0 {
		return apierror.ErrUserNotFound
	}
	return nil
}

// GetByProvider fetches the link record for a given provider + providerID pair.
// This is the primary lookup used during OAuth callback and email/password login
// (where provider = "email" and providerID = the user's email address).
func (r *pgAuthProviderRepository) GetByProvider(ctx context.Context, provider domain.AuthProvider, providerID string) (*domain.LinkedAuthProvider, error) {
	const q = `
		SELECT id, user_id, provider, provider_id,
		       access_token, refresh_token, linked_at, last_used_at
		FROM user_auth_providers
		WHERE provider = $1 AND provider_id = $2`

	row := r.pool.QueryRow(ctx, q, provider, providerID)
	p, err := scanAuthProvider(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apierror.ErrUserNotFound
		}
		return nil, mapPgError(err, "get auth provider by provider")
	}
	return p, nil
}

// ListByUser returns every provider linked to the given user.
func (r *pgAuthProviderRepository) ListByUser(ctx context.Context, userID domain.UserID) ([]*domain.LinkedAuthProvider, error) {
	const q = `
		SELECT id, user_id, provider, provider_id,
		       access_token, refresh_token, linked_at, last_used_at
		FROM user_auth_providers
		WHERE user_id = $1
		ORDER BY linked_at ASC`

	rows, err := r.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, mapPgError(err, "list auth providers by user")
	}
	defer rows.Close()

	var out []*domain.LinkedAuthProvider
	for rows.Next() {
		p, err := scanAuthProvider(rows)
		if err != nil {
			return nil, mapPgError(err, "scan auth provider row")
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, mapPgError(err, "iterate auth provider rows")
	}
	return out, nil
}

// ─── Scanner ──────────────────────────────────────────────────────────────────

func scanAuthProvider(row rowScanner) (*domain.LinkedAuthProvider, error) {
	var p domain.LinkedAuthProvider
	var linkedAt time.Time
	err := row.Scan(
		&p.ID, &p.UserID, &p.Provider, &p.ProviderID,
		&p.AccessToken, &p.RefreshToken, &linkedAt, &p.LastUsedAt,
	)
	if err != nil {
		return nil, err
	}
	p.LinkedAt = linkedAt
	return &p, nil
}

// Compile-time assertion.
var _ AuthProviderRepository = (*pgAuthProviderRepository)(nil)
