BEGIN;
CREATE TABLE calendars (
    id                  TEXT PRIMARY KEY,
    owner_id            TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name                TEXT NOT NULL,
    description         TEXT NOT NULL DEFAULT '',
    color               TEXT NOT NULL DEFAULT '#f5a623',
    timezone            TEXT NOT NULL DEFAULT 'UTC',
    default_visibility  TEXT NOT NULL DEFAULT 'private',
    default_busy_status TEXT NOT NULL DEFAULT 'busy',
    is_default          BOOLEAN NOT NULL DEFAULT false,
    deleted_at          TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_calendars_owner_id ON calendars(owner_id);

CREATE TABLE calendar_members (
    calendar_id TEXT NOT NULL REFERENCES calendars(id) ON DELETE CASCADE,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role        TEXT NOT NULL DEFAULT 'viewer', -- owner, editor, viewer
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (calendar_id, user_id)
);

CREATE INDEX idx_calendar_members_user_id ON calendar_members(user_id);

CREATE TABLE calendar_sync_state (
    calendar_id     TEXT PRIMARY KEY REFERENCES calendars(id) ON DELETE CASCADE,
    ctag            TEXT NOT NULL DEFAULT '',
    sync_token      TEXT NOT NULL DEFAULT '',
    last_changed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
COMMIT;
