package calendar

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/FyrmForge/hamr/pkg/respond"
	"github.com/labstack/echo/v4"

	"github.com/FyrmForge/huginn/internal/repo"
	"github.com/FyrmForge/huginn/internal/service"
	"github.com/FyrmForge/huginn/internal/web/components"
)

type handler struct {
	calendarService *service.CalendarService
	eventService    *service.EventService
}

func NewHandler(calendarService *service.CalendarService, eventService *service.EventService) *handler {
	return &handler{
		calendarService: calendarService,
		eventService:    eventService,
	}
}

// GET / — month view
func (h *handler) Month(c echo.Context) error {
	user := components.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized)
	}

	if err := h.calendarService.EnsureDefaults(c.Request().Context(), user.ID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "setup error")
	}

	year, month := currentYearMonth(c)
	calendars, occs, err := h.loadMonthData(c, user, year, month)
	if err != nil {
		return err
	}

	grid := buildMonthGrid(year, month, occs)

	return respond.HTML(c, http.StatusOK, monthPage(c, monthViewData{
		Calendars: calendars,
		Grid:      grid,
		Year:      year,
		Month:     month,
	}))
}

// GET /week
func (h *handler) Week(c echo.Context) error {
	user := components.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized)
	}
	if err := h.calendarService.EnsureDefaults(c.Request().Context(), user.ID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError)
	}
	weekStart := currentWeekStart(c)
	calendars, occs, err := h.loadWeekData(c, user, weekStart)
	if err != nil {
		return err
	}
	return respond.HTML(c, http.StatusOK, weekPage(c, weekViewData{
		Calendars: calendars,
		Days:      buildWeekGrid(weekStart, occs),
		WeekStart: weekStart,
	}))
}

// GET /week/grid — HTMX partial swap for week prev/next navigation.
func (h *handler) WeekGrid(c echo.Context) error {
	user := components.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized)
	}
	weekStart := currentWeekStart(c)
	_, occs, err := h.loadWeekData(c, user, weekStart)
	if err != nil {
		return err
	}
	return respond.HTML(c, http.StatusOK, weekGrid(buildWeekGrid(weekStart, occs), weekStart))
}

// GET /calendar/grid?year=&month= — HTMX partial swap for prev/next navigation.
func (h *handler) Grid(c echo.Context) error {
	user := components.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized)
	}

	year, month := currentYearMonth(c)
	_, occs, err := h.loadMonthData(c, user, year, month)
	if err != nil {
		return err
	}
	grid := buildMonthGrid(year, month, occs)

	return respond.HTML(c, http.StatusOK, monthGrid(grid, year, month))
}

func (h *handler) loadMonthData(c echo.Context, user *repo.User, year int, month time.Month) ([]*repo.Calendar, []service.Occurrence, error) {
	calendars, err := h.calendarService.ListForUser(c.Request().Context(), user.ID)
	if err != nil {
		return nil, nil, echo.NewHTTPError(http.StatusInternalServerError, "load calendars error")
	}

	ids := make([]string, len(calendars))
	for i, cal := range calendars {
		ids[i] = cal.ID
	}

	from := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 1, 0)
	from = from.AddDate(0, 0, -6)
	to = to.AddDate(0, 0, 6)

	occs, err := h.eventService.ExpandForRange(c.Request().Context(), ids, from, to)
	if err != nil {
		return nil, nil, echo.NewHTTPError(http.StatusInternalServerError, "load events error")
	}

	return calendars, occs, nil
}

func currentYearMonth(c echo.Context) (int, time.Month) {
	now := time.Now()
	year := now.Year()
	month := now.Month()
	if y, err := strconv.Atoi(c.QueryParam("year")); err == nil && y > 2000 {
		year = y
	}
	if m, err := strconv.Atoi(c.QueryParam("month")); err == nil && m >= 1 && m <= 12 {
		month = time.Month(m)
	}
	return year, month
}

// MonthDay is a single cell in the month grid.
type MonthDay struct {
	Date       time.Time
	OtherMonth bool
	IsToday    bool
	Events     []service.Occurrence
}

