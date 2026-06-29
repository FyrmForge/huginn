-- Dev seed data for huginn. Anchored to 2026-06-29 (Monday).
-- Run: psql $DATABASE_URL -f scripts/seed-dev.sql

DO $$ DECLARE
  uid TEXT := 'c0b2769a-4a87-4428-b11b-803df72d3a1f';
  cal_personal TEXT := '457a32e5-d147-46b2-90f0-651c1ca8300f';
  cal_work     TEXT := 'b9281edc-2346-43ff-8a56-d9d658a8a5f2';
  cal_other    TEXT := 'cb5889e4-bde1-4f09-bb4f-4ea6d31d5a4f';
BEGIN

-- Delete existing seed events to make idempotent
DELETE FROM events WHERE created_by = 'c0b2769a-4a87-4428-b11b-803df72d3a1f' AND ownership = 'native';

-- ── WORK ──────────────────────────────────────────────────────────────────────

-- Daily standup (Mon-Fri, 9:00-9:15am)
INSERT INTO events (id, calendar_id, uid, title, description, start_at, end_at, timezone, all_day,
  status, visibility, busy_status, rrule, exdates, rdates, raw_ics, etag, ownership, created_by, updated_by, created_at, updated_at)
VALUES (
  gen_random_uuid(), cal_work, gen_random_uuid()::text || '@huginn',
  'Daily Standup', 'Quick team sync — blockers, progress, plan',
  '2026-06-22 09:00:00+00', '2026-06-22 09:15:00+00',
  'UTC', false, 'confirmed', 'private', 'busy',
  'FREQ=WEEKLY;BYDAY=MO,TU,WE,TH,FR;INTERVAL=1', '', '', '', gen_random_uuid()::text,
  'native', uid, uid, now(), now()
);

-- Sprint planning (bi-weekly Monday, 10:00-11:30am)
INSERT INTO events (id, calendar_id, uid, title, description, start_at, end_at, timezone, all_day,
  status, visibility, busy_status, rrule, exdates, rdates, raw_ics, etag, ownership, created_by, updated_by, created_at, updated_at)
VALUES (
  gen_random_uuid(), cal_work, gen_random_uuid()::text || '@huginn',
  'Sprint Planning', 'Bi-weekly sprint kick-off',
  '2026-06-22 10:00:00+00', '2026-06-22 11:30:00+00',
  'UTC', false, 'confirmed', 'private', 'busy',
  'FREQ=WEEKLY;BYDAY=MO;INTERVAL=2', '', '', '', gen_random_uuid()::text,
  'native', uid, uid, now(), now()
);

-- 1:1 with Sarah (weekly Wednesday, 2:00-2:30pm)
INSERT INTO events (id, calendar_id, uid, title, description, start_at, end_at, timezone, all_day,
  status, visibility, busy_status, rrule, exdates, rdates, raw_ics, etag, ownership, created_by, updated_by, created_at, updated_at)
VALUES (
  gen_random_uuid(), cal_work, gen_random_uuid()::text || '@huginn',
  '1:1 with Sarah', 'Weekly 1:1 — career, projects, feedback',
  '2026-06-24 14:00:00+00', '2026-06-24 14:30:00+00',
  'UTC', false, 'confirmed', 'private', 'busy',
  'FREQ=WEEKLY;BYDAY=WE;INTERVAL=1', '', '', '', gen_random_uuid()::text,
  'native', uid, uid, now(), now()
);

-- Product Review (one-time, this Friday Jul 3 at 3pm)
INSERT INTO events (id, calendar_id, uid, title, description, start_at, end_at, timezone, all_day,
  status, visibility, busy_status, rrule, exdates, rdates, raw_ics, etag, ownership, created_by, updated_by, created_at, updated_at)
VALUES (
  gen_random_uuid(), cal_work, gen_random_uuid()::text || '@huginn',
  'Product Review', 'Q3 product roadmap review with leadership',
  '2026-07-03 15:00:00+00', '2026-07-03 16:30:00+00',
  'UTC', false, 'confirmed', 'private', 'busy',
  '', '', '', '', gen_random_uuid()::text,
  'native', uid, uid, now(), now()
);

