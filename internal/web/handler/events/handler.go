package events

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/FyrmForge/hamr/pkg/respond"
	"github.com/FyrmForge/hamr/pkg/validate"
	"github.com/labstack/echo/v4"

	"github.com/FyrmForge/huginn/internal/repo"
	"github.com/FyrmForge/huginn/internal/service"
	"github.com/FyrmForge/huginn/internal/web/components"
	"github.com/FyrmForge/huginn/internal/web/components/form"
)

type handler struct {
	calendarService *service.CalendarService
	eventService    *service.EventService
	settingsService *service.SettingsService

	FormRules validate.Form
}

func NewHandler(calendarService *service.CalendarService, eventService *service.EventService, settingsService *service.SettingsService) *handler {
	return &handler{
		calendarService: calendarService,
		eventService:    eventService,
		settingsService: settingsService,
		FormRules: validate.NewForm(
			validate.WithOOBRenderer(form.OOBValidator),
			validate.Field("title", validate.Required),
			validate.Field("calendar_id", validate.Required),
			validate.Field("start_at", validate.Required),
			validate.Field("end_at", validate.Required),
		),
	}
}

func (h *handler) userLoc(c echo.Context, userID string) *time.Location {
	s, err := h.settingsService.GetOrDefault(c.Request().Context(), userID)
	if err != nil || s.Timezone == "" {
		return time.UTC
	}
	loc, err := time.LoadLocation(s.Timezone)
	if err != nil {
		return time.UTC
	}
	return loc
}

// canEditCalendar returns true if userID has owner or editor access to calendarID.
func (h *handler) canEditCalendar(c echo.Context, calendarID, userID string) bool {
	role, err := h.calendarService.GetUserRole(c.Request().Context(), calendarID, userID)
	return err == nil && (role == "owner" || role == "editor")
}

// GET /events/new?date=YYYY-MM-DD
func (h *handler) New(c echo.Context) error {
	user := components.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized)
	}

	calendars, err := h.calendarService.ListEditableForUser(c.Request().Context(), user.ID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	loc := h.userLoc(c, user.ID)
	start := time.Now().In(loc).Truncate(time.Hour)
	if d := c.QueryParam("date"); d != "" {
		if t, err := time.Parse("2006-01-02", d); err == nil {
			start = time.Date(t.Year(), t.Month(), t.Day(), start.Hour(), 0, 0, 0, loc)
		}
	}
	end := start.Add(time.Hour)

	f := EventForm{
		StartAt:       start.Format("2006-01-02T15:04"),
		EndAt:         end.Format("2006-01-02T15:04"),
		RecurInterval: 1,
		RecurEndType:  "never",
	}

	return respond.HTML(c, http.StatusOK, eventModal(c, f, calendars, nil, "", "", ""))
}

// POST /events
func (h *handler) Create(c echo.Context) error {
	user := components.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized)
	}

	var f EventForm
	if err := c.Bind(&f); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest)
	}
	if f.RecurInterval <= 0 {
		f.RecurInterval = 1
	}

	if errs := h.FormRules.Validate(c); errs != nil {
		calendars, _ := h.calendarService.ListEditableForUser(c.Request().Context(), user.ID)
		return respond.HTML(c, http.StatusUnprocessableEntity, eventModal(c, f, calendars, errs, "", "", ""))
	}

	if !h.canEditCalendar(c, f.CalendarID, user.ID) {
		calendars, _ := h.calendarService.ListEditableForUser(c.Request().Context(), user.ID)
		return respond.HTML(c, http.StatusUnprocessableEntity, eventModal(c, f, calendars, map[string]string{"general": "You don't have permission to add events to this calendar"}, "", "", ""))
	}

	loc := h.userLoc(c, user.ID)
	in, err := f.toInput(loc)
	if err != nil {
		calendars, _ := h.calendarService.ListEditableForUser(c.Request().Context(), user.ID)
		return respond.HTML(c, http.StatusUnprocessableEntity, eventModal(c, f, calendars, map[string]string{"general": "Invalid date/time"}, "", "", ""))
	}

	_, err = h.eventService.Create(c.Request().Context(), user.ID, f.CalendarID, in)
	if err != nil {
		calendars, _ := h.calendarService.ListEditableForUser(c.Request().Context(), user.ID)
		return respond.HTML(c, http.StatusUnprocessableEntity, eventModal(c, f, calendars, map[string]string{"general": "Failed to save event"}, "", "", ""))
	}

	c.Response().Header().Set("HX-Trigger", "huginn:refresh-calendar")
	return respond.HTML(c, http.StatusOK, modalClosed())
}

