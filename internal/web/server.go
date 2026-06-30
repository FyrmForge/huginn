package web

import (
	"context"
	"net/http"

	"github.com/FyrmForge/hamr/pkg/auth"
	hamrmw "github.com/FyrmForge/hamr/pkg/middleware"
	"github.com/FyrmForge/hamr/pkg/server"
	"github.com/FyrmForge/hamr/pkg/storage"
	"github.com/FyrmForge/hamr/pkg/websocket"
	"github.com/labstack/echo/v4"

	"github.com/FyrmForge/huginn/internal/caldav"
	"github.com/FyrmForge/huginn/internal/middleware"
	"github.com/FyrmForge/huginn/internal/repo"
	"github.com/FyrmForge/huginn/internal/service"
	"github.com/FyrmForge/huginn/internal/web/components"
	"github.com/FyrmForge/huginn/internal/web/handler/admin"
	authlogin "github.com/FyrmForge/huginn/internal/web/handler/auth/login"
	authoidc "github.com/FyrmForge/huginn/internal/web/handler/auth/oidc"
	"github.com/FyrmForge/huginn/internal/web/handler/auth/register"
	"github.com/FyrmForge/huginn/internal/web/handler/calendar"
	"github.com/FyrmForge/huginn/internal/web/handler/calendars"
	"github.com/FyrmForge/huginn/internal/web/handler/connections"
	"github.com/FyrmForge/huginn/internal/web/handler/devices"
	"github.com/FyrmForge/huginn/internal/web/handler/events"
	"github.com/FyrmForge/huginn/internal/web/handler/importexport"
	"github.com/FyrmForge/huginn/internal/web/handler/routing"
	"github.com/FyrmForge/huginn/internal/web/handler/settings"
)

// Deps holds the dependencies for route registration.
type Deps struct {
	Store               repo.Store
	BaseURL             string
	StaticBaseURL       string
	DevMode             bool
	DevAuthEmail        string
	AllowRegistration   bool
	OIDCService         *service.OIDCService
	SessionManager      *auth.SessionManager
	AuthService         *service.AuthService
	CalendarService     *service.CalendarService
	EventService        *service.EventService
	SettingsService     *service.SettingsService
	ImportExportService *service.ImportExportService
	CalDAVService       *service.CalDAVService
	SyncService         *service.SyncService
	RoutingService      *service.RoutingService
	FileStorage         storage.FileStorage
	Hub                 *websocket.Hub
}

