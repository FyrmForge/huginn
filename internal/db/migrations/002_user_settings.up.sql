BEGIN;
CREATE TABLE user_settings (
    user_id                     TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    timezone                    TEXT NOT NULL DEFAULT 'UTC',
    locale                      TEXT NOT NULL DEFAULT 'en',
    date_format                 TEXT NOT NULL DEFAULT 'YYYY-MM-DD',
    time_format                 TEXT NOT NULL DEFAULT '24h',
    first_day_of_week           INT  NOT NULL DEFAULT 1, -- 0=Sun, 1=Mon
    default_view                TEXT NOT NULL DEFAULT 'month',
    default_event_duration_mins INT  NOT NULL DEFAULT 60,
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
COMMIT;
