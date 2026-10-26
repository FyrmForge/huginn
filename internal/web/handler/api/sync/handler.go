// Package sync exposes PUT /api/sync/calendars/:externalID — the endpoint a
// device agent (cmd/huginn-sync-mac) pushes full calendar snapshots to.
// Authenticated with an app password via Basic Auth, same as CalDAV.
package sync

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/FyrmForge/huginn/internal/service"
)

type Handler struct {
	caldav *service.CalDAVService
	push   *service.PushSyncService
}

func NewHandler(caldav *service.CalDAVService, push *service.PushSyncService) *Handler {
	return &Handler{caldav: caldav, push: push}
}

func (h *Handler) RegisterRoutes(e *echo.Echo) {
	e.PUT("/api/sync/calendars/:externalID", h.PutSnapshot)
}

func (h *Handler) PutSnapshot(c echo.Context) error {
	_, pass, ok := c.Request().BasicAuth()
	if !ok {
		c.Response().Header().Set("WWW-Authenticate", `Basic realm="Huginn Sync"`)
		return c.NoContent(http.StatusUnauthorized)
	}
	user, err := h.caldav.Authenticate(c.Request().Context(), pass)
	if err != nil {
		return c.NoContent(http.StatusUnauthorized)
	}

	var snap service.Snapshot
	if err := c.Bind(&snap); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid json body"})
	}
	res, err := h.push.ApplySnapshot(c.Request().Context(), user.ID, c.Param("externalID"), snap)
	if err != nil {
		c.Logger().Errorf("push sync: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "sync failed"})
	}
	return c.JSON(http.StatusOK, res)
}
