-- Refresh-token rotation: group tokens into a session family so rotating one
-- token (issue new, revoke old) can revoke the whole chain on theft/reuse.
-- Existing rows are backfilled as their own single-token family so no one is
-- logged out by the deploy.
ALTER TABLE refresh_tokens ADD COLUMN session_id UUID;
ALTER TABLE refresh_tokens ADD COLUMN revoked_reason TEXT;

UPDATE refresh_tokens SET session_id = refresh_token_id WHERE session_id IS NULL;

ALTER TABLE refresh_tokens ALTER COLUMN session_id SET NOT NULL;