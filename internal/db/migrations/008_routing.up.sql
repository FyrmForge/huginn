BEGIN;
-- User-defined routing rules for auto-filing imported events.
CREATE TABLE routing_rules (
    id              TEXT PRIMARY KEY,
    user_id         TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    -- Rule type: source_domain, keyword, source_calendar
    rule_type       TEXT NOT NULL,
    -- The value to match against (e.g. "company.com", "standup", "Work Events (google)")
    match_value     TEXT NOT NULL,
    -- Case-insensitive flag for keyword matching
    case_sensitive  BOOLEAN NOT NULL DEFAULT FALSE,
    -- Target calendar to route matched events into
    target_calendar_id TEXT NOT NULL REFERENCES calendars(id) ON DELETE CASCADE,
    -- Priority: lower number = higher priority
    priority        INT NOT NULL DEFAULT 100,
    enabled         BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_routing_rules_user_id ON routing_rules(user_id);

-- Audit log: records which rule (if any) routed each imported event.
CREATE TABLE routing_audit (
    id          TEXT PRIMARY KEY,
    event_id    TEXT NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    rule_id     TEXT REFERENCES routing_rules(id) ON DELETE SET NULL,
    reason      TEXT NOT NULL DEFAULT '', -- human-readable explanation
    routed_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_routing_audit_event_id ON routing_audit(event_id);
COMMIT;
