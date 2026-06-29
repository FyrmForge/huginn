package admin

import (
	"net/http"
	"time"

	"github.com/FyrmForge/hamr/pkg/middleware"
	"github.com/FyrmForge/hamr/pkg/respond"
	"github.com/labstack/echo/v4"

	"github.com/FyrmForge/huginn/internal/repo"
	"github.com/FyrmForge/huginn/internal/web/components"
)

func roleClass(role string) string {
	if role == "admin" {
		return "text-huginn-accent"
	}
	return "text-huginn-mute"
}

func fmtTime(t time.Time) string {
	return t.Format("2006-01-02")
}

type handler struct {
	store repo.Store
}

func NewHandler(store repo.Store) *handler {
	return &handler{store: store}
}

type pageData struct {
	Stats *repo.Stats
	Users []*repo.User
}

// GET /admin
func (h *handler) Page(c echo.Context) error {
	ctx := c.Request().Context()
	stats, err := h.store.AdminStats(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	users, err := h.store.ListAllUsers(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return respond.HTML(c, http.StatusOK, adminPage(c, pageData{Stats: stats, Users: users}))
}

// POST /admin/users/:id/role
func (h *handler) SetRole(c echo.Context) error {
	ctx := c.Request().Context()
	me := components.GetUser(c)
	id := c.Param("id")
	if me != nil && me.ID == id {
		middleware.SetFlash(c, "Cannot change your own role", middleware.FlashError)
		return respond.Redirect(c, "/admin")
	}
	user, err := h.store.GetUserByID(ctx, id)
	if err != nil || user == nil {
		return echo.NewHTTPError(http.StatusNotFound)
	}
	role := c.FormValue("role")
	if role != "admin" && role != "user" {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid role")
	}
	user.Role = role
	user.UpdatedAt = time.Now()
	if err := h.store.UpdateUser(ctx, user); err != nil {
		middleware.SetFlash(c, "Update failed: "+err.Error(), middleware.FlashError)
	}
	return respond.Redirect(c, "/admin")
}
