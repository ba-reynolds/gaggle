-- Quotes count trigger. The 000007 migration added the quotes_count column but
-- never wired up a trigger for it, so quote counts were always 0.
CREATE OR REPLACE FUNCTION update_quotes_count()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        IF NEW.quoted_post_id IS NOT NULL THEN
            UPDATE posts SET quotes_count = quotes_count + 1 WHERE post_id = NEW.quoted_post_id;
        END IF;
        RETURN NEW;
    ELSIF TG_OP = 'DELETE' THEN
        IF OLD.quoted_post_id IS NOT NULL THEN
            UPDATE posts SET quotes_count = quotes_count - 1 WHERE post_id = OLD.quoted_post_id;
        END IF;
        RETURN OLD;
    END IF;
    RETURN NULL;
END;
$$ language 'plpgsql';

CREATE TRIGGER maintain_quotes_count
    AFTER INSERT OR DELETE ON posts
    FOR EACH ROW
    EXECUTE FUNCTION update_quotes_count();

-- User settings, stored as JSONB so the frontend contract can evolve without
-- new migrations. GET/PATCH /users/settings read/write this row.
CREATE TABLE IF NOT EXISTS user_settings (
    "user_id" INTEGER PRIMARY KEY REFERENCES users(user_id) ON DELETE CASCADE,
    "settings" JSONB NOT NULL DEFAULT '{
        "notifications": {"email": true, "push": true, "mentions": true},
        "privacy": {"profileVisibility": "public", "showOnlineStatus": true, "allowTagging": true},
        "appearance": {"theme": "system", "fontSize": "medium"},
        "language": "en"
    }'::jsonb,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