-- Team Offsite (all-day Mon-Wed next week Jul 6-8)
INSERT INTO events (id, calendar_id, uid, title, description, start_at, end_at, timezone, all_day,
  status, visibility, busy_status, rrule, exdates, rdates, raw_ics, etag, ownership, created_by, updated_by, created_at, updated_at)
VALUES (
  gen_random_uuid(), cal_work, gen_random_uuid()::text || '@huginn',
  'Team Offsite', 'Engineering offsite — Lake Tahoe',
  '2026-07-06 00:00:00+00', '2026-07-08 00:00:00+00',
  'UTC', true, 'confirmed', 'private', 'busy',
  '', '', '', '', gen_random_uuid()::text,
  'native', uid, uid, now(), now()
);

-- Architecture Review (weekly Thursday, 4pm-5pm)
INSERT INTO events (id, calendar_id, uid, title, description, start_at, end_at, timezone, all_day,
  status, visibility, busy_status, rrule, exdates, rdates, raw_ics, etag, ownership, created_by, updated_by, created_at, updated_at)
VALUES (
  gen_random_uuid(), cal_work, gen_random_uuid()::text || '@huginn',
  'Architecture Review', 'Weekly design & tech review',
  '2026-06-25 16:00:00+00', '2026-06-25 17:00:00+00',
  'UTC', false, 'confirmed', 'private', 'busy',
  'FREQ=WEEKLY;BYDAY=TH;INTERVAL=1', '', '', '', gen_random_uuid()::text,
  'native', uid, uid, now(), now()
);

-- ── PERSONAL ─────────────────────────────────────────────────────────────────

-- Gym (Mon / Wed / Fri, 6:30-7:30am)
INSERT INTO events (id, calendar_id, uid, title, description, start_at, end_at, timezone, all_day,
  status, visibility, busy_status, rrule, exdates, rdates, raw_ics, etag, ownership, created_by, updated_by, created_at, updated_at)
VALUES (
  gen_random_uuid(), cal_personal, gen_random_uuid()::text || '@huginn',
  'Gym', 'Push/pull/legs split',
  '2026-06-22 06:30:00+00', '2026-06-22 07:30:00+00',
  'UTC', false, 'confirmed', 'private', 'busy',
  'FREQ=WEEKLY;BYDAY=MO,WE,FR;INTERVAL=1', '', '', '', gen_random_uuid()::text,
  'native', uid, uid, now(), now()
);

-- Dentist appointment (one-time, next Tuesday Jul 1 at 11am)
INSERT INTO events (id, calendar_id, uid, title, description, start_at, end_at, timezone, all_day,
  status, visibility, busy_status, rrule, exdates, rdates, raw_ics, etag, ownership, created_by, updated_by, created_at, updated_at)
VALUES (
  gen_random_uuid(), cal_personal, gen_random_uuid()::text || '@huginn',
  'Dentist', 'Annual check-up — Dr. Patel, Elm St Dental',
  '2026-07-01 11:00:00+00', '2026-07-01 12:00:00+00',
  'UTC', false, 'confirmed', 'private', 'busy',
  '', '', '', '', gen_random_uuid()::text,
  'native', uid, uid, now(), now()
);

-- Lunch with Alex (bi-weekly Thursday, 12:30-1:30pm)
INSERT INTO events (id, calendar_id, uid, title, description, start_at, end_at, timezone, all_day,
  status, visibility, busy_status, rrule, exdates, rdates, raw_ics, etag, ownership, created_by, updated_by, created_at, updated_at)
VALUES (
  gen_random_uuid(), cal_personal, gen_random_uuid()::text || '@huginn',
  'Lunch with Alex', '',
  '2026-06-25 12:30:00+00', '2026-06-25 13:30:00+00',
  'UTC', false, 'confirmed', 'private', 'free',
  'FREQ=WEEKLY;BYDAY=TH;INTERVAL=2', '', '', '', gen_random_uuid()::text,
  'native', uid, uid, now(), now()
);