// GET /events/:id/edit
// If ?rid= is set and no ?scope=: show scope picker for recurring occurrence.
// If ?rid= and ?scope=: show edit form for that scope.
// Otherwise: show edit form for the event/series.
func (h *handler) Edit(c echo.Context) error {
	user := components.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized)
	}

	e, err := h.eventService.GetByID(c.Request().Context(), c.Param("id"))
	if err != nil || e == nil {
		return echo.NewHTTPError(http.StatusNotFound)
	}

	rid := c.QueryParam("rid")
	scope := c.QueryParam("scope")

	// Recurring occurrence with no scope yet: default to this single occurrence.
	// The scope selector lives inside the modal; switching it reloads the form.
	if e.RRule != "" && rid != "" && scope == "" {
		scope = "this"
	}

	// Check user can edit this event's calendar; viewers get a read-only modal.
	if !h.canEditCalendar(c, e.CalendarID, user.ID) {
		role, _ := h.calendarService.GetUserRole(c.Request().Context(), e.CalendarID, user.ID)
		if role == "" {
			return echo.NewHTTPError(http.StatusForbidden)
		}
		loc := h.userLoc(c, user.ID)
		dtFmt := "02 Jan 2006, 15:04"
		if e.AllDay {
			dtFmt = "02 Jan 2006"
		}
		calName := ""
		if cal, err := h.calendarService.GetByID(c.Request().Context(), user.ID, e.CalendarID); err == nil && cal != nil {
			calName = cal.Name
		}
		createdBy := ""
		if e.CreatedBy != nil {
			createdBy = h.eventService.GetUserDisplay(c.Request().Context(), *e.CreatedBy)
		}
		return respond.HTML(c, http.StatusOK, eventReadOnlyModal(
			e.Title, calName,
			e.StartAt.In(loc).Format(dtFmt),
			e.EndAt.In(loc).Format(dtFmt),
			e.Description, e.Location, createdBy,
		))
	}

	calendars, err := h.calendarService.ListEditableForUser(c.Request().Context(), user.ID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	loc := h.userLoc(c, user.ID)

	// For scope=this: pre-fill from exception data if exists, else from occurrence time.
	if scope == "this" && rid != "" {
		recurrenceID, _ := time.Parse(time.RFC3339, rid)
		ex, _ := h.eventService.GetException(c.Request().Context(), e.ID, recurrenceID)
		start, end := recurrenceID, recurrenceID.Add(e.EndAt.Sub(e.StartAt))
		title := e.Title
		description := e.Description
		location := e.Location
		allDay := e.AllDay
		if ex != nil {
			start = ex.StartAt
			end = ex.EndAt
			title = ex.Title
			description = ex.Description
			location = ex.Location
			allDay = ex.AllDay
		}
		dtFmt := "2006-01-02T15:04"
		if allDay {
			dtFmt = "2006-01-02"
		}
		f := EventForm{
			Title:       title,
			Description: description,
			Location:    location,
			CalendarID:  e.CalendarID,
			StartAt:     start.In(loc).Format(dtFmt),
			EndAt:       end.In(loc).Format(dtFmt),
			AllDay:      allDay,
		}
		return respond.HTML(c, http.StatusOK, eventModal(c, f, calendars, nil, e.ID, rid, scope))
	}

	// scope=all or scope=future or non-recurring: edit master/series.
	f := eventToForm(e, loc)
	if e.CreatedBy != nil {
		f.CreatedByName = h.eventService.GetUserDisplay(c.Request().Context(), *e.CreatedBy)
	}
	return respond.HTML(c, http.StatusOK, eventModal(c, f, calendars, nil, e.ID, rid, scope))
}

// GET /events/:id/confirm-delete
func (h *handler) ConfirmDelete(c echo.Context) error {
	user := components.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized)
	}
	e, err := h.eventService.GetByID(c.Request().Context(), c.Param("id"))
	if err != nil || e == nil {
		return echo.NewHTTPError(http.StatusNotFound)
	}
	if _, err := h.calendarService.GetByID(c.Request().Context(), user.ID, e.CalendarID); err != nil {
		return echo.NewHTTPError(http.StatusForbidden)
	}
	rid := c.QueryParam("rid")
	scope := c.QueryParam("scope")
	// Recurring occurrence: scope was chosen in the edit modal; default to this
	// occurrence so an empty scope never falls through to deleting the series.
	if e.RRule != "" && rid != "" && scope == "" {
		scope = "this"
	}
	return respond.HTML(c, http.StatusOK, confirmDeleteModal(e.ID, e.Title, rid, scope))
}

