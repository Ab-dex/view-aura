package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Ab-dex/view-aura/internal/modules/user/domain"
	apierror "github.com/Ab-dex/view-aura/internal/platform/error"
)

// pgUniqueViolation is the SQLState code for unique constraint violations.
const pgUniqueViolation = "23505"

// pgUserRepository is the Postgres-backed implementation of UserRepository.
type pgUserRepository struct {
	pool *pgxpool.Pool
}

// NewUserRepository constructs a Postgres UserRepository.
func NewUserRepository(pool *pgxpool.Pool) UserRepository {
	return &pgUserRepository{pool: pool}
}

func (r *pgUserRepository) Create(ctx context.Context, u *domain.User) (*domain.User, error) {
	const q = `
		INSERT INTO users (
			id, email, email_verified, username, display_name,
			role, status, auth_provider, provider_id,
			locale, timezone, country, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9,
			$10, $11, $12, NOW(), NOW()
		)
		RETURNING id, email, email_verified, username, display_name,
		          role, status, auth_provider, provider_id,
		          locale, timezone, country,
		          is_deleted, deleted_at, created_at, updated_at`

	row := r.pool.QueryRow(ctx, q,
		u.ID, u.Email, u.EmailVerified, u.Username, u.DisplayName,
		u.Role, u.Status, u.AuthProvider, u.ProviderID,
		u.Locale, u.Timezone, u.Country,
	)

	result, err := scanUser(row)
	if err != nil {
		return nil, mapPgError(err, "create user")
	}
	return result, nil
}

func (r *pgUserRepository) GetByID(ctx context.Context, id domain.UserID) (*domain.User, error) {
	const q = `
		SELECT id, email, email_verified, username, display_name,
		       role, status, auth_provider, provider_id,
		       locale, timezone, country,
		       is_deleted, deleted_at, created_at, updated_at
		FROM users
		WHERE id = $1 AND is_deleted = FALSE`

	row := r.pool.QueryRow(ctx, q, id)
	u, err := scanUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apierror.ErrUserNotFound
		}
		return nil, mapPgError(err, "get user by id")
	}
	return u, nil
}

func (r *pgUserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	const q = `
		SELECT id, email, email_verified, username, display_name,
		       role, status, auth_provider, provider_id,
		       locale, timezone, country,
		       is_deleted, deleted_at, created_at, updated_at
		FROM users
		WHERE LOWER(email) = LOWER($1) AND is_deleted = FALSE`

	row := r.pool.QueryRow(ctx, q, email)
	u, err := scanUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apierror.ErrUserNotFound
		}
		return nil, mapPgError(err, "get user by email")
	}
	return u, nil
}

func (r *pgUserRepository) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	const q = `
		SELECT id, email, email_verified, username, display_name,
		       role, status, auth_provider, provider_id,
		       locale, timezone, country,
		       is_deleted, deleted_at, created_at, updated_at
		FROM users
		WHERE username = $1 AND is_deleted = FALSE`

	row := r.pool.QueryRow(ctx, q, username)
	u, err := scanUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apierror.ErrUserNotFound
		}
		return nil, mapPgError(err, "get user by username")
	}
	return u, nil
}

func (r *pgUserRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	const q = `SELECT EXISTS(SELECT 1 FROM users WHERE LOWER(email) = LOWER($1))`
	var exists bool
	if err := r.pool.QueryRow(ctx, q, email).Scan(&exists); err != nil {
		return false, mapPgError(err, "exists by email")
	}
	return exists, nil
}

func (r *pgUserRepository) ExistsByUsername(ctx context.Context, username string) (bool, error) {
	const q = `SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)`
	var exists bool
	if err := r.pool.QueryRow(ctx, q, username).Scan(&exists); err != nil {
		return false, mapPgError(err, "exists by username")
	}
	return exists, nil
}

