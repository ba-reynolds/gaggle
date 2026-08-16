-- 1. LIKES TABLE
CREATE TABLE IF NOT EXISTS post_likes (
    "like_id" SERIAL PRIMARY KEY,
    "post_id" INTEGER NOT NULL REFERENCES posts(post_id) ON DELETE CASCADE,
    "user_id" INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    -- Ensure a user can only like a post once
    UNIQUE("post_id", "user_id")
);

-- 2. REPOSTS TABLE (only tracks reposts, not quotes)
CREATE TABLE IF NOT EXISTS post_reposts (
    "repost_id" SERIAL PRIMARY KEY,
    "original_post_id" INTEGER NOT NULL REFERENCES posts(post_id) ON DELETE CASCADE,
    "user_id" INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    -- Ensure a user can only repost a post once
    UNIQUE("original_post_id", "user_id")
);

-- 3. BOOKMARK CATEGORIES TABLE
CREATE TABLE IF NOT EXISTS bookmark_categories (
    "category_id" SERIAL PRIMARY KEY,
    "user_id" INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    "category_name" VARCHAR(50) NOT NULL,
    "color" VARCHAR(7) DEFAULT '#1DA1F2', -- Hex color code
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    -- Ensure unique category names per user
    UNIQUE("user_id", "category_name")
);

-- 4. BOOKMARKS TABLE
CREATE TABLE IF NOT EXISTS post_bookmarks (
    "bookmark_id" SERIAL PRIMARY KEY,
    "post_id" INTEGER NOT NULL REFERENCES posts(post_id) ON DELETE CASCADE,
    "user_id" INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    "category_id" INTEGER REFERENCES bookmark_categories(category_id) ON DELETE SET NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    -- Ensure a user can only bookmark a post once
    UNIQUE("post_id", "user_id")
);

