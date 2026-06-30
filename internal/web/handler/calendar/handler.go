package calendar

import (
	"fmt"
	"net/http"
	"sort"
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

	return respond.HTML(c, http.StatusOK, monthPage(c, monthViewData{
		Calendars: calendars,
		Weeks:     buildMonthWeeks(year, month, occs),
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
		Week:      buildWeekGrid(weekStart, occs),
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
	calendars, occs, err := h.loadWeekData(c, user, weekStart)
	if err != nil {
		return err
	}
	return respond.HTML(c, http.StatusOK, weekGrid(buildWeekGrid(weekStart, occs), weekStart, calendarMap(calendars)))
}

// GET /calendar/grid?year=&month= — HTMX partial swap for prev/next navigation.
func (h *handler) Grid(c echo.Context) error {
	user := components.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized)
	}

	year, month := currentYearMonth(c)
	calendars, occs, err := h.loadMonthData(c, user, year, month)
	if err != nil {
		return err
	}
	return respond.HTML(c, http.StatusOK, monthGrid(buildMonthWeeks(year, month, occs), year, month, calendarMap(calendars)))
}

// calendarMap indexes calendars by ID for quick name/colour lookup at render.
func calendarMap(cals []*repo.Calendar) map[string]*repo.Calendar {
	m := make(map[string]*repo.Calendar, len(cals))
	for _, c := range cals {
		m[c.ID] = c
	}
	return m
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
	// ym=YYYY-MM from the native month picker takes precedence.
	if ym := c.QueryParam("ym"); ym != "" {
		if t, err := time.Parse("2006-01", ym); err == nil {
			return t.Year(), t.Month()
		}
	}
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
	IsPast     bool
	Events     []service.Occurrence
}

// buildMonthWeeks builds the 6-week month grid. Single-day events live in their
// day cell; multi-day events become spanning bars on each week they cross.
func buildMonthWeeks(year int, month time.Month, occs []service.Occurrence) []MonthWeek {
	first := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	startOffset := int(first.Weekday()) - 1
	if startOffset < 0 {
		startOffset = 6
	}
	start := first.AddDate(0, 0, -startOffset)
	today := time.Now().UTC().Truncate(24 * time.Hour)

	single := make(map[time.Time][]service.Occurrence)
	var multi []service.Occurrence
	for _, o := range occs {
		if isMultiDay(o) {
			multi = append(multi, o)
			continue
		}
		s, _ := occDayRange(o)
		single[s] = append(single[s], o)
	}

	weeks := make([]MonthWeek, 6)
	for w := range weeks {
		weekStart := start.AddDate(0, 0, w*7)
		days := make([]MonthDay, 7)
		for i := range days {
			d := weekStart.AddDate(0, 0, i)
			days[i] = MonthDay{
				Date:       d,
				OtherMonth: d.Month() != month,
				IsToday:    d.Equal(today),
				IsPast:     d.Before(today),
				Events:     single[d],
			}
		}
		bars, lanes := barsForWeek(weekStart, multi)
		weeks[w] = MonthWeek{Days: days, Bars: bars, Lanes: lanes}
	}
	return weeks
}

type monthViewData struct {
	Calendars []*repo.Calendar
	Weeks     []MonthWeek
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
	Timed   []WeekOccurrence
}

// SpanBar is a multi-day or all-day event drawn as a horizontal bar spanning
// columns within one week row (clamped to that week).
type SpanBar struct {
	Occ       service.Occurrence
	StartCol  int // 0-based column within the row
	EndCol    int // inclusive
	Lane      int
	ClipLeft  bool // event started before this row (no left cap)
	ClipRight bool // event continues after this row (no right cap)
}

// Span returns the number of columns the bar covers.
func (b SpanBar) Span() int { return b.EndCol - b.StartCol + 1 }

// MonthWeek is one row of the month grid: 7 day cells plus the multi-day bars
// that span across them.
type MonthWeek struct {
	Days  []MonthDay
	Bars  []SpanBar
	Lanes int
}

// WeekData is the full week view: 7 day columns, the multi-day/all-day span
// bars across the top, and how many lanes those bars need.
type WeekData struct {
	Days  []WeekDay
	Bars  []SpanBar
	Lanes int
}

type weekViewData struct {
	Calendars []*repo.Calendar
	Week      WeekData
	WeekStart time.Time
}

// packLanes assigns each bar the lowest lane whose previous bar ends before this
// one starts (greedy interval packing). Returns the number of lanes used.
func packLanes(bars []SpanBar) int {
	sort.SliceStable(bars, func(i, j int) bool {
		if bars[i].StartCol != bars[j].StartCol {
			return bars[i].StartCol < bars[j].StartCol
		}
		return bars[i].Span() > bars[j].Span()
	})
	laneEnd := []int{} // last occupied column per lane
	for i := range bars {
		placed := -1
		for l := range laneEnd {
			if laneEnd[l] < bars[i].StartCol {
				placed = l
				break
			}
		}
		if placed == -1 {
			laneEnd = append(laneEnd, bars[i].EndCol)
			bars[i].Lane = len(laneEnd) - 1
		} else {
			laneEnd[placed] = bars[i].EndCol
			bars[i].Lane = placed
		}
	}
	return len(laneEnd)
}

// occDayRange returns the inclusive [startDay, endDay] (UTC, day-truncated) an
// occurrence covers.
func occDayRange(o service.Occurrence) (time.Time, time.Time) {
	s := o.Start.UTC().Truncate(24 * time.Hour)
	e := o.End.UTC().Truncate(24 * time.Hour)
	if e.Before(s) {
		e = s
	}
	return s, e
}

func isMultiDay(o service.Occurrence) bool {
	s, e := occDayRange(o)
	return e.After(s)
}

// barsForWeek clamps the given multi-day/all-day occurrences to [weekStart,
// weekStart+6] and returns the bars overlapping that week, lane-packed.
func barsForWeek(weekStart time.Time, occs []service.Occurrence) ([]SpanBar, int) {
	weekEnd := weekStart.AddDate(0, 0, 6)
	var bars []SpanBar
	for _, o := range occs {
		s, e := occDayRange(o)
		if e.Before(weekStart) || s.After(weekEnd) {
			continue
		}
		segStart := s
		if segStart.Before(weekStart) {
			segStart = weekStart
		}
		segEnd := e
		if segEnd.After(weekEnd) {
			segEnd = weekEnd
		}
		bars = append(bars, SpanBar{
			Occ:       o,
			StartCol:  int(segStart.Sub(weekStart).Hours()) / 24,
			EndCol:    int(segEnd.Sub(weekStart).Hours()) / 24,
			ClipLeft:  s.Before(weekStart),
			ClipRight: e.After(weekEnd),
		})
	}
	lanes := packLanes(bars)
	return bars, lanes
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
			// Snap any picked day to the Monday of its week (no-op for the
			// prev/next nav, which already passes Mondays).
			twd := int(t.Weekday())
			if twd == 0 {
				twd = 7
			}
			monday = t.AddDate(0, 0, -(twd - 1)).UTC()
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

// buildWeekGrid lays out a week: timed single-day events go in their day
// column; all-day and multi-day events become span bars across the top.
func buildWeekGrid(weekStart time.Time, occs []service.Occurrence) WeekData {
	today := time.Now().UTC().Truncate(24 * time.Hour)

	days := make([]WeekDay, 7)
	for i := range days {
		d := weekStart.AddDate(0, 0, i)
		days[i] = WeekDay{Date: d, IsToday: d.Equal(today)}
	}

	var spanning []service.Occurrence
	for _, o := range occs {
		if o.AllDay || isMultiDay(o) {
			spanning = append(spanning, o)
			continue
		}
		startDay := o.Start.UTC().Truncate(24 * time.Hour)
		idx := int(startDay.Sub(weekStart).Hours()) / 24
		if idx < 0 || idx > 6 {
			continue
		}
		start := o.Start.UTC()
		end := o.End.UTC()
		topPx := start.Hour()*60 + start.Minute()
		durPx := (end.Hour()*60 + end.Minute()) - topPx
		if durPx < 20 {
			durPx = 20
		}
		days[idx].Timed = append(days[idx].Timed, WeekOccurrence{Occ: o, TopPx: topPx, HeightPx: durPx})
	}

	bars, lanes := barsForWeek(weekStart, spanning)
	return WeekData{Days: days, Bars: bars, Lanes: lanes}
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
