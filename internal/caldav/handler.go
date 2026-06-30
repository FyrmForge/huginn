// Package caldav implements a minimal CalDAV server for device sync.
// Supports: OPTIONS, PROPFIND, REPORT (calendar-query, calendar-multiget,
// sync-collection, free-busy-query), GET, PUT, DELETE, MKCALENDAR.
// Compliant with RFC 4791 and tested against iOS and DAVx5 (Android).
package caldav

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/FyrmForge/huginn/internal/ics"
	"github.com/FyrmForge/huginn/internal/repo"
	"github.com/FyrmForge/huginn/internal/service"
)

func xmlEscape(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

// Handler handles CalDAV protocol requests.
type Handler struct {
	caldavService   *service.CalDAVService
	calendarService *service.CalendarService
	eventService    *service.EventService
}

func NewHandler(caldavService *service.CalDAVService, calendarService *service.CalendarService, eventService *service.EventService) *Handler {
	return &Handler{
		caldavService:   caldavService,
		calendarService: calendarService,
		eventService:    eventService,
	}
}

// authUser extracts and authenticates the user from Basic Auth.
// If a username is provided it must match the token owner's ID or email.
func (h *Handler) authUser(c echo.Context) (*repo.User, error) {
	username, pass, ok := c.Request().BasicAuth()
	if !ok {
		return nil, fmt.Errorf("missing basic auth")
	}
	user, err := h.caldavService.Authenticate(c.Request().Context(), pass)
	if err != nil {
		return nil, err
	}
	if username != "" && username != user.ID && username != user.Email {
		return nil, fmt.Errorf("username mismatch")
	}
	return user, nil
}

func requireAuth(c echo.Context) bool {
	c.Response().Header().Set("WWW-Authenticate", `Basic realm="Huginn CalDAV"`)
	c.Response().WriteHeader(http.StatusUnauthorized)
	return false
}

// --- HTTP handlers ---

// Options handles OPTIONS requests — advertises DAV capabilities.
func (h *Handler) Options(c echo.Context) error {
	c.Response().Header().Set("DAV", "1, 2, 3, calendar-access")
	c.Response().Header().Set("Allow", "OPTIONS, GET, HEAD, PUT, DELETE, PROPFIND, REPORT, MKCALENDAR")
	c.Response().Header().Set("Content-Length", "0")
	return c.NoContent(http.StatusOK)
}

// WellKnown redirects /.well-known/caldav to the principal URL.
func (h *Handler) WellKnown(c echo.Context) error {
	c.Response().Header().Set("Location", "/dav/")
	return c.NoContent(http.StatusMovedPermanently)
}

// Principal handles PROPFIND /dav/ — principal resource.
func (h *Handler) Principal(c echo.Context) error {
	user, err := h.authUser(c)
	if err != nil {
		requireAuth(c)
		return nil
	}
	return davResponse(c, http.StatusMultiStatus, propfindPrincipal(user.ID))
}

// CalendarHome handles PROPFIND /dav/calendars/:userID/
func (h *Handler) CalendarHome(c echo.Context) error {
	user, err := h.authUser(c)
	if err != nil {
		requireAuth(c)
		return nil
	}
	if c.Param("userID") != user.ID {
		return c.NoContent(http.StatusForbidden)
	}
	d := depthHeader(c)
	if d == "0" {
		return davResponse(c, http.StatusMultiStatus, propfindCalendarHomeDepth0(user.ID))
	}
	calendars, err := h.calendarService.ListForUser(c.Request().Context(), user.ID)
	if err != nil {
		return c.NoContent(http.StatusInternalServerError)
	}
	return davResponse(c, http.StatusMultiStatus, propfindCalendarHome(user.ID, calendars))
}

// CalendarCollection handles PROPFIND /dav/calendars/:userID/:calID/
func (h *Handler) CalendarCollection(c echo.Context) error {
	user, err := h.authUser(c)
	if err != nil {
		requireAuth(c)
		return nil
	}
	if c.Param("userID") != user.ID {
		return c.NoContent(http.StatusForbidden)
	}
	calID := c.Param("calID")
	cal, err := h.calendarService.GetByID(c.Request().Context(), user.ID, calID)
	if err != nil || cal == nil {
		return c.NoContent(http.StatusNotFound)
	}

	d := depthHeader(c)
	if d == "0" {
		stamp, _ := h.eventService.GetSyncStamp(c.Request().Context(), calID)
		return davResponse(c, http.StatusMultiStatus, propfindCalendarDepth0(user.ID, cal, stamp))
	}

	from := time.Now().AddDate(-1, 0, 0)
	to := time.Now().AddDate(1, 0, 0)
	events, err := h.eventService.ListForRange(c.Request().Context(), []string{calID}, from, to)
	if err != nil {
		return c.NoContent(http.StatusInternalServerError)
	}
	return davResponse(c, http.StatusMultiStatus, propfindCalendar(user.ID, cal, events))
}

// PropfindEvent handles PROPFIND /dav/calendars/:userID/:calID/:eventUID
func (h *Handler) PropfindEvent(c echo.Context) error {
	user, err := h.authUser(c)
	if err != nil {
		requireAuth(c)
		return nil
	}
	if c.Param("userID") != user.ID {
		return c.NoContent(http.StatusForbidden)
	}
	calID := c.Param("calID")
	eventID := strings.TrimSuffix(c.Param("eventUID"), ".ics")
	event, err := h.eventService.GetByID(c.Request().Context(), eventID)
	if err != nil || event == nil || event.CalendarID != calID {
		return c.NoContent(http.StatusNotFound)
	}
	return davResponse(c, http.StatusMultiStatus, propfindEventResource(user.ID, calID, event))
}

// GetEvent handles GET /dav/calendars/:userID/:calID/:eventUID.ics
func (h *Handler) GetEvent(c echo.Context) error {
	user, err := h.authUser(c)
	if err != nil {
		requireAuth(c)
		return nil
	}
	if c.Param("userID") != user.ID {
		return c.NoContent(http.StatusForbidden)
	}
	calID := c.Param("calID")
	cal, err := h.calendarService.GetByID(c.Request().Context(), user.ID, calID)
	if err != nil || cal == nil {
		return c.NoContent(http.StatusNotFound)
	}
	eventID := strings.TrimSuffix(c.Param("eventUID"), ".ics")
	event, err := h.eventService.GetByID(c.Request().Context(), eventID)
	if err != nil || event == nil || event.CalendarID != calID {
		return c.NoContent(http.StatusNotFound)
	}

	exs, _ := h.eventService.LoadExceptions(c.Request().Context(), event.ID)
	exsByMaster := map[string][]*repo.EventException{event.ID: exs}
	c.Response().Header().Set("Content-Type", "text/calendar; charset=utf-8")
	c.Response().Header().Set("ETag", `"`+event.ETag+`"`)
	return ics.WriteWithExceptions(c.Response().Writer, cal.Name, []*repo.Event{event}, exsByMaster)
}

// PutEvent handles PUT /dav/calendars/:userID/:calID/:eventUID.ics
// Creates a new event or updates an existing one with the same UID.
func (h *Handler) PutEvent(c echo.Context) error {
	user, err := h.authUser(c)
	if err != nil {
		requireAuth(c)
		return nil
	}
	if c.Param("userID") != user.ID {
		return c.NoContent(http.StatusForbidden)
	}
	calID := c.Param("calID")
	cal, err := h.calendarService.GetByID(c.Request().Context(), user.ID, calID)
	if err != nil || cal == nil {
		return c.NoContent(http.StatusNotFound)
	}

	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	parsed, err := ics.Parse(strings.NewReader(string(body)))
	if err != nil || len(parsed) == 0 {
		return davError(c, http.StatusBadRequest, "valid-calendar-data")
	}

	// Separate master VEVENT from exception VEVENTs (those with RECURRENCE-ID).
	var master *ics.Event
	var exceptionVEVENTs []ics.Event
	for i := range parsed {
		if !parsed[i].RecurrenceID.IsZero() {
			exceptionVEVENTs = append(exceptionVEVENTs, parsed[i])
		} else if master == nil {
			master = &parsed[i]
		}
	}
	if master == nil {
		return davError(c, http.StatusBadRequest, "valid-calendar-data")
	}
	p := master

	existing, _ := h.eventService.GetByUID(c.Request().Context(), calID, p.UID)

	// Conditional create: If-None-Match: * means "fail if resource exists"
	if c.Request().Header.Get("If-None-Match") == "*" && existing != nil {
		return davError(c, http.StatusPreconditionFailed, "no-uid-conflict")
	}
	// Conditional update: If-Match means "fail if ETag doesn't match"
	if ifMatch := c.Request().Header.Get("If-Match"); ifMatch != "" && existing != nil {
		want := strings.Trim(ifMatch, `"`)
		if existing.ETag != want {
			return c.NoContent(http.StatusPreconditionFailed)
		}
	}

	endAt := p.EndAt
	if endAt.IsZero() {
		endAt = p.StartAt.Add(time.Hour)
	}

	// Encode EXDATEs from the ICS into a comma-separated string for storage.
	var exdates []string
	for _, t := range p.Exdates {
		exdates = append(exdates, t.UTC().Format(time.RFC3339))
	}
	var rdates []string
	for _, t := range p.Rdates {
		rdates = append(rdates, t.UTC().Format(time.RFC3339))
	}

	eventID := strings.TrimSuffix(c.Param("eventUID"), ".ics")
	in := service.EventInput{
		Title:       p.Summary,
		Description: p.Description,
		Location:    p.Location,
		StartAt:     p.StartAt,
		EndAt:       endAt,
		Timezone:    "UTC",
		AllDay:      p.AllDay,
		RRule:       p.RRule,
		Ownership:   "caldav_created",
	}
	if existing == nil {
		in.ID = eventID
	}

	var masterID string
	if existing != nil {
		if err := h.eventService.Update(c.Request().Context(), user.ID, existing.ID, in); err != nil {
			return c.NoContent(http.StatusInternalServerError)
		}
		// Update exdates/rdates directly (not in EventInput to keep the interface simple).
		if err := h.eventService.SetExdatesRdates(c.Request().Context(), existing.ID,
			strings.Join(exdates, ","), strings.Join(rdates, ",")); err != nil {
			return c.NoContent(http.StatusInternalServerError)
		}
		masterID = existing.ID
		updated, _ := h.eventService.GetByID(c.Request().Context(), existing.ID)
		if updated != nil {
			c.Response().Header().Set("ETag", `"`+updated.ETag+`"`)
		}
	} else {
		created, err := h.eventService.Create(c.Request().Context(), user.ID, calID, in)
		if err != nil {
			return c.NoContent(http.StatusInternalServerError)
		}
		if strings.Join(exdates, ",") != "" || strings.Join(rdates, ",") != "" {
			_ = h.eventService.SetExdatesRdates(c.Request().Context(), created.ID,
				strings.Join(exdates, ","), strings.Join(rdates, ","))
		}
		masterID = created.ID
		c.Response().Header().Set("ETag", `"`+created.ETag+`"`)
	}

	// Store exception VEVENTs as EventException rows.
	for _, ev := range exceptionVEVENTs {
		exEndAt := ev.EndAt
		if exEndAt.IsZero() {
			exEndAt = ev.StartAt.Add(time.Hour)
		}
		if err := h.eventService.UpsertCalDAVException(c.Request().Context(), user.ID, masterID, ev.RecurrenceID, service.EventInput{
			Title:       ev.Summary,
			Description: ev.Description,
			Location:    ev.Location,
			StartAt:     ev.StartAt,
			EndAt:       exEndAt,
			Timezone:    "UTC",
			AllDay:      ev.AllDay,
		}, ev.Status == "cancelled"); err != nil {
			return c.NoContent(http.StatusInternalServerError)
		}
	}

	if existing != nil {
		return c.NoContent(http.StatusNoContent)
	}
	return c.NoContent(http.StatusCreated)
}

// DeleteEvent handles DELETE /dav/calendars/:userID/:calID/:eventUID.ics
func (h *Handler) DeleteEvent(c echo.Context) error {
	user, err := h.authUser(c)
	if err != nil {
		requireAuth(c)
		return nil
	}
	if c.Param("userID") != user.ID {
		return c.NoContent(http.StatusForbidden)
	}
	calID := c.Param("calID")
	if _, err := h.calendarService.GetByID(c.Request().Context(), user.ID, calID); err != nil {
		return c.NoContent(http.StatusForbidden)
	}
	eventID := strings.TrimSuffix(c.Param("eventUID"), ".ics")
	event, err := h.eventService.GetByID(c.Request().Context(), eventID)
	if err != nil || event == nil || event.CalendarID != calID {
		return c.NoContent(http.StatusNotFound)
	}
	if ifMatch := c.Request().Header.Get("If-Match"); ifMatch != "" {
		want := strings.Trim(ifMatch, `"`)
		if event.ETag != want {
			return c.NoContent(http.StatusPreconditionFailed)
		}
	}
	if err := h.eventService.Delete(c.Request().Context(), user.ID, eventID); err != nil {
		return c.NoContent(http.StatusNotFound)
	}
	return c.NoContent(http.StatusNoContent)
}

// Report handles REPORT on a calendar collection; routes by report type.
func (h *Handler) Report(c echo.Context) error {
	user, err := h.authUser(c)
	if err != nil {
		requireAuth(c)
		return nil
	}
	if c.Param("userID") != user.ID {
		return c.NoContent(http.StatusForbidden)
	}
	calID := c.Param("calID")
	cal, err := h.calendarService.GetByID(c.Request().Context(), user.ID, calID)
	if err != nil || cal == nil {
		return c.NoContent(http.StatusNotFound)
	}

	body, _ := io.ReadAll(c.Request().Body)

	switch detectReport(body) {
	case "calendar-multiget":
		return h.handleMultiget(c, user, cal, body)
	case "sync-collection":
		return h.handleSyncCollection(c, user, cal, body)
	case "free-busy-query":
		return h.handleFreeBusy(c, user, cal, body)
	default:
		return h.handleCalendarQuery(c, user, cal, body)
	}
}

func (h *Handler) handleCalendarQuery(c echo.Context, user *repo.User, cal *repo.Calendar, body []byte) error {
	from, to, ok := extractTimeRange(body)
	if !ok {
		from = time.Now().AddDate(-1, 0, 0)
		to = time.Now().AddDate(1, 0, 0)
	}
	events, err := h.eventService.ListForRange(c.Request().Context(), []string{cal.ID}, from, to)
	if err != nil {
		return c.NoContent(http.StatusInternalServerError)
	}
	exsByMaster := h.loadExceptionMap(c, events)
	return davResponse(c, http.StatusMultiStatus, reportCalendarQuery(user.ID, cal, events, exsByMaster))
}

func (h *Handler) loadExceptionMap(c echo.Context, events []*repo.Event) map[string][]*repo.EventException {
	m := make(map[string][]*repo.EventException)
	for _, e := range events {
		if e.RRule != "" {
			exs, _ := h.eventService.LoadExceptions(c.Request().Context(), e.ID)
			m[e.ID] = exs
		}
	}
	return m
}

func (h *Handler) handleMultiget(c echo.Context, user *repo.User, cal *repo.Calendar, body []byte) error {
	hrefs := extractHrefs(body)
	eventMap := make(map[string]*repo.Event, len(hrefs))
	var events []*repo.Event
	for _, href := range hrefs {
		parts := strings.Split(strings.TrimSuffix(href, "/"), "/")
		eventID := strings.TrimSuffix(parts[len(parts)-1], ".ics")
		e, _ := h.eventService.GetByID(c.Request().Context(), eventID)
		if e != nil && e.CalendarID != cal.ID {
			e = nil
		}
		eventMap[href] = e
		if e != nil {
			events = append(events, e)
		}
	}
	exsByMaster := h.loadExceptionMap(c, events)
	return davResponse(c, http.StatusMultiStatus, reportCalendarMultiget(user.ID, cal, eventMap, hrefs, exsByMaster))
}

func (h *Handler) handleSyncCollection(c echo.Context, user *repo.User, cal *repo.Calendar, body []byte) error {
	tokenStr := extractSyncToken(body)
	var since time.Time
	if tokenStr != "" {
		if t, ok := parseSyncToken(tokenStr); ok {
			since = t
		}
	}

	events, err := h.eventService.ListModifiedSince(c.Request().Context(), cal.ID, since)
	if err != nil {
		return c.NoContent(http.StatusInternalServerError)
	}

	stamp, _ := h.eventService.GetSyncStamp(c.Request().Context(), cal.ID)
	newToken := buildSyncToken(cal.ID, stamp)
	return davResponse(c, http.StatusMultiStatus, reportSyncCollection(user.ID, cal, events, newToken))
}

func (h *Handler) handleFreeBusy(c echo.Context, user *repo.User, cal *repo.Calendar, body []byte) error {
	from, to, ok := extractTimeRange(body)
	if !ok {
		from = time.Now().AddDate(-1, 0, 0)
		to = time.Now().AddDate(1, 0, 0)
	}
	events, err := h.eventService.ListForRange(c.Request().Context(), []string{cal.ID}, from, to)
	if err != nil {
		return c.NoContent(http.StatusInternalServerError)
	}
	c.Response().Header().Set("Content-Type", "text/calendar; charset=utf-8")
	c.Response().WriteHeader(http.StatusOK)
	_, err = c.Response().Write([]byte(reportFreeBusy(cal, events, from, to)))
	return err
}

// MkCalendar handles MKCALENDAR /dav/calendars/:userID/:calID/
func (h *Handler) MkCalendar(c echo.Context) error {
	user, err := h.authUser(c)
	if err != nil {
		requireAuth(c)
		return nil
	}
	if c.Param("userID") != user.ID {
		return c.NoContent(http.StatusForbidden)
	}
	calID := c.Param("calID")

	// Reject if already exists.
	existing, _ := h.calendarService.GetByID(c.Request().Context(), user.ID, calID)
	if existing != nil {
		return c.NoContent(http.StatusMethodNotAllowed)
	}

	// Parse displayname from body if present.
	name := calID
	color := "#4f8ef7"
	if body, readErr := io.ReadAll(c.Request().Body); readErr == nil {
		if n := extractXMLText(body, "displayname"); n != "" {
			name = n
		}
		if clr := extractXMLText(body, "calendar-color"); clr != "" {
			color = clr
		}
	}

	if _, err := h.calendarService.CreateWithID(c.Request().Context(), user.ID, calID, name, color); err != nil {
		return c.NoContent(http.StatusInternalServerError)
	}
	return c.NoContent(http.StatusCreated)
}

// --- Helpers ---

func davResponse(c echo.Context, status int, body string) error {
	c.Response().Header().Set("Content-Type", "application/xml; charset=utf-8")
	c.Response().WriteHeader(status)
	_, err := c.Response().Write([]byte(`<?xml version="1.0" encoding="utf-8"?>` + body))
	return err
}

func davError(c echo.Context, status int, condition string) error {
	ns := `xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav"`
	body := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?><D:error %s><C:%s/></D:error>`, ns, condition)
	c.Response().Header().Set("Content-Type", "application/xml; charset=utf-8")
	c.Response().WriteHeader(status)
	_, err := c.Response().Write([]byte(body))
	return err
}

func depthHeader(c echo.Context) string {
	d := c.Request().Header.Get("Depth")
	if d == "" {
		return "1"
	}
	return d
}

func buildSyncToken(calID string, stamp time.Time) string {
	return fmt.Sprintf("https://huginn.local/ns/sync/%s/%d", calID, stamp.UnixNano())
}

func parseSyncToken(token string) (time.Time, bool) {
	parts := strings.Split(token, "/")
	if len(parts) == 0 {
		return time.Time{}, false
	}
	ns, err := strconv.ParseInt(parts[len(parts)-1], 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	return time.Unix(0, ns), true
}

func maxStamp(cal *repo.Calendar, events []*repo.Event) time.Time {
	t := cal.UpdatedAt
	for _, e := range events {
		if e.UpdatedAt.After(t) {
			t = e.UpdatedAt
		}
	}
	return t
}

func detectReport(body []byte) string {
	s := string(body)
	switch {
	case strings.Contains(s, "calendar-multiget"):
		return "calendar-multiget"
	case strings.Contains(s, "sync-collection"):
		return "sync-collection"
	case strings.Contains(s, "free-busy-query"):
		return "free-busy-query"
	default:
		return "calendar-query"
	}
}

var reHref = regexp.MustCompile(`<[^>]*href[^>]*>([^<]+)<`)

func extractHrefs(body []byte) []string {
	matches := reHref.FindAllSubmatch(body, -1)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		out = append(out, strings.TrimSpace(string(m[1])))
	}
	return out
}

var reTimeRange = regexp.MustCompile(`time-range[^>]*start="([^"]+)"[^>]*end="([^"]+)"`)

func extractTimeRange(body []byte) (from, to time.Time, ok bool) {
	m := reTimeRange.FindSubmatch(body)
	if m == nil {
		return
	}
	layout := "20060102T150405Z"
	from, errF := time.Parse(layout, string(m[1]))
	to, errT := time.Parse(layout, string(m[2]))
	if errF != nil || errT != nil {
		return
	}
	return from, to, true
}

var reSyncToken = regexp.MustCompile(`<[^>]*sync-token[^>]*>([^<]+)<`)

func extractSyncToken(body []byte) string {
	m := reSyncToken.FindSubmatch(body)
	if m == nil {
		return ""
	}
	return strings.TrimSpace(string(m[1]))
}

var reXMLText = regexp.MustCompile(`<[^>]*(%s)[^>]*>([^<]+)<`)

func extractXMLText(body []byte, tag string) string {
	re := regexp.MustCompile(`<[^>]*` + regexp.QuoteMeta(tag) + `[^>]*>([^<]+)<`)
	m := re.FindSubmatch(body)
	if m == nil {
		return ""
	}
	return strings.TrimSpace(string(m[1]))
}

func supportedReportSet() string {
	return `<supported-report-set>
      <supported-report><report><C:calendar-query/></report></supported-report>
      <supported-report><report><C:calendar-multiget/></report></supported-report>
      <supported-report><report><C:free-busy-query/></report></supported-report>
      <supported-report><report><sync-collection/></report></supported-report>
    </supported-report-set>`
}

// --- XML response builders ---

const davNS = `xmlns="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:CS="http://calendarserver.org/ns/" xmlns:A="http://apple.com/ns/ical/"`

func propfindPrincipal(userID string) string {
	return fmt.Sprintf(`<multistatus %s>
  <response>
    <href>/dav/</href>
    <propstat>
      <prop>
        <resourcetype><principal/></resourcetype>
        <displayname>Huginn User</displayname>
        <principal-URL><href>/dav/</href></principal-URL>
        <C:calendar-home-set><href>/dav/calendars/%s/</href></C:calendar-home-set>
        <current-user-principal><href>/dav/</href></current-user-principal>
        <C:schedule-inbox-URL><href>/dav/calendars/%s/inbox/</href></C:schedule-inbox-URL>
        <C:schedule-outbox-URL><href>/dav/calendars/%s/outbox/</href></C:schedule-outbox-URL>
        <C:addressbook-home-set/>
        %s
      </prop>
      <status>HTTP/1.1 200 OK</status>
    </propstat>
  </response>
</multistatus>`, davNS, userID, userID, userID, supportedReportSet())
}

func propfindCalendarHomeDepth0(userID string) string {
	return fmt.Sprintf(`<multistatus %s>
  <response>
    <href>/dav/calendars/%s/</href>
    <propstat>
      <prop>
        <resourcetype><collection/></resourcetype>
        <displayname>Calendars</displayname>
      </prop>
      <status>HTTP/1.1 200 OK</status>
    </propstat>
  </response>
</multistatus>`, davNS, userID)
}

func propfindCalendarHome(userID string, calendars []*repo.Calendar) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`<multistatus %s>`, davNS))
	sb.WriteString(fmt.Sprintf(`
  <response>
    <href>/dav/calendars/%s/</href>
    <propstat>
      <prop><resourcetype><collection/></resourcetype></prop>
      <status>HTTP/1.1 200 OK</status>
    </propstat>
  </response>`, userID))
	for _, cal := range calendars {
		sb.WriteString(fmt.Sprintf(`
  <response>
    <href>/dav/calendars/%s/%s/</href>
    <propstat>
      <prop>
        <resourcetype><collection/><C:calendar/></resourcetype>
        <displayname>%s</displayname>
        <CS:getctag>%s</CS:getctag>
        <C:supported-calendar-component-set>
          <C:comp name="VEVENT"/>
          <C:comp name="VTODO"/>
        </C:supported-calendar-component-set>
        <A:calendar-color>%s</A:calendar-color>
      </prop>
      <status>HTTP/1.1 200 OK</status>
    </propstat>
  </response>`, userID, cal.ID, xmlEscape(cal.Name), xmlEscape(cal.UpdatedAt.Format(time.RFC3339Nano)), xmlEscape(cal.Color)))
	}
	sb.WriteString("\n</multistatus>")
	return sb.String()
}

