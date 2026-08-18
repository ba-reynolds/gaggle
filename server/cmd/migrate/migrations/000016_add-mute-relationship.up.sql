-- Allow a 'mute' relationship type so a user can receive content from
-- someone while silencing their notifications.
ALTER TABLE user_relationships
    DROP CONSTRAINT IF EXISTS user_relationships_relationship_type_check,
    ADD CONSTRAINT user_relationships_relationship_type_check
        CHECK (relationship_type IN ('follow', 'block', 'mute'));