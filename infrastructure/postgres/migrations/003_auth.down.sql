-- ============================================================
--  Reverse Migration 003 — Auth module additions
--
--  Drops (in dependency order):
--    auth_verification_tokens — token store (no dependents)
--    mfa_enrollments          — MFA enrollment records (no dependents)
--
--  Reverts column additions on users:
--    mfa_enabled              — denormalised MFA flag
--
--  Does NOT drop:
--    users.role_id            — potentially introduced by migration 001.
--                               If role_id was added solely by 003, uncomment
--                               the ALTER TABLE statement at the bottom.
-- ============================================================

BEGIN;

-- ─── Drop indexes explicitly (defensive — CASCADE handles it, but explicit
--     drops make the rollback log easier to audit) ──────────────────────────

DROP INDEX IF EXISTS idx_avt_token_hash;
DROP INDEX IF EXISTS idx_avt_user_purpose;
DROP INDEX IF EXISTS idx_avt_expires_at;
DROP INDEX IF EXISTS idx_mfa_enrollments_user;

-- ─── Drop tables ─────────────────────────────────────────────────────────────

DROP TABLE IF EXISTS auth_verification_tokens;
DROP TABLE IF EXISTS mfa_enrollments;

-- ─── Revert users column additions ───────────────────────────────────────────

ALTER TABLE users DROP COLUMN IF EXISTS mfa_enabled;

-- ─── Conditionally revert role_id ────────────────────────────────────────────
-- Uncomment ONLY if role_id was introduced solely by migration 003 and is
-- confirmed absent from migration 001 in your environment.
--
-- ALTER TABLE users DROP COLUMN IF EXISTS role_id;

COMMIT;