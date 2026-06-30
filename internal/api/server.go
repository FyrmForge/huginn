package api

import (
	"github.com/FyrmForge/hamr/pkg/server"

	"github.com/FyrmForge/huginn/internal/api/handler/health"
	"github.com/FyrmForge/huginn/internal/middleware"
	"github.com/FyrmForge/huginn/internal/repo"
)

// Deps holds the dependencies for API route registration.
type Deps struct {
	Store repo.Store
}

// RegisterRoutes registers all API route handlers on the server.
func RegisterRoutes(srv *server.Server, deps *Deps) {
	api := srv.Echo().Group("/api")
	api.Use(middleware.Logging())

	healthHandler := health.NewHandler(deps.Store)
	api.GET("/health", healthHandler.Health)
}
