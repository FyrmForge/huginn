-- Dev seed for huginn — plenty of example data.
--
-- Self-contained and idempotent: creates the dev user (matching DEV_AUTH_EMAIL),
-- two helper users for sharing, ~6 calendars, ~28 events (recurring / all-day /
-- multi-day / past / today / future / tentative / cancelled), recurrence
-- exceptions, user settings, devices, and routing rules.
--
-- All dates are anchored to the MONDAY OF THE CURRENT WEEK, so the month/week
-- views are always populated no matter when you run it.
--
-- Run: psql "$DATABASE_URL" -f scripts/seed-dev.sql
-- (or: ./scripts/db-shell.sh -f scripts/seed-dev.sql)

DO $$
DECLARE
  -- Users (looked up by email; created with these IDs only if absent)
  dev_id   TEXT;
  alice_id TEXT;
  bob_id   TEXT;

  -- Calendars (fixed IDs → deterministic, re-runnable)
  cal_personal TEXT := '11111111-1111-1111-1111-111111111101';
  cal_work     TEXT := '11111111-1111-1111-1111-111111111102';
  cal_health   TEXT := '11111111-1111-1111-1111-111111111103';
  cal_social   TEXT := '11111111-1111-1111-1111-111111111104';
  cal_travel   TEXT := '11111111-1111-1111-1111-111111111105';
  cal_alice    TEXT := '11111111-1111-1111-1111-1111111111a1'; -- owned by alice, shared to dev

  -- Anchor: Monday of the current week.
  wk DATE := date_trunc('week', CURRENT_DATE)::date;

  -- Master event id for the standup (we attach exceptions to it)
  ev_standup TEXT := '22222222-2222-2222-2222-222222222201';
BEGIN

-- ── Users ────────────────────────────────────────────────────────────────────
-- Reuse the existing dev user if dev-login already created it; otherwise create.
SELECT id INTO dev_id FROM users WHERE email = 'dev@huginn.local';
IF dev_id IS NULL THEN
  dev_id := '33333333-3333-3333-3333-333333333301';
  INSERT INTO users (id, email, password_hash, name, role, active)
  VALUES (dev_id, 'dev@huginn.local', 'NO_PASSWORD', 'Dev User', 'admin', true);
ELSE
  UPDATE users SET role = 'admin', name = 'Dev User' WHERE id = dev_id;
END IF;

SELECT id INTO alice_id FROM users WHERE email = 'alice@test.com';
IF alice_id IS NULL THEN
  alice_id := '33333333-3333-3333-3333-333333333302';
  INSERT INTO users (id, email, password_hash, name, role, active)
  VALUES (alice_id, 'alice@test.com', 'NO_PASSWORD', 'Alice', 'user', true);
END IF;

SELECT id INTO bob_id FROM users WHERE email = 'bob@test.com';
IF bob_id IS NULL THEN
  bob_id := '33333333-3333-3333-3333-333333333303';
  INSERT INTO users (id, email, password_hash, name, role, active)
  VALUES (bob_id, 'bob@test.com', 'NO_PASSWORD', 'Bob', 'user', true);
END IF;

-- ── Clean slate for seeded data (cascades events/members/exceptions/rules) ────
DELETE FROM calendars     WHERE owner_id IN (dev_id, alice_id) ;
DELETE FROM app_passwords WHERE user_id = dev_id;
DELETE FROM routing_rules WHERE user_id = dev_id;

-- ── Calendars ────────────────────────────────────────────────────────────────
INSERT INTO calendars (id, owner_id, name, description, color, timezone, is_default) VALUES
  (cal_personal, dev_id,   'Personal',     'Day-to-day life',        '#bf5af2', 'UTC', true),
  (cal_work,     dev_id,   'Work',         'Meetings & deadlines',   '#4f8ef7', 'UTC', false),
  (cal_health,   dev_id,   'Health',       'Gym, runs, appointments','#7dd181', 'UTC', false),
  (cal_social,   dev_id,   'Social',       'Friends & events',       '#f5a623', 'UTC', false),
  (cal_travel,   dev_id,   'Travel',       'Trips & flights',        '#f24d6d', 'UTC', false),
  (cal_alice,    alice_id, 'Alice''s Team','Shared by Alice',        '#e879f9', 'UTC', false);

