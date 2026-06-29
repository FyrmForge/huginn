package components

import (
	"github.com/FyrmForge/hamr/pkg/middleware"
	"github.com/labstack/echo/v4"

	"github.com/FyrmForge/huginn/internal/repo"
)

// BaseURL is the application's public origin (e.g. "https://example.com").
// Empty in dev; set from main via the BASE_URL env var.
var BaseURL string

// StaticBaseURL is the base URL prefix for static assets.
// Set from main before any templates render.
var StaticBaseURL = "/static"

// FlashAlertClass returns CSS classes for a flash type.
func FlashAlertClass(t middleware.FlashType) string {
	switch t {
	case middleware.FlashSuccess:
		return "alert-success"
	case middleware.FlashError:
		return "alert-error"
	case middleware.FlashWarning:
		return "alert-warning"
	default:
		return "alert-info"
	}
}

// StaticURL returns the full URL for a static asset, using the fingerprinted
// path from the manifest when available (production), or the plain path (dev).
func StaticURL(path string) string {
	if StaticManifest != nil {
		if fp, ok := StaticManifest[path]; ok {
			return StaticBaseURL + "/" + fp
		}
	}
	return StaticBaseURL + "/" + path
}

// AbsoluteURL returns an absolute URL for the given path by prepending BaseURL.
// When BaseURL is empty (local dev), the path is returned as-is.
func AbsoluteURL(path string) string {
	if BaseURL == "" {
		return path
	}
	return BaseURL + path
}

// GetUser returns the authenticated user from the Echo context, or nil
// if no user is loaded (e.g. guest pages).
func GetUser(c echo.Context) *repo.User {
	u, _ := middleware.GetSubject(c).(*repo.User)
	return u
}
