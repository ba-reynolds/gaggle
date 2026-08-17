-- Adds the "lists" subsystem: public, owner-managed lists of users.
--   * lists         — a named collection owned by one user
--   * list_members  — users added to a list (composite PK, mirrors house style)

CREATE TABLE lists (
    list_id     SERIAL PRIMARY KEY,
    owner_id    INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    name        VARCHAR(100) NOT NULL,
    description VARCHAR(300) NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (owner_id, name)
);

CREATE TABLE list_members (
    list_id    INTEGER NOT NULL REFERENCES lists(list_id) ON DELETE CASCADE,
    user_id    INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    added_at   TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (list_id, user_id)
);

CREATE INDEX list_members_user_idx ON list_members (user_id);