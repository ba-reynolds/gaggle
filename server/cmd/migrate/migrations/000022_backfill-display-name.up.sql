-- Backfill profile display names. Accounts created before the store started
-- defaulting display_name to the username stored '' in user_profiles
-- (000004 has no column default), so they rendered with no visible name.
-- Mirror the creation default: display_name = username.
UPDATE user_profiles up
SET display_name = u.username
FROM users u
WHERE up.user_id = u.user_id
  AND up.display_name = '';