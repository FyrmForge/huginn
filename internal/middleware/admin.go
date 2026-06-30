package middleware

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/FyrmForge/huginn/internal/web/components"
)

// RequireAdmin rejects requests from non-admin users with 403.
func RequireAdmin() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			u := components.GetUser(c)
			if u == nil || u.Role != "admin" {
				return echo.NewHTTPError(http.StatusForbidden, "admin only")
			}
			return next(c)
		}
	}
}
