package calendars

import (
	"net/http"

	"github.com/FyrmForge/hamr/pkg/middleware"
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

	CreateFormRules validate.Form
	EditFormRules   validate.Form
}

func NewHandler(calendarService *service.CalendarService) *handler {
	rules := validate.NewForm(
		validate.WithOOBRenderer(form.OOBValidator),
		validate.Field("name", validate.Required),
		validate.Field("color", validate.Required),
		validate.Field("timezone", validate.Required),
	)
	return &handler{
		calendarService: calendarService,
		CreateFormRules: rules,
		EditFormRules:   rules,
	}
}

// GET /calendars
func (h *handler) List(c echo.Context) error {
	user := components.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized)
	}
	cals, err := h.calendarService.ListForUser(c.Request().Context(), user.ID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError)
	}
	return respond.HTML(c, http.StatusOK, calendarsPage(c, cals, user.ID))
}

// GET /calendars/new
func (h *handler) New(c echo.Context) error {
	return respond.HTML(c, http.StatusOK, calendarFormPage(c, CalendarForm{Color: "#4f8ef7", Timezone: "UTC"}, nil, nil, ""))
}

// POST /calendars
func (h *handler) Create(c echo.Context) error {
	user := components.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized)
	}

	var f CalendarForm
	if err := c.Bind(&f); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest)
	}

	if errs := h.CreateFormRules.Validate(c); errs != nil {
		return respond.HTML(c, http.StatusUnprocessableEntity, calendarFormPage(c, f, errs, nil, ""))
	}

	_, err := h.calendarService.Create(c.Request().Context(), user.ID, f.Name, f.Description, f.Color, f.Timezone)
	if err != nil {
		return respond.HTML(c, http.StatusUnprocessableEntity, calendarFormPage(c, f, map[string]string{"general": "Failed to create calendar"}, nil, ""))
	}

	middleware.SetFlash(c, "Calendar created", middleware.FlashSuccess)
	return respond.Redirect(c, "/calendars")
}

// GET /calendars/:id/edit
func (h *handler) Edit(c echo.Context) error {
	user := components.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized)
	}

	cal, err := h.calendarService.GetByID(c.Request().Context(), user.ID, c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound)
	}

	isOwner := cal.OwnerID == user.ID
	var members []*repo.CalendarMemberInfo
	if isOwner {
		members, _ = h.calendarService.ListMembersWithUsers(c.Request().Context(), cal.ID, user.ID)
		if members == nil {
			members = []*repo.CalendarMemberInfo{} // non-nil sentinel so template shows sharing section
		}
	}

	f := CalendarForm{
		Name:        cal.Name,
		Description: cal.Description,
		Color:       cal.Color,
		Timezone:    cal.Timezone,
	}
	return respond.HTML(c, http.StatusOK, calendarFormPage(c, f, nil, members, cal.ID))
}

// POST /calendars/:id
func (h *handler) Update(c echo.Context) error {
	user := components.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized)
	}

	var f CalendarForm
	if err := c.Bind(&f); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest)
	}

	if errs := h.EditFormRules.Validate(c); errs != nil {
		return respond.HTML(c, http.StatusUnprocessableEntity, calendarFormPage(c, f, errs, nil, c.Param("id")))
	}

	err := h.calendarService.Update(c.Request().Context(), user.ID, c.Param("id"), f.Name, f.Description, f.Color, f.Timezone)
	if err != nil {
		return respond.HTML(c, http.StatusUnprocessableEntity, calendarFormPage(c, f, map[string]string{"general": err.Error()}, nil, c.Param("id")))
	}

	middleware.SetFlash(c, "Calendar updated", middleware.FlashSuccess)
	return respond.Redirect(c, "/calendars")
}

// GET /calendars/:id/confirm-delete
func (h *handler) ConfirmDelete(c echo.Context) error {
	user := components.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized)
	}
	cal, err := h.calendarService.GetByID(c.Request().Context(), user.ID, c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound)
	}
	return respond.HTML(c, http.StatusOK, calendarConfirmDeletePage(c, cal))
}

// POST /calendars/:id/delete
func (h *handler) Delete(c echo.Context) error {
	user := components.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized)
	}

	if err := h.calendarService.Delete(c.Request().Context(), user.ID, c.Param("id")); err != nil {
		middleware.SetFlash(c, err.Error(), middleware.FlashError)
		return respond.Redirect(c, "/calendars")
	}

	middleware.SetFlash(c, "Calendar deleted", middleware.FlashSuccess)
	return respond.Redirect(c, "/calendars")
}

// POST /calendars/:id/share
func (h *handler) Share(c echo.Context) error {
	user := components.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized)
	}
	email := c.FormValue("email")
	role := c.FormValue("role")
	err := h.calendarService.ShareWith(c.Request().Context(), c.Param("id"), user.ID, email, role)
	if err != nil {
		middleware.SetFlash(c, err.Error(), middleware.FlashError)
	} else {
		middleware.SetFlash(c, "Shared with "+email, middleware.FlashSuccess)
	}
	return respond.Redirect(c, "/calendars/"+c.Param("id")+"/edit")
}

// POST /calendars/:id/members/:uid/remove
func (h *handler) Unshare(c echo.Context) error {
	user := components.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized)
	}
	err := h.calendarService.Unshare(c.Request().Context(), c.Param("id"), user.ID, c.Param("uid"))
	if err != nil {
		middleware.SetFlash(c, err.Error(), middleware.FlashError)
	}
	return respond.Redirect(c, "/calendars/"+c.Param("id")+"/edit")
}

// POST /calendars/:id/leave
func (h *handler) Leave(c echo.Context) error {
	user := components.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized)
	}
	err := h.calendarService.Leave(c.Request().Context(), c.Param("id"), user.ID)
	if err != nil {
		middleware.SetFlash(c, err.Error(), middleware.FlashError)
	} else {
		middleware.SetFlash(c, "Left calendar", middleware.FlashSuccess)
	}
	return respond.Redirect(c, "/calendars")
}

type CalendarForm struct {
	Name        string `form:"name"`
	Description string `form:"description"`
	Color       string `form:"color"`
	Timezone    string `form:"timezone"`
}