func (r *pgUserRepository) Update(ctx context.Context, u *domain.User) (*domain.User, error) {
	const q = `
		UPDATE users
		SET email         = $2,
		    email_verified = $3,
		    username      = $4,
		    display_name  = $5,
		    role          = $6,
		    status        = $7,
		    locale        = $8,
		    timezone      = $9,
		    country       = $10,
		    updated_at    = NOW()
		WHERE id = $1 AND is_deleted = FALSE
		RETURNING id, email, email_verified, username, display_name,
		          role, status, auth_provider, provider_id,
		          locale, timezone, country,
		          is_deleted, deleted_at, created_at, updated_at`

	row := r.pool.QueryRow(ctx, q,
		u.ID, u.Email, u.EmailVerified, u.Username, u.DisplayName,
		u.Role, u.Status, u.Locale, u.Timezone, u.Country,
	)
	result, err := scanUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apierror.ErrUserNotFound
		}
		return nil, mapPgError(err, "update user")
	}
	return result, nil
}

func (r *pgUserRepository) SoftDelete(ctx context.Context, id domain.UserID) error {
	const q = `
		UPDATE users
		SET is_deleted = TRUE, deleted_at = NOW(), status = 'deleted', updated_at = NOW()
		WHERE id = $1 AND is_deleted = FALSE`

	ct, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return mapPgError(err, "soft delete user")
	}
	if ct.RowsAffected() == 0 {
		return apierror.ErrUserNotFound
	}
	return nil
}

// ─── Security repository ──────────────────────────────────────────────────────

type pgSecurityRepository struct {
	pool *pgxpool.Pool
}

// NewSecurityRepository constructs a Postgres SecurityRepository.
func NewSecurityRepository(pool *pgxpool.Pool) SecurityRepository {
	return &pgSecurityRepository{pool: pool}
}

const maxFailedLogins = 5
const lockDuration = 15 * time.Minute

func (r *pgSecurityRepository) Upsert(ctx context.Context, sec *domain.UserSecurity) (*domain.UserSecurity, error) {
	const q = `
		INSERT INTO user_security (
			user_id, failed_login_count, last_failed_login, locked_until,
			risk_score, two_factor_enabled, last_login_at, last_login_ip, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,NOW())
		ON CONFLICT (user_id) DO UPDATE SET
			failed_login_count = EXCLUDED.failed_login_count,
			last_failed_login  = EXCLUDED.last_failed_login,
			locked_until       = EXCLUDED.locked_until,
			risk_score         = EXCLUDED.risk_score,
			two_factor_enabled = EXCLUDED.two_factor_enabled,
			last_login_at      = EXCLUDED.last_login_at,
			last_login_ip      = EXCLUDED.last_login_ip,
			updated_at         = NOW()
		RETURNING user_id, failed_login_count, last_failed_login, locked_until,
		          risk_score, two_factor_enabled, last_login_at, last_login_ip, updated_at`

	row := r.pool.QueryRow(ctx, q,
		sec.UserID, sec.FailedLoginCount, sec.LastFailedLogin, sec.LockedUntil,
		sec.RiskScore, sec.TwoFactorEnabled, sec.LastLoginAt, sec.LastLoginIP,
	)
	return scanSecurity(row)
}

func (r *pgSecurityRepository) GetByUserID(ctx context.Context, userID domain.UserID) (*domain.UserSecurity, error) {
	const q = `
		SELECT user_id, failed_login_count, last_failed_login, locked_until,
		       risk_score, two_factor_enabled, last_login_at, last_login_ip, updated_at
		FROM user_security WHERE user_id = $1`

	row := r.pool.QueryRow(ctx, q, userID)
	sec, err := scanSecurity(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Return a zero record rather than an error — security state is
			// lazily created on first login.
			return &domain.UserSecurity{UserID: userID}, nil
		}
		return nil, mapPgError(err, "get security by user id")
	}
	return sec, nil
}