-- 5. POST VIEWS TABLE
CREATE TABLE IF NOT EXISTS post_views (
    "view_id" SERIAL PRIMARY KEY,
    "post_id" INTEGER NOT NULL REFERENCES posts(post_id) ON DELETE CASCADE,
    "user_id" INTEGER REFERENCES users(user_id) ON DELETE SET NULL, -- NULL for anonymous views
    "ip_address" INET, -- For anonymous view tracking
    "user_agent" TEXT,
    "viewed_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 6. POST ENGAGEMENT COUNTS (denormalized for performance)
ALTER TABLE posts 
ADD COLUMN quoted_post_id INTEGER REFERENCES posts(post_id),
ADD COLUMN likes_count INTEGER DEFAULT 0,
ADD COLUMN reposts_count INTEGER DEFAULT 0,
ADD COLUMN quotes_count INTEGER DEFAULT 0,
ADD COLUMN bookmarks_count INTEGER DEFAULT 0,
ADD COLUMN views_count INTEGER DEFAULT 0,
ADD COLUMN replies_count INTEGER DEFAULT 0; -- For thread replies

-- INDEXES for performance optimization

-- Supports looking up all likes for a given post (e.g., get all users who liked a post)
CREATE INDEX idx_post_likes_post_id ON post_likes(post_id);
-- Supports efficiently retrieving all posts liked by a user in descending order of like time
CREATE INDEX idx_likes_user_created ON post_likes(user_id, created_at DESC);

-- Supports retrieving all users who reposted or quoted a specific post
CREATE INDEX idx_post_reposts_original_post_id ON post_reposts(original_post_id);
-- Supports filtering and joining on original_post_id with user_id for engagement queries
CREATE INDEX idx_reposts_post_user ON post_reposts(original_post_id, user_id);

-- Supports getting all bookmark categories for a specific user
CREATE INDEX idx_bookmark_categories_user_id ON bookmark_categories(user_id);

-- Supports retrieving all bookmarks for a user, ordered by bookmark time descending
CREATE INDEX idx_bookmarks_user_created ON post_bookmarks(user_id, created_at DESC);
-- Supports filtering bookmarks by both user and category, sorted by time for prioritized views
CREATE INDEX idx_bookmarks_user_category_created ON post_bookmarks(user_id, category_id, created_at DESC);


-- TRIGGERS to maintain engagement counts

-- Likes count trigger
CREATE OR REPLACE FUNCTION update_likes_count()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        UPDATE posts SET likes_count = likes_count + 1 WHERE post_id = NEW.post_id;
        RETURN NEW;
    ELSIF TG_OP = 'DELETE' THEN
        UPDATE posts SET likes_count = likes_count - 1 WHERE post_id = OLD.post_id;
        RETURN OLD;
    END IF;
    RETURN NULL;
END;
$$ language 'plpgsql';

CREATE TRIGGER maintain_likes_count
    AFTER INSERT OR DELETE ON post_likes
    FOR EACH ROW
    EXECUTE FUNCTION update_likes_count();

-- Reposts count trigger
CREATE OR REPLACE FUNCTION update_reposts_count()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        UPDATE posts SET reposts_count = reposts_count + 1 WHERE post_id = NEW.original_post_id;
        RETURN NEW;
    ELSIF TG_OP = 'DELETE' THEN
        UPDATE posts SET reposts_count = reposts_count - 1 WHERE post_id = OLD.original_post_id;
        RETURN OLD;
    END IF;
    RETURN NULL;
END;
$$ language 'plpgsql';

CREATE TRIGGER maintain_reposts_count
    AFTER INSERT OR DELETE ON post_reposts
    FOR EACH ROW
    EXECUTE FUNCTION update_reposts_count();

-- Bookmarks count trigger
CREATE OR REPLACE FUNCTION update_bookmarks_count()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        UPDATE posts SET bookmarks_count = bookmarks_count + 1 WHERE post_id = NEW.post_id;
        RETURN NEW;
    ELSIF TG_OP = 'DELETE' THEN
        UPDATE posts SET bookmarks_count = bookmarks_count - 1 WHERE post_id = OLD.post_id;
        RETURN OLD;
    END IF;
    RETURN NULL;
END;
$$ language 'plpgsql';

CREATE TRIGGER maintain_bookmarks_count
    AFTER INSERT OR DELETE ON post_bookmarks
    FOR EACH ROW
    EXECUTE FUNCTION update_bookmarks_count();

-- Views count trigger (increment only, no decrement)
CREATE OR REPLACE FUNCTION update_views_count()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        UPDATE posts SET views_count = views_count + 1 WHERE post_id = NEW.post_id;
        RETURN NEW;
    END IF;
    RETURN NULL;
END;
$$ language 'plpgsql';

CREATE TRIGGER maintain_views_count
    AFTER INSERT ON post_views
    FOR EACH ROW
    EXECUTE FUNCTION update_views_count();

-- Replies count trigger (for existing posts table)
CREATE OR REPLACE FUNCTION update_replies_count()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        IF NEW.parent_id IS NOT NULL THEN
            UPDATE posts SET replies_count = replies_count + 1 WHERE post_id = NEW.parent_id;
        END IF;
        RETURN NEW;
    ELSIF TG_OP = 'DELETE' THEN
        IF OLD.parent_id IS NOT NULL THEN
            UPDATE posts SET replies_count = replies_count - 1 WHERE post_id = OLD.parent_id;
        END IF;
        RETURN OLD;
    END IF;
    RETURN NULL;
END;
$$ language 'plpgsql';

CREATE TRIGGER maintain_replies_count
    AFTER INSERT OR DELETE ON posts
    FOR EACH ROW
    EXECUTE FUNCTION update_replies_count();

-- UTILITY FUNCTIONS

-- Function to create a default "General" category for new users
CREATE OR REPLACE FUNCTION create_default_bookmark_category()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO bookmark_categories (user_id, category_name, color)
    VALUES (NEW.user_id, 'General', '#1DA1F2');
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Create default category when user profile is created
CREATE TRIGGER create_default_bookmark_category_trigger
    AFTER INSERT ON user_profiles
    FOR EACH ROW
    EXECUTE FUNCTION create_default_bookmark_category();
