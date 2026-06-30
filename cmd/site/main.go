package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"time"

	_ "github.com/joho/godotenv/autoload"

	"github.com/FyrmForge/hamr/pkg/auth"
	"github.com/FyrmForge/hamr/pkg/config"
	"github.com/FyrmForge/hamr/pkg/db"
	"github.com/FyrmForge/hamr/pkg/logging"
	"github.com/FyrmForge/hamr/pkg/middleware"
	"github.com/FyrmForge/hamr/pkg/server"
	"github.com/FyrmForge/hamr/pkg/storage"
	"github.com/FyrmForge/hamr/pkg/websocket"
	"github.com/FyrmForge/huginn/internal/api"
	"github.com/FyrmForge/huginn/internal/connector"
	appdb "github.com/FyrmForge/huginn/internal/db"
	"github.com/FyrmForge/huginn/internal/repo/postgres"
	"github.com/FyrmForge/huginn/internal/service"
	"github.com/FyrmForge/huginn/internal/web"
	"github.com/FyrmForge/huginn/internal/web/components"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "dev"

var (
	envPort                = config.GetEnvOrDefaultInt("PORT", 8080)
	envGoogleClientID      = config.GetEnvOrDefault("GOOGLE_CLIENT_ID", "")
	envGoogleClientSecret  = config.GetEnvOrDefault("GOOGLE_CLIENT_SECRET", "")
	envOutlookClientID     = config.GetEnvOrDefault("OUTLOOK_CLIENT_ID", "")
	envOutlookClientSecret = config.GetEnvOrDefault("OUTLOOK_CLIENT_SECRET", "")
	// DEV_MODE defaults to false (fail closed in prod). Local dev sets
	// DEV_MODE=true via .env so the scaffolded `.env` ships with it set
	// explicitly. This makes the STRIPE_MOCK production guard actually
	// guard — a leftover STRIPE_MOCK=true in a prod deploy without
	// DEV_MODE explicitly set would otherwise slip through.
	envDevMode        = config.GetEnvOrDefaultBool("DEV_MODE", false)
	envBaseURL        = config.GetEnvOrDefault("BASE_URL", "")
	envDatabaseURL    = config.GetEnvOrDefault("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/huginn?sslmode=disable")
	envStaticBaseURL  = config.GetEnvOrDefault("STATIC_BASE_URL", "/static")
	envStoragePath    = config.GetEnvOrDefault("STORAGE_PATH", "./uploads")
	envTrustedProxies = config.GetEnvCSV("TRUSTED_PROXIES")
	// DEV_AUTH_EMAIL: when set, replaces the login form with a one-click dev bypass.
	// The user is created on first login. Never set this in production.
	envDevAuthEmail = config.GetEnvOrDefault("DEV_AUTH_EMAIL", "")

	// OIDC — all optional. Set OIDC_ISSUER to enable SSO login.
	envOIDCIssuer       = config.GetEnvOrDefault("OIDC_ISSUER", "")
	envOIDCClientID     = config.GetEnvOrDefault("OIDC_CLIENT_ID", "")
	envOIDCClientSecret = config.GetEnvOrDefault("OIDC_CLIENT_SECRET", "")
	envOIDCAdminClaim   = config.GetEnvOrDefault("OIDC_ADMIN_CLAIM", "")
	envOIDCAdminValue   = config.GetEnvOrDefault("OIDC_ADMIN_VALUE", "")
	// ALLOW_REGISTRATION: set to "true" to enable the /register page.
	// Disabled by default — use OIDC for user provisioning.
	envAllowRegistration = config.GetEnvOrDefault("ALLOW_REGISTRATION", "") == "true"
)

