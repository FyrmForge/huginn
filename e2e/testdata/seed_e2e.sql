-- E2E test seed data.
-- All test accounts use password: Test1234!
-- Argon2id hash (m=65536,t=3,p=2) with deterministic salt "e2e_test_salt!!!"

INSERT INTO users (id, email, password_hash, name, role, active, created_at, updated_at)
VALUES
    ('e2e-admin-001', 'admin@test.com', '$argon2id$v=19$m=65536,t=3,p=2$ZTJlX3Rlc3Rfc2FsdCEhIQ$bomHV69Vpgt6LTl83QWN02LhEdsWsb9+b6ZC2VYML9w', 'Test Admin', 'admin', true, NOW(), NOW()),
    ('e2e-user-001', 'user@test.com', '$argon2id$v=19$m=65536,t=3,p=2$ZTJlX3Rlc3Rfc2FsdCEhIQ$bomHV69Vpgt6LTl83QWN02LhEdsWsb9+b6ZC2VYML9w', 'Test User', 'user', true, NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- CalDAV test data for e2e-user-001.
-- App password plain token: huginn-e2e-caldav-token-001
-- SHA256 hash of the plain token (used for lookup).
INSERT INTO app_passwords (id, user_id, name, token_hash, permissions, created_at)
VALUES ('e2e-apppass-001', 'e2e-user-001', 'e2e-curl-test', '811b86357a7da1d26bb4bc5dff6d22be7a076c5d9e2dc14dc2223fc529a60072', 'caldav', NOW())
ON CONFLICT (id) DO NOTHING;

INSERT INTO calendars (id, owner_id, name, color, timezone, is_default, created_at, updated_at)
VALUES ('e2e-cal-001', 'e2e-user-001', 'E2E Calendar', '#4f8ef7', 'UTC', true, NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

INSERT INTO calendar_members (calendar_id, user_id, role)
VALUES ('e2e-cal-001', 'e2e-user-001', 'owner')
ON CONFLICT DO NOTHING;

INSERT INTO events (id, calendar_id, uid, title, description, start_at, end_at, timezone, status, etag, ownership, created_by, updated_by, created_at, updated_at)
VALUES
  ('e2e-event-001', 'e2e-cal-001', 'e2e-event-001@huginn', 'E2E Standup', 'Daily sync', '2026-06-29 10:00:00+00', '2026-06-29 10:30:00+00', 'UTC', 'confirmed', 'e2e-etag-001', 'native', 'e2e-user-001', 'e2e-user-001', NOW(), NOW()),
  ('e2e-event-002', 'e2e-cal-001', 'e2e-event-002@huginn', 'E2E Planning', 'Weekly planning', '2026-06-30 14:00:00+00', '2026-06-30 15:00:00+00', 'UTC', 'confirmed', 'e2e-etag-002', 'native', 'e2e-user-001', 'e2e-user-001', NOW(), NOW())
ON CONFLICT (id) DO NOTHING;
