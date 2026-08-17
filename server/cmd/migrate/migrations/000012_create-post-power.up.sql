ALTER TABLE posts ADD COLUMN edited_at TIMESTAMPTZ;
ALTER TABLE posts ADD COLUMN is_pinned BOOLEAN NOT NULL DEFAULT FALSE;

CREATE UNIQUE INDEX posts_one_pinned_per_author_idx
    ON posts (author_id) WHERE is_pinned = TRUE AND soft_deleted = FALSE;

CREATE TABLE post_edits (
    edit_id BIGSERIAL PRIMARY KEY,
    post_id INTEGER NOT NULL REFERENCES posts(post_id) ON DELETE CASCADE,
    content_before VARCHAR(280) NOT NULL,
    edited_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX post_edits_post_idx ON post_edits (post_id, edited_at DESC, edit_id DESC);

CREATE TABLE polls (
    poll_id SERIAL PRIMARY KEY,
    post_id INTEGER NOT NULL UNIQUE REFERENCES posts(post_id) ON DELETE CASCADE,
    question VARCHAR(140) NOT NULL,
    ends_at TIMESTAMPTZ
);

CREATE TABLE poll_options (
    option_id SERIAL PRIMARY KEY,
    poll_id INTEGER NOT NULL REFERENCES polls(poll_id) ON DELETE CASCADE,
    label VARCHAR(100) NOT NULL,
    position INTEGER NOT NULL CHECK (position BETWEEN 1 AND 4),
    UNIQUE (poll_id, position)
);

CREATE TABLE poll_votes (
    poll_id INTEGER NOT NULL REFERENCES polls(poll_id) ON DELETE CASCADE,
    option_id INTEGER NOT NULL REFERENCES poll_options(option_id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (poll_id, user_id)
);

CREATE INDEX poll_votes_option_idx ON poll_votes (option_id);
