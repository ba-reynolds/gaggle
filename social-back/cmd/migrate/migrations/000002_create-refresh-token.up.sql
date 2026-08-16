CREATE TABLE refresh_tokens (
    "refresh_token_id" UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    "user_id" INT NOT NULL REFERENCES users("user_id"),
    "token_hash" TEXT NOT NULL,
    "issued_at" TIMESTAMPTZ NOT NULL,
    "expires_at" TIMESTAMPTZ NOT NULL,
    "revoked" BOOLEAN NOT NULL DEFAULT FALSE,
    "revoked_at" TIMESTAMPTZ,
    "user_agent" VARCHAR(255),
    "ip_address" INET
);