// RegisterRoutes registers all web route handlers on the server.
func RegisterRoutes(srv *server.Server, deps *Deps) {
	e := srv.Echo()

	// WebSocket endpoint.
	e.GET("/ws", deps.Hub.Handler())

	// Content Security Policy.
	csp := "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'"
	csp += "; connect-src 'self' ws: wss:"
	csp += "; img-src 'self' data:"

	// Site routes.
	site := e.Group("")
	site.Use(middleware.Logging())
	site.Use(hamrmw.ErrorPages(components.ErrorPage))
	site.Use(hamrmw.SecureWithConfig(hamrmw.SecureConfig{
		ContentSecurityPolicy: csp,
	}))
	site.Use(hamrmw.FlashWithConfig(hamrmw.FlashConfig{Secure: !deps.DevMode}))
	site.Use(hamrmw.CSRFWithConfig(hamrmw.CSRFConfig{Secure: !deps.DevMode}))

	bauth := hamrmw.NewBrowserAuth(deps.SessionManager,
		hamrmw.WithSubjectLoader(func(reqCtx context.Context, id string) (any, error) {
			return deps.Store.GetUserByID(reqCtx, id)
		}),
		hamrmw.WithLoginRedirect("/login"),
		hamrmw.WithHomeRedirect("/"),
	)
	site.Use(bauth.Load())

	// OIDC routes (only registered when OIDC is configured).
	if deps.OIDCService != nil {
		oidcHandler := authoidc.NewHandler(deps.OIDCService, deps.AuthService, deps.SessionManager)
		site.GET("/auth/oidc/login", oidcHandler.Login)
		site.GET("/auth/oidc/callback", oidcHandler.Callback)
	}

	// Auth routes.
	loginHandler := authlogin.NewHandler(deps.AuthService, deps.SessionManager, deps.DevAuthEmail)
	site.GET("/login", loginHandler.Page, bauth.RequireNotAuth())
	site.POST("/login", loginHandler.Submit, bauth.RequireNotAuth())
	site.POST("/login/validate/:field", loginHandler.FormRules.ValidationHandler("field"), bauth.RequireNotAuth())
	site.POST("/logout", loginHandler.Logout, bauth.RequireAuth())
	if deps.DevAuthEmail != "" {
		site.POST("/dev-login", loginHandler.DevLogin, bauth.RequireNotAuth())
	}

	if deps.AllowRegistration {
		registerHandler := register.NewHandler(deps.AuthService, deps.SessionManager)
		site.GET("/register", registerHandler.Page, bauth.RequireNotAuth())
		site.POST("/register", registerHandler.Submit, bauth.RequireNotAuth())
		site.POST("/register/validate/:field", registerHandler.FormRules.ValidationHandler("field"), bauth.RequireNotAuth())
	}

	// Calendar views.
	calHandler := calendar.NewHandler(deps.CalendarService, deps.EventService)
	site.GET("/", calHandler.Month, bauth.RequireAuth())
	site.GET("/week", calHandler.Week, bauth.RequireAuth())
	site.GET("/week/grid", calHandler.WeekGrid, bauth.RequireAuth())
	site.GET("/calendar/grid", calHandler.Grid, bauth.RequireAuth())

	// Event CRUD.
	evHandler := events.NewHandler(deps.CalendarService, deps.EventService, deps.SettingsService)
	site.GET("/events/new", evHandler.New, bauth.RequireAuth())
	site.GET("/events/close", func(c echo.Context) error { return c.NoContent(http.StatusOK) }, bauth.RequireAuth())
	site.POST("/events", evHandler.Create, bauth.RequireAuth())
	site.GET("/events/:id/edit", evHandler.Edit, bauth.RequireAuth())
	site.GET("/events/:id/confirm-delete", evHandler.ConfirmDelete, bauth.RequireAuth())
	site.POST("/events/:id", evHandler.Update, bauth.RequireAuth())
	site.DELETE("/events/:id", evHandler.Delete, bauth.RequireAuth())

	// Calendars management.
	calMgmtHandler := calendars.NewHandler(deps.CalendarService)
	site.GET("/calendars", calMgmtHandler.List, bauth.RequireAuth())
	site.GET("/calendars/new", calMgmtHandler.New, bauth.RequireAuth())
	site.POST("/calendars", calMgmtHandler.Create, bauth.RequireAuth())
	site.GET("/calendars/:id/edit", calMgmtHandler.Edit, bauth.RequireAuth())
	site.GET("/calendars/:id/confirm-delete", calMgmtHandler.ConfirmDelete, bauth.RequireAuth())
	site.POST("/calendars/:id", calMgmtHandler.Update, bauth.RequireAuth())
	site.POST("/calendars/:id/delete", calMgmtHandler.Delete, bauth.RequireAuth())
	site.POST("/calendars/:id/share", calMgmtHandler.Share, bauth.RequireAuth())
	site.POST("/calendars/:id/members/:uid/remove", calMgmtHandler.Unshare, bauth.RequireAuth())
	site.POST("/calendars/:id/leave", calMgmtHandler.Leave, bauth.RequireAuth())

	// Settings (unified page: general, devices, connections, routing rules, account).
	settingsHandler := settings.NewHandler(deps.SettingsService, deps.CalDAVService, deps.SyncService, deps.RoutingService, deps.CalendarService)
	site.GET("/settings", settingsHandler.Page, bauth.RequireAuth())
	site.POST("/settings", settingsHandler.Save, bauth.RequireAuth())

	// Admin dashboard (role = "admin" required).
	adminHandler := admin.NewHandler(deps.Store)
	site.GET("/admin", adminHandler.Page, bauth.RequireAuth(), middleware.RequireAdmin())
	site.POST("/admin/users/:id/role", adminHandler.SetRole, bauth.RequireAuth(), middleware.RequireAdmin())

	// Import / Export.
	ixHandler := importexport.NewHandler(deps.CalendarService, deps.ImportExportService)
	site.GET("/import", ixHandler.ImportPage, bauth.RequireAuth())
	site.POST("/import", ixHandler.Import, bauth.RequireAuth())
	site.GET("/export", ixHandler.Export, bauth.RequireAuth())

	// External calendar connections (Google, Outlook OAuth).
	connHandler := connections.NewHandler(deps.SyncService, deps.BaseURL)
	site.GET("/connections/:provider/connect", connHandler.Connect, bauth.RequireAuth())
	site.GET("/connections/callback", connHandler.Callback, bauth.RequireAuth())
	site.POST("/connections/:id/sync", connHandler.Sync, bauth.RequireAuth())
	site.POST("/connections/:id/disconnect", connHandler.Disconnect, bauth.RequireAuth())

	// Routing rules.
	routingHandler := routing.NewHandler(deps.RoutingService, deps.CalendarService)
	site.GET("/routing/new", routingHandler.New, bauth.RequireAuth())
	site.POST("/routing", routingHandler.Create, bauth.RequireAuth())
	site.GET("/routing/:id/edit", routingHandler.Edit, bauth.RequireAuth())
	site.POST("/routing/:id", routingHandler.Update, bauth.RequireAuth())
	site.POST("/routing/:id/delete", routingHandler.Delete, bauth.RequireAuth())

	// Devices / App passwords.
	devHandler := devices.NewHandler(deps.CalDAVService)
	site.POST("/settings/devices", devHandler.Create, bauth.RequireAuth())
	site.POST("/settings/devices/:id/revoke", devHandler.Revoke, bauth.RequireAuth())

	// CalDAV protocol (Basic Auth, no session).
	caldav.NewHandler(deps.CalDAVService, deps.CalendarService, deps.EventService).RegisterRoutes(e)
}
