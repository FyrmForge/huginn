package routing

import (
	"net/http"
	"strconv"

	"github.com/FyrmForge/hamr/pkg/middleware"
	"github.com/FyrmForge/hamr/pkg/respond"
	"github.com/labstack/echo/v4"

	"github.com/FyrmForge/huginn/internal/service"
	"github.com/FyrmForge/huginn/internal/web/components"
)

type handler struct {
	routing  *service.RoutingService
	calendar *service.CalendarService
}

func NewHandler(routing *service.RoutingService, calendar *service.CalendarService) *handler {
	return &handler{routing: routing, calendar: calendar}
}

// GET /routing
func (h *handler) Page(c echo.Context) error {
	user := components.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized)
	}
	rules, _ := h.routing.ListForUser(c.Request().Context(), user.ID)
	cals, _ := h.calendar.ListForUser(c.Request().Context(), user.ID)
	return respond.HTML(c, http.StatusOK, routingPage(c, rules, cals))
}

// GET /routing/new
func (h *handler) New(c echo.Context) error {
	user := components.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized)
	}
	cals, _ := h.calendar.ListForUser(c.Request().Context(), user.ID)
	return respond.HTML(c, http.StatusOK, ruleFormPage(c, nil, cals))
}

// POST /routing
func (h *handler) Create(c echo.Context) error {
	user := components.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized)
	}
	in := formToInput(c)
	if _, err := h.routing.Create(c.Request().Context(), user.ID, in); err != nil {
		middleware.SetFlash(c, "Create failed: "+err.Error(), middleware.FlashError)
	} else {
		middleware.SetFlash(c, "Rule created", middleware.FlashSuccess)
	}
	return respond.Redirect(c, "/settings")
}

// GET /routing/:id/edit
func (h *handler) Edit(c echo.Context) error {
	user := components.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized)
	}
	rules, _ := h.routing.ListForUser(c.Request().Context(), user.ID)
	var found *ruleVM
	for _, r := range rules {
		if r.ID == c.Param("id") {
			found = &ruleVM{RoutingRule: r}
			break
		}
	}
	if found == nil {
		return echo.NewHTTPError(http.StatusNotFound)
	}
	cals, _ := h.calendar.ListForUser(c.Request().Context(), user.ID)
	return respond.HTML(c, http.StatusOK, ruleFormPage(c, found, cals))
}

// POST /routing/:id
func (h *handler) Update(c echo.Context) error {
	user := components.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized)
	}
	in := formToInput(c)
	if err := h.routing.Update(c.Request().Context(), user.ID, c.Param("id"), in); err != nil {
		middleware.SetFlash(c, "Update failed: "+err.Error(), middleware.FlashError)
	} else {
		middleware.SetFlash(c, "Rule updated", middleware.FlashSuccess)
	}
	return respond.Redirect(c, "/settings")
}

// POST /routing/:id/delete
func (h *handler) Delete(c echo.Context) error {
	user := components.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized)
	}
	if err := h.routing.Delete(c.Request().Context(), user.ID, c.Param("id")); err != nil {
		middleware.SetFlash(c, "Delete failed: "+err.Error(), middleware.FlashError)
	} else {
		middleware.SetFlash(c, "Rule deleted", middleware.FlashSuccess)
	}
	return respond.Redirect(c, "/settings")
}

func formToInput(c echo.Context) service.RuleInput {
	priority, _ := strconv.Atoi(c.FormValue("priority"))
	if priority == 0 {
		priority = 100
	}
	return service.RuleInput{
		Name:             c.FormValue("name"),
		RuleType:         c.FormValue("rule_type"),
		MatchValue:       c.FormValue("match_value"),
		CaseSensitive:    c.FormValue("case_sensitive") == "on",
		TargetCalendarID: c.FormValue("target_calendar_id"),
		Priority:         priority,
	}
}
