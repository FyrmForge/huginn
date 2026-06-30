package caldav

import (
	"strings"
	"testing"
	"time"

	"github.com/FyrmForge/huginn/internal/repo"
)

// --- xmlEscape ---

func TestXmlEscape(t *testing.T) {
	cases := []struct{ in, want string }{
		{"plain", "plain"},
		{"a&b", "a&amp;b"},
		{"<tag>", "&lt;tag&gt;"},
		{`say "hi"`, "say &#34;hi&#34;"},
		{"it's", "it&#39;s"},
	}
	for _, c := range cases {
		got := xmlEscape(c.in)
		if got != c.want {
			t.Errorf("xmlEscape(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// --- propfindPrincipal ---

func TestPropfindPrincipal(t *testing.T) {
	out := propfindPrincipal("user-123")
	mustContain(t, out, "/dav/calendars/user-123/")
	mustContain(t, out, "<principal/>")
	mustContain(t, out, "calendar-home-set")
	mustContain(t, out, "current-user-principal")
	mustContain(t, out, "principal-URL")
	mustContain(t, out, "schedule-inbox-URL")
	mustContain(t, out, "schedule-outbox-URL")
	mustContain(t, out, "addressbook-home-set")
	mustContain(t, out, "supported-report-set")
}

// --- propfindCalendarHome ---

func TestPropfindCalendarHome_Depth0(t *testing.T) {
	out := propfindCalendarHomeDepth0("uid-1")
	mustContain(t, out, "/dav/calendars/uid-1/")
	mustContain(t, out, "<collection/>")
	if strings.Contains(out, "calendar-color") {
		t.Error("depth:0 must not list individual calendars")
	}
}

func TestPropfindCalendarHome_WithCalendars(t *testing.T) {
	cals := []*repo.Calendar{
		{ID: "cal-a", Name: "Work", Color: "#ff0000", UpdatedAt: time.Now()},
		{ID: "cal-b", Name: "Personal & Family", Color: "#00ff00", UpdatedAt: time.Now()},
	}
	out := propfindCalendarHome("uid-1", cals)
	mustContain(t, out, "/dav/calendars/uid-1/cal-a/")
	mustContain(t, out, "/dav/calendars/uid-1/cal-b/")
	mustContain(t, out, "Personal &amp; Family")
	mustContain(t, out, "<C:calendar/>")
	mustContain(t, out, "CS:getctag")
	mustContain(t, out, "A:calendar-color")
	mustContain(t, out, "supported-calendar-component-set")
}

// --- propfindCalendar ---

func TestPropfindCalendar_Depth0(t *testing.T) {
	cal := &repo.Calendar{ID: "cal-1", Name: "Test", Color: "#abc", UpdatedAt: time.Now()}
	out := propfindCalendarDepth0("uid-1", cal, time.Now())
	mustContain(t, out, "/dav/calendars/uid-1/cal-1/")
	mustContain(t, out, "CS:getctag")
	mustContain(t, out, "sync-token")
	mustContain(t, out, "huginn.local/ns/sync/cal-1/")
	mustContain(t, out, "supported-calendar-component-set")
	mustContain(t, out, "supported-report-set")
	mustContain(t, out, "A:calendar-color")
	if strings.Contains(out, ".ics") {
		t.Error("depth:0 must not list event resources")
	}
}

func TestPropfindCalendar_Depth1_NoEvents(t *testing.T) {
	cal := &repo.Calendar{ID: "cal-1", Name: "Test", UpdatedAt: time.Now()}
	out := propfindCalendar("uid-1", cal, nil)
	mustContain(t, out, "/dav/calendars/uid-1/cal-1/")
	mustContain(t, out, "<C:calendar/>")
	mustContain(t, out, "CS:getctag")
	mustContain(t, out, "sync-token")
	if strings.Contains(out, ".ics") {
		t.Error("expected no event entries with nil events")
	}
}

func TestPropfindCalendar_Depth1_WithEvents(t *testing.T) {
	cal := &repo.Calendar{ID: "cal-1", Name: "Test", UpdatedAt: time.Now()}
	events := []*repo.Event{
		{ID: "ev-1", ETag: "etag-abc", StartAt: time.Now(), EndAt: time.Now().Add(time.Hour), UpdatedAt: time.Now()},
		{ID: "ev-2", ETag: "etag-xyz", StartAt: time.Now(), EndAt: time.Now().Add(time.Hour), UpdatedAt: time.Now()},
	}
	out := propfindCalendar("uid-1", cal, events)
	mustContain(t, out, "/dav/calendars/uid-1/cal-1/ev-1.ics")
	mustContain(t, out, "/dav/calendars/uid-1/cal-1/ev-2.ics")
	mustContain(t, out, `"etag-abc"`)
	mustContain(t, out, `"etag-xyz"`)
	mustContain(t, out, "getcontenttype")
	// PROPFIND must NOT inline ICS content
	if strings.Contains(out, "BEGIN:VCALENDAR") {
		t.Error("PROPFIND must not include inline ICS data")
	}
}

// --- propfindEventResource ---

func TestPropfindEventResource(t *testing.T) {
	e := &repo.Event{ID: "ev-1", ETag: "etag-001"}
	out := propfindEventResource("uid-1", "cal-1", e)
	mustContain(t, out, "/dav/calendars/uid-1/cal-1/ev-1.ics")
	mustContain(t, out, `"etag-001"`)
	mustContain(t, out, "text/calendar")
	mustContain(t, out, "<resourcetype/>")
}

// --- reportCalendarQuery ---

func TestReportCalendarQuery_IncludesICS(t *testing.T) {
	cal := &repo.Calendar{ID: "cal-1", Name: "Test"}
	events := []*repo.Event{{
		ID:      "ev-1",
		ETag:    "etag-001",
		UID:     "uid-001@huginn",
		Title:   "Standup",
		Status:  "confirmed",
		StartAt: time.Date(2026, 6, 29, 10, 0, 0, 0, time.UTC),
		EndAt:   time.Date(2026, 6, 29, 10, 30, 0, 0, time.UTC),
	}}
	out := reportCalendarQuery("uid-1", cal, events, nil)
	mustContain(t, out, "/dav/calendars/uid-1/cal-1/ev-1.ics")
	mustContain(t, out, `"etag-001"`)
	mustContain(t, out, "C:calendar-data")
	mustContain(t, out, "BEGIN:VCALENDAR")
	mustContain(t, out, "SUMMARY:Standup")
	mustContain(t, out, "20260629T100000Z")
}

// --- reportCalendarMultiget ---

func TestReportCalendarMultiget_Found(t *testing.T) {
	cal := &repo.Calendar{ID: "cal-1", Name: "Test"}
	e := &repo.Event{
		ID: "ev-1", ETag: "etag-001", UID: "u1@huginn", Title: "Meeting",
		Status: "confirmed", StartAt: time.Now(), EndAt: time.Now().Add(time.Hour),
	}
	hrefs := []string{"/dav/calendars/uid-1/cal-1/ev-1.ics"}
	out := reportCalendarMultiget("uid-1", cal, map[string]*repo.Event{hrefs[0]: e}, hrefs, nil)
	mustContain(t, out, "/dav/calendars/uid-1/cal-1/ev-1.ics")
	mustContain(t, out, `"etag-001"`)
	mustContain(t, out, "BEGIN:VCALENDAR")
	mustContain(t, out, "200 OK")
}

func TestReportCalendarMultiget_NotFound(t *testing.T) {
	cal := &repo.Calendar{ID: "cal-1", Name: "Test"}
	hrefs := []string{"/dav/calendars/uid-1/cal-1/missing.ics"}
	out := reportCalendarMultiget("uid-1", cal, map[string]*repo.Event{}, hrefs, nil)
	mustContain(t, out, "404 Not Found")
	if strings.Contains(out, "BEGIN:VCALENDAR") {
		t.Error("404 response must not contain ICS data")
	}
}

// --- reportSyncCollection ---

func TestReportSyncCollection_LiveEvents(t *testing.T) {
	cal := &repo.Calendar{ID: "cal-1", Name: "Test"}
	events := []*repo.Event{
		{ID: "ev-1", ETag: "etag-001", DeletedAt: nil},
	}
	token := "https://huginn.local/ns/sync/cal-1/12345"
	out := reportSyncCollection("uid-1", cal, events, token)
	mustContain(t, out, "/dav/calendars/uid-1/cal-1/ev-1.ics")
	mustContain(t, out, `"etag-001"`)
	mustContain(t, out, "200 OK")
	mustContain(t, out, "sync-token")
	mustContain(t, out, token)
}

func TestReportSyncCollection_DeletedEvent(t *testing.T) {
	cal := &repo.Calendar{ID: "cal-1", Name: "Test"}
	deletedAt := time.Now()
	events := []*repo.Event{
		{ID: "ev-del", ETag: "old-etag", DeletedAt: &deletedAt},
	}
	out := reportSyncCollection("uid-1", cal, events, "token")
	mustContain(t, out, "/dav/calendars/uid-1/cal-1/ev-del.ics")
	mustContain(t, out, "404 Not Found")
	if strings.Contains(out, "getetag") {
		t.Error("deleted event must not include getetag propstat")
	}
}

// --- detectReport ---

func TestDetectReport(t *testing.T) {
	cases := []struct {
		body string
		want string
	}{
		{`<C:calendar-multiget xmlns:C="urn:ietf:params:xml:ns:caldav">`, "calendar-multiget"},
		{`<D:sync-collection xmlns:D="DAV:">`, "sync-collection"},
		{`<C:free-busy-query xmlns:C="urn:ietf:params:xml:ns:caldav">`, "free-busy-query"},
		{`<C:calendar-query xmlns:C="urn:ietf:params:xml:ns:caldav">`, "calendar-query"},
		{`<anything>`, "calendar-query"},
	}
	for _, c := range cases {
		got := detectReport([]byte(c.body))
		if got != c.want {
			t.Errorf("detectReport(%q) = %q, want %q", c.body, got, c.want)
		}
	}
}

// --- extractHrefs ---

func TestExtractHrefs(t *testing.T) {
	body := `<D:calendar-multiget>
  <D:href>/dav/calendars/u/c/ev1.ics</D:href>
  <D:href>/dav/calendars/u/c/ev2.ics</D:href>
</D:calendar-multiget>`
	hrefs := extractHrefs([]byte(body))
	if len(hrefs) != 2 {
		t.Fatalf("expected 2 hrefs, got %d", len(hrefs))
	}
	mustContain(t, hrefs[0], "ev1.ics")
	mustContain(t, hrefs[1], "ev2.ics")
}

// --- parseSyncToken ---

func TestParseSyncToken(t *testing.T) {
	stamp := time.Date(2026, 6, 29, 10, 0, 0, 0, time.UTC)
	token := buildSyncToken("cal-1", stamp)
	parsed, ok := parseSyncToken(token)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if !parsed.Equal(stamp) {
		t.Errorf("got %v, want %v", parsed, stamp)
	}
}

func TestParseSyncToken_Empty(t *testing.T) {
	_, ok := parseSyncToken("")
	if ok {
		t.Error("expected ok=false for empty token")
	}
}

// --- extractSyncToken ---

func TestExtractSyncToken(t *testing.T) {
	body := `<D:sync-collection>
  <D:sync-token>https://huginn.local/ns/sync/cal/12345</D:sync-token>
</D:sync-collection>`
	got := extractSyncToken([]byte(body))
	if got != "https://huginn.local/ns/sync/cal/12345" {
		t.Errorf("got %q", got)
	}
}

func TestExtractSyncToken_Empty(t *testing.T) {
	body := `<D:sync-collection><D:sync-token></D:sync-token></D:sync-collection>`
	got := extractSyncToken([]byte(body))
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

// --- reportFreeBusy ---

func TestReportFreeBusy(t *testing.T) {
	cal := &repo.Calendar{ID: "cal-1", Name: "Test"}
	events := []*repo.Event{
		{
			StartAt:    time.Date(2026, 6, 29, 10, 0, 0, 0, time.UTC),
			EndAt:      time.Date(2026, 6, 29, 11, 0, 0, 0, time.UTC),
			BusyStatus: "busy",
		},
	}
	from := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	out := reportFreeBusy(cal, events, from, to)
	mustContain(t, out, "BEGIN:VCALENDAR")
	mustContain(t, out, "BEGIN:VFREEBUSY")
	mustContain(t, out, "FREEBUSY:20260629T100000Z/20260629T110000Z")
	mustContain(t, out, "END:VFREEBUSY")
}

func TestReportFreeBusy_SkipsFreeEvents(t *testing.T) {
	cal := &repo.Calendar{ID: "cal-1", Name: "Test"}
	events := []*repo.Event{
		{
			StartAt:    time.Now(),
			EndAt:      time.Now().Add(time.Hour),
			BusyStatus: "free",
		},
	}
	from, to := time.Now().AddDate(-1, 0, 0), time.Now().AddDate(1, 0, 0)
	out := reportFreeBusy(cal, events, from, to)
	if strings.Contains(out, "FREEBUSY:") {
		t.Error("free events must not appear in FREEBUSY")
	}
}

func mustContain(t *testing.T, s, sub string) {
	t.Helper()
	if !strings.Contains(s, sub) {
		t.Errorf("expected output to contain %q", sub)
	}
}