func (r *pgSecurityRepository) IncrementFailedLogins(ctx context.Context, userID domain.UserID) (*domain.UserSecurity, error) {
	const q = `
		INSERT INTO user_security (user_id, failed_login_count, last_failed_login, updated_at)
		VALUES ($1, 1, NOW(), NOW())
		ON CONFLICT (user_id) DO UPDATE
		SET failed_login_count = user_security.failed_login_count + 1,
		    last_failed_login  = NOW(),
		    locked_until = CASE
		        WHEN user_security.failed_login_count + 1 >= $2
		        THEN NOW() + ($3::interval)
		        ELSE user_security.locked_until
		    END,
		    updated_at = NOW()
		RETURNING user_id, failed_login_count, last_failed_login, locked_until,
		          risk_score, two_factor_enabled, last_login_at, last_login_ip, updated_at`

	row := r.pool.QueryRow(ctx, q, userID, maxFailedLogins, fmt.Sprintf("%d minutes", int(lockDuration.Minutes())))
	return scanSecurity(row)
}

func (r *pgSecurityRepository) ResetFailedLogins(ctx context.Context, userID domain.UserID) error {
	const q = `
		UPDATE user_security
		SET failed_login_count = 0, locked_until = NULL, updated_at = NOW()
		WHERE user_id = $1`
	_, err := r.pool.Exec(ctx, q, userID)
	return mapPgError(err, "reset failed logins")
}

func (r *pgSecurityRepository) LockUntil(ctx context.Context, userID domain.UserID, until interface{}) error {
	const q = `UPDATE user_security SET locked_until = $2, updated_at = NOW() WHERE user_id = $1`
	_, err := r.pool.Exec(ctx, q, userID, until)
	return mapPgError(err, "lock until")
}

func (r *pgSecurityRepository) RecordLogin(ctx context.Context, userID domain.UserID, ip string) error {
	const q = `
		INSERT INTO user_security (user_id, last_login_at, last_login_ip, updated_at)
		VALUES ($1, NOW(), $2, NOW())
		ON CONFLICT (user_id) DO UPDATE
		SET last_login_at = NOW(), last_login_ip = $2, updated_at = NOW()`
	_, err := r.pool.Exec(ctx, q, userID, ip)
	return mapPgError(err, "record login")
}

// ─── Profile repository ───────────────────────────────────────────────────────

type pgProfileRepository struct {
	pool *pgxpool.Pool
}

// NewProfileRepository constructs a Postgres ProfileRepository.
func NewProfileRepository(pool *pgxpool.Pool) ProfileRepository {
	return &pgProfileRepository{pool: pool}
}

func (r *pgProfileRepository) Upsert(ctx context.Context, p *domain.UserProfile) (*domain.UserProfile, error) {
	const q = `
		INSERT INTO user_profiles (user_id, avatar_url, banner_url, bio, website, birthdate, gender, visibility, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,NOW())
		ON CONFLICT (user_id) DO UPDATE SET
			avatar_url = EXCLUDED.avatar_url,
			banner_url = EXCLUDED.banner_url,
			bio        = EXCLUDED.bio,
			website    = EXCLUDED.website,
			birthdate  = EXCLUDED.birthdate,
			gender     = EXCLUDED.gender,
			visibility = EXCLUDED.visibility,
			updated_at = NOW()
		RETURNING user_id, avatar_url, banner_url, bio, website, birthdate, gender, visibility, updated_at`

	row := r.pool.QueryRow(ctx, q,
		p.UserID, p.AvatarURL, p.BannerURL, p.Bio, p.Website,
		p.Birthdate, p.Gender, p.Visibility,
	)
	return scanProfile(row)
}

func (r *pgProfileRepository) GetByUserID(ctx context.Context, userID domain.UserID) (*domain.UserProfile, error) {
	const q = `
		SELECT user_id, avatar_url, banner_url, bio, website, birthdate, gender, visibility, updated_at
		FROM user_profiles WHERE user_id = $1`

	row := r.pool.QueryRow(ctx, q, userID)
	profile, err := scanProfile(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &domain.UserProfile{UserID: userID, Visibility: domain.VisibilityPublic}, nil
		}
		return nil, mapPgError(err, "get profile by user id")
	}
	return profile, nil
}

// ─── Preferences repository ───────────────────────────────────────────────────

type pgPreferencesRepository struct {
	pool *pgxpool.Pool
}

// NewPreferencesRepository constructs a Postgres PreferencesRepository.
func NewPreferencesRepository(pool *pgxpool.Pool) PreferencesRepository {
	return &pgPreferencesRepository{pool: pool}
}

