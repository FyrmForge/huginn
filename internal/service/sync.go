package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/FyrmForge/huginn/internal/connector"
	"github.com/FyrmForge/huginn/internal/repo"
)

// SyncService manages OAuth connections and imports events from external providers.
type SyncService struct {
	store    repo.Store
	calendar *CalendarService
	event    *EventService
	routing  *RoutingService
	google   *connector.GoogleProvider
	outlook  *connector.OutlookProvider
}

func NewSyncService(store repo.Store, calendar *CalendarService, event *EventService, routing *RoutingService) *SyncService {
	return &SyncService{store: store, calendar: calendar, event: event, routing: routing}
}

// Configure sets provider credentials at startup (or nil if not configured).
func (s *SyncService) Configure(google *connector.GoogleProvider, outlook *connector.OutlookProvider) {
	s.google = google
	s.outlook = outlook
}

// AuthorizeURL returns the OAuth redirect URL for the given provider.
func (s *SyncService) AuthorizeURL(provider, state, redirectURI string) (string, error) {
	switch provider {
	case "google":
		if s.google == nil {
			return "", fmt.Errorf("google provider not configured")
		}
		return s.google.AuthorizeURL(state, redirectURI), nil
	case "outlook":
		if s.outlook == nil {
			return "", fmt.Errorf("outlook provider not configured")
		}
		return s.outlook.AuthorizeURL(state, redirectURI), nil
	}
	return "", fmt.Errorf("unknown provider: %s", provider)
}