func propfindCalendarDepth0(userID string, cal *repo.Calendar, stamp time.Time) string {
	token := buildSyncToken(cal.ID, stamp)
	return fmt.Sprintf(`<multistatus %s>
  <response>
    <href>/dav/calendars/%s/%s/</href>
    <propstat>
      <prop>
        <resourcetype><collection/><C:calendar/></resourcetype>
        <displayname>%s</displayname>
        <C:calendar-description>%s</C:calendar-description>
        <A:calendar-color>%s</A:calendar-color>
        <C:calendar-timezone>UTC</C:calendar-timezone>
        <C:supported-calendar-component-set>
          <C:comp name="VEVENT"/>
          <C:comp name="VTODO"/>
        </C:supported-calendar-component-set>
        <CS:getctag>%s</CS:getctag>
        <sync-token>%s</sync-token>
        %s
      </prop>
      <status>HTTP/1.1 200 OK</status>
    </propstat>
  </response>
</multistatus>`, davNS, userID, cal.ID,
		xmlEscape(cal.Name), xmlEscape(cal.Description), xmlEscape(cal.Color),
		xmlEscape(stamp.Format(time.RFC3339Nano)), xmlEscape(token), supportedReportSet())
}

func propfindCalendar(userID string, cal *repo.Calendar, events []*repo.Event) string {
	stamp := maxStamp(cal, events)
	token := buildSyncToken(cal.ID, stamp)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`<multistatus %s>`, davNS))
	sb.WriteString(fmt.Sprintf(`
  <response>
    <href>/dav/calendars/%s/%s/</href>
    <propstat>
      <prop>
        <resourcetype><collection/><C:calendar/></resourcetype>
        <displayname>%s</displayname>
        <C:calendar-description>%s</C:calendar-description>
        <A:calendar-color>%s</A:calendar-color>
        <C:calendar-timezone>UTC</C:calendar-timezone>
        <C:supported-calendar-component-set>
          <C:comp name="VEVENT"/>
          <C:comp name="VTODO"/>
        </C:supported-calendar-component-set>
        <CS:getctag>%s</CS:getctag>
        <sync-token>%s</sync-token>
        %s
      </prop>
      <status>HTTP/1.1 200 OK</status>
    </propstat>
  </response>`, userID, cal.ID,
		xmlEscape(cal.Name), xmlEscape(cal.Description), xmlEscape(cal.Color),
		xmlEscape(stamp.Format(time.RFC3339Nano)), xmlEscape(token), supportedReportSet()))
	for _, e := range events {
		sb.WriteString(fmt.Sprintf(`
  <response>
    <href>/dav/calendars/%s/%s/%s.ics</href>
    <propstat>
      <prop>
        <resourcetype/>
        <getetag>"%s"</getetag>
        <getcontenttype>text/calendar; charset=utf-8</getcontenttype>
      </prop>
      <status>HTTP/1.1 200 OK</status>
    </propstat>
  </response>`, userID, cal.ID, e.ID, e.ETag))
	}
	sb.WriteString("\n</multistatus>")
	return sb.String()
}

