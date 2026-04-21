package repository

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Ab-dex/view-aura/internal/modules/auth/domain"
	userdomain "github.com/Ab-dex/view-aura/internal/modules/user/domain"
	apierror "github.com/Ab-dex/view-aura/internal/platform/error"
)

// pgUniqueViolation is the SQLState code for unique constraint violations.
const pgUniqueViolation = "23505"

// ─── VerificationTokenRepository — Postgres ───────────────────────────────────

type pgVerificationTokenRepo struct{ pool *pgxpool.Pool }

func NewVerificationTokenRepository(pool *pgxpool.Pool) VerificationTokenRepository {
	return &pgVerificationTokenRepo{pool: pool}
}

const tokenCols = `id, user_id, purpose, token_hash, expires_at, redeemed_at, created_at, ip_address, user_agent`

func (r *pgVerificationTokenRepo) Create(ctx context.Context, t *domain.VerificationToken) error {
	const q = `
		INSERT INTO auth_verification_tokens
			(id, user_id, purpose, token_hash, expires_at, ip_address, user_agent, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,NOW())`
	_, err := r.pool.Exec(ctx, q,
		t.ID, t.UserID, string(t.Purpose), t.TokenHash,
		t.ExpiresAt, t.IPAddress, t.UserAgent,
	)
	if err != nil {
		return fmt.Errorf("auth: create verification token: %w", err)
	}
	return nil
}

func (r *pgVerificationTokenRepo) GetByHash(ctx context.Context, hash string) (*domain.VerificationToken, error) {
	const q = `SELECT ` + tokenCols + `
		FROM auth_verification_tokens
		WHERE token_hash = $1
		LIMIT 1`
	row := r.pool.QueryRow(ctx, q, hash)
	return scanToken(row)
}

func (r *pgVerificationTokenRepo) MarkRedeemed(ctx context.Context, id string) error {
	const q = `
		UPDATE auth_verification_tokens
		SET redeemed_at = NOW()
		WHERE id = $1 AND redeemed_at IS NULL`
	ct, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("auth: mark token redeemed: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return apierror.ErrTokenInvalid
	}
	return nil
}

func (r *pgVerificationTokenRepo) DeleteExpired(ctx context.Context, purpose domain.VerificationPurpose) (int64, error) {
	const q = `
		DELETE FROM auth_verification_tokens
		WHERE purpose = $1 AND expires_at < NOW()`
	ct, err := r.pool.Exec(ctx, q, string(purpose))
	if err != nil {
		return 0, fmt.Errorf("auth: delete expired tokens: %w", err)
	}
	return ct.RowsAffected(), nil
}

func (r *pgVerificationTokenRepo) DeleteAllForUser(ctx context.Context, userID string, purpose domain.VerificationPurpose) error {
	const q = `
		DELETE FROM auth_verification_tokens
		WHERE user_id = $1 AND purpose = $2`
	_, err := r.pool.Exec(ctx, q, userID, string(purpose))
	if err != nil {
		return fmt.Errorf("auth: delete user tokens: %w", err)
	}
	return nil
}

func scanToken(row pgx.Row) (*domain.VerificationToken, error) {
	var (
		t         domain.VerificationToken
		purpose   string
		expiresAt time.Time
		createdAt time.Time
	)
	err := row.Scan(
		&t.ID, &t.UserID, &purpose, &t.TokenHash,
		&expiresAt, &t.RedeemedAt, &createdAt,
		&t.IPAddress, &t.UserAgent,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apierror.ErrTokenInvalid
	}
	if err != nil {
		return nil, fmt.Errorf("auth: scan token: %w", err)
	}
	t.Purpose = domain.VerificationPurpose(purpose)
	t.ExpiresAt = expiresAt
	t.CreatedAt = createdAt
	return &t, nil
}

// ─── MFARepository — composed Postgres + Redis ───────────────────────────────

// mfaRepository delegates Postgres enrollment operations to pgMFARepo and
// Redis challenge operations to redisMFARepo.
type mfaRepository struct {
	pg  *pgMFARepo
	rdb *redisMFARepo
}

// NewMFARepository composes the Postgres enrollment store with the Redis
// challenge store.  encKey must be exactly 32 bytes (AES-256-GCM).
func NewMFARepository(pool *pgxpool.Pool, rdb redisClient, encKey []byte) (MFARepository, error) {
	if len(encKey) != 32 {
		return nil, fmt.Errorf("auth: MFA encryption key must be 32 bytes, got %d", len(encKey))
	}
	return &mfaRepository{
		pg:  &pgMFARepo{pool: pool, encKey: encKey},
		rdb: &redisMFARepo{rdb: rdb},
	}, nil
}

