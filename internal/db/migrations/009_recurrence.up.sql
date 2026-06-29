BEGIN;

ALTER TABLE events ADD COLUMN rrule TEXT NOT NULL DEFAULT '';
ALTER TABLE events ADD COLUMN exdates TEXT NOT NULL DEFAULT '';
ALTER TABLE events ADD COLUMN rdates TEXT NOT NULL DEFAULT '';

CREATE INDEX idx_events_rrule ON events(calendar_id) WHERE rrule <> '' AND deleted_at IS NULL;

CREATE TABLE event_exceptions (
    id              TEXT PRIMARY KEY,
    master_event_id TEXT NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    recurrence_id   TIMESTAMPTZ NOT NULL,  -- original occurrence start (RFC 5545 RECURRENCE-ID)
    title           TEXT NOT NULL DEFAULT '',
    description     TEXT NOT NULL DEFAULT '',
    location        TEXT NOT NULL DEFAULT '',
    start_at        TIMESTAMPTZ NOT NULL,
    end_at          TIMESTAMPTZ NOT NULL,
    timezone        TEXT NOT NULL DEFAULT 'UTC',
    all_day         BOOLEAN NOT NULL DEFAULT false,
    status          TEXT NOT NULL DEFAULT 'confirmed',
    is_deleted      BOOLEAN NOT NULL DEFAULT false, -- true = this occurrence is cancelled (EXDATE)
    etag            TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_event_exceptions_unique ON event_exceptions(master_event_id, recurrence_id);
CREATE INDEX idx_event_exceptions_master ON event_exceptions(master_event_id);

COMMIT;
