-- Drop triggers on post_likes and related function
DROP TRIGGER IF EXISTS maintain_likes_count ON post_likes;
DROP FUNCTION IF EXISTS update_likes_count();

-- Drop triggers on post_reposts and related function
DROP TRIGGER IF EXISTS maintain_reposts_count ON post_reposts;
DROP FUNCTION IF EXISTS update_reposts_count();

-- Drop triggers on post_bookmarks and related function
DROP TRIGGER IF EXISTS maintain_bookmarks_count ON post_bookmarks;
DROP FUNCTION IF EXISTS update_bookmarks_count();

-- Drop triggers on post_views and related function
DROP TRIGGER IF EXISTS maintain_views_count ON post_views;
DROP FUNCTION IF EXISTS update_views_count();

-- Drop triggers on posts (replies count) and related function
DROP TRIGGER IF EXISTS maintain_replies_count ON posts;
DROP FUNCTION IF EXISTS update_replies_count();

-- Drop trigger and function for default bookmark category creation on user_profiles
DROP TRIGGER IF EXISTS create_default_bookmark_category_trigger ON user_profiles;
DROP FUNCTION IF EXISTS create_default_bookmark_category();

-- Drop all indexes created for performance optimizations

DROP INDEX IF EXISTS idx_post_likes_post_id;
DROP INDEX IF EXISTS idx_likes_user_created;

DROP INDEX IF EXISTS idx_post_reposts_original_post_id;
DROP INDEX IF EXISTS idx_reposts_post_user;

DROP INDEX IF EXISTS idx_bookmark_categories_user_id;

DROP INDEX IF EXISTS idx_bookmarks_user_created;
DROP INDEX IF EXISTS idx_bookmarks_user_category_created;

-- Drop all tables created (must drop tables with dependencies last)
DROP TABLE IF EXISTS post_views;
DROP TABLE IF EXISTS post_bookmarks;
DROP TABLE IF EXISTS bookmark_categories;
DROP TABLE IF EXISTS post_reposts;
DROP TABLE IF EXISTS post_likes;

-- Remove columns added to posts table
ALTER TABLE posts
    DROP COLUMN IF EXISTS likes_count,
    DROP COLUMN IF EXISTS reposts_count,
    DROP COLUMN IF EXISTS quotes_count,
    DROP COLUMN IF EXISTS bookmarks_count,
    DROP COLUMN IF EXISTS views_count,
    DROP COLUMN IF EXISTS replies_count,
    DROP COLUMN IF EXISTS quoted_post_id;