func main() {
	generateFlag := flag.Bool("generate", false, "generate static pages and exit")
	flag.Parse()

	log := logging.New(!envDevMode)
	slog.SetDefault(log)

	components.StaticBaseURL = envStaticBaseURL

	// Base URL (cookie domain & CORS).
	baseOrigin, baseDomain, err := config.ParseBaseURL(envBaseURL)
	if err != nil {
		log.Error("invalid BASE_URL", "error", err)
		os.Exit(1)
	}
	components.BaseURL = baseOrigin

	// Server.
	srv, err := server.New(
		server.WithPort(envPort),
		server.WithDevMode(envDevMode),
		server.WithStaticDir("static"),
		server.WithStaticDistDir("dist"),
		server.WithGeneratedDir("generated"),
		// TRUSTED_PROXIES: comma-separated CIDRs of upstream proxies/load
		// balancers allowed to set X-Forwarded-For (drives client-IP detection
		// and the rate-limit key). Empty/unset ignores X-Forwarded-For so a
		// direct client can't spoof its IP; set it to your LB ranges behind one.
		server.WithTrustedProxies(envTrustedProxies...),
	)
	if err != nil {
		log.Error("failed to create server", "error", err)
		os.Exit(1)
	}

	if baseOrigin != "" {
		srv.Echo().Use(middleware.CORSWithConfig(middleware.CORSConfig{
			AllowOrigins:     []string{baseOrigin},
			AllowCredentials: true,
		}))
	}

	if *generateFlag {
		if err := srv.GenerateStatic("generated"); err != nil {
			log.Error("generate static pages failed", "error", err)
			os.Exit(1)
		}
		return
	}

	// Database.
	connectCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	database, err := db.ConnectContext(connectCtx, envDatabaseURL)
	cancel()
	if err != nil {
		log.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}

	// Run migrations at startup.
	if err := db.Migrate(database, appdb.MigrateConfig()); err != nil {
		log.Error("migration failed", "error", err)
		os.Exit(1)
	}
	log.Info("migrations completed")
	store := postgres.NewStore(database)

	// Sessions.
	sessionManager := auth.NewSessionManager(store,
		auth.WithCookieSecure(!envDevMode),
		auth.WithCookieDomain(baseDomain),
	)

	// OIDC service (nil when OIDC_ISSUER is not set).
	oidcService, err := service.NewOIDCService(context.Background(), service.OIDCConfig{
		Issuer:       envOIDCIssuer,
		ClientID:     envOIDCClientID,
		ClientSecret: envOIDCClientSecret,
		RedirectURL:  envBaseURL + "/auth/oidc/callback",
		AdminClaim:   envOIDCAdminClaim,
		AdminValue:   envOIDCAdminValue,
	})
	if err != nil {
		log.Error("oidc init failed", "error", err)
		os.Exit(1)
	}

	// Auth service.
	authService := service.NewAuthService(store)

	// Calendar & event services.
	calendarService := service.NewCalendarService(store)
	eventService := service.NewEventService(store)
	settingsService := service.NewSettingsService(store)
	importExportService := service.NewImportExportService(store)
	caldavService := service.NewCalDAVService(store)
	routingService := service.NewRoutingService(store)

	// Sync service with optional provider connectors.
	syncService := service.NewSyncService(store, calendarService, eventService, routingService)
	var googleProvider *connector.GoogleProvider
	var outlookProvider *connector.OutlookProvider
	if envGoogleClientID != "" {
		googleProvider = connector.NewGoogleProvider(envGoogleClientID, envGoogleClientSecret)
	}
	if envOutlookClientID != "" {
		outlookProvider = connector.NewOutlookProvider(envOutlookClientID, envOutlookClientSecret)
	}
	syncService.Configure(googleProvider, outlookProvider)

	// File storage (local).
	fileStorage, err := storage.NewLocalStorage(envStoragePath)
	if err != nil {
		log.Error("failed to init storage", "error", err)
		os.Exit(1)
	}

	// WebSocket hub.
	hub := websocket.NewHub(websocket.WithLogger(log))

	api.RegisterRoutes(srv, &api.Deps{
		Store: store,
	})

	web.RegisterRoutes(srv, &web.Deps{
		Store:               store,
		BaseURL:             baseOrigin,
		StaticBaseURL:       envStaticBaseURL,
		DevMode:             envDevMode,
		DevAuthEmail:        envDevAuthEmail,
		AllowRegistration:   envAllowRegistration,
		OIDCService:         oidcService,
		SessionManager:      sessionManager,
		AuthService:         authService,
		CalendarService:     calendarService,
		EventService:        eventService,
		SettingsService:     settingsService,
		ImportExportService: importExportService,
		CalDAVService:       caldavService,
		SyncService:         syncService,
		RoutingService:      routingService,
		FileStorage:         fileStorage,
		Hub:                 hub,
	})

	log.Info("starting server", "version", version, "port", envPort, "devMode", envDevMode)
	if err := srv.Start(); err != nil {
		log.Error("server stopped", "error", err)
		hub.Close()
		os.Exit(1)
	}
}