-- Sharing: Alice's calendar → dev (viewer); dev's Work → alice (editor)
INSERT INTO calendar_members (calendar_id, user_id, role) VALUES
  (cal_alice, dev_id,   'viewer'),
  (cal_work,  alice_id, 'editor');

-- ── User settings ────────────────────────────────────────────────────────────
INSERT INTO user_settings (user_id, timezone, default_view, first_day_of_week, default_event_duration_mins)
VALUES (dev_id, 'UTC', 'month', 1, 60)
ON CONFLICT (user_id) DO UPDATE
  SET timezone = EXCLUDED.timezone, default_view = EXCLUDED.default_view;

-- ── Events ───────────────────────────────────────────────────────────────────
-- Helper note: timestamps are built from `wk` (Monday this week) + day/time offsets.

-- WORK ----------------------------------------------------------------------
-- Daily standup (Mon–Fri 09:00, recurring from 3 weeks ago) — has exceptions below.
INSERT INTO events (id, calendar_id, uid, title, description, location, start_at, end_at, timezone,
  all_day, status, visibility, busy_status, rrule, exdates, rdates, raw_ics, etag, ownership, created_by, updated_by)
VALUES (ev_standup, cal_work, gen_random_uuid()::text||'@huginn', 'Daily Standup',
  'Blockers, progress, plan', 'Zoom',
  (wk - 21)::timestamptz + interval '9 hours', (wk - 21)::timestamptz + interval '9 hours 15 minutes',
  'UTC', false, 'confirmed', 'private', 'busy',
  'FREQ=WEEKLY;BYDAY=MO,TU,WE,TH,FR', '', '', '', gen_random_uuid()::text, 'native', dev_id, dev_id);

INSERT INTO events (id, calendar_id, uid, title, description, location, start_at, end_at, timezone,
  all_day, status, visibility, busy_status, rrule, exdates, rdates, raw_ics, etag, ownership, created_by, updated_by)
VALUES
  (gen_random_uuid(), cal_work, gen_random_uuid()::text||'@huginn', 'Sprint Planning', 'Bi-weekly kick-off', 'Room 4',
    (wk)::timestamptz + interval '10 hours', (wk)::timestamptz + interval '11 hours 30 minutes',
    'UTC', false, 'confirmed', 'private', 'busy', 'FREQ=WEEKLY;BYDAY=MO;INTERVAL=2', '', '', '', gen_random_uuid()::text, 'native', dev_id, dev_id),
  (gen_random_uuid(), cal_work, gen_random_uuid()::text||'@huginn', '1:1 with Sarah', 'Career & projects', '',
    (wk - 7 + 2)::timestamptz + interval '14 hours', (wk - 7 + 2)::timestamptz + interval '14 hours 30 minutes',
    'UTC', false, 'confirmed', 'private', 'busy', 'FREQ=WEEKLY;BYDAY=WE', '', '', '', gen_random_uuid()::text, 'native', dev_id, dev_id),
  (gen_random_uuid(), cal_work, gen_random_uuid()::text||'@huginn', 'Architecture Review', 'Design & tech review', '',
    (wk - 14 + 3)::timestamptz + interval '16 hours', (wk - 14 + 3)::timestamptz + interval '17 hours',
    'UTC', false, 'confirmed', 'private', 'busy', 'FREQ=WEEKLY;BYDAY=TH', '', '', '', gen_random_uuid()::text, 'native', dev_id, dev_id),
  (gen_random_uuid(), cal_work, gen_random_uuid()::text||'@huginn', 'Sprint Review', 'Demo & retro', 'Room 4',
    (wk + 4)::timestamptz + interval '15 hours', (wk + 4)::timestamptz + interval '16 hours',
    'UTC', false, 'confirmed', 'private', 'busy', '', '', '', '', gen_random_uuid()::text, 'native', dev_id, dev_id),
  (gen_random_uuid(), cal_work, gen_random_uuid()::text||'@huginn', 'Quarterly Planning', 'Tentative — awaiting confirmation', '',
    (wk + 8)::timestamptz + interval '13 hours', (wk + 8)::timestamptz + interval '15 hours',
    'UTC', false, 'tentative', 'private', 'busy', '', '', '', '', gen_random_uuid()::text, 'native', dev_id, dev_id),
  (gen_random_uuid(), cal_work, gen_random_uuid()::text||'@huginn', 'Team Offsite', 'Engineering offsite — Lake Tahoe', 'Tahoe',
    (wk + 7)::timestamptz, (wk + 10)::timestamptz,
    'UTC', true, 'confirmed', 'private', 'busy', '', '', '', '', gen_random_uuid()::text, 'native', dev_id, dev_id);

