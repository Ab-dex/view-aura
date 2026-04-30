-- ============================================================
--  Migration 003 — Auth module additions
--
--  Adds:
--    auth_verification_tokens — persistent token store for email
--                               verification, password reset, etc.
--    mfa_enrollments          — TOTP / email-OTP second-factor
--
--  Modifies:
--    users — adds mfa_enabled flag (fast auth-path check)
--    users — adds role_id FK if migration 001 did not already add it
--
--  The following tables already exist from migration 001 and are
--  used by the auth module WITHOUT modification:
--    user_auth_providers  — OAuth provider links + password hashes
--    user_sessions        — session lifecycle (refresh token hash, revoked_at)
--
--  The following Redis key-spaces are used by auth (no schema change):
--    auth:token:{purpose}:{sha256_hash}  — hot-path token lookup (TTL mirror)
--    auth:user_token:{user_id}:{purpose} — secondary index for DeleteForUser
--    auth:oauth_state:{state}            — PKCE state nonce (10 min TTL)
--
--  Redis vs DB strategy:
--    Redis holds the hot-path TTL cache. This table is the source of
--    truth — used for audit, revocation sweep, and recovery when Redis
--    is flushed. The repository layer writes both atomically.
-- ============================================================

BEGIN;

-- ─── Auth verification tokens ────────────────────────────────────────────────
-- Persistent store for all short-lived auth tokens:
--   email_verification  — sent after registration
--   password_reset      — sent on forgot-password flow
--   email_change        — sent when user requests an email address change
--   magic_link          — passwordless login link
--   mfa_backup          — used during MFA recovery flow
--
-- Design notes:
--   • token_hash stores SHA-256(raw_token). The raw token is never persisted.
--   • One active token per (user_id, purpose) is enforced at the app layer
--     by calling DeleteForUser before inserting a new token.
--   • redeemed_at is set atomically on consumption; subsequent use attempts
--     are rejected by the repository even if expires_at has not passed.
--   • revoked_at allows admin/server-side invalidation (e.g. on password
--     change, all outstanding reset tokens are revoked).
--   • ip_address + user_agent provide an audit trail for security review.

CREATE TABLE IF NOT EXISTS auth_verification_tokens (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    purpose     TEXT        NOT NULL
                CHECK (purpose IN (
                    'email_verification',
                    'password_reset',
                    'email_change',
                    'magic_link',
                    'mfa_backup'
                )),
    -- SHA-256 hex digest of the raw token. Raw token is never stored.
    token_hash  TEXT        NOT NULL UNIQUE,
    -- For email_change: the new address being confirmed.
    -- NULL for all other purposes.
    new_email   CITEXT,
    expires_at  TIMESTAMPTZ NOT NULL,
    -- Set once on first successful consumption. Immutable after that.
    redeemed_at     TIMESTAMPTZ,
    -- Set when explicitly invalidated before use (password changed,
    -- new token issued, admin revoke).
    revoked_at  TIMESTAMPTZ,
    ip_address  INET,
    user_agent  TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Lookup by hash on every token verification request (hot path).
CREATE INDEX idx_avt_token_hash   ON auth_verification_tokens(token_hash);

-- Lookup all tokens for a user + purpose (used by DeleteForUser before
-- issuing a replacement token, and for the revoke-all-on-password-change path).
CREATE INDEX idx_avt_user_purpose ON auth_verification_tokens(user_id, purpose);

-- Expiry sweep: background job deletes rows where expires_at < NOW().
-- Partial index skips already-consumed and revoked rows — they are either
-- archived or pending deletion and should not appear in sweep scans.
CREATE INDEX idx_avt_expires_at   ON auth_verification_tokens(expires_at ASC)
    WHERE redeemed_at IS NULL AND revoked_at IS NULL;

COMMENT ON TABLE  auth_verification_tokens IS
    'Short-lived auth tokens for email verification, password reset, magic link, and MFA recovery. '
    'token_hash is SHA-256(raw_token); the raw value is never persisted. '
    'One active token per (user_id, purpose) is maintained by the repository layer.';

COMMENT ON COLUMN auth_verification_tokens.token_hash IS
    'SHA-256 hex digest of the raw bearer token. Used as the lookup key on the verification path.';

COMMENT ON COLUMN auth_verification_tokens.new_email IS
    'Populated only for purpose=email_change. Holds the candidate address awaiting confirmation.';

COMMENT ON COLUMN auth_verification_tokens.redeemed_at IS
    'Timestamp of first successful consumption. Set atomically; never updated after that.';

COMMENT ON COLUMN auth_verification_tokens.revoked_at IS
    'Explicit server-side invalidation timestamp. Set on password change, token replacement, or admin action.';

-- ─── MFA enrollments ─────────────────────────────────────────────────────────
-- Stores TOTP secrets (AES-256-GCM encrypted by the repository layer)
-- and bcrypt-hashed single-use recovery codes.
-- One row per user — upserted on every enrollment change.

CREATE TABLE IF NOT EXISTS mfa_enrollments (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    method          TEXT        NOT NULL DEFAULT 'totp'
                    CHECK (method IN ('totp', 'email')),
    -- AES-256-GCM encrypted base32 TOTP secret.
    -- NULL for email-OTP-only enrollments.
    totp_secret     TEXT,
    -- Array of bcrypt-hashed single-use backup codes.
    -- Stored as TEXT[] rather than a separate table to allow atomic upsert.
    recovery_codes  TEXT[]      NOT NULL DEFAULT '{}',
    enabled         BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- Only one active enrollment per user.
    UNIQUE (user_id)
);

CREATE INDEX idx_mfa_enrollments_user ON mfa_enrollments(user_id);

COMMENT ON TABLE  mfa_enrollments IS
    'TOTP and email-OTP second-factor enrollment records. One row per user. '
    'TOTP secrets are AES-256-GCM encrypted by the repository layer before storage.';

COMMENT ON COLUMN mfa_enrollments.totp_secret IS
    'Base64-encoded AES-256-GCM ciphertext of the base32 TOTP secret. NULL for email-OTP-only enrollments.';

COMMENT ON COLUMN mfa_enrollments.recovery_codes IS
    'bcrypt-hashed single-use backup codes. Each code is removed on use. Array length decrements as codes are consumed.';

-- ─── Users: add mfa_enabled flag ─────────────────────────────────────────────
-- Fast auth-path check — avoids a JOIN to mfa_enrollments on every login
-- to determine whether a second factor is required.
-- Kept in sync by the auth service: set TRUE in ConfirmTOTP, FALSE in DisableMFA.

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS mfa_enabled BOOLEAN NOT NULL DEFAULT FALSE;

COMMENT ON COLUMN users.mfa_enabled IS
    'Denormalised flag — TRUE when mfa_enrollments.enabled = TRUE for this user. '
    'Avoids a JOIN on the hot login path. Kept in sync by the auth service layer.';

-- ─── Users: add role_id FK (no-op if migration 001 already added it) ─────────
-- Migration 001 introduced the roles table and a role_id column.
-- This block is a safety net for environments where 001 was applied
-- without the role_id column (e.g. early dev branches).

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'users' AND column_name = 'role_id'
    ) THEN
        ALTER TABLE users
            ADD COLUMN role_id UUID REFERENCES roles(id)
                DEFAULT (SELECT id FROM roles WHERE name = 'user');

        -- Backfill existing rows using the text role column if it exists.
        UPDATE users u
        SET role_id = (SELECT id FROM roles WHERE name = u.role);

        -- Enforce NOT NULL once backfilled.
        ALTER TABLE users ALTER COLUMN role_id SET NOT NULL;
    END IF;
END $$;

COMMIT;