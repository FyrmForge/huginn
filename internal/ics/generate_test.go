package ics

import (
	"strings"
	"testing"
	"time"

	"github.com/FyrmForge/huginn/internal/repo"
)

func write(events ...*repo.Event) string {
	var sb strings.Builder
	_ = Write(&sb, "Test", events)
	return sb.String()
}

func TestWrite_Structure(t *testing.T) {
	out := write()
	mustContain(t, out, "BEGIN:VCALENDAR")
	mustContain(t, out, "VERSION:2.0")
	mustContain(t, out, "PRODID:")
	mustContain(t, out, "X-WR-CALNAME:Test")
	mustContain(t, out, "X-WR-TIMEZONE:UTC")
	mustContain(t, out, "END:VCALENDAR")
}

func TestWrite_Event_TimedEntry(t *testing.T) {
	now := time.Date(2026, 6, 29, 12, 0, 0, 0, time.UTC)
	e := &repo.Event{
		UID:       "test-uid@huginn",
		Title:     "Team Meeting",
		Status:    "confirmed",
		StartAt:   time.Date(2026, 6, 29, 14, 0, 0, 0, time.UTC),
		EndAt:     time.Date(2026, 6, 29, 15, 0, 0, 0, time.UTC),
		CreatedAt: now,
		UpdatedAt: now,
	}
	out := write(e)
	mustContain(t, out, "BEGIN:VEVENT")
	mustContain(t, out, "UID:test-uid@huginn")
	mustContain(t, out, "SUMMARY:Team Meeting")
	mustContain(t, out, "DTSTART:20260629T140000Z")
	mustContain(t, out, "DTEND:20260629T150000Z")
	mustContain(t, out, "STATUS:CONFIRMED")
	mustContain(t, out, "TRANSP:OPAQUE")
	mustContain(t, out, "DTSTAMP:")
	mustContain(t, out, "CREATED:")
	mustContain(t, out, "LAST-MODIFIED:")
	mustContain(t, out, "SEQUENCE:")
	mustContain(t, out, "END:VEVENT")
}

func TestWrite_Event_AllDay(t *testing.T) {
	e := &repo.Event{
		UID:     "allday@huginn",
		Title:   "Holiday",
		Status:  "confirmed",
		AllDay:  true,
		StartAt: time.Date(2026, 12, 25, 0, 0, 0, 0, time.UTC),
		EndAt:   time.Date(2026, 12, 26, 0, 0, 0, 0, time.UTC),
	}
	out := write(e)
	mustContain(t, out, "DTSTART;VALUE=DATE:20261225")
	mustContain(t, out, "DTEND;VALUE=DATE:20261226")
	if strings.Contains(out, "DTSTART:20261225T") || strings.Contains(out, "DTEND:20261226T") {
		t.Error("all-day DTSTART/DTEND must not include time component")
	}
}

func TestWrite_Event_FreeTransp(t *testing.T) {
	e := &repo.Event{
		UID:        "free@huginn",
		Status:     "confirmed",
		BusyStatus: "free",
		StartAt:    time.Now(),
		EndAt:      time.Now().Add(time.Hour),
	}
	out := write(e)
	mustContain(t, out, "TRANSP:TRANSPARENT")
}

func TestWrite_Event_BusyTransp(t *testing.T) {
	e := &repo.Event{
		UID:        "busy@huginn",
		Status:     "confirmed",
		BusyStatus: "busy",
		StartAt:    time.Now(),
		EndAt:      time.Now().Add(time.Hour),
	}
	out := write(e)
	mustContain(t, out, "TRANSP:OPAQUE")
}

func TestWrite_Event_Escaping(t *testing.T) {
	e := &repo.Event{
		UID:         "esc@huginn",
		Title:       "Meeting, Room 3; Conf",
		Description: "Bring lunch & notes\nSee you there",
		Status:      "confirmed",
		StartAt:     time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC),
		EndAt:       time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC),
	}
	out := write(e)
	mustContain(t, out, `SUMMARY:Meeting\, Room 3\; Conf`)
	mustContain(t, out, `lunch & notes\n`)
}

func TestWrite_Event_LineFolding(t *testing.T) {
	long := strings.Repeat("A", 100)
	e := &repo.Event{
		UID:         "fold@huginn",
		Title:       "Short",
		Description: long,
		Status:      "confirmed",
		StartAt:     time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC),
		EndAt:       time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC),
	}
	out := write(e)
	for _, line := range strings.Split(out, "\r\n") {
		if len(line) > 75 {
			t.Errorf("line exceeds 75 chars (len=%d): %q", len(line), line)
		}
	}
}

func TestWrite_Event_CREATED_LAST_MODIFIED(t *testing.T) {
	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	updated := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	e := &repo.Event{
		UID:       "timestamps@huginn",
		Status:    "confirmed",
		StartAt:   time.Now(),
		EndAt:     time.Now().Add(time.Hour),
		CreatedAt: created,
		UpdatedAt: updated,
	}
	out := write(e)
	mustContain(t, out, "CREATED:20260101T000000Z")
	mustContain(t, out, "LAST-MODIFIED:20260601T000000Z")
}

func TestWrite_MultipleEvents(t *testing.T) {
	e1 := &repo.Event{UID: "a@huginn", Title: "Alpha", Status: "confirmed", StartAt: time.Now(), EndAt: time.Now().Add(time.Hour)}
	e2 := &repo.Event{UID: "b@huginn", Title: "Beta", Status: "confirmed", StartAt: time.Now(), EndAt: time.Now().Add(time.Hour)}
	out := write(e1, e2)
	if strings.Count(out, "BEGIN:VEVENT") != 2 {
		t.Errorf("expected 2 VEVENT blocks, got %d", strings.Count(out, "BEGIN:VEVENT"))
	}
	mustContain(t, out, "SUMMARY:Alpha")
	mustContain(t, out, "SUMMARY:Beta")
}

func mustContain(t *testing.T, s, sub string) {
	t.Helper()
	if !strings.Contains(s, sub) {
		t.Errorf("output missing %q", sub)
	}
}
