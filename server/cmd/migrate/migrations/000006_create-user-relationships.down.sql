-- Drop the trigger first to prevent dependency issues
DROP TRIGGER IF EXISTS maintain_relationship_counts ON user_relationships;

-- Drop the function associated with the trigger
DROP FUNCTION IF EXISTS update_relationship_counts;

-- Remove the follower/following count columns from user_profiles
ALTER TABLE user_profiles
DROP COLUMN IF EXISTS followers_count,
DROP COLUMN IF EXISTS following_count;

-- Drop indexes associated with user_relationships
DROP INDEX IF EXISTS idx_user_relationships_follower_type;
DROP INDEX IF EXISTS idx_user_relationships_following_type;
DROP INDEX IF EXISTS idx_user_relationships_follower_following;

-- Drop the user_relationships table
DROP TABLE IF EXISTS user_relationships;
