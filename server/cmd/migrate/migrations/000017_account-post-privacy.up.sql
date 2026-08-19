-- Post + account privacy.
-- posts.visibility controls who may see an individual post:
--   'public'      -> anyone
--   'followers'   -> the author and their followers
--   'mentions'    -> the author and users mentioned by @username in the content
-- posts.mentioned_user_ids is the resolved id set of @mentions at write time
-- (used only by the 'mentions' visibility rule).
ALTER TABLE posts
    ADD COLUMN visibility TEXT NOT NULL DEFAULT 'public',
    ADD COLUMN mentioned_user_ids INTEGER[] NOT NULL DEFAULT '{}';

ALTER TABLE posts
    ADD CONSTRAINT posts_visibility_check
        CHECK (visibility IN ('public', 'followers', 'mentions'));

-- users.is_private is the source of truth used at query time (mirrors the
-- JSONB user_settings.privacy.profileVisibility, which drives the UI).
ALTER TABLE users
    ADD COLUMN is_private BOOLEAN NOT NULL DEFAULT FALSE;

-- Backfill is_private from existing settings. profileVisibility is one of
-- 'public' | 'private' | 'friends'; both 'private' and 'friends' mean "only
-- my followers can see my posts" (this app has no distinct 'friends' circle).
UPDATE users u
SET is_private = TRUE
WHERE u.user_id IN (
    SELECT user_id
    FROM user_settings s
    WHERE s.settings->'privacy'->>'profileVisibility' IN ('private', 'friends')
);