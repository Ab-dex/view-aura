-- ============================================================
--  Migration 003 — Auth module additions
--  Adds: mfa_enrollments
--  Modifies: users (MFA flag), platform/config additions documented below
--
--  The following tables already exist from migration 001 and are
--  used by the auth module WITHOUT modification:
--    user_auth_providers  — OAuth provider links + password hashes
--    user_sessions        — session lifecycle (refresh token hash, revoked_at)
--
--  The following Redis key-spaces are used by auth (no schema change):
--    auth:token:{purpose}:{sha256_hash}  — verification & reset tokens (TTL)
--    auth:user_token:{user_id}:{purpose} — secondary index for DeleteForUser
--    auth:oauth_state:{state}            — PKCE state nonce (10 min TTL)
-- ============================================================

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

-- ─── Users table: add mfa_enabled flag for fast auth-path check ───────────────
-- Avoids a JOIN to mfa_enrollments on every login to determine if MFA is needed.
-- Kept in sync by the auth service (set true in ConfirmTOTP, false in DisableMFA).

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS mfa_enabled BOOLEAN NOT NULL DEFAULT FALSE;

-- ─── Users table: add role_id FK (if migration 001 did not already add it) ───
-- The initial schema uses a CHECK constraint on the role TEXT column.
-- Migration 001 added roles/permissions/role_permissions tables.
-- This is a no-op if role_id already exists.

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'users' AND column_name = 'role_id'
    ) THEN
        ALTER TABLE users
            ADD COLUMN role_id UUID REFERENCES roles(id)
                DEFAULT (SELECT id FROM roles WHERE name = 'user');

        -- Backfill existing rows using the text role column.
        UPDATE users u
        SET role_id = (SELECT id FROM roles WHERE name = u.role);

        -- Once backfilled, enforce NOT NULL.
        ALTER TABLE users ALTER COLUMN role_id SET NOT NULL;
    END IF;
END $$;

COMMENT ON TABLE  mfa_enrollments IS 'TOTP and email-OTP second-factor enrollment records. One row per user. TOTP secrets are AES-256-GCM encrypted by the repository layer before storage.';
COMMENT ON COLUMN mfa_enrollments.totp_secret IS 'Base64-encoded AES-256-GCM ciphertext of the base32 TOTP secret. NULL for email-OTP-only enrollments.';
COMMENT ON COLUMN mfa_enrollments.recovery_codes IS 'bcrypt-hashed single-use backup codes. Each code is removed on use. Array len decrements as codes are consumed.';