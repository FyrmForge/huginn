- [ ] server wide timezone through env
- [ ] setting another cal as default


## macOS calendar push sync (cmd/huginn-sync-mac) — needs more work, future
First version is in (one-way Calendar.app → huginn). Not ready for real use
until these are fixed (code review 2026-10-26):
- [ ] **Sync loop:** the agent pushes every Calendar.app calendar, including
      ones Calendar.app gets from huginn over CalDAV, so mirrors get mirrored
      again every run and calendars multiply. Skip calendars whose source is
      the huginn server, and keep mirror calendars out of the CalDAV listing.
- [ ] **Prune deletes user events:** `DeleteEventsNotIn` has no ownership
      filter, so an event added to a mirror calendar (web UI / CalDAV) is
      deleted on the next sync. Prune only `imported_readonly`, or reject
      writes to calendars with an `external_id`.
- [ ] **All-day events a day early east of UTC:** the agent sends local
      midnight as a UTC instant; huginn stores all-day as UTC-midnight dates.
      Send the calendar date at 00:00Z for all-day events.
- [ ] **Large calendars:** whole-calendar snapshot hits hamr's 2MB body limit
      / 30s timeout; a timeout leaves a half-applied sync with no prune.
      Batch the upload or raise the limits on this route.
- [ ] **>4 year window:** EventKit silently truncates ranges over 4 years
      while the snapshot still claims the full window, so the tail gets
      pruned. Clamp `to` to `from + 4y` or query in chunks.
- [ ] Not yet run against a real Postgres or compiled on macOS.
