package importexport

import (
	"fmt"
	"net/http"

	"github.com/FyrmForge/hamr/pkg/middleware"
	"github.com/FyrmForge/hamr/pkg/respond"
	"github.com/labstack/echo/v4"

	"github.com/FyrmForge/huginn/internal/service"
	"github.com/FyrmForge/huginn/internal/web/components"
)

type handler struct {
	calendarService     *service.CalendarService
	importExportService *service.ImportExportService
}

func NewHandler(calendarService *service.CalendarService, importExportService *service.ImportExportService) *handler {
	return &handler{
		calendarService:     calendarService,
		importExportService: importExportService,
	}
}

// GET /import
func (h *handler) ImportPage(c echo.Context) error {
	user := components.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized)
	}
	cals, err := h.calendarService.ListForUser(c.Request().Context(), user.ID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError)
	}
	return respond.HTML(c, http.StatusOK, importPage(c, cals, ""))
}

// POST /import
func (h *handler) Import(c echo.Context) error {
	user := components.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized)
	}

	calendarID := c.FormValue("calendar_id")
	if calendarID == "" {
		cals, _ := h.calendarService.ListForUser(c.Request().Context(), user.ID)
		return respond.HTML(c, http.StatusUnprocessableEntity, importPage(c, cals, "Select a calendar"))
	}

	file, err := c.FormFile("ics_file")
	if err != nil {
		cals, _ := h.calendarService.ListForUser(c.Request().Context(), user.ID)
		return respond.HTML(c, http.StatusUnprocessableEntity, importPage(c, cals, "Select an ICS file"))
	}

	src, err := file.Open()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError)
	}
	defer func() { _ = src.Close() }()

	count, err := h.importExportService.ImportICS(c.Request().Context(), user.ID, calendarID, src)
	if err != nil {
		cals, _ := h.calendarService.ListForUser(c.Request().Context(), user.ID)
		return respond.HTML(c, http.StatusUnprocessableEntity, importPage(c, cals, "Import failed: "+err.Error()))
	}

	middleware.SetFlash(c, fmt.Sprintf("Imported %d events", count), middleware.FlashSuccess)
	return respond.Redirect(c, "/")
}

// GET /export?calendar_id=
func (h *handler) Export(c echo.Context) error {
	user := components.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized)
	}

	calendarID := c.QueryParam("calendar_id")
	if calendarID == "" {
		cals, err := h.calendarService.ListForUser(c.Request().Context(), user.ID)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError)
		}
		return respond.HTML(c, http.StatusOK, exportPage(c, cals))
	}

	c.Response().Header().Set("Content-Type", "text/calendar; charset=utf-8")
	c.Response().Header().Set("Content-Disposition", "attachment; filename=\"huginn-calendar.ics\"")
	c.Response().WriteHeader(http.StatusOK)

	return h.importExportService.ExportICS(c.Request().Context(), calendarID, c.Response().Writer)
}
