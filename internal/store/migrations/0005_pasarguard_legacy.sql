-- Maps PasarGuard user ids to Nexon users so legacy subscription tokens keep resolving.
CREATE TABLE legacy_users (
    legacy_id INTEGER PRIMARY KEY,
    user_id   INTEGER NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE
);