-- PERSONAL ------------------------------------------------------------------
INSERT INTO events (id, calendar_id, uid, title, description, location, start_at, end_at, timezone,
  all_day, status, visibility, busy_status, rrule, exdates, rdates, raw_ics, etag, ownership, created_by, updated_by)
VALUES
  (gen_random_uuid(), cal_personal, gen_random_uuid()::text||'@huginn', 'Dentist', 'Annual check-up — Dr. Patel', 'Elm St Dental',
    (wk + 1)::timestamptz + interval '11 hours', (wk + 1)::timestamptz + interval '12 hours',
    'UTC', false, 'confirmed', 'private', 'busy', '', '', '', '', gen_random_uuid()::text, 'native', dev_id, dev_id),
  (gen_random_uuid(), cal_personal, gen_random_uuid()::text||'@huginn', 'Call Mom', '', '',
    (wk - 7 + 6)::timestamptz + interval '18 hours', (wk - 7 + 6)::timestamptz + interval '18 hours 30 minutes',
    'UTC', false, 'confirmed', 'private', 'free', 'FREQ=WEEKLY;BYDAY=SU', '', '', '', gen_random_uuid()::text, 'native', dev_id, dev_id),
  (gen_random_uuid(), cal_personal, gen_random_uuid()::text||'@huginn', 'Pay Rent', 'Monthly', '',
    date_trunc('month', CURRENT_DATE)::timestamptz, date_trunc('month', CURRENT_DATE)::timestamptz,
    'UTC', true, 'confirmed', 'private', 'free', 'FREQ=MONTHLY', '', '', '', gen_random_uuid()::text, 'native', dev_id, dev_id),
  (gen_random_uuid(), cal_personal, gen_random_uuid()::text||'@huginn', 'Car Service', 'Oil change & MOT', 'Joe''s Garage',
    (wk + 12)::timestamptz + interval '8 hours 30 minutes', (wk + 12)::timestamptz + interval '10 hours',
    'UTC', false, 'confirmed', 'private', 'busy', '', '', '', '', gen_random_uuid()::text, 'native', dev_id, dev_id);

-- HEALTH --------------------------------------------------------------------
INSERT INTO events (id, calendar_id, uid, title, description, location, start_at, end_at, timezone,
  all_day, status, visibility, busy_status, rrule, exdates, rdates, raw_ics, etag, ownership, created_by, updated_by)
VALUES
  (gen_random_uuid(), cal_health, gen_random_uuid()::text||'@huginn', 'Gym', 'Push / pull / legs', 'PureGym',
    (wk - 21)::timestamptz + interval '6 hours 30 minutes', (wk - 21)::timestamptz + interval '7 hours 30 minutes',
    'UTC', false, 'confirmed', 'private', 'busy', 'FREQ=WEEKLY;BYDAY=MO,WE,FR', '', '', '', gen_random_uuid()::text, 'native', dev_id, dev_id),
  (gen_random_uuid(), cal_health, gen_random_uuid()::text||'@huginn', 'Morning Run', '5k through the park', '',
    (wk - 14)::timestamptz + interval '7 hours', (wk - 14)::timestamptz + interval '7 hours 30 minutes',
    'UTC', false, 'confirmed', 'private', 'busy',
    'FREQ=DAILY;UNTIL='||to_char((wk + 28)::timestamptz, 'YYYYMMDD')||'T235959Z', '', '', '', gen_random_uuid()::text, 'native', dev_id, dev_id),
  (gen_random_uuid(), cal_health, gen_random_uuid()::text||'@huginn', 'Yoga', 'Vinyasa flow', 'Studio 12',
    (wk - 7 + 5)::timestamptz + interval '9 hours', (wk - 7 + 5)::timestamptz + interval '10 hours',
    'UTC', false, 'confirmed', 'private', 'busy', 'FREQ=WEEKLY;BYDAY=SA', '', '', '', gen_random_uuid()::text, 'native', dev_id, dev_id),
  (gen_random_uuid(), cal_health, gen_random_uuid()::text||'@huginn', 'Physio (cancelled)', 'Knee follow-up — cancelled', '',
    (wk + 2)::timestamptz + interval '17 hours', (wk + 2)::timestamptz + interval '17 hours 45 minutes',
    'UTC', false, 'cancelled', 'private', 'free', '', '', '', '', gen_random_uuid()::text, 'native', dev_id, dev_id);