func (r *pgPreferencesRepository) Upsert(ctx context.Context, p *domain.UserPreferences) (*domain.UserPreferences, error) {
	const q = `
		INSERT INTO user_preferences (
			user_id, preferred_genres, disliked_genres, preferred_languages,
			adult_content, dark_mode, autoplay_trailers, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,NOW())
		ON CONFLICT (user_id) DO UPDATE SET
			preferred_genres    = EXCLUDED.preferred_genres,
			disliked_genres     = EXCLUDED.disliked_genres,
			preferred_languages = EXCLUDED.preferred_languages,
			adult_content       = EXCLUDED.adult_content,
			dark_mode           = EXCLUDED.dark_mode,
			autoplay_trailers   = EXCLUDED.autoplay_trailers,
			updated_at          = NOW()
		RETURNING user_id, preferred_genres, disliked_genres, preferred_languages,
		          adult_content, dark_mode, autoplay_trailers, updated_at`

	row := r.pool.QueryRow(ctx, q,
		p.UserID, p.PreferredGenres, p.DislikedGenres, p.PreferredLanguages,
		p.AdultContent, p.DarkMode, p.AutoplayTrailers,
	)
	return scanPreferences(row)
}

func (r *pgPreferencesRepository) GetByUserID(ctx context.Context, userID domain.UserID) (*domain.UserPreferences, error) {
	const q = `
		SELECT user_id, preferred_genres, disliked_genres, preferred_languages,
		       adult_content, dark_mode, autoplay_trailers, updated_at
		FROM user_preferences WHERE user_id = $1`

	row := r.pool.QueryRow(ctx, q, userID)
	prefs, err := scanPreferences(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &domain.UserPreferences{UserID: userID, DarkMode: true}, nil
		}
		return nil, mapPgError(err, "get preferences by user id")
	}
	return prefs, nil
}

// ─── Scanner helpers ──────────────────────────────────────────────────────────

type rowScanner interface {
	Scan(dest ...any) error
}

func scanUser(row rowScanner) (*domain.User, error) {
	var u domain.User
	err := row.Scan(
		&u.ID, &u.Email, &u.EmailVerified, &u.Username, &u.DisplayName,
		&u.Role, &u.Status, &u.AuthProvider, &u.ProviderID,
		&u.Locale, &u.Timezone, &u.Country,
		&u.IsDeleted, &u.DeletedAt, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func scanSecurity(row rowScanner) (*domain.UserSecurity, error) {
	var s domain.UserSecurity
	err := row.Scan(
		&s.UserID, &s.FailedLoginCount, &s.LastFailedLogin, &s.LockedUntil,
		&s.RiskScore, &s.TwoFactorEnabled, &s.LastLoginAt, &s.LastLoginIP, &s.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func scanProfile(row rowScanner) (*domain.UserProfile, error) {
	var p domain.UserProfile
	err := row.Scan(
		&p.UserID, &p.AvatarURL, &p.BannerURL, &p.Bio, &p.Website,
		&p.Birthdate, &p.Gender, &p.Visibility, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func scanPreferences(row rowScanner) (*domain.UserPreferences, error) {
	var p domain.UserPreferences
	err := row.Scan(
		&p.UserID, &p.PreferredGenres, &p.DislikedGenres, &p.PreferredLanguages,
		&p.AdultContent, &p.DarkMode, &p.AutoplayTrailers, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// ─── Error mapping ────────────────────────────────────────────────────────────

// mapPgError translates pgconn and pgx errors to apierror types so service
// and handler layers never need to import database packages.
func mapPgError(err error, op string) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case pgUniqueViolation:
			switch pgErr.ConstraintName {
			case "users_email_key":
				return apierror.ErrEmailAlreadyExists.WithCause(err)
			case "users_username_key":
				return apierror.ErrUsernameAlreadyExists.WithCause(err)
			}
			return apierror.Conflict("duplicate record").WithCause(err)
		}
	}
	return apierror.DatabaseError(fmt.Errorf("%s: %w", op, err))
}
