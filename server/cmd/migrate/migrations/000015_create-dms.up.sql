-- Adds the Direct Messages subsystem: 1:1 private conversations.
--   * conversations — one row per participant pair (canonical a < b ordering)
--   * messages      — per-conversation history with an unread marker

CREATE TABLE conversations (
    conversation_id SERIAL PRIMARY KEY,
    participant_a   INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    participant_b   INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_message_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (participant_a < participant_b),
    UNIQUE (participant_a, participant_b)
);

CREATE INDEX conversations_last_message_idx ON conversations (last_message_at DESC);

CREATE TABLE messages (
    message_id      SERIAL PRIMARY KEY,
    conversation_id INTEGER NOT NULL REFERENCES conversations(conversation_id) ON DELETE CASCADE,
    sender_id       INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    body            VARCHAR(2000) NOT NULL,
    read_at         TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX messages_conversation_created_idx ON messages (conversation_id, created_at DESC, message_id DESC);
CREATE INDEX messages_unread_idx ON messages (conversation_id, sender_id) WHERE read_at IS NULL;