-- SOCIAL --------------------------------------------------------------------
INSERT INTO events (id, calendar_id, uid, title, description, location, start_at, end_at, timezone,
  all_day, status, visibility, busy_status, rrule, exdates, rdates, raw_ics, etag, ownership, created_by, updated_by)
VALUES
  (gen_random_uuid(), cal_social, gen_random_uuid()::text||'@huginn', 'Book Club', 'Currently: Piranesi', 'Mia''s place',
    date_trunc('month', CURRENT_DATE)::timestamptz + interval '3 days 19 hours', date_trunc('month', CURRENT_DATE)::timestamptz + interval '3 days 21 hours',
    'UTC', false, 'confirmed', 'private', 'busy', 'FREQ=MONTHLY;BYDAY=1TH', '', '', '', gen_random_uuid()::text, 'native', dev_id, dev_id),
  (gen_random_uuid(), cal_social, gen_random_uuid()::text||'@huginn', 'Mia''s Birthday', 'Bring cake. Rooftop.', 'The Rooftop',
    (wk + 5)::timestamptz, (wk + 5)::timestamptz,
    'UTC', true, 'confirmed', 'private', 'busy', '', '', '', '', gen_random_uuid()::text, 'native', dev_id, dev_id),
  (gen_random_uuid(), cal_social, gen_random_uuid()::text||'@huginn', 'Dinner with Alex', '', 'Trattoria',
    (wk - 7 + 3)::timestamptz + interval '19 hours', (wk - 7 + 3)::timestamptz + interval '21 hours',
    'UTC', false, 'confirmed', 'private', 'busy', 'FREQ=WEEKLY;BYDAY=TH;INTERVAL=2', '', '', '', gen_random_uuid()::text, 'native', dev_id, dev_id),
  (gen_random_uuid(), cal_social, gen_random_uuid()::text||'@huginn', 'Concert — The National', 'Tentative, tickets pending', 'O2 Arena',
    (wk + 18)::timestamptz + interval '20 hours', (wk + 18)::timestamptz + interval '23 hours',
    'UTC', false, 'tentative', 'public', 'busy', '', '', '', '', gen_random_uuid()::text, 'native', dev_id, dev_id);

-- TRAVEL --------------------------------------------------------------------
INSERT INTO events (id, calendar_id, uid, title, description, location, start_at, end_at, timezone,
  all_day, status, visibility, busy_status, rrule, exdates, rdates, raw_ics, etag, ownership, created_by, updated_by)
VALUES
  (gen_random_uuid(), cal_travel, gen_random_uuid()::text||'@huginn', 'Flight → NYC', 'BA117, Terminal 5', 'LHR',
    (wk + 13)::timestamptz + interval '7 hours 40 minutes', (wk + 13)::timestamptz + interval '15 hours 30 minutes',
    'UTC', false, 'confirmed', 'private', 'busy', '', '', '', '', gen_random_uuid()::text, 'native', dev_id, dev_id),
  (gen_random_uuid(), cal_travel, gen_random_uuid()::text||'@huginn', 'DevConf', 'Talks + workshops', 'New York',
    (wk + 14)::timestamptz, (wk + 17)::timestamptz,
    'UTC', true, 'confirmed', 'private', 'busy', '', '', '', '', gen_random_uuid()::text, 'native', dev_id, dev_id),
  (gen_random_uuid(), cal_travel, gen_random_uuid()::text||'@huginn', 'Weekend Trip — Cotswolds', '', '',
    (wk + 5)::timestamptz, (wk + 7)::timestamptz,
    'UTC', true, 'confirmed', 'private', 'free', '', '', '', '', gen_random_uuid()::text, 'native', dev_id, dev_id);

