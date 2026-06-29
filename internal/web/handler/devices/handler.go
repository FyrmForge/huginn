package devices

import (
	"net/http"

	"github.com/FyrmForge/hamr/pkg/middleware"
	"github.com/FyrmForge/hamr/pkg/respond"
	"github.com/labstack/echo/v4"

	"github.com/FyrmForge/huginn/internal/service"
	"github.com/FyrmForge/huginn/internal/web/components"
)

type handler struct {
	caldavService *service.CalDAVService
}

func NewHandler(caldavService *service.CalDAVService) *handler {
	return &handler{caldavService: caldavService}
}

// GET /settings/devices
func (h *handler) Page(c echo.Context) error {
	user := components.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized)
	}
	aps, err := h.caldavService.ListAppPasswords(c.Request().Context(), user.ID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError)
	}
	return respond.HTML(c, http.StatusOK, devicesPage(c, aps, ""))
}

// POST /settings/devices — create a new app password.
func (h *handler) Create(c echo.Context) error {
	user := components.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized)
	}

	name := c.FormValue("name")
	if name == "" {
		middleware.SetFlash(c, "Device name is required", middleware.FlashError)
		return respond.Redirect(c, "/settings")
	}

	plain, _, err := h.caldavService.CreateAppPassword(c.Request().Context(), user.ID, name)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	// Store token in a short-lived cookie so the settings page can display it once.
	c.SetCookie(&http.Cookie{Name: "_new_device_token", Value: plain, MaxAge: 60, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode})
	return respond.Redirect(c, "/settings")
}

// POST /settings/devices/:id/revoke
func (h *handler) Revoke(c echo.Context) error {
	user := components.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized)
	}

	if err := h.caldavService.RevokeAppPassword(c.Request().Context(), user.ID, c.Param("id")); err != nil {
		middleware.SetFlash(c, "Revoke failed: "+err.Error(), middleware.FlashError)
	} else {
		middleware.SetFlash(c, "Device revoked", middleware.FlashSuccess)
	}
	return respond.Redirect(c, "/settings")
}
