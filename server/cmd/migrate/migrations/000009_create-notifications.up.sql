CREATE TABLE notifications (
    notification_id BIGSERIAL PRIMARY KEY,
    recipient_id INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    actor_id INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    notification_type VARCHAR(32) NOT NULL CHECK (notification_type IN ('like', 'repost', 'quote', 'reply', 'follow', 'mention')),
    post_id INTEGER REFERENCES posts(post_id) ON DELETE SET NULL,
    read_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX notifications_recipient_created_idx
    ON notifications (recipient_id, created_at DESC, notification_id DESC);

CREATE INDEX notifications_unread_idx
    ON notifications (recipient_id)
    WHERE read_at IS NULL;
