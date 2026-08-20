-- Reverse the display_name backfill. Lossy: a user who manually set their
-- display name equal to their username would also be reset to ''.
UPDATE user_profiles up
SET display_name = ''
FROM users u
WHERE up.user_id = u.user_id
  AND up.display_name = u.username;