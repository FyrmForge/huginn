BEGIN;
CREATE TABLE events (
    id          TEXT PRIMARY KEY,
    calendar_id TEXT NOT NULL REFERENCES calendars(id) ON DELETE CASCADE,
    uid         TEXT NOT NULL,                  -- iCalendar UID
    title       TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    location    TEXT NOT NULL DEFAULT '',
    start_at    TIMESTAMPTZ NOT NULL,
    end_at      TIMESTAMPTZ NOT NULL,
    timezone    TEXT NOT NULL DEFAULT 'UTC',
    all_day     BOOLEAN NOT NULL DEFAULT false,
    status      TEXT NOT NULL DEFAULT 'confirmed', -- confirmed, tentative, cancelled
    visibility  TEXT NOT NULL DEFAULT 'private',
    busy_status TEXT NOT NULL DEFAULT 'busy',
    raw_ics     TEXT NOT NULL DEFAULT '',
    etag        TEXT NOT NULL DEFAULT '',
    ownership   TEXT NOT NULL DEFAULT 'native',  -- native, caldav_created, imported_readonly
    created_by  TEXT REFERENCES users(id),
    updated_by  TEXT REFERENCES users(id),
    deleted_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_events_calendar_id ON events(calendar_id);
CREATE INDEX idx_events_start_at    ON events(start_at);
CREATE INDEX idx_events_uid         ON events(uid);
-- Partial index for non-deleted events (the common query path).
CREATE INDEX idx_events_active ON events(calendar_id, start_at) WHERE deleted_at IS NULL;

CREATE TABLE event_sources (
    event_id          TEXT PRIMARY KEY REFERENCES events(id) ON DELETE CASCADE,
    source_type       TEXT NOT NULL, -- google, outlook, ics, caldav
    source_account_id TEXT NOT NULL DEFAULT '',
    source_calendar_id TEXT NOT NULL DEFAULT '',
    source_event_id   TEXT NOT NULL,
    source_etag       TEXT NOT NULL DEFAULT '',
    source_updated_at TIMESTAMPTZ,
    last_synced_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    payload_hash      TEXT NOT NULL DEFAULT ''
);

CREATE INDEX idx_event_sources_source_event_id ON event_sources(source_type, source_event_id);
COMMIT;
