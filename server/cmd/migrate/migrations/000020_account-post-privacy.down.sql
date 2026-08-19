ALTER TABLE posts
    DROP CONSTRAINT IF EXISTS posts_visibility_check;

ALTER TABLE posts
    DROP COLUMN mentioned_user_ids,
    DROP COLUMN visibility;

ALTER TABLE users
    DROP COLUMN is_private;