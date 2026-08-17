DROP TABLE IF EXISTS poll_votes;
DROP TABLE IF EXISTS poll_options;
DROP TABLE IF EXISTS polls;
DROP TABLE IF EXISTS post_edits;
DROP INDEX IF EXISTS posts_one_pinned_per_author_idx;
ALTER TABLE posts DROP COLUMN IF EXISTS is_pinned;
ALTER TABLE posts DROP COLUMN IF EXISTS edited_at;
