-- Users table - Core user information
CREATE TABLE IF NOT EXISTS users (
    "user_id" SERIAL PRIMARY KEY,
    "username" VARCHAR(16) NOT NULL,
    "email" VARCHAR(96) NOT NULL,
    "password" VARCHAR(255) NOT NULL,
    "soft_deleted" BOOLEAN NOT NULL DEFAULT FALSE,
    "soft_deleted_at" TIMESTAMPTZ,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- We create a new index for lowercase usernames and emails, because each element the index
-- holds must be unique this is a way to enforce unique case-insensitive usernames and emails

-- Another benefit of this is that we can query for usernames and emails faster thanks to the
-- index, but to do this we must use `LOWER("username")` and `LOWER("email")` in our query. E.g.
-- SELECT * FROM users WHERE LOWER(username) = LOWER($data-passed-from-program)
-- SELECT * FROM users WHERE LOWER(email) = LOWER($data-passed-from-program)

-- If we do not do this, then the database falls back to a plain-old sequential
-- scan.
CREATE UNIQUE INDEX IF NOT EXISTS unique_email_case_insensitive ON users (LOWER("email"));
CREATE UNIQUE INDEX IF NOT EXISTS unique_username ON users (LOWER("username"));