func propfindEventResource(userID, calID string, e *repo.Event) string {
	return fmt.Sprintf(`<multistatus %s>
  <response>
    <href>/dav/calendars/%s/%s/%s.ics</href>
    <propstat>
      <prop>
        <resourcetype/>
        <getetag>"%s"</getetag>
        <getcontenttype>text/calendar; charset=utf-8</getcontenttype>
      </prop>
      <status>HTTP/1.1 200 OK</status>
    </propstat>
  </response>
</multistatus>`, davNS, userID, calID, e.ID, e.ETag)
}

func reportCalendarQuery(userID string, cal *repo.Calendar, events []*repo.Event, exsByMaster map[string][]*repo.EventException) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`<multistatus %s>`, davNS))
	sb.WriteString(fmt.Sprintf(`
  <response>
    <href>/dav/calendars/%s/%s/</href>
    <propstat>
      <prop>
        <resourcetype><collection/><C:calendar/></resourcetype>
        <displayname>%s</displayname>
      </prop>
      <status>HTTP/1.1 200 OK</status>
    </propstat>
  </response>`, userID, cal.ID, xmlEscape(cal.Name)))
	for _, e := range events {
		var buf bytes.Buffer
		_ = ics.WriteWithExceptions(&buf, cal.Name, []*repo.Event{e}, exsByMaster)
		sb.WriteString(fmt.Sprintf(`
  <response>
    <href>/dav/calendars/%s/%s/%s.ics</href>
    <propstat>
      <prop>
        <getetag>"%s"</getetag>
        <C:calendar-data>%s</C:calendar-data>
      </prop>
      <status>HTTP/1.1 200 OK</status>
    </propstat>
  </response>`, userID, cal.ID, e.ID, e.ETag, xmlEscape(buf.String())))
	}
	sb.WriteString("\n</multistatus>")
	return sb.String()
}