-- Grocery shopping (every Saturday, 10-11am)
INSERT INTO events (id, calendar_id, uid, title, description, start_at, end_at, timezone, all_day,
  status, visibility, busy_status, rrule, exdates, rdates, raw_ics, etag, ownership, created_by, updated_by, created_at, updated_at)
VALUES (
  gen_random_uuid(), cal_personal, gen_random_uuid()::text || '@huginn',
  'Groceries', '',
  '2026-06-27 10:00:00+00', '2026-06-27 11:00:00+00',
  'UTC', false, 'confirmed', 'private', 'free',
  'FREQ=WEEKLY;BYDAY=SA;INTERVAL=1', '', '', '', gen_random_uuid()::text,
  'native', uid, uid, now(), now()
);

-- Morning run (every day, 7am-7:30am, starting today)
INSERT INTO events (id, calendar_id, uid, title, description, start_at, end_at, timezone, all_day,
  status, visibility, busy_status, rrule, exdates, rdates, raw_ics, etag, ownership, created_by, updated_by, created_at, updated_at)
VALUES (
  gen_random_uuid(), cal_personal, gen_random_uuid()::text || '@huginn',
  'Morning Run', '5k through the park',
  '2026-06-15 07:00:00+00', '2026-06-15 07:30:00+00',
  'UTC', false, 'confirmed', 'private', 'busy',
  'FREQ=DAILY;INTERVAL=1;UNTIL=20260731T235959Z', '', '', '', gen_random_uuid()::text,
  'native', uid, uid, now(), now()
);

-- ── OTHER EVENTS ──────────────────────────────────────────────────────────────

-- Book Club (monthly, first Thursday, 7pm-9pm)
INSERT INTO events (id, calendar_id, uid, title, description, start_at, end_at, timezone, all_day,
  status, visibility, busy_status, rrule, exdates, rdates, raw_ics, etag, ownership, created_by, updated_by, created_at, updated_at)
VALUES (
  gen_random_uuid(), cal_other, gen_random_uuid()::text || '@huginn',
  'Book Club', 'Currently reading: Piranesi',
  '2026-06-04 19:00:00+00', '2026-06-04 21:00:00+00',
  'UTC', false, 'confirmed', 'private', 'busy',
  'FREQ=MONTHLY;BYDAY=1TH;INTERVAL=1', '', '', '', gen_random_uuid()::text,
  'native', uid, uid, now(), now()
);

-- Mia's Birthday Party (one-time, next Saturday Jul 4)
INSERT INTO events (id, calendar_id, uid, title, description, start_at, end_at, timezone, all_day,
  status, visibility, busy_status, rrule, exdates, rdates, raw_ics, etag, ownership, created_by, updated_by, created_at, updated_at)
VALUES (
  gen_random_uuid(), cal_other, gen_random_uuid()::text || '@huginn',
  'Mia''s Birthday', 'Bring cake. Rooftop at 7pm.',
  '2026-07-04 19:00:00+00', '2026-07-04 22:00:00+00',
  'UTC', false, 'confirmed', 'private', 'busy',
  '', '', '', '', gen_random_uuid()::text,
  'native', uid, uid, now(), now()
);

-- Summer Solstice Hike (all-day, already passed but visible in month view)
INSERT INTO events (id, calendar_id, uid, title, description, start_at, end_at, timezone, all_day,
  status, visibility, busy_status, rrule, exdates, rdates, raw_ics, etag, ownership, created_by, updated_by, created_at, updated_at)
VALUES (
  gen_random_uuid(), cal_other, gen_random_uuid()::text || '@huginn',
  'Solstice Hike', 'Mt. Tamalpais sunrise hike',
  '2026-06-21 00:00:00+00', '2026-06-21 00:00:00+00',
  'UTC', true, 'confirmed', 'private', 'free',
  '', '', '', '', gen_random_uuid()::text,
  'native', uid, uid, now(), now()
);

END $$;