func (r *mfaRepository) CreateEnrollment(ctx context.Context, e *domain.MFAEnrollment) error {
	return r.pg.CreateEnrollment(ctx, e)
}
func (r *mfaRepository) GetEnrollment(ctx context.Context, userID string) (*domain.MFAEnrollment, error) {
	return r.pg.GetEnrollment(ctx, userID)
}
func (r *mfaRepository) MarkEnrollmentVerified(ctx context.Context, id string) error {
	return r.pg.MarkEnrollmentVerified(ctx, id)
}
func (r *mfaRepository) UpdateLastUsed(ctx context.Context, id string) error {
	return r.pg.UpdateLastUsed(ctx, id)
}
func (r *mfaRepository) DeleteEnrollment(ctx context.Context, userID string) error {
	return r.pg.DeleteEnrollment(ctx, userID)
}
func (r *mfaRepository) SaveBackupCodes(ctx context.Context, enrollmentID string, hashes []string) error {
	return r.pg.SaveBackupCodes(ctx, enrollmentID, hashes)
}
func (r *mfaRepository) ConsumeBackupCode(ctx context.Context, enrollmentID, hash string) (bool, error) {
	return r.pg.ConsumeBackupCode(ctx, enrollmentID, hash)
}
func (r *mfaRepository) CreateChallenge(ctx context.Context, ch *domain.MFAChallenge) error {
	return r.rdb.CreateChallenge(ctx, ch)
}
func (r *mfaRepository) GetChallenge(ctx context.Context, id string) (*domain.MFAChallenge, error) {
	return r.rdb.GetChallenge(ctx, id)
}
func (r *mfaRepository) DeleteChallenge(ctx context.Context, id string) error {
	return r.rdb.DeleteChallenge(ctx, id)
}

// ─── pgMFARepo — Postgres side ───────────────────────────────────────────────

type pgMFARepo struct {
	pool   *pgxpool.Pool
	encKey []byte // 32-byte AES-256 key, injected from Vault
}

const enrollCols = `id, user_id, method, encrypted_secret, backup_code_hashes, verified, enrolled_at, last_used_at`

func (r *pgMFARepo) CreateEnrollment(ctx context.Context, e *domain.MFAEnrollment) error {
	enc, err := r.encrypt(e.EncryptedSecret)
	if err != nil {
		return fmt.Errorf("auth: encrypt totp secret: %w", err)
	}
	const q = `
		INSERT INTO mfa_enrollments
			(id, user_id, method, encrypted_secret, backup_code_hashes, verified, enrolled_at)
		VALUES ($1,$2,$3,$4,$5,$6,NOW())`
	_, err = r.pool.Exec(ctx, q,
		e.ID, e.UserID, string(e.Method), enc,
		e.BackupCodeHashes, e.Verified,
	)
	if err != nil {
		return fmt.Errorf("auth: create mfa enrollment: %w", err)
	}
	return nil
}

func (r *pgMFARepo) GetEnrollment(ctx context.Context, userID string) (*domain.MFAEnrollment, error) {
	const q = `SELECT ` + enrollCols + ` FROM mfa_enrollments WHERE user_id = $1`
	row := r.pool.QueryRow(ctx, q, userID)

	var (
		e       domain.MFAEnrollment
		method  string
		encBlob []byte
		codes   []string
	)
	err := row.Scan(
		&e.ID, &e.UserID, &method, &encBlob,
		&codes, &e.Verified, &e.EnrolledAt, &e.LastUsedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil // no enrollment — caller decides what to do
	}
	if err != nil {
		return nil, fmt.Errorf("auth: get mfa enrollment: %w", err)
	}
	e.Method = domain.MFAMethod(method)
	e.BackupCodeHashes = codes

	// Decrypt the TOTP secret — nil is valid for email-only MFA.
	e.EncryptedSecret, err = r.decrypt(encBlob)
	if err != nil {
		return nil, fmt.Errorf("auth: decrypt totp secret: %w", err)
	}
	return &e, nil
}

func (r *pgMFARepo) MarkEnrollmentVerified(ctx context.Context, id string) error {
	const q = `UPDATE mfa_enrollments SET verified = TRUE, enrolled_at = NOW() WHERE id = $1`
	ct, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("auth: mark enrollment verified: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return apierror.NotFound("MFA enrollment not found")
	}
	return nil
}

func (r *pgMFARepo) UpdateLastUsed(ctx context.Context, id string) error {
	// Called asynchronously — errors are non-fatal, so we don't wrap.
	_, err := r.pool.Exec(ctx, `UPDATE mfa_enrollments SET last_used_at = NOW() WHERE id = $1`, id)
	return err
}

