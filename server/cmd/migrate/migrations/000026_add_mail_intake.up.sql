-- Mail intake: inbound mail mirrored from the Cloudflare Email Worker
-- (POST /mail/inbound). Same columns as the local dev sink (logs/mail/mail.db)
-- so orchid's mail MCP reads either backend unchanged.
CREATE TABLE IF NOT EXISTS mail_messages (
    id         TEXT PRIMARY KEY,
    ts         TEXT NOT NULL,
    from_addr  TEXT,
    to_addr    TEXT,
    subject    TEXT,
    body       TEXT,
    message_id TEXT
);

-- Cloudflare delivery is at-least-once; dedupe on the MIME Message-ID.
-- NULL message_id rows (missing/unparseable header) never collide in Postgres.
CREATE UNIQUE INDEX IF NOT EXISTS idx_mail_messages_message_id ON mail_messages (message_id);