// POST /events/:id
func (h *handler) Update(c echo.Context) error {
	user := components.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized)
	}

	var f EventForm
	if err := c.Bind(&f); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest)
	}
	if f.RecurInterval <= 0 {
		f.RecurInterval = 1
	}

	rid := c.QueryParam("rid")
	scope := c.QueryParam("scope")

	if errs := h.FormRules.Validate(c); errs != nil {
		calendars, _ := h.calendarService.ListEditableForUser(c.Request().Context(), user.ID)
		return respond.HTML(c, http.StatusUnprocessableEntity, eventModal(c, f, calendars, errs, c.Param("id"), rid, scope))
	}

	// Permission check.
	e, err := h.eventService.GetByID(c.Request().Context(), c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError)
	}
	if e == nil {
		return echo.NewHTTPError(http.StatusNotFound)
	}
	if !h.canEditCalendar(c, e.CalendarID, user.ID) {
		return echo.NewHTTPError(http.StatusForbidden, "view-only access")
	}

	loc := h.userLoc(c, user.ID)
	in, err := f.toInput(loc)
	if err != nil {
		calendars, _ := h.calendarService.ListEditableForUser(c.Request().Context(), user.ID)
		return respond.HTML(c, http.StatusUnprocessableEntity, eventModal(c, f, calendars, map[string]string{"general": "Invalid date/time"}, c.Param("id"), rid, scope))
	}

	var updateErr error
	switch {
	case scope == "this" && rid != "":
		recurrenceID, _ := time.Parse(time.RFC3339, rid)
		updateErr = h.eventService.UpdateOccurrence(c.Request().Context(), user.ID, c.Param("id"), recurrenceID, in)
	case scope == "future" && rid != "":
		recurrenceID, _ := time.Parse(time.RFC3339, rid)
		updateErr = h.eventService.UpdateThisAndFuture(c.Request().Context(), user.ID, c.Param("id"), recurrenceID, in)
	default:
		updateErr = h.eventService.Update(c.Request().Context(), user.ID, c.Param("id"), in)
	}

	if updateErr != nil {
		calendars, _ := h.calendarService.ListEditableForUser(c.Request().Context(), user.ID)
		return respond.HTML(c, http.StatusUnprocessableEntity, eventModal(c, f, calendars, map[string]string{"general": updateErr.Error()}, c.Param("id"), rid, scope))
	}

	c.Response().Header().Set("HX-Trigger", "huginn:refresh-calendar")
	return respond.HTML(c, http.StatusOK, modalClosed())
}

// DELETE /events/:id
func (h *handler) Delete(c echo.Context) error {
	user := components.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized)
	}

	rid := c.QueryParam("rid")
	scope := c.QueryParam("scope")

	// Permission check.
	ev, err := h.eventService.GetByID(c.Request().Context(), c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError)
	}
	if ev == nil {
		return echo.NewHTTPError(http.StatusNotFound)
	}
	if !h.canEditCalendar(c, ev.CalendarID, user.ID) {
		return echo.NewHTTPError(http.StatusForbidden, "view-only access")
	}

	var deleteErr error
	switch {
	case scope == "this" && rid != "":
		recurrenceID, _ := time.Parse(time.RFC3339, rid)
		deleteErr = h.eventService.DeleteOccurrence(c.Request().Context(), user.ID, c.Param("id"), recurrenceID)
	case scope == "future" && rid != "":
		recurrenceID, _ := time.Parse(time.RFC3339, rid)
		deleteErr = h.eventService.DeleteThisAndFuture(c.Request().Context(), user.ID, c.Param("id"), recurrenceID)
	default:
		deleteErr = h.eventService.Delete(c.Request().Context(), user.ID, c.Param("id"))
	}

	if deleteErr != nil {
		return echo.NewHTTPError(http.StatusForbidden, deleteErr.Error())
	}

	c.Response().Header().Set("HX-Trigger", "huginn:refresh-calendar")
	return respond.HTML(c, http.StatusOK, modalClosed())
}

// eventToForm converts a repo.Event (master) to an EventForm for the edit UI.
func eventToForm(e *repo.Event, loc *time.Location) EventForm {
	f := EventForm{
		Title:       e.Title,
		Description: e.Description,
		Location:    e.Location,
		CalendarID:  e.CalendarID,
		AllDay:      e.AllDay,
	}
	if e.AllDay {
		f.StartAt = e.StartAt.In(loc).Format("2006-01-02")
		f.EndAt = e.EndAt.In(loc).Format("2006-01-02")
	} else {
		f.StartAt = e.StartAt.In(loc).Format("2006-01-02T15:04")
		f.EndAt = e.EndAt.In(loc).Format("2006-01-02T15:04")
	}
	if e.RRule != "" {
		f.RecurEnabled = true
		f.RecurFreq, f.RecurInterval, f.RecurByDay, f.RecurEndType, f.RecurUntil, f.RecurCount = parseRRule(e.RRule)
	}
	if f.RecurInterval == 0 {
		f.RecurInterval = 1
	}
	if f.RecurEndType == "" {
		f.RecurEndType = "never"
	}
	return f
}

