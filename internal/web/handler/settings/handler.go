package settings

import (
	"net/http"

	"github.com/FyrmForge/hamr/pkg/middleware"
	"github.com/FyrmForge/hamr/pkg/respond"
	"github.com/labstack/echo/v4"

	"github.com/FyrmForge/huginn/internal/repo"
	"github.com/FyrmForge/huginn/internal/service"
	"github.com/FyrmForge/huginn/internal/web/components"
)

type handler struct {
	settings *service.SettingsService
	caldav   *service.CalDAVService
	sync     *service.SyncService
	routing  *service.RoutingService
	calendar *service.CalendarService
}

func NewHandler(
	settings *service.SettingsService,
	caldav *service.CalDAVService,
	sync *service.SyncService,
	routing *service.RoutingService,
	calendar *service.CalendarService,
) *handler {
	return &handler{
		settings: settings,
		caldav:   caldav,
		sync:     sync,
		routing:  routing,
		calendar: calendar,
	}
}

// GET /settings
func (h *handler) Page(c echo.Context) error {
	user := components.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized)
	}
	return respond.HTML(c, http.StatusOK, settingsPage(c, h.loadPage(c, user)))
}

// POST /settings
func (h *handler) Save(c echo.Context) error {
	user := components.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized)
	}
	var f SettingsForm
	if err := c.Bind(&f); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest)
	}
	if err := h.settings.Save(c.Request().Context(), f.toSettings(user.ID)); err != nil {
		middleware.SetFlash(c, "Save failed: "+err.Error(), middleware.FlashError)
	} else {
		middleware.SetFlash(c, "Settings saved", middleware.FlashSuccess)
	}
	return respond.Redirect(c, "/settings")
}

func (h *handler) loadPage(c echo.Context, user *repo.User) PageData {
	ctx := c.Request().Context()

	us, _ := h.settings.GetOrDefault(ctx, user.ID)
	devices, _ := h.caldav.ListAppPasswords(ctx, user.ID)
	conns, _ := h.sync.ListConnections(ctx, user.ID)
	rules, _ := h.routing.ListForUser(ctx, user.ID)
	cals, _ := h.calendar.ListForUser(ctx, user.ID)

	// Consume one-shot new-device-token cookie set by the devices handler.
	newToken := ""
	if cookie, err := c.Cookie("_new_device_token"); err == nil {
		newToken = cookie.Value
		c.SetCookie(&http.Cookie{Name: "_new_device_token", Value: "", MaxAge: -1, Path: "/"})
	}

	return PageData{
		Form:      formFromSettings(us),
		Devices:   devices,
		NewToken:  newToken,
		Conns:     conns,
		Rules:     rules,
		Calendars: cals,
		User:      user,
	}
}

// PageData bundles everything the unified settings page needs.
type PageData struct {
	Form      SettingsForm
	Devices   []*repo.AppPassword
	NewToken  string
	Conns     []*repo.SyncConnection
	Rules     []*repo.RoutingRule
	Calendars []*repo.Calendar
	User      *repo.User
}

type SettingsForm struct {
	Timezone                string `form:"timezone"`
	DateFormat              string `form:"date_format"`
	TimeFormat              string `form:"time_format"`
	FirstDayOfWeek          int    `form:"first_day_of_week"`
	DefaultView             string `form:"default_view"`
	DefaultEventDurationMin int    `form:"default_event_duration_mins"`
}

func formFromSettings(us *repo.UserSettings) SettingsForm {
	if us == nil {
		return SettingsForm{}
	}
	return SettingsForm{
		Timezone:                us.Timezone,
		DateFormat:              us.DateFormat,
		TimeFormat:              us.TimeFormat,
		FirstDayOfWeek:          us.FirstDayOfWeek,
		DefaultView:             us.DefaultView,
		DefaultEventDurationMin: us.DefaultEventDurationMin,
	}
}

func (f SettingsForm) toSettings(userID string) *repo.UserSettings {
	return &repo.UserSettings{
		UserID:                  userID,
		Timezone:                f.Timezone,
		Locale:                  "en",
		DateFormat:              f.DateFormat,
		TimeFormat:              f.TimeFormat,
		FirstDayOfWeek:          f.FirstDayOfWeek,
		DefaultView:             f.DefaultView,
		DefaultEventDurationMin: f.DefaultEventDurationMin,
	}
}
