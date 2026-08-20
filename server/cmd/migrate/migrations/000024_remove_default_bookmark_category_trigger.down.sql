CREATE OR REPLACE FUNCTION create_default_bookmark_category()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO bookmark_categories (user_id, category_name, color)
    VALUES (NEW.user_id, 'General', '#1DA1F2');
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER create_default_bookmark_category_trigger
    AFTER INSERT ON user_profiles
    FOR EACH ROW
    EXECUTE FUNCTION create_default_bookmark_category();
