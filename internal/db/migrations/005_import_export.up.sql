BEGIN;
CREATE TABLE import_jobs (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    source_type TEXT NOT NULL, -- ics, google, outlook, backup
    status      TEXT NOT NULL DEFAULT 'pending', -- pending, running, done, failed
    calendar_id TEXT REFERENCES calendars(id) ON DELETE SET NULL,
    filename    TEXT NOT NULL DEFAULT '',
    error       TEXT NOT NULL DEFAULT '',
    events_imported INT NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_import_jobs_user_id ON import_jobs(user_id);

CREATE TABLE exports (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    export_type TEXT NOT NULL, -- ics, backup
    status      TEXT NOT NULL DEFAULT 'pending',
    filename    TEXT NOT NULL DEFAULT '',
    file_path   TEXT NOT NULL DEFAULT '',
    error       TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_exports_user_id ON exports(user_id);
COMMIT;
