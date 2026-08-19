-- Post mentions: @username tags written into post content, resolved to the
-- real user they point at. Mirrors post_hashtags, but mentions reference the
-- users table directly (no catalog needed — users are first-class entities).
CREATE TABLE post_mentions (
    post_id INTEGER NOT NULL REFERENCES posts(post_id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (post_id, user_id)
);

CREATE INDEX post_mentions_user_idx ON post_mentions (user_id, post_id);