-- ALICE'S TEAM (shared → dev sees these read-only) --------------------------
INSERT INTO events (id, calendar_id, uid, title, description, location, start_at, end_at, timezone,
  all_day, status, visibility, busy_status, rrule, exdates, rdates, raw_ics, etag, ownership, created_by, updated_by)
VALUES
  (gen_random_uuid(), cal_alice, gen_random_uuid()::text||'@huginn', 'Team Sync', 'Alice''s team weekly', '',
    (wk - 7 + 1)::timestamptz + interval '13 hours', (wk - 7 + 1)::timestamptz + interval '13 hours 45 minutes',
    'UTC', false, 'confirmed', 'private', 'busy', 'FREQ=WEEKLY;BYDAY=TU', '', '', '', gen_random_uuid()::text, 'native', alice_id, alice_id),
  (gen_random_uuid(), cal_alice, gen_random_uuid()::text||'@huginn', 'Release Day', 'v2.0 ships', '',
    (wk + 11)::timestamptz, (wk + 11)::timestamptz,
    'UTC', true, 'confirmed', 'private', 'busy', '', '', '', '', gen_random_uuid()::text, 'native', alice_id, alice_id);

-- ── Recurrence exceptions on the Daily Standup ───────────────────────────────
-- This Wednesday: moved to 09:30 (modified occurrence)
INSERT INTO event_exceptions (id, master_event_id, recurrence_id, title, description, location,
  start_at, end_at, timezone, all_day, status, is_deleted, etag)
VALUES (gen_random_uuid(), ev_standup,
  (wk + 2)::timestamptz + interval '9 hours', 'Daily Standup (moved)', 'Pushed back 30 min', 'Zoom',
  (wk + 2)::timestamptz + interval '9 hours 30 minutes', (wk + 2)::timestamptz + interval '9 hours 45 minutes',
  'UTC', false, 'confirmed', false, gen_random_uuid()::text);

-- This Friday: cancelled (deleted occurrence)
INSERT INTO event_exceptions (id, master_event_id, recurrence_id, title, description, location,
  start_at, end_at, timezone, all_day, status, is_deleted, etag)
VALUES (gen_random_uuid(), ev_standup,
  (wk + 4)::timestamptz + interval '9 hours', '', '', '',
  (wk + 4)::timestamptz + interval '9 hours', (wk + 4)::timestamptz + interval '9 hours 15 minutes',
  'UTC', false, 'cancelled', true, gen_random_uuid()::text);

-- ── Devices (app passwords) — token hashes are placeholders, not real tokens ──
INSERT INTO app_passwords (id, user_id, name, token_hash, permissions, last_used_at, created_at) VALUES
  (gen_random_uuid(), dev_id, 'iPhone',     repeat('a', 64), 'caldav', now() - interval '2 hours', now() - interval '20 days'),
  (gen_random_uuid(), dev_id, 'Thunderbird', repeat('b', 64), 'caldav', now() - interval '5 days', now() - interval '40 days');
-- One revoked device
INSERT INTO app_passwords (id, user_id, name, token_hash, permissions, revoked_at, created_at)
VALUES (gen_random_uuid(), dev_id, 'Old Laptop', repeat('c', 64), 'caldav', now() - interval '10 days', now() - interval '90 days');

-- ── Routing rules ────────────────────────────────────────────────────────────
INSERT INTO routing_rules (id, user_id, name, rule_type, match_value, case_sensitive, target_calendar_id, priority, enabled) VALUES
  (gen_random_uuid(), dev_id, 'Standups → Work',     'keyword',       'standup',     false, cal_work,   10, true),
  (gen_random_uuid(), dev_id, 'Company mail → Work',  'source_domain', 'company.com', false, cal_work,   20, true),
  (gen_random_uuid(), dev_id, 'Flights → Travel',     'keyword',       'flight',      false, cal_travel, 30, true);

RAISE NOTICE 'Seed complete for % (admin). Calendars: 6, plus shared. Events anchored to week of %.', 'dev@huginn.local', wk;
END $$;