// parseRRule extracts fields from an RRULE string for form pre-fill.
func parseRRule(ruleStr string) (freq string, interval int, byday, endType, until string, count int) {
	interval = 1
	endType = "never"
	for _, part := range strings.Split(ruleStr, ";") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "FREQ":
			freq = kv[1]
		case "INTERVAL":
			_, _ = fmt.Sscanf(kv[1], "%d", &interval)
		case "BYDAY":
			byday = kv[1]
		case "UNTIL":
			endType = "until"
			// Parse UNTIL back to YYYY-MM-DD.
			if t, err := time.Parse("20060102T150405Z", kv[1]); err == nil {
				until = t.Format("2006-01-02")
			} else if t, err := time.Parse("20060102", kv[1]); err == nil {
				until = t.Format("2006-01-02")
			}
		case "COUNT":
			endType = "count"
			_, _ = fmt.Sscanf(kv[1], "%d", &count)
		}
	}
	return
}

// EventForm holds form values for create/edit.
type EventForm struct {
	Title       string `form:"title"`
	Description string `form:"description"`
	Location    string `form:"location"`
	CalendarID  string `form:"calendar_id"`
	StartAt     string `form:"start_at"`
	EndAt       string `form:"end_at"`
	AllDay      bool   `form:"all_day"`
	// Display-only (not bound from form)
	CreatedByName string `form:"-"`
	// Recurrence fields
	RecurEnabled  bool   `form:"recur_enabled"`
	RecurFreq     string `form:"recur_freq"`
	RecurInterval int    `form:"recur_interval"`
	RecurByDay    string `form:"recur_byday"`    // comma-separated: MO,TU,WE,TH,FR,SA,SU
	RecurEndType  string `form:"recur_end_type"` // never, until, count
	RecurUntil    string `form:"recur_until"`    // YYYY-MM-DD
	RecurCount    int    `form:"recur_count"`
}

func parseEventTime(val string, allDay bool, loc *time.Location) (time.Time, error) {
	if allDay {
		t, err := time.Parse("2006-01-02", val)
		return t, err
	}
	return time.ParseInLocation("2006-01-02T15:04", val, loc)
}

func (f EventForm) toInput(loc *time.Location) (service.EventInput, error) {
	start, err := parseEventTime(f.StartAt, f.AllDay, loc)
	if err != nil {
		return service.EventInput{}, err
	}
	end, err := parseEventTime(f.EndAt, f.AllDay, loc)
	if err != nil {
		return service.EventInput{}, err
	}
	return service.EventInput{
		Title:       f.Title,
		Description: f.Description,
		Location:    f.Location,
		StartAt:     start,
		EndAt:       end,
		Timezone:    loc.String(),
		AllDay:      f.AllDay,
		RRule:       f.buildRRule(),
	}, nil
}

func (f EventForm) buildRRule() string {
	if !f.RecurEnabled || f.RecurFreq == "" {
		return ""
	}
	freq := f.RecurFreq
	interval := f.RecurInterval
	if interval <= 0 {
		interval = 1
	}
	parts := []string{"FREQ=" + freq, fmt.Sprintf("INTERVAL=%d", interval)}
	if freq == "WEEKLY" && f.RecurByDay != "" {
		// RecurByDay arrives as form value(s); normalize to comma-separated.
		parts = append(parts, "BYDAY="+f.RecurByDay)
	}
	switch f.RecurEndType {
	case "until":
		if f.RecurUntil != "" {
			if t, err := time.Parse("2006-01-02", f.RecurUntil); err == nil {
				parts = append(parts, "UNTIL="+t.UTC().Format("20060102T235959Z"))
			}
		}
	case "count":
		if f.RecurCount > 0 {
			parts = append(parts, fmt.Sprintf("COUNT=%d", f.RecurCount))
		}
	}
	return strings.Join(parts, ";")
}

// SelectedDays returns a map of RRULE day codes that are selected, for template rendering.
func (f EventForm) SelectedDays() map[string]bool {
	m := make(map[string]bool)
	for _, d := range strings.Split(f.RecurByDay, ",") {
		m[strings.TrimSpace(d)] = true
	}
	return m
}
