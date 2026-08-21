-- Google OAuth: link Google identity to local user
ALTER TABLE users ADD COLUMN IF NOT EXISTS google_id VARCHAR(255);
ALTER TABLE users ADD COLUMN IF NOT EXISTS auth_provider VARCHAR(20) NOT NULL DEFAULT 'local';
-- OAuth users have no password; allow NULL so account creation doesn't need a placeholder hash
ALTER TABLE users ALTER COLUMN password DROP NOT NULL;
-- google_id must be unique when present (NULLs not considered equal in Postgres, so partial index)
CREATE UNIQUE INDEX IF NOT EXISTS unique_google_id ON users (google_id) WHERE google_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_users_google_id ON users (google_id);
