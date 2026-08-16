DROP TABLE IF EXISTS user_settings;

DROP TRIGGER IF EXISTS maintain_quotes_count ON posts;
DROP FUNCTION IF EXISTS update_quotes_count();
