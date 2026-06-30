package caldav

import "github.com/labstack/echo/v4"

// RegisterRoutes mounts all CalDAV routes onto the Echo instance.
func (h *Handler) RegisterRoutes(e *echo.Echo) {
	e.Any("/.well-known/caldav", h.WellKnown)
	dav := e.Group("/dav")
	dav.OPTIONS("/", h.Options)
	dav.OPTIONS("/*", h.Options)
	dav.Match([]string{"PROPFIND"}, "/", h.Principal)
	dav.Match([]string{"PROPFIND"}, "/calendars/:userID/", h.CalendarHome)
	dav.Match([]string{"PROPFIND"}, "/calendars/:userID/:calID/", h.CalendarCollection)
	dav.Match([]string{"PROPFIND"}, "/calendars/:userID/:calID", h.CalendarCollection)
	dav.Match([]string{"REPORT"}, "/calendars/:userID/:calID/", h.Report)
	dav.Match([]string{"REPORT"}, "/calendars/:userID/:calID", h.Report)
	dav.Match([]string{"PROPFIND"}, "/calendars/:userID/:calID/:eventUID", h.PropfindEvent)
	dav.GET("/calendars/:userID/:calID/:eventUID", h.GetEvent)
	dav.PUT("/calendars/:userID/:calID/:eventUID", h.PutEvent)
	dav.DELETE("/calendars/:userID/:calID/:eventUID", h.DeleteEvent)
	dav.Match([]string{"MKCALENDAR"}, "/calendars/:userID/:calID/", h.MkCalendar)
	dav.Match([]string{"MKCALENDAR"}, "/calendars/:userID/:calID", h.MkCalendar)
}
