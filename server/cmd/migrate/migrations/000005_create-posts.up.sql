CREATE TABLE IF NOT EXISTS posts (
    "post_id" SERIAL PRIMARY KEY,
    "content" VARCHAR(280) NOT NULL,
    "author_id" INT REFERENCES users(user_id),
    "parent_id" INT REFERENCES posts(post_id),
    "soft_deleted" BOOLEAN NOT NULL DEFAULT FALSE,
    "soft_deleted_at" TIMESTAMPTZ,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS post_media (
    "post_id" INTEGER NOT NULL REFERENCES posts(post_id) ON DELETE CASCADE,
    "media_uuid" UUID NOT NULL REFERENCES media(media_uuid) ON DELETE CASCADE,
    "position" INTEGER NOT NULL CHECK (position BETWEEN 1 AND 4),
    "alt_text" VARCHAR(200),
    PRIMARY KEY ("post_id", "media_uuid")
);