func reportCalendarMultiget(userID string, cal *repo.Calendar, eventMap map[string]*repo.Event, hrefs []string, exsByMaster map[string][]*repo.EventException) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`<multistatus %s>`, davNS))
	for _, href := range hrefs {
		e := eventMap[href]
		if e == nil {
			sb.WriteString(fmt.Sprintf(`
  <response>
    <href>%s</href>
    <status>HTTP/1.1 404 Not Found</status>
  </response>`, xmlEscape(href)))
			continue
		}
		var buf bytes.Buffer
		_ = ics.WriteWithExceptions(&buf, cal.Name, []*repo.Event{e}, exsByMaster)
		sb.WriteString(fmt.Sprintf(`
  <response>
    <href>%s</href>
    <propstat>
      <prop>
        <getetag>"%s"</getetag>
        <C:calendar-data>%s</C:calendar-data>
      </prop>
      <status>HTTP/1.1 200 OK</status>
    </propstat>
  </response>`, xmlEscape(href), e.ETag, xmlEscape(buf.String())))
	}
	sb.WriteString("\n</multistatus>")
	return sb.String()
}

func reportSyncCollection(userID string, cal *repo.Calendar, events []*repo.Event, newToken string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`<multistatus %s>`, davNS))
	for _, e := range events {
		if e.DeletedAt != nil {
			sb.WriteString(fmt.Sprintf(`
  <response>
    <href>/dav/calendars/%s/%s/%s.ics</href>
    <status>HTTP/1.1 404 Not Found</status>
  </response>`, userID, cal.ID, e.ID))
			continue
		}
		sb.WriteString(fmt.Sprintf(`
  <response>
    <href>/dav/calendars/%s/%s/%s.ics</href>
    <propstat>
      <prop>
        <getetag>"%s"</getetag>
      </prop>
      <status>HTTP/1.1 200 OK</status>
    </propstat>
  </response>`, userID, cal.ID, e.ID, e.ETag))
	}
	sb.WriteString(fmt.Sprintf(`
  <sync-token>%s</sync-token>`, xmlEscape(newToken)))
	sb.WriteString("\n</multistatus>")
	return sb.String()
}

func reportFreeBusy(cal *repo.Calendar, events []*repo.Event, from, to time.Time) string {
	const layout = "20060102T150405Z"
	var fb strings.Builder
	for _, e := range events {
		if e.BusyStatus == "free" {
			continue
		}
		fb.WriteString("FREEBUSY:")
		fb.WriteString(e.StartAt.UTC().Format(layout))
		fb.WriteString("/")
		fb.WriteString(e.EndAt.UTC().Format(layout))
		fb.WriteString("\r\n")
	}
	return "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Huginn//Huginn Calendar//EN\r\n" +
		"BEGIN:VFREEBUSY\r\n" +
		"DTSTART:" + from.UTC().Format(layout) + "\r\n" +
		"DTEND:" + to.UTC().Format(layout) + "\r\n" +
		fb.String() +
		"END:VFREEBUSY\r\nEND:VCALENDAR\r\n"
}