// buildMonthGrid builds a 6-week calendar grid for the given month.
func buildMonthGrid(year int, month time.Month, occs []service.Occurrence) []MonthDay {
	first := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	startOffset := int(first.Weekday()) - 1
	if startOffset < 0 {
		startOffset = 6
	}
	start := first.AddDate(0, 0, -startOffset)
	today := time.Now().UTC().Truncate(24 * time.Hour)

	byDate := make(map[time.Time][]service.Occurrence)
	for _, o := range occs {
		startDay := o.Start.UTC().Truncate(24 * time.Hour)
		endDay := o.End.UTC().Truncate(24 * time.Hour)
		for d := startDay; !d.After(endDay); d = d.AddDate(0, 0, 1) {
			byDate[d] = append(byDate[d], o)
		}
	}

	grid := make([]MonthDay, 42)
	for i := range grid {
		d := start.AddDate(0, 0, i)
		grid[i] = MonthDay{
			Date:       d,
			OtherMonth: d.Month() != month,
			IsToday:    d.Equal(today),
			Events:     byDate[d],
		}
	}
	return grid
}

type monthViewData struct {
	Calendars []*repo.Calendar
	Grid      []MonthDay
	Year      int
	Month     time.Month
}


// WeekOccurrence wraps an occurrence with its pixel offsets within a 1440px day column.
type WeekOccurrence struct {
	Occ      service.Occurrence
	TopPx    int
	HeightPx int
}

// WeekDay is a single day column in the week grid.
type WeekDay struct {
	Date    time.Time
	IsToday bool
	AllDay  []service.Occurrence
	Timed   []WeekOccurrence
}

type weekViewData struct {
	Calendars []*repo.Calendar
	Days      []WeekDay
	WeekStart time.Time
}

func currentWeekStart(c echo.Context) time.Time {
	now := time.Now().UTC()
	wd := int(now.Weekday())
	if wd == 0 {
		wd = 7
	}
	monday := now.AddDate(0, 0, -(wd - 1))
	monday = time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, time.UTC)
	if w := c.QueryParam("week"); w != "" {
		if t, err := time.Parse("2006-01-02", w); err == nil {
			monday = t.UTC()
		}
	}
	return monday
}

func (h *handler) loadWeekData(c echo.Context, user *repo.User, weekStart time.Time) ([]*repo.Calendar, []service.Occurrence, error) {
	calendars, err := h.calendarService.ListForUser(c.Request().Context(), user.ID)
	if err != nil {
		return nil, nil, echo.NewHTTPError(http.StatusInternalServerError, "load calendars error")
	}
	ids := make([]string, len(calendars))
	for i, cal := range calendars {
		ids[i] = cal.ID
	}
	occs, err := h.eventService.ExpandForRange(c.Request().Context(), ids, weekStart, weekStart.AddDate(0, 0, 7))
	if err != nil {
		return nil, nil, echo.NewHTTPError(http.StatusInternalServerError, "load events error")
	}
	return calendars, occs, nil
}

func buildWeekGrid(weekStart time.Time, occs []service.Occurrence) []WeekDay {
	today := time.Now().UTC().Truncate(24 * time.Hour)
	byDate := make(map[time.Time][]service.Occurrence)
	for _, o := range occs {
		startDay := o.Start.UTC().Truncate(24 * time.Hour)
		if o.AllDay {
			endDay := o.End.UTC().Truncate(24 * time.Hour)
			for d := startDay; !d.After(endDay); d = d.AddDate(0, 0, 1) {
				byDate[d] = append(byDate[d], o)
			}
		} else {
			byDate[startDay] = append(byDate[startDay], o)
		}
	}
	days := make([]WeekDay, 7)
	for i := range days {
		d := weekStart.AddDate(0, 0, i)
		wd := WeekDay{Date: d, IsToday: d.Equal(today)}
		for _, o := range byDate[d] {
			if o.AllDay {
				wd.AllDay = append(wd.AllDay, o)
			} else {
				start := o.Start.UTC()
				end := o.End.UTC()
				topPx := start.Hour()*60 + start.Minute()
				durPx := (end.Hour()*60 + end.Minute()) - topPx
				if durPx < 20 {
					durPx = 20
				}
				wd.Timed = append(wd.Timed, WeekOccurrence{Occ: o, TopPx: topPx, HeightPx: durPx})
			}
		}
		days[i] = wd
	}
	return days
}

func hasAllDay(days []WeekDay) bool {
	for _, d := range days {
		if len(d.AllDay) > 0 {
			return true
		}
	}
	return false
}

func weekHours() []int {
	hours := make([]int, 24)
	for i := range hours {
		hours[i] = i
	}
	return hours
}

func prevWeekURL(weekStart time.Time) string {
	return fmt.Sprintf("/week/grid?week=%s", weekStart.AddDate(0, 0, -7).Format("2006-01-02"))
}

func nextWeekURL(weekStart time.Time) string {
	return fmt.Sprintf("/week/grid?week=%s", weekStart.AddDate(0, 0, 7).Format("2006-01-02"))
}
