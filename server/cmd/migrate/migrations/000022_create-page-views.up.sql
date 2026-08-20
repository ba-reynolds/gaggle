-- Records a page view / API request so admins can see visit & traffic stats.
--   * user_id     — the authenticated user, if any (NULL for anonymous visits)
--   * ip          — the request's source address (INET, port stripped)
--   * method/path — which endpoint was hit (page views are GETs only)
--   * status      — HTTP status code of the response
--   * created_at  — when the view happened

CREATE TABLE page_views (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT REFERENCES users(user_id) ON DELETE SET NULL,
    ip         INET,
    method     TEXT NOT NULL,
    path       TEXT NOT NULL,
    status     INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX page_views_created_at_idx ON page_views (created_at DESC);
CREATE INDEX page_views_user_id_idx ON page_views (user_id);