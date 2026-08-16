-- User profiles table - Public profile information
CREATE TABLE IF NOT EXISTS user_profiles (
    "profile_id" SERIAL PRIMARY KEY,
    "user_id" INTEGER NOT NULL REFERENCES users(user_id),
    "display_name" VARCHAR(50) NOT NULL,
    "bio" VARCHAR(160) DEFAULT '',
    "profile_picture_uuid" UUID REFERENCES media(media_uuid),
    "banner_uuid" UUID REFERENCES media(media_uuid),
    "birth_date" DATE,
    "location" VARCHAR(30) DEFAULT '',
    "website" VARCHAR(50) DEFAULT '',
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

