# Plan: Routing Rules & Connectors

## 1. Event source field

Tag events at ingest with their origin so routing rules can filter by it.

Sources: `manual`, `caldav`, `google`, `outlook`, future: `cal-link`, etc.

Routing rule gets a `source` condition — fire only when event came from a specific origin.
Fixes the current `source_calendar` type which matches on CalendarID (fragile).

## 2. LLM categorization as a rule type

Add `llm` as a `RuleType` alongside `keyword`, `source_domain`, `source_calendar`.
Falls through to LLM when simpler rules don't match, or can be explicitly ordered by priority.

## 3. Multiple connectors per provider

Support multiple accounts of the same provider (e.g. personal Gmail + work Gmail).
Connections table needs a user-defined label/alias per connection.
Sync loop iterates all connections for a user rather than one-per-provider.
