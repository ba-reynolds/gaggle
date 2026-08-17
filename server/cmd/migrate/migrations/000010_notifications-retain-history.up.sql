ALTER TABLE notifications
    DROP CONSTRAINT notifications_post_id_fkey,
    ADD CONSTRAINT notifications_post_id_fkey
        FOREIGN KEY (post_id) REFERENCES posts(post_id) ON DELETE SET NULL;
