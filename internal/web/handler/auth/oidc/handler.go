package oidc

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"

	hamrauth "github.com/FyrmForge/hamr/pkg/auth"
	"github.com/FyrmForge/hamr/pkg/logging"
	"github.com/FyrmForge/hamr/pkg/respond"
	"github.com/labstack/echo/v4"

	"github.com/FyrmForge/huginn/internal/auth"
	"github.com/FyrmForge/huginn/internal/service"
)

const stateCookie = "_oidc_state"

type handler struct {
	oidc           *service.OIDCService
	authService    *service.AuthService
	sessionManager *hamrauth.SessionManager
}

func NewHandler(oidc *service.OIDCService, authService *service.AuthService, sm *hamrauth.SessionManager) *handler {
	return &handler{oidc: oidc, authService: authService, sessionManager: sm}
}

// GET /auth/oidc/login — redirect to provider.
func (h *handler) Login(c echo.Context) error {
	state, err := randomState()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "state error")
	}
	c.SetCookie(&http.Cookie{
		Name:     stateCookie,
		Value:    state,
		Path:     "/",
		MaxAge:   300,
		HttpOnly: true,
		Secure:   h.sessionManager.CookieSecure(),
		SameSite: http.SameSiteLaxMode,
	})
	return respond.Redirect(c, h.oidc.AuthURL(state))
}

// GET /auth/oidc/callback — exchange code, upsert user, create session.
func (h *handler) Callback(c echo.Context) error {
	log := logging.FromContext(c.Request().Context())

	cookie, err := c.Cookie(stateCookie)
	if err != nil || cookie.Value != c.QueryParam("state") {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid state")
	}
	c.SetCookie(&http.Cookie{Name: stateCookie, Path: "/", MaxAge: -1})

	claims, err := h.oidc.Exchange(c.Request().Context(), c.QueryParam("code"))
	if err != nil {
		log.Error("oidc exchange failed", "error", err)
		return echo.NewHTTPError(http.StatusBadRequest, "OIDC exchange failed")
	}
	if claims.Email == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "no email in OIDC token")
	}

	role := "user"
	if claims.IsAdmin {
		role = "admin"
	}

	user, err := h.authService.UpsertOIDCUser(c.Request().Context(), claims.Email, claims.Name, claims.AvatarURL, role)
	if err != nil {
		log.Error("upsert oidc user failed", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "user error")
	}

	session, err := h.sessionManager.CreateSession(c.Request().Context(), user.ID, nil)
	if err != nil {
		log.Error("create session failed", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "session error")
	}

	auth.SetSession(c, h.sessionManager, session)
	return respond.Redirect(c, "/")
}

func randomState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
