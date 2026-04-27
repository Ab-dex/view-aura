package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	authdomain "github.com/Ab-dex/view-aura/internal/modules/auth/domain"
	"github.com/Ab-dex/view-aura/internal/modules/user/domain"
	shareddb "github.com/Ab-dex/view-aura/internal/platform/db"
	apierror "github.com/Ab-dex/view-aura/internal/platform/error"
)

type pgAuthProviderRepository struct {
	pool *pgxpool.Pool
}

// NewAuthProviderRepository constructs a Postgres AuthProviderRepository.
func NewAuthProviderRepository(pool *pgxpool.Pool) AuthProviderRepository {
	return &pgAuthProviderRepository{pool: pool}
}

func (r *pgAuthProviderRepository) conn(ctx context.Context) shareddb.Executor {
	return shareddb.Conn(ctx, r.pool)
}

// Link inserts a new provider row, or updates the tokens + last_used_at if the
// (user_id, provider, provider_id) triple already exists.
func (r *pgAuthProviderRepository) Link(ctx context.Context, p *authdomain.LinkedAuthProvider) error {
	const q = `
		INSERT INTO user_auth_providers (
			id, user_id, provider, provider_user_id,
			access_token, refresh_token, linked_at, last_used_at
		) VALUES ($1, $2, $3, $4, $5, $6, NOW(), $7)
		ON CONFLICT (user_id, provider) DO UPDATE SET
			provider_user_id   = EXCLUDED.provider_user_id,
			access_token  = EXCLUDED.access_token,
			refresh_token = EXCLUDED.refresh_token,
			last_used_at  = EXCLUDED.last_used_at`

	fmt.Print(p.UserID)

	userID, err := uuid.Parse(string(p.UserID))
	if err != nil {
		return err
	}

	_, err = r.conn(ctx).Exec(ctx, q,
		p.ID, userID, p.Provider, p.ProviderID,
		p.AccessToken, p.RefreshToken, p.LastUsedAt,
	)
	return mapPgError(err, "link auth provider")
}

// Unlink removes the provider row for the given user + provider combination.
func (r *pgAuthProviderRepository) Unlink(ctx context.Context, userID domain.UserID, provider authdomain.AuthProvider) error {
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
func (r *pgAuthProviderRepository) GetByProvider(ctx context.Context, provider authdomain.AuthProvider, providerID string) (*authdomain.LinkedAuthProvider, error) {
	const q = `
		SELECT id, user_id, provider, provider_user_id,
		       access_token, refresh_token, linked_at, last_used_at
		FROM user_auth_providers
		WHERE provider = $1 AND provider_user_id = $2`

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

func (r *pgAuthProviderRepository) GetByProviderID(ctx context.Context, provider authdomain.AuthProvider, providerID string) (*authdomain.LinkedAuthProvider, error) {
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
		return nil, mapPgError(err, "get auth provider by provider ID")
	}
	return p, nil
}

// ListByUser returns every provider linked to the given user.
func (r *pgAuthProviderRepository) ListByUser(ctx context.Context, userID domain.UserID) ([]*authdomain.LinkedAuthProvider, error) {
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

	var out []*authdomain.LinkedAuthProvider
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

func scanAuthProvider(row rowScanner) (*authdomain.LinkedAuthProvider, error) {
	var p authdomain.LinkedAuthProvider
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
