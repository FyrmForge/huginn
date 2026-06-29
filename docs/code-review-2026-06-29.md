# Code Review — 2026-06-29

87 agents, 76 candidates, 15 confirmed. Deduplicated to 8 distinct issues.

---

## CRITICAL — CalDAV authorization broken at root

### 1. `internal/service/calendar.go:281` — GetByID skips membership check

`GetByID` calls `get(minRole="any")` which returns any calendar with no membership check. Every CalDAV handler (`GetEvent`, `PutEvent`, `Report`, `CalendarCollection`) uses this as its sole access gate. Any authenticated CalDAV user who knows a victim's calendar UUID can read, write, and enumerate all its events.

**Trigger:** Attacker has a valid app-password. Sends `PROPFIND /dav/calendars/{attackerID}/{victimCalID}/` — URL userID check passes (own ID), `GetByID` returns victim's calendar, all events returned.

---

### 2. `internal/caldav/handler.go:339` — DeleteEvent has no membership check

`DeleteEvent` doesn't call `GetByID` at all. Checks only that event's `CalendarID == calID` from URL. Any authenticated CalDAV user who knows an event UUID and its calendar UUID can delete it.

**Trigger:** `DELETE /dav/calendars/{attackerID}/{victimCalID}/{eventID}.ics` — no membership check, event deleted.

---

### 3. `internal/caldav/handler.go:48` — authUser ignores username from Basic Auth

Only the token (app-password) is used for identity lookup; the username field is discarded. A CalDAV client sending `username=alice` + `bob's token` authenticates as Bob.

**Trigger:** Any CalDAV client configured with wrong username — no validation, no warning.

---

### 4. `internal/caldav/handler.go:417` — handleMultiget cross-calendar event leak

Fetches events by client-supplied hrefs without verifying they belong to the queried calendar. A legitimate viewer of calendar A can query multiget with hrefs from calendar B to retrieve its events.

**Trigger:** Viewer of calendar A sends `REPORT` multiget to `/dav/calendars/self/{calAID}/` with hrefs pointing to events in calendar B.

---

## HIGH — Web handler authorization gaps

### 5. `internal/web/handler/events/handler.go:232` — ConfirmDelete IDOR leaks event titles

Fetches the event and renders its title in the HTML modal with no calendar membership check. Any authenticated user can leak private event titles by enumerating event UUIDs.

**Trigger:** `GET /events/{foreignEventID}/confirm-delete` — returns 200 with the event title.

---

### 6. `internal/web/handler/events/handler.go:273,317` — Update and Delete swallow GetByID error

Both `Update` and `Delete` do `e, _ := GetByID(...)` — error is discarded. On transient DB error `e == nil`, the auth check (`if e != nil && !canEditCalendar`) is skipped. If the DB recovers before the second fetch inside the service layer, the unauthorized write/delete succeeds.

**Trigger:** Transient DB error on first fetch → auth guard skipped → DB recovers → service-layer second fetch succeeds → unauthorized mutation written.

---

## MEDIUM

### 7. `internal/web/handler/auth/oidc/handler.go:35` — OIDC state cookie missing Secure flag

Session and flash cookies are correctly gated by `!devMode`, but the `_oidc_state` cookie is set with no `Secure` attribute. Transmitted over HTTP in any redirect step of the OIDC flow.

**Trigger:** Production deployment with HTTP→HTTPS redirect — state cookie sent in plaintext, capturable by network observer.

---

## LOW

### 8. `internal/caldav/handler.go:310` — UpsertCalDAVException errors silently discarded

`UpsertCalDAVException` called in a loop with `_ =`. Failed exception persists return 204 as if successful; recurring event exception data is silently lost.

**Trigger:** DB error during any per-occurrence exception persist in a `PUT` of a recurring event — client gets 204 but the exception was not saved.
