// Package connector defines the interfaces for external calendar providers
// (Google, Outlook) and keeps provider-specific types out of the core domain.
package connector

import (
	"context"
	"time"
)

// Token holds OAuth tokens. Connectors receive and return this, never storing
// directly — the caller persists them via SyncConnectionToken.
type Token struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
	Scope        string
}

// ExternalEvent is the normalised representation of an event from any provider.
// Provider-specific fields do not appear here.
type ExternalEvent struct {
	ProviderID  string // stable ID in the external system
	UID         string // iCalendar UID if present
	Title       string
	Description string
	Location    string
	StartAt     time.Time
	EndAt       time.Time
	AllDay      bool
	Status      string
	UpdatedAt   time.Time
	ProviderETag string
}

// OAuthProvider handles the OAuth flow for a provider.
type OAuthProvider interface {
	// AuthorizeURL returns the URL to redirect the user to for consent.
	AuthorizeURL(state, redirectURI string) string
	// Exchange swaps an auth code for tokens.
	Exchange(ctx context.Context, code, redirectURI string) (*Token, error)
	// Refresh obtains a new access token using the refresh token.
	Refresh(ctx context.Context, refreshToken string) (*Token, error)
}

// CalendarConnector fetches events from an external provider.
type CalendarConnector interface {
	// ListCalendars returns the external calendar IDs and names.
	ListCalendars(ctx context.Context, tok *Token) ([]ExternalCalendar, error)
	// FullSync fetches all events from a calendar.
	FullSync(ctx context.Context, tok *Token, calendarID string) ([]ExternalEvent, string, error)
	// IncrementalSync fetches events changed since syncToken.
	IncrementalSync(ctx context.Context, tok *Token, calendarID, syncToken string) ([]ExternalEvent, string, error)
}

// ExternalCalendar is a calendar listing from an external provider.
type ExternalCalendar struct {
	ID          string
	Name        string
	Description string
	Color       string
	Primary     bool
}
