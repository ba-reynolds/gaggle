-- De-duplicate views: a single logged-in user viewing a post counts once. The
-- GET /posts/{postID} endpoint records a view on EVERY request, and the React
-- Query refetches that like/bookmark mutations trigger re-hit it — so without
-- this, liking or bookmarking a post bumped its view count.
--
-- A partial unique index on (post_id, user_id) makes each authenticated user
-- count at most once per post. Anonymous views keep their own rows (no user_id),
-- so they are excluded from the constraint.

-- 1. Compact existing duplicates (keep the most recent row per user) and
--    correct the denormalized views_count alongside the data.
DELETE FROM post_views pv
USING post_views newer
WHERE newer.post_id = pv.post_id
  AND newer.user_id = pv.user_id
  AND pv.user_id IS NOT NULL
  AND newer.view_id > pv.view_id;

WITH kept AS (
    SELECT post_id, COUNT(*) - 1 AS excess
    FROM post_views
    WHERE user_id IS NOT NULL
    GROUP BY post_id
)
UPDATE posts p
SET views_count = p.views_count - kept.excess
FROM kept
WHERE p.post_id = kept.post_id;

-- 2. Enforce it going forward.
CREATE UNIQUE INDEX post_views_user_dedup_idx
    ON post_views (post_id, user_id)
    WHERE user_id IS NOT NULL;