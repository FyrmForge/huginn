BEGIN;
CREATE TABLE app_passwords (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    token_hash  TEXT NOT NULL,
    permissions TEXT NOT NULL DEFAULT 'caldav',
    last_used_at TIMESTAMPTZ,
    revoked_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_app_passwords_user_id ON app_passwords(user_id);
CREATE INDEX idx_app_passwords_token_hash ON app_passwords(token_hash);
COMMIT;
