BEGIN;
-- OAuth provider credentials (admin-configured per instance).
CREATE TABLE oauth_provider_configs (
    id            TEXT PRIMARY KEY,
    provider      TEXT NOT NULL UNIQUE, -- google, outlook
    client_id     TEXT NOT NULL,
    client_secret TEXT NOT NULL, -- stored encrypted in future; plaintext for now
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- User-level sync connections (one per external account).
CREATE TABLE sync_connections (
    id              TEXT PRIMARY KEY,
    user_id         TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider        TEXT NOT NULL, -- google, outlook
    external_email  TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'active', -- active, error, revoked
    last_synced_at  TIMESTAMPTZ,
    last_error      TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sync_connections_user_id ON sync_connections(user_id);

-- OAuth tokens for sync connections.
CREATE TABLE sync_connection_tokens (
    connection_id TEXT PRIMARY KEY REFERENCES sync_connections(id) ON DELETE CASCADE,
    access_token  TEXT NOT NULL,
    refresh_token TEXT NOT NULL DEFAULT '',
    expires_at    TIMESTAMPTZ NOT NULL,
    scope         TEXT NOT NULL DEFAULT '',
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
COMMIT;
