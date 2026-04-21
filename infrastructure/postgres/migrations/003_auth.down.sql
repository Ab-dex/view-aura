-- Reverse migration 003 — Auth module
ALTER TABLE users DROP COLUMN IF EXISTS mfa_enabled;
DROP TABLE IF EXISTS mfa_enrollments;

-- Note: role_id column is NOT dropped here because it was potentially added
-- by migration 001 as well. If role_id was introduced solely by 003, add:
-- ALTER TABLE users DROP COLUMN IF EXISTS role_id;