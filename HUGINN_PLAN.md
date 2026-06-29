# Huginn Final Build Plan

## Product

Huginn is a self-hosted calendar companion to Muninn.

Principle:

```text
Muninn remembers.
Huginn plans.

Huginn replaces the calendar part of ownCloud/Nextcloud.

Core goals

- OIDC web login
- Multiple calendars per user
- Shared calendars between users
- User settings/config
- Event CRUD
- ICS import/export
- Native backup/restore
- CalDAV device sync
- Google Calendar import
- Outlook Calendar import
- Rule-based routing
- Later: model-based routing

Out of scope

- Sending invites
- RSVP handling
- Attendee tracking
- Free/busy scheduling
- Mail server integration
- Two-way sync back to Google/Outlook
- Public API
- Mobile/desktop apps


---

Architecture

Web UI / CalDAV / Imports / Jobs
        ↓
Services
        ↓
Repositories
        ↓
Postgres

Rules:

- Postgres is source of truth.
- CalDAV is only a sync protocol layer.
- Google/Outlook are import sources.
- Business logic lives in services.
- Handlers do not write directly to DB.
- Provider/protocol types must not leak into core domain.

Service examples:

UserService
SettingsService
CalendarService
EventService
SharingService
ImportService
RoutingService
CalDAVService
ExportService


---

Repo assumption

parent/
  muninn/
  huginn/

Before building Huginn UI:

- Inspect ../muninn
- Reuse its layout
- Reuse its styling
- Reuse navigation patterns
- Reuse components where practical

Huginn should feel like the calendar module/companion of Muninn.


---

Tech stack

Framework: HAMR
Backend: Go
Views: Templ
Interactivity: HTMX + Alpine.js
Database: Postgres
Auth: OIDC
Device sync: CalDAV
Jobs: Go/HAMR background jobs
External APIs:
  - Google Calendar API
  - Microsoft Graph

Use custom JS only where the calendar grid needs it.


---

Core data model

Tables:

users
user_settings
calendars
calendar_members
calendar_settings
calendar_sync_state
events
event_sources
sync_connections
sync_connection_tokens
oauth_provider_configs
import_jobs
routing_rules
routing_decisions
exports
audit_log
app_passwords

events

Store both parsed fields and raw ICS.

id
calendar_id
uid
title
description
location
start_at
end_at
timezone
all_day
status
visibility
busy_status
raw_ics
etag
version
ownership
created_by
updated_by
deleted_at
created_at
updated_at

Ownership values:

native
caldav_created
imported_readonly

MVP rule:

Google/Outlook imported events are read-only mirrors.
Native/CalDAV-created events are owned by Huginn.

event_sources

event_id
source_type
source_account_id
source_calendar_id
source_event_id
source_etag
source_updated_at
last_synced_at
payload_hash

Never rely on title/time as primary identity.

Use:

internal_id
iCalendar UID
source_event_id


---

Auth

Web UI

OIDC login

CalDAV

Use app passwords/device tokens.

Flow:

1. User logs into Huginn via OIDC
2. Settings → Devices / CalDAV
3. Create app password
4. Name it: iPhone / Thunderbird / DAVx5
5. Huginn shows token once
6. Device uses email + app password

Table:

app_passwords
- id
- user_id
- name
- token_hash
- permissions
- last_used_at
- revoked_at
- created_at

Rules:

- Store only hash
- Show token once
- Allow revoke
- Track last used
- Optional expiry later


---

OAuth for Google/Outlook

Self-hosted first.

Admin configures provider credentials:

Google client ID/secret
Microsoft client ID/secret

Later optional:

Official Huginn/FyrmForge OAuth app

Support both:

- instance-level OAuth credentials
- optional official app credentials later

Separate provider auth from sync:

type OAuthProvider interface {
    AuthorizeURL(...)
    Exchange(...)
    Refresh(...)
}

type CalendarConnector interface {
    ListCalendars(...)
    SyncEvents(...)
}

Connectors:

GoogleConnector
OutlookConnector
ICSConnector


---

Calendar model

Default calendars:

Personal
Work
Shared with Partner
Other Events

Calendar fields:

name
description
color
timezone
owner
routing_description
default_visibility
default_busy_status

Sharing roles:

Owner  = full control
Editor = create/edit/delete
Viewer = read-only

Later:

busy-only viewer
hide descriptions
hide locations


---

Settings

User settings:

timezone
locale
language
date_format
time_format
first_day_of_week
weekend_days
default_view
default_calendar_id
default_event_duration
default_reminder
visible_calendars
fallback_calendar_id
routing_confidence_threshold

Settings sections:

General
Calendar Display
Event Defaults
Calendars
Sharing
Imports
Routing
Devices / CalDAV
Import & Export
Account


---

Import/export

Native backup

Use versioned ZIP format.

huginn-backup.zip
  manifest.json
  account.json
  settings.json
  calendars.json
  calendar_members.json
  events.json
  routing_rules.json
  sources.json
  ics/
    personal.ics
    work.ics

Support:

export one calendar
export selected calendars
export full account
import as new calendar
merge into existing calendar
replace calendar
restore account

Version all formats:

backup format
config format
import/export format


---

CalDAV MVP

Required:

PROPFIND
REPORT calendar-query
REPORT sync-collection
GET
PUT
DELETE
ETags
sync tokens
calendar discovery
permissions

Use:

calendar_sync_state
- calendar_id
- ctag
- sync_token
- last_changed_at
- version
- updated_at

Supported:

- read events
- create events
- edit events
- delete events
- simple recurring events
- shared calendar permissions

Not supported:

- CalDAV scheduling
- invites
- RSVP
- attendee workflow

Test first with:

DAVx5 or Thunderbird

Apple Calendar later.


---

Third-party imports

Integration methods:

1. Full sync prototype
2. Incremental sync
3. Webhooks later as triggers only

Google:

initial full sync
then syncToken incremental sync

Outlook:

initial full sync
then Microsoft Graph delta query

Webhook rule:

Webhook only says “something changed”.
Always run incremental sync after webhook.

Imported events stay source-owned.


---

Routing

Start with rules.

Routing order:

1. Privacy rules
2. Manual overrides
3. Source account rules
4. Source calendar rules
5. Email/domain rules
6. Keyword rules
7. Fallback calendar
8. Later: model classifier

Router interface:

type Router interface {
    Route(ctx context.Context, event NormalizedEvent, calendars []Calendar) (RoutingDecision, error)
}

Implement:

RuleRouter
ModelRouter later
HybridRouter later

Always log decisions:

event_id
chosen_calendar_id
decision_type
confidence
reason
created_at

Fallback:

Other Events

Shared calendar rule:

Low-confidence events must never go to Shared with Partner.


---

Jobs

Imports and background work should be jobs, not long HTTP requests.

Jobs:

SyncGoogle
SyncOutlook
ImportICS
ExportBackup
RestoreBackup
CleanupDeletedEvents
ExpandRecurrence later
ScheduledBackup later


---

Event bus

Use internal events to avoid tight coupling.

Examples:

EventCreated
EventUpdated
EventDeleted
CalendarShared
ImportFinished
SyncFailed
BackupCreated

Consumers:

audit log
sync metadata
future notifications
future search/indexing


---

Build phases

Phase 0 — Foundation

- HAMR scaffold
- Project structure
- Migrations
- Services/repositories
- OIDC
- Basic layout
- Inspect ../muninn UI

Done when:

User can log in.
Project structure is clean.
Muninn UI conventions are documented.

Phase 1 — Core Calendar

- Users
- User settings
- Calendars
- Calendar sharing
- Event CRUD
- Month/week/day/agenda views

Done when:

Huginn works as a local web calendar.

Phase 2 — Portability

- Native backup export
- Native backup restore
- ICS import
- ICS export

Done when:

User can move calendar data in and out.

Phase 3 — Device Sync

- App passwords
- CalDAV discovery
- GET/PUT/DELETE
- PROPFIND
- REPORT
- ETags
- Sync tokens
- Permissions

Done when:

A phone/desktop calendar client can sync with Huginn.

Phase 4 — External Imports

- OAuth provider config
- Connector interface
- Google import
- Outlook import
- Incremental sync
- Source mapping

Done when:

Google/Outlook events appear in Huginn.

Phase 5 — Rule Routing

- Source rules
- Domain rules
- Keyword rules
- Manual overrides
- Fallback calendar
- Routing audit

Done when:

Imported events automatically route to correct calendars.

Phase 6 — Model Routing

- Model classifier
- Confidence threshold
- Reason logging
- Manual correction feedback

Done when:

Ambiguous imported events can be classified by AI.

Phase 7 — Polish

- Better recurrence handling
- event_instances cache if needed
- Timezone edge cases
- Mobile web polish
- Drag/drop
- Event resizing
- Apple Calendar compatibility
- Scheduled backups


---

Coding agent rules

Do not build everything at once.
Work in vertical slices.
Every phase must leave the app working.
Run tests/build after each phase.
Do not let handlers write directly to DB.
Do not leak CalDAV/Google/Outlook types into core services.
Store raw ICS plus parsed fields.
Keep imported Google/Outlook events read-only.
Prefer boring solutions.
Keep dependencies minimal.

For each feature:

1. Add migration
2. Add repository
3. Add service
4. Add handler
5. Add Templ/HTMX UI if needed
6. Add tests
7. Run build/tests


---

Final readiness

Ready to start development.

Remaining unknowns are implementation details, not blockers:

CalDAV client quirks
recurrence edge cases
timezone handling
Google/Outlook sync edge cases
Muninn UI specifics

Start Phase 0.
