package repo

import (
	"context"
	"time"

	"github.com/FyrmForge/hamr/pkg/auth"
)

// Store defines the data access interface for the application.
type Store interface {
	// Health checks the database connection.
	Health(ctx context.Context) error

	// Session persistence — satisfies auth.SessionStore.
	auth.SessionStore

	// --- Users ---
	GetUserByID(ctx context.Context, id string) (*User, error)
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	CreateUser(ctx context.Context, user *User) error
	UpdateUser(ctx context.Context, user *User) error
	ListAllUsers(ctx context.Context) ([]*User, error)
	AdminStats(ctx context.Context) (*Stats, error)

	// --- User settings ---
	GetUserSettings(ctx context.Context, userID string) (*UserSettings, error)
	UpsertUserSettings(ctx context.Context, s *UserSettings) error

	// --- Calendars ---
	CreateCalendar(ctx context.Context, cal *Calendar) error
	GetCalendarByID(ctx context.Context, id string) (*Calendar, error)
	ListCalendarsByUser(ctx context.Context, userID string) ([]*Calendar, error)
	UpdateCalendar(ctx context.Context, cal *Calendar) error
	DeleteCalendar(ctx context.Context, id string) error

	// --- Calendar members ---
	AddCalendarMember(ctx context.Context, m *CalendarMember) error
	RemoveCalendarMember(ctx context.Context, calendarID, userID string) error
	ListCalendarMembers(ctx context.Context, calendarID string) ([]*CalendarMember, error)
	ListCalendarMembersWithUsers(ctx context.Context, calendarID string) ([]*CalendarMemberInfo, error)
	GetCalendarMember(ctx context.Context, calendarID, userID string) (*CalendarMember, error)
	CalendarAudience(ctx context.Context, calendarID string) ([]string, error)

	// --- App passwords (CalDAV device tokens) ---
	CreateAppPassword(ctx context.Context, ap *AppPassword) error
	GetAppPasswordByHash(ctx context.Context, hash string) (*AppPassword, error)
	ListAppPasswords(ctx context.Context, userID string) ([]*AppPassword, error)
	RevokeAppPassword(ctx context.Context, id string) error
	TouchAppPassword(ctx context.Context, id string) error

	// --- OAuth provider configs ---
	GetOAuthProviderConfig(ctx context.Context, provider string) (*OAuthProviderConfig, error)
	UpsertOAuthProviderConfig(ctx context.Context, cfg *OAuthProviderConfig) error

	// --- Sync connections ---
	CreateSyncConnection(ctx context.Context, sc *SyncConnection) error
	GetSyncConnectionByID(ctx context.Context, id string) (*SyncConnection, error)
	ListSyncConnectionsByUser(ctx context.Context, userID string) ([]*SyncConnection, error)
	UpdateSyncConnection(ctx context.Context, sc *SyncConnection) error
	DeleteSyncConnection(ctx context.Context, id string) error
	UpsertSyncConnectionToken(ctx context.Context, tok *SyncConnectionToken) error
	GetSyncConnectionToken(ctx context.Context, connectionID string) (*SyncConnectionToken, error)

	// --- Routing rules ---
	CreateRoutingRule(ctx context.Context, r *RoutingRule) error
	GetRoutingRuleByID(ctx context.Context, id string) (*RoutingRule, error)
	ListRoutingRulesByUser(ctx context.Context, userID string) ([]*RoutingRule, error)
	UpdateRoutingRule(ctx context.Context, r *RoutingRule) error
	DeleteRoutingRule(ctx context.Context, id string) error
	CreateRoutingAudit(ctx context.Context, a *RoutingAudit) error

	// --- Events ---
	CreateEvent(ctx context.Context, e *Event) error
	GetEventByID(ctx context.Context, id string) (*Event, error)
	GetEventByUID(ctx context.Context, calendarID, uid string) (*Event, error)
	ListEvents(ctx context.Context, calendarID string, from, to time.Time) ([]*Event, error)
	ListEventsByCalendars(ctx context.Context, calendarIDs []string, from, to time.Time) ([]*Event, error)
	ListEventsModifiedSince(ctx context.Context, calendarID string, since time.Time) ([]*Event, error)
	GetCalendarSyncStamp(ctx context.Context, calendarID string) (time.Time, error)
	UpdateEvent(ctx context.Context, e *Event) error
	DeleteEvent(ctx context.Context, id string) error

	// --- Event exceptions (recurring series overrides) ---
	UpsertEventException(ctx context.Context, ex *EventException) error
	GetEventException(ctx context.Context, masterID string, recurrenceID time.Time) (*EventException, error)
	ListEventExceptions(ctx context.Context, masterID string) ([]*EventException, error)
	DeleteEventException(ctx context.Context, masterID string, recurrenceID time.Time) error
}
