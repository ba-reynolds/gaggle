-- News attachment on posts.
-- A post may carry at most one news link. The server scrapes the URL's
-- OpenGraph metadata (title, image, site name) at create time so a feed card
-- can render the article's headline + photo preview without a client fetch.
CREATE TABLE post_news (
    post_id   INTEGER PRIMARY KEY REFERENCES posts (post_id) ON DELETE CASCADE,
    url       TEXT NOT NULL,
    title     TEXT NOT NULL DEFAULT '',
    image_url TEXT NOT NULL DEFAULT '',
    site_name TEXT NOT NULL DEFAULT ''
);