-- User relationships table
CREATE TABLE user_relationships (
    "relationship_id" SERIAL PRIMARY KEY,
    "follower_id" INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    "following_id" INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    "relationship_type" VARCHAR(20) NOT NULL CHECK (relationship_type IN ('follow', 'block')),
    "created_at" TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    
    -- Ensure a user can't have multiple relationships of the same type with another user
    UNIQUE("follower_id", "following_id", "relationship_type"),
    
    -- Ensure users can't follow/block themselves
    CHECK ("follower_id" != "following_id")
);

-- Indexes for performance
CREATE INDEX idx_user_relationships_follower_type ON user_relationships(follower_id, relationship_type);
CREATE INDEX idx_user_relationships_following_type ON user_relationships(following_id, relationship_type);
CREATE INDEX idx_user_relationships_follower_following ON user_relationships(follower_id, following_id);


-- Add follower/following counts to user_profiles for performance
ALTER TABLE user_profiles 
ADD COLUMN followers_count INTEGER DEFAULT 0,
ADD COLUMN following_count INTEGER DEFAULT 0;

-- Functions to maintain counts
CREATE OR REPLACE FUNCTION update_relationship_counts()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        IF NEW.relationship_type = 'follow' THEN
            -- Increment following count for follower
            UPDATE user_profiles 
            SET following_count = following_count + 1 
            WHERE user_id = NEW.follower_id;
            
            -- Increment followers count for followed user
            UPDATE user_profiles 
            SET followers_count = followers_count + 1 
            WHERE user_id = NEW.following_id;
        END IF;
        RETURN NEW;
    ELSIF TG_OP = 'DELETE' THEN
        IF OLD.relationship_type = 'follow' THEN
            -- Decrement following count for follower
            UPDATE user_profiles 
            SET following_count = following_count - 1 
            WHERE user_id = OLD.follower_id;
            
            -- Decrement followers count for followed user
            UPDATE user_profiles 
            SET followers_count = followers_count - 1 
            WHERE user_id = OLD.following_id;
        END IF;
        RETURN OLD;
    END IF;
    RETURN NULL;
END;
$$ language 'plpgsql';

CREATE TRIGGER maintain_relationship_counts
    AFTER INSERT OR DELETE ON user_relationships
    FOR EACH ROW
    EXECUTE FUNCTION update_relationship_counts();