func (r *pgMFARepo) DeleteEnrollment(ctx context.Context, userID string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM mfa_enrollments WHERE user_id = $1`, userID)
	if err != nil {
		return fmt.Errorf("auth: delete mfa enrollment: %w", err)
	}
	return nil
}

// SaveBackupCodes replaces the entire backup-code hash set for an enrollment.
// Called once during ConfirmTOTP — the caller bcrypt-hashes each code first.
func (r *pgMFARepo) SaveBackupCodes(ctx context.Context, enrollmentID string, hashes []string) error {
	const q = `UPDATE mfa_enrollments SET backup_code_hashes = $2 WHERE id = $1`
	_, err := r.pool.Exec(ctx, q, enrollmentID, hashes)
	if err != nil {
		return fmt.Errorf("auth: save backup codes: %w", err)
	}
	return nil
}

// ConsumeBackupCode atomically finds and removes a matching backup code hash.
//
// Uses array_remove in a single UPDATE to avoid a SELECT+UPDATE race.
// Returns true if the code existed (and was consumed), false if not found.
func (r *pgMFARepo) ConsumeBackupCode(ctx context.Context, enrollmentID, hash string) (bool, error) {
	const q = `
		UPDATE mfa_enrollments
		SET backup_code_hashes = array_remove(backup_code_hashes, $2::text)
		WHERE id = $1
		  AND $2::text = ANY(backup_code_hashes)
		RETURNING id`
	var id string
	err := r.pool.QueryRow(ctx, q, enrollmentID, hash).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("auth: consume backup code: %w", err)
	}
	return true, nil
}

// ─── AES-256-GCM helpers ──────────────────────────────────────────────────────

// encrypt returns base64-encoded(nonce || ciphertext), or nil if input is nil.
func (r *pgMFARepo) encrypt(plaintext []byte) ([]byte, error) {
	if plaintext == nil {
		return nil, nil
	}
	block, err := aes.NewCipher(r.encKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	sealed := gcm.Seal(nonce, nonce, plaintext, nil)
	encoded := make([]byte, base64.StdEncoding.EncodedLen(len(sealed)))
	base64.StdEncoding.Encode(encoded, sealed)
	return encoded, nil
}

// decrypt reverses encrypt; returns nil if encoded is nil.
func (r *pgMFARepo) decrypt(encoded []byte) ([]byte, error) {
	if encoded == nil {
		return nil, nil
	}
	data := make([]byte, base64.StdEncoding.DecodedLen(len(encoded)))
	n, err := base64.StdEncoding.Decode(data, encoded)
	if err != nil {
		return nil, fmt.Errorf("auth: base64 decode: %w", err)
	}
	data = data[:n]

	block, err := aes.NewCipher(r.encKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(data) < gcm.NonceSize() {
		return nil, fmt.Errorf("auth: ciphertext too short")
	}
	nonce, ciphertext := data[:gcm.NonceSize()], data[gcm.NonceSize():]
	return gcm.Open(nil, nonce, ciphertext, nil)
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

func (r *pgSecurityRepository) GetByUserID(ctx context.Context, userID userdomain.UserID) (*domain.UserSecurity, error) {
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

func (r *pgSecurityRepository) IncrementFailedLogins(ctx context.Context, userID userdomain.UserID) (*domain.UserSecurity, error) {
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

func (r *pgSecurityRepository) ResetFailedLogins(ctx context.Context, userID userdomain.UserID) error {
	const q = `
		UPDATE user_security
		SET failed_login_count = 0, locked_until = NULL, updated_at = NOW()
		WHERE user_id = $1`
	_, err := r.pool.Exec(ctx, q, userID)
	return mapPgError(err, "reset failed logins")
}

func (r *pgSecurityRepository) LockUntil(ctx context.Context, userID userdomain.UserID, until interface{}) error {
	const q = `UPDATE user_security SET locked_until = $2, updated_at = NOW() WHERE user_id = $1`
	_, err := r.pool.Exec(ctx, q, userID, until)
	return mapPgError(err, "lock until")
}

func (r *pgSecurityRepository) RecordLogin(ctx context.Context, userID userdomain.UserID, ip string) error {
	const q = `
		INSERT INTO user_security (user_id, last_login_at, last_login_ip, updated_at)
		VALUES ($1, NOW(), $2, NOW())
		ON CONFLICT (user_id) DO UPDATE
		SET last_login_at = NOW(), last_login_ip = $2, updated_at = NOW()`
	_, err := r.pool.Exec(ctx, q, userID, ip)
	return mapPgError(err, "record login")
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

type rowScanner interface {
	Scan(dest ...any) error
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
