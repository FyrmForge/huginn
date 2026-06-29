//go:build e2e

package e2e

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

const (
	// CalDAV test credentials seeded in testdata/seed_e2e.sql.
	caldavUser  = "user@test.com"
	caldavToken = "huginn-e2e-caldav-token-001"

	// Seeded IDs.
	caldavUserID   = "e2e-user-001"
	caldavCalID    = "e2e-cal-001"
	caldavEventID  = "e2e-event-001"
	caldavEventID2 = "e2e-event-002"
)

func caldavDo(t *testing.T, method, path, user, token string, body string) *http.Response {
	t.Helper()
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, serverURL+path, bodyReader)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if user != "" {
		req.SetBasicAuth(user, token)
	}
	req.Header.Set("Content-Type", "application/xml; charset=utf-8")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	return resp
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(b)
}

func mustContainStr(t *testing.T, s, sub string) {
	t.Helper()
	if !strings.Contains(s, sub) {
		t.Errorf("response missing %q", sub)
	}
}

// --- Auth ---

func TestCalDAV_NoAuth_Returns401(t *testing.T) {
	resp := caldavDo(t, "PROPFIND", "/dav/", "", "", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
	if resp.Header.Get("WWW-Authenticate") == "" {
		t.Error("expected WWW-Authenticate header")
	}
}

func TestCalDAV_WrongToken_Returns401(t *testing.T) {
	resp := caldavDo(t, "PROPFIND", "/dav/", caldavUser, "wrong-token", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

// --- OPTIONS (RFC 4791 §5.1) ---

func TestCalDAV_Options_DavHeader(t *testing.T) {
	req, _ := http.NewRequest("OPTIONS", serverURL+"/dav/", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("OPTIONS: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	dav := resp.Header.Get("DAV")
	mustContainStr(t, dav, "calendar-access")
	mustContainStr(t, dav, "1")
	mustContainStr(t, dav, "2")
	mustContainStr(t, dav, "3")
}

func TestCalDAV_Options_AllowHeader(t *testing.T) {
	req, _ := http.NewRequest("OPTIONS", serverURL+"/dav/calendars/"+caldavUserID+"/"+caldavCalID+"/", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("OPTIONS: %v", err)
	}
	defer resp.Body.Close()
	allow := resp.Header.Get("Allow")
	mustContainStr(t, allow, "PROPFIND")
	mustContainStr(t, allow, "REPORT")
	mustContainStr(t, allow, "PUT")
	mustContainStr(t, allow, "DELETE")
}

// --- Discovery chain (RFC 4791 §6) ---

func TestCalDAV_WellKnown_Redirects(t *testing.T) {
	req, _ := http.NewRequest("PROPFIND", serverURL+"/.well-known/caldav", nil)
	req.SetBasicAuth(caldavUser, caldavToken)
	resp, err := (&http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}).Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMovedPermanently {
		t.Errorf("expected 301, got %d", resp.StatusCode)
	}
	if !strings.Contains(resp.Header.Get("Location"), "/dav/") {
		t.Errorf("expected redirect to /dav/, got %q", resp.Header.Get("Location"))
	}
}

func TestCalDAV_Principal_Returns207(t *testing.T) {
	resp := caldavDo(t, "PROPFIND", "/dav/", caldavUser, caldavToken,
		`<?xml version="1.0"?><propfind xmlns="DAV:"><prop><current-user-principal/></prop></propfind>`)
	body := readBody(t, resp)
	if resp.StatusCode != http.StatusMultiStatus {
		t.Errorf("expected 207, got %d\n%s", resp.StatusCode, body)
	}
	mustContainStr(t, body, "calendar-home-set")
	mustContainStr(t, body, fmt.Sprintf("/dav/calendars/%s/", caldavUserID))
	mustContainStr(t, body, "principal-URL")
	mustContainStr(t, body, "schedule-inbox-URL")
	mustContainStr(t, body, "schedule-outbox-URL")
	mustContainStr(t, body, "supported-report-set")
}

func TestCalDAV_CalendarHome_ListsCalendars(t *testing.T) {
	path := fmt.Sprintf("/dav/calendars/%s/", caldavUserID)
	resp := caldavDo(t, "PROPFIND", path, caldavUser, caldavToken,
		`<?xml version="1.0"?><propfind xmlns="DAV:"><prop><resourcetype/><displayname/></prop></propfind>`)
	body := readBody(t, resp)
	if resp.StatusCode != http.StatusMultiStatus {
		t.Errorf("expected 207, got %d\n%s", resp.StatusCode, body)
	}
	mustContainStr(t, body, caldavCalID)
	mustContainStr(t, body, "E2E Calendar")
	mustContainStr(t, body, "getctag")
	mustContainStr(t, body, "calendar-color")
}

func TestCalDAV_CalendarHome_Depth0(t *testing.T) {
	path := fmt.Sprintf("/dav/calendars/%s/", caldavUserID)
	req, _ := http.NewRequest("PROPFIND", serverURL+path, strings.NewReader(
		`<?xml version="1.0"?><propfind xmlns="DAV:"><prop><resourcetype/></prop></propfind>`))
	req.SetBasicAuth(caldavUser, caldavToken)
	req.Header.Set("Depth", "0")
	req.Header.Set("Content-Type", "application/xml")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	body := readBody(t, resp)
	if resp.StatusCode != http.StatusMultiStatus {
		t.Errorf("expected 207, got %d\n%s", resp.StatusCode, body)
	}
	if strings.Contains(body, caldavCalID) {
		t.Error("Depth:0 must not list individual calendars")
	}
}

func TestCalDAV_CalendarCollection_Depth0_HasCtag(t *testing.T) {
	path := fmt.Sprintf("/dav/calendars/%s/%s/", caldavUserID, caldavCalID)
	req, _ := http.NewRequest("PROPFIND", serverURL+path, strings.NewReader(
		`<?xml version="1.0"?><propfind xmlns="DAV:"><prop><getctag/></prop></propfind>`))
	req.SetBasicAuth(caldavUser, caldavToken)
	req.Header.Set("Depth", "0")
	req.Header.Set("Content-Type", "application/xml")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	body := readBody(t, resp)
	if resp.StatusCode != http.StatusMultiStatus {
		t.Errorf("expected 207, got %d\n%s", resp.StatusCode, body)
	}
	mustContainStr(t, body, "getctag")
	mustContainStr(t, body, "sync-token")
	mustContainStr(t, body, "huginn.local/ns/sync/")
	if strings.Contains(body, caldavEventID+".ics") {
		t.Error("Depth:0 must not list event resources")
	}
}

func TestCalDAV_CalendarCollection_Depth1_ListsEvents(t *testing.T) {
	path := fmt.Sprintf("/dav/calendars/%s/%s/", caldavUserID, caldavCalID)
	resp := caldavDo(t, "PROPFIND", path, caldavUser, caldavToken,
		`<?xml version="1.0"?><propfind xmlns="DAV:"><prop><getetag/></prop></propfind>`)
	body := readBody(t, resp)
	if resp.StatusCode != http.StatusMultiStatus {
		t.Errorf("expected 207, got %d\n%s", resp.StatusCode, body)
	}
	mustContainStr(t, body, caldavEventID+".ics")
	mustContainStr(t, body, caldavEventID2+".ics")
	mustContainStr(t, body, "getetag")
	mustContainStr(t, body, "supported-calendar-component-set")
	mustContainStr(t, body, "getctag")
	mustContainStr(t, body, "sync-token")
}

// --- PROPFIND on individual event ---

func TestCalDAV_PropfindEvent_ReturnsEtag(t *testing.T) {
	path := fmt.Sprintf("/dav/calendars/%s/%s/%s.ics", caldavUserID, caldavCalID, caldavEventID)
	resp := caldavDo(t, "PROPFIND", path, caldavUser, caldavToken,
		`<?xml version="1.0"?><propfind xmlns="DAV:"><prop><getetag/></prop></propfind>`)
	body := readBody(t, resp)
	if resp.StatusCode != http.StatusMultiStatus {
		t.Errorf("expected 207, got %d\n%s", resp.StatusCode, body)
	}
	mustContainStr(t, body, "getetag")
	mustContainStr(t, body, "text/calendar")
	mustContainStr(t, body, caldavEventID+".ics")
}

// --- GET single event ---

func TestCalDAV_GetEvent_ReturnsICS(t *testing.T) {
	path := fmt.Sprintf("/dav/calendars/%s/%s/%s.ics", caldavUserID, caldavCalID, caldavEventID)
	resp := caldavDo(t, "GET", path, caldavUser, caldavToken, "")
	body := readBody(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d\n%s", resp.StatusCode, body)
	}
	if !strings.Contains(resp.Header.Get("Content-Type"), "text/calendar") {
		t.Errorf("expected text/calendar, got %q", resp.Header.Get("Content-Type"))
	}
	mustContainStr(t, body, "BEGIN:VCALENDAR")
	mustContainStr(t, body, "BEGIN:VEVENT")
	mustContainStr(t, body, "E2E Standup")
	mustContainStr(t, body, "TRANSP:")
	mustContainStr(t, body, "CREATED:")
	mustContainStr(t, body, "LAST-MODIFIED:")
	mustContainStr(t, body, "END:VEVENT")
	mustContainStr(t, body, "END:VCALENDAR")
	if resp.Header.Get("ETag") == "" {
		t.Error("expected ETag response header")
	}
}

// --- REPORT: calendar-query ---

func TestCalDAV_Report_CalendarQuery_IncludesICS(t *testing.T) {
	path := fmt.Sprintf("/dav/calendars/%s/%s/", caldavUserID, caldavCalID)
	resp := caldavDo(t, "REPORT", path, caldavUser, caldavToken,
		`<?xml version="1.0"?><C:calendar-query xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
		  <D:prop><D:getetag/><C:calendar-data/></D:prop>
		  <C:filter><C:comp-filter name="VCALENDAR"/></C:filter>
		</C:calendar-query>`)
	body := readBody(t, resp)
	if resp.StatusCode != http.StatusMultiStatus {
		t.Errorf("expected 207, got %d\n%s", resp.StatusCode, body)
	}
	mustContainStr(t, body, "C:calendar-data")
	mustContainStr(t, body, "BEGIN:VCALENDAR")
	mustContainStr(t, body, "E2E Standup")
}

// --- REPORT: calendar-multiget ---

func TestCalDAV_Report_CalendarMultiget_Found(t *testing.T) {
	path := fmt.Sprintf("/dav/calendars/%s/%s/", caldavUserID, caldavCalID)
	href1 := fmt.Sprintf("/dav/calendars/%s/%s/%s.ics", caldavUserID, caldavCalID, caldavEventID)
	href2 := fmt.Sprintf("/dav/calendars/%s/%s/%s.ics", caldavUserID, caldavCalID, caldavEventID2)
	body := fmt.Sprintf(`<?xml version="1.0"?><C:calendar-multiget xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
	  <D:prop><D:getetag/><C:calendar-data/></D:prop>
	  <D:href>%s</D:href>
	  <D:href>%s</D:href>
	</C:calendar-multiget>`, href1, href2)
	resp := caldavDo(t, "REPORT", path, caldavUser, caldavToken, body)
	rbody := readBody(t, resp)
	if resp.StatusCode != http.StatusMultiStatus {
		t.Errorf("expected 207, got %d\n%s", resp.StatusCode, rbody)
	}
	mustContainStr(t, rbody, caldavEventID+".ics")
	mustContainStr(t, rbody, caldavEventID2+".ics")
	mustContainStr(t, rbody, "BEGIN:VCALENDAR")
	mustContainStr(t, rbody, "E2E Standup")
	mustContainStr(t, rbody, "E2E Planning")
}

func TestCalDAV_Report_CalendarMultiget_NotFound(t *testing.T) {
	path := fmt.Sprintf("/dav/calendars/%s/%s/", caldavUserID, caldavCalID)
	reqBody := fmt.Sprintf(`<?xml version="1.0"?><C:calendar-multiget xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
	  <D:prop><D:getetag/><C:calendar-data/></D:prop>
	  <D:href>/dav/calendars/%s/%s/does-not-exist.ics</D:href>
	</C:calendar-multiget>`, caldavUserID, caldavCalID)
	resp := caldavDo(t, "REPORT", path, caldavUser, caldavToken, reqBody)
	rbody := readBody(t, resp)
	if resp.StatusCode != http.StatusMultiStatus {
		t.Errorf("expected 207, got %d\n%s", resp.StatusCode, rbody)
	}
	mustContainStr(t, rbody, "404 Not Found")
}

// --- REPORT: sync-collection ---

func TestCalDAV_Report_SyncCollection_InitialSync(t *testing.T) {
	path := fmt.Sprintf("/dav/calendars/%s/%s/", caldavUserID, caldavCalID)
	resp := caldavDo(t, "REPORT", path, caldavUser, caldavToken,
		`<?xml version="1.0"?><D:sync-collection xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
		  <D:sync-token></D:sync-token>
		  <D:sync-level>1</D:sync-level>
		  <D:prop><D:getetag/></D:prop>
		</D:sync-collection>`)
	body := readBody(t, resp)
	if resp.StatusCode != http.StatusMultiStatus {
		t.Errorf("expected 207, got %d\n%s", resp.StatusCode, body)
	}
	mustContainStr(t, body, caldavEventID+".ics")
	mustContainStr(t, body, caldavEventID2+".ics")
	mustContainStr(t, body, "sync-token")
	mustContainStr(t, body, "huginn.local/ns/sync/")
}

func TestCalDAV_Report_SyncCollection_Incremental_NoChanges(t *testing.T) {
	path := fmt.Sprintf("/dav/calendars/%s/%s/", caldavUserID, caldavCalID)

	// Get the current token via initial sync.
	initResp := caldavDo(t, "REPORT", path, caldavUser, caldavToken,
		`<D:sync-collection xmlns:D="DAV:"><D:sync-token></D:sync-token><D:sync-level>1</D:sync-level><D:prop><D:getetag/></D:prop></D:sync-collection>`)
	initBody := readBody(t, initResp)
	if initResp.StatusCode != http.StatusMultiStatus {
		t.Fatalf("initial sync failed: %d", initResp.StatusCode)
	}

	// Extract token from response.
	token := extractTokenFromBody(initBody)
	if token == "" {
		t.Fatal("no sync-token in initial sync response")
	}

	// Incremental sync with current token — should return no events.
	body := fmt.Sprintf(`<D:sync-collection xmlns:D="DAV:"><D:sync-token>%s</D:sync-token><D:sync-level>1</D:sync-level><D:prop><D:getetag/></D:prop></D:sync-collection>`, token)
	resp := caldavDo(t, "REPORT", path, caldavUser, caldavToken, body)
	rbody := readBody(t, resp)
	if resp.StatusCode != http.StatusMultiStatus {
		t.Errorf("expected 207, got %d\n%s", resp.StatusCode, rbody)
	}
	mustContainStr(t, rbody, "sync-token")
}

// --- REPORT: free-busy-query ---

func TestCalDAV_Report_FreeBusy(t *testing.T) {
	path := fmt.Sprintf("/dav/calendars/%s/%s/", caldavUserID, caldavCalID)
	resp := caldavDo(t, "REPORT", path, caldavUser, caldavToken,
		`<?xml version="1.0"?><C:free-busy-query xmlns:C="urn:ietf:params:xml:ns:caldav">
		  <C:time-range start="20260101T000000Z" end="20270101T000000Z"/>
		</C:free-busy-query>`)
	body := readBody(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d\n%s", resp.StatusCode, body)
	}
	if !strings.Contains(resp.Header.Get("Content-Type"), "text/calendar") {
		t.Errorf("expected text/calendar, got %q", resp.Header.Get("Content-Type"))
	}
	mustContainStr(t, body, "BEGIN:VFREEBUSY")
}

// --- PUT: create and update ---

func TestCalDAV_Put_Create_Returns201WithETag(t *testing.T) {
	icsBody := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Test//EN\r\nBEGIN:VEVENT\r\n" +
		"UID:e2e-create-test-001@huginn\r\nSUMMARY:Create Test\r\n" +
		"DTSTART:20260701T090000Z\r\nDTEND:20260701T100000Z\r\nSTATUS:CONFIRMED\r\n" +
		"END:VEVENT\r\nEND:VCALENDAR\r\n"

	putPath := fmt.Sprintf("/dav/calendars/%s/%s/e2e-create-test-001.ics", caldavUserID, caldavCalID)
	req, _ := http.NewRequest("PUT", serverURL+putPath, strings.NewReader(icsBody))
	req.SetBasicAuth(caldavUser, caldavToken)
	req.Header.Set("Content-Type", "text/calendar; charset=utf-8")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("PUT expected 201, got %d", resp.StatusCode)
	}
	if resp.Header.Get("ETag") == "" {
		t.Error("PUT response must include ETag header")
	}
}

func TestCalDAV_Put_IfNoneMatch_Conflict(t *testing.T) {
	// e2e-event-001 already exists.
	icsBody := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Test//EN\r\nBEGIN:VEVENT\r\n" +
		"UID:e2e-event-001@huginn\r\nSUMMARY:Duplicate\r\n" +
		"DTSTART:20260701T090000Z\r\nDTEND:20260701T100000Z\r\nSTATUS:CONFIRMED\r\n" +
		"END:VEVENT\r\nEND:VCALENDAR\r\n"
	putPath := fmt.Sprintf("/dav/calendars/%s/%s/%s.ics", caldavUserID, caldavCalID, caldavEventID)
	req, _ := http.NewRequest("PUT", serverURL+putPath, strings.NewReader(icsBody))
	req.SetBasicAuth(caldavUser, caldavToken)
	req.Header.Set("Content-Type", "text/calendar; charset=utf-8")
	req.Header.Set("If-None-Match", "*")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusPreconditionFailed {
		t.Errorf("expected 412, got %d", resp.StatusCode)
	}
}

func TestCalDAV_Put_Update_Returns204(t *testing.T) {
	icsBody := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Test//EN\r\nBEGIN:VEVENT\r\n" +
		"UID:e2e-event-001@huginn\r\nSUMMARY:Updated Standup\r\n" +
		"DTSTART:20260629T100000Z\r\nDTEND:20260629T103000Z\r\nSTATUS:CONFIRMED\r\n" +
		"END:VEVENT\r\nEND:VCALENDAR\r\n"
	putPath := fmt.Sprintf("/dav/calendars/%s/%s/%s.ics", caldavUserID, caldavCalID, caldavEventID)
	req, _ := http.NewRequest("PUT", serverURL+putPath, strings.NewReader(icsBody))
	req.SetBasicAuth(caldavUser, caldavToken)
	req.Header.Set("Content-Type", "text/calendar; charset=utf-8")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("expected 204, got %d", resp.StatusCode)
	}
	if resp.Header.Get("ETag") == "" {
		t.Error("PUT update response must include ETag header")
	}
}

func TestCalDAV_Put_IfMatch_Conflict(t *testing.T) {
	icsBody := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Test//EN\r\nBEGIN:VEVENT\r\n" +
		"UID:e2e-event-002@huginn\r\nSUMMARY:Stale Update\r\n" +
		"DTSTART:20260630T140000Z\r\nDTEND:20260630T150000Z\r\nSTATUS:CONFIRMED\r\n" +
		"END:VEVENT\r\nEND:VCALENDAR\r\n"
	putPath := fmt.Sprintf("/dav/calendars/%s/%s/%s.ics", caldavUserID, caldavCalID, caldavEventID2)
	req, _ := http.NewRequest("PUT", serverURL+putPath, strings.NewReader(icsBody))
	req.SetBasicAuth(caldavUser, caldavToken)
	req.Header.Set("Content-Type", "text/calendar; charset=utf-8")
	req.Header.Set("If-Match", `"wrong-etag"`)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusPreconditionFailed {
		t.Errorf("expected 412, got %d", resp.StatusCode)
	}
}

// --- DELETE ---

func TestCalDAV_Delete_Returns204(t *testing.T) {
	// First create an event to delete.
	icsBody := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Test//EN\r\nBEGIN:VEVENT\r\n" +
		"UID:e2e-delete-me@huginn\r\nSUMMARY:Delete Me\r\n" +
		"DTSTART:20260801T090000Z\r\nDTEND:20260801T100000Z\r\nSTATUS:CONFIRMED\r\n" +
		"END:VEVENT\r\nEND:VCALENDAR\r\n"
	putPath := fmt.Sprintf("/dav/calendars/%s/%s/e2e-delete-me.ics", caldavUserID, caldavCalID)
	putReq, _ := http.NewRequest("PUT", serverURL+putPath, strings.NewReader(icsBody))
	putReq.SetBasicAuth(caldavUser, caldavToken)
	putReq.Header.Set("Content-Type", "text/calendar; charset=utf-8")
	putResp, _ := http.DefaultClient.Do(putReq)
	putResp.Body.Close()
	if putResp.StatusCode != http.StatusCreated {
		t.Fatalf("setup PUT failed: %d", putResp.StatusCode)
	}

	delReq, _ := http.NewRequest("DELETE", serverURL+putPath, nil)
	delReq.SetBasicAuth(caldavUser, caldavToken)
	delResp, err := http.DefaultClient.Do(delReq)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	delResp.Body.Close()
	if delResp.StatusCode != http.StatusNoContent {
		t.Errorf("expected 204, got %d", delResp.StatusCode)
	}
}

func TestCalDAV_Delete_IfMatch_Conflict(t *testing.T) {
	path := fmt.Sprintf("/dav/calendars/%s/%s/%s.ics", caldavUserID, caldavCalID, caldavEventID2)
	req, _ := http.NewRequest("DELETE", serverURL+path, nil)
	req.SetBasicAuth(caldavUser, caldavToken)
	req.Header.Set("If-Match", `"wrong-etag"`)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusPreconditionFailed {
		t.Errorf("expected 412, got %d", resp.StatusCode)
	}
}

// --- MKCALENDAR ---

func TestCalDAV_MkCalendar_Creates201(t *testing.T) {
	path := fmt.Sprintf("/dav/calendars/%s/e2e-new-cal-001/", caldavUserID)
	req, _ := http.NewRequest("MKCALENDAR", serverURL+path,
		strings.NewReader(`<?xml version="1.0"?><C:mkcalendar xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav"><D:set><D:prop><D:displayname>New Test Calendar</D:displayname></D:prop></D:set></C:mkcalendar>`))
	req.SetBasicAuth(caldavUser, caldavToken)
	req.Header.Set("Content-Type", "application/xml")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("MKCALENDAR: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected 201, got %d", resp.StatusCode)
	}

	// Verify it appears in calendar home listing.
	homeResp := caldavDo(t, "PROPFIND",
		fmt.Sprintf("/dav/calendars/%s/", caldavUserID),
		caldavUser, caldavToken,
		`<propfind xmlns="DAV:"><prop><displayname/></prop></propfind>`)
	homeBody := readBody(t, homeResp)
	mustContainStr(t, homeBody, "e2e-new-cal-001")
}

func TestCalDAV_MkCalendar_ConflictOnDuplicate(t *testing.T) {
	path := fmt.Sprintf("/dav/calendars/%s/%s/", caldavUserID, caldavCalID)
	req, _ := http.NewRequest("MKCALENDAR", serverURL+path, nil)
	req.SetBasicAuth(caldavUser, caldavToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("MKCALENDAR: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", resp.StatusCode)
	}
}

// --- Access control ---

func TestCalDAV_ForbiddenOnOtherUserCalendar(t *testing.T) {
	// Try to access admin's calendar space as regular user.
	path := fmt.Sprintf("/dav/calendars/%s/", "e2e-admin-001")
	resp := caldavDo(t, "PROPFIND", path, caldavUser, caldavToken, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
}

// --- helpers ---

func extractTokenFromBody(body string) string {
	start := strings.Index(body, "<sync-token>")
	if start == -1 {
		return ""
	}
	start += len("<sync-token>")
	end := strings.Index(body[start:], "</sync-token>")
	if end == -1 {
		return ""
	}
	return strings.TrimSpace(body[start : start+end])
}
