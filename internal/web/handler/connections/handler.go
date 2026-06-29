package connections

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"

	"github.com/FyrmForge/hamr/pkg/middleware"
	"github.com/FyrmForge/hamr/pkg/respond"
	"github.com/labstack/echo/v4"

	"github.com/FyrmForge/huginn/internal/service"
	"github.com/FyrmForge/huginn/internal/web/components"
)

type handler struct {
	sync    *service.SyncService
	baseURL string
}

func NewHandler(sync *service.SyncService, baseURL string) *handler {
	return &handler{sync: sync, baseURL: baseURL}
}

// GET /connections
func (h *handler) Page(c echo.Context) error {
	user := components.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized)
	}
	conns, err := h.sync.ListConnections(c.Request().Context(), user.ID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError)
	}
	return respond.HTML(c, http.StatusOK, connectionsPage(c, conns))
}

// GET /connections/:provider/connect — start OAuth flow.
func (h *handler) Connect(c echo.Context) error {
	provider := c.Param("provider")
	state := randomHex(16)
	// ponytail: state stored in cookie; upgrade to session-backed state for CSRF strictness if needed
	c.SetCookie(&http.Cookie{Name: "oauth_state", Value: state, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode})
	redirectURI := h.redirectURI(c)
	url, err := h.sync.AuthorizeURL(provider, state, redirectURI)
	if err != nil {
		middleware.SetFlash(c, fmt.Sprintf("%s not configured: %s", provider, err), middleware.FlashError)
		return respond.Redirect(c, "/settings")
	}
	return c.Redirect(http.StatusTemporaryRedirect, url)
}

// GET /connections/callback — handle OAuth callback.
func (h *handler) Callback(c echo.Context) error {
	user := components.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized)
	}
	code     := c.QueryParam("code")
	state    := c.QueryParam("state")
	provider := c.QueryParam("scope") // used as fallback; actual provider from state cookie is cleaner
	_ = provider

	// Validate state.
	cookie, err := c.Cookie("oauth_state")
	if err != nil || cookie.Value != state {
		middleware.SetFlash(c, "OAuth state mismatch", middleware.FlashError)
		return respond.Redirect(c, "/settings")
	}
	c.SetCookie(&http.Cookie{Name: "oauth_state", Value: "", MaxAge: -1, Path: "/"})

	// The provider name is encoded into the state param prefix by the redirect.
	// Since we can't encode it there (state is opaque), detect via URL or fallback.
	// ponytail: derive provider from the query param google sets ("scope" contains "googleapis")
	//   or outlook sets; real solution = embed provider in state.
	detectedProvider := detectProvider(c)
	if detectedProvider == "" {
		middleware.SetFlash(c, "Could not detect OAuth provider", middleware.FlashError)
		return respond.Redirect(c, "/settings")
	}

	redirectURI := h.redirectURI(c)
	_, err = h.sync.CompleteOAuth(c.Request().Context(), user.ID, detectedProvider, code, redirectURI)
	if err != nil {
		middleware.SetFlash(c, "OAuth failed: "+err.Error(), middleware.FlashError)
		return respond.Redirect(c, "/settings")
	}

	middleware.SetFlash(c, detectedProvider+" connected", middleware.FlashSuccess)
	return respond.Redirect(c, "/settings")
}

// POST /connections/:id/sync
func (h *handler) Sync(c echo.Context) error {
	user := components.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized)
	}
	if err := h.sync.Sync(c.Request().Context(), user.ID, c.Param("id")); err != nil {
		middleware.SetFlash(c, "Sync failed: "+err.Error(), middleware.FlashError)
	} else {
		middleware.SetFlash(c, "Sync complete", middleware.FlashSuccess)
	}
	return respond.Redirect(c, "/settings")
}

// POST /connections/:id/disconnect
func (h *handler) Disconnect(c echo.Context) error {
	user := components.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized)
	}
	if err := h.sync.Disconnect(c.Request().Context(), user.ID, c.Param("id")); err != nil {
		middleware.SetFlash(c, "Disconnect failed: "+err.Error(), middleware.FlashError)
	} else {
		middleware.SetFlash(c, "Disconnected", middleware.FlashSuccess)
	}
	return respond.Redirect(c, "/settings")
}

func (h *handler) redirectURI(c echo.Context) string {
	base := h.baseURL
	if base == "" {
		base = "http://" + c.Request().Host
	}
	return base + "/connections/callback"
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// detectProvider sniffs which OAuth provider issued the callback from query params.
func detectProvider(c echo.Context) string {
	// Google sets "scope" in the callback; Microsoft sets "session_state"
	if c.QueryParam("session_state") != "" {
		return "outlook"
	}
	return "google"
}