// CompleteOAuth exchanges the code, stores tokens, and creates a SyncConnection.
func (s *SyncService) CompleteOAuth(ctx context.Context, userID, provider, code, redirectURI string) (*repo.SyncConnection, error) {
	tok, err := s.exchange(ctx, provider, code, redirectURI)
	if err != nil {
		return nil, fmt.Errorf("exchange: %w", err)
	}

	now := time.Now()
	sc := &repo.SyncConnection{
		ID:        uuid.New().String(),
		UserID:    userID,
		Provider:  provider,
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.store.CreateSyncConnection(ctx, sc); err != nil {
		return nil, fmt.Errorf("create connection: %w", err)
	}

	st := &repo.SyncConnectionToken{
		ConnectionID: sc.ID,
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		ExpiresAt:    tok.ExpiresAt,
		Scope:        tok.Scope,
	}
	if err := s.store.UpsertSyncConnectionToken(ctx, st); err != nil {
		return nil, fmt.Errorf("store token: %w", err)
	}

	return sc, nil
}

// Sync runs a full or incremental import for a connection.
func (s *SyncService) Sync(ctx context.Context, userID, connectionID string) error {
	sc, err := s.store.GetSyncConnectionByID(ctx, connectionID)
	if err != nil || sc == nil {
		return fmt.Errorf("connection not found")
	}
	if sc.UserID != userID {
		return fmt.Errorf("not your connection")
	}

	tok, err := s.refreshIfNeeded(ctx, sc)
	if err != nil {
		return fmt.Errorf("token refresh: %w", err)
	}

	cals, err := s.listCalendars(ctx, sc.Provider, tok)
	if err != nil {
		return s.markError(ctx, sc, err)
	}

	for _, cal := range cals {
		huginnCal, err := s.ensureImportCalendar(ctx, userID, sc.Provider, cal)
		if err != nil {
			continue
		}
		events, _, err := s.fullSync(ctx, sc.Provider, tok, cal.ID)
		if err != nil {
			continue
		}
		for _, ev := range events {
			if ev.StartAt.IsZero() {
				continue
			}
			end := ev.EndAt
			if end.IsZero() {
				end = ev.StartAt.Add(time.Hour)
			}
			in := EventInput{
				Title:       ev.Title,
				Description: ev.Description,
				Location:    ev.Location,
				StartAt:     ev.StartAt,
				EndAt:       end,
				AllDay:      ev.AllDay,
				Timezone:    "UTC",
				Ownership:   "imported_readonly",
			}
			created, _ := s.event.Create(ctx, userID, huginnCal.ID, in)
			if created != nil {
				if targetCalID, rerr := s.routing.Route(ctx, userID, created); rerr == nil && targetCalID != "" && targetCalID != huginnCal.ID {
					in.Ownership = "imported_readonly"
					_, _ = s.event.Create(ctx, userID, targetCalID, in)
					_ = s.event.Delete(ctx, userID, created.ID)
				}
			}
		}
	}

	now := time.Now()
	sc.LastSyncedAt = &now
	sc.Status = "active"
	sc.LastError = ""
	return s.store.UpdateSyncConnection(ctx, sc)
}

// ListConnections returns all sync connections for a user.
func (s *SyncService) ListConnections(ctx context.Context, userID string) ([]*repo.SyncConnection, error) {
	return s.store.ListSyncConnectionsByUser(ctx, userID)
}

// Disconnect removes a sync connection and its tokens.
func (s *SyncService) Disconnect(ctx context.Context, userID, connectionID string) error {
	sc, err := s.store.GetSyncConnectionByID(ctx, connectionID)
	if err != nil || sc == nil {
		return fmt.Errorf("connection not found")
	}
	if sc.UserID != userID {
		return fmt.Errorf("not your connection")
	}
	return s.store.DeleteSyncConnection(ctx, connectionID)
}

// --- helpers ---

func (s *SyncService) exchange(ctx context.Context, provider, code, redirectURI string) (*connector.Token, error) {
	switch provider {
	case "google":
		return s.google.Exchange(ctx, code, redirectURI)
	case "outlook":
		return s.outlook.Exchange(ctx, code, redirectURI)
	}
	return nil, fmt.Errorf("unknown provider: %s", provider)
}

func (s *SyncService) refreshIfNeeded(ctx context.Context, sc *repo.SyncConnection) (*connector.Token, error) {
	st, err := s.store.GetSyncConnectionToken(ctx, sc.ID)
	if err != nil || st == nil {
		return nil, fmt.Errorf("no token stored")
	}
	tok := &connector.Token{
		AccessToken:  st.AccessToken,
		RefreshToken: st.RefreshToken,
		ExpiresAt:    st.ExpiresAt,
		Scope:        st.Scope,
	}
	if time.Until(st.ExpiresAt) < 5*time.Minute {
		var fresh *connector.Token
		switch sc.Provider {
		case "google":
			fresh, err = s.google.Refresh(ctx, st.RefreshToken)
		case "outlook":
			fresh, err = s.outlook.Refresh(ctx, st.RefreshToken)
		default:
			return nil, fmt.Errorf("unknown provider")
		}
		if err != nil {
			return nil, err
		}
		tok = fresh
		_ = s.store.UpsertSyncConnectionToken(ctx, &repo.SyncConnectionToken{
			ConnectionID: sc.ID,
			AccessToken:  fresh.AccessToken,
			RefreshToken: fresh.RefreshToken,
			ExpiresAt:    fresh.ExpiresAt,
			Scope:        fresh.Scope,
		})
	}
	return tok, nil
}

func (s *SyncService) listCalendars(ctx context.Context, provider string, tok *connector.Token) ([]connector.ExternalCalendar, error) {
	switch provider {
	case "google":
		return s.google.ListCalendars(ctx, tok)
	case "outlook":
		return s.outlook.ListCalendars(ctx, tok)
	}
	return nil, fmt.Errorf("unknown provider")
}

func (s *SyncService) fullSync(ctx context.Context, provider string, tok *connector.Token, calID string) ([]connector.ExternalEvent, string, error) {
	switch provider {
	case "google":
		return s.google.FullSync(ctx, tok, calID)
	case "outlook":
		return s.outlook.FullSync(ctx, tok, calID)
	}
	return nil, "", fmt.Errorf("unknown provider")
}

func (s *SyncService) ensureImportCalendar(ctx context.Context, userID, provider string, cal connector.ExternalCalendar) (*repo.Calendar, error) {
	cals, _ := s.calendar.ListForUser(ctx, userID)
	name := cal.Name + " (" + provider + ")"
	for _, c := range cals {
		if c.Name == name {
			return c, nil
		}
	}
	return s.calendar.Create(ctx, userID, name, "", cal.Color, "UTC")
}

func (s *SyncService) markError(ctx context.Context, sc *repo.SyncConnection, cause error) error {
	sc.Status = "error"
	sc.LastError = cause.Error()
	_ = s.store.UpdateSyncConnection(ctx, sc)
	return cause
}
