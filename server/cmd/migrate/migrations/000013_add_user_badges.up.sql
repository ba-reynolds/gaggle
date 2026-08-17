-- Adds the "profile badges" subsystem:
--   * users.is_admin            — gates the admin API / admin UI
--   * badges                    — badge catalog (seeded earned + admin-created assigned)
--   * user_badges               — admin-assigned badge grants (earned badges are computed on read)

ALTER TABLE users ADD COLUMN is_admin BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE badges (
    badge_id    SERIAL PRIMARY KEY,
    key         VARCHAR(50) NOT NULL UNIQUE,
    label       VARCHAR(60) NOT NULL,
    description VARCHAR(200) NOT NULL,
    icon        VARCHAR(50) NOT NULL,
    kind        VARCHAR(10) NOT NULL CHECK (kind IN ('earned', 'assigned')),
    criteria    JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE user_badges (
    user_id    INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    badge_id   INTEGER NOT NULL REFERENCES badges(badge_id) ON DELETE CASCADE,
    granted_by INTEGER REFERENCES users(user_id) ON DELETE SET NULL,
    granted_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, badge_id)
);

CREATE INDEX user_badges_badge_idx ON user_badges (badge_id);

-- Seed the earned (auto-computed) badge catalog.
INSERT INTO badges (key, label, description, icon, kind, criteria) VALUES
    ('early_adopter',   'Early Adopter',   'Account is over 6 months old',                  'Sprout',    'earned', '{"metric":"account_age_days","min":180}'),
    ('prolific_poster', 'Prolific Poster', 'Has created over 100 posts',                   'PenLine',   'earned', '{"metric":"posts_count","min":100}'),
    ('popular',         'Popular',         'Has over 1,000 followers',                     'Users',     'earned', '{"metric":"followers_count","min":1000}'),
    ('beloved',         'Beloved',         'Has received over 5,000 likes across posts',   'Heart',     'earned', '{"metric":"likes_received","min":5000}');

-- Seed the first admin user (the demo account).
UPDATE users SET is_admin = TRUE WHERE username = 'alice';