package repo

import "time"

// Stats holds aggregate counts for the admin dashboard.
type Stats struct {
	TotalUsers       int
	AdminUsers       int
	TotalCalendars   int
	TotalEvents      int
	TotalConnections int
	TotalDevices     int
}

// CalendarMemberInfo joins a calendar_member row with the user's display data.
type CalendarMemberInfo struct {
	CalendarID  string    `db:"calendar_id"`
	UserID      string    `db:"user_id"`
	Role        string    `db:"role"`
	MemberSince time.Time `db:"created_at"`
	Email       string    `db:"email"`
	Name        string    `db:"name"`
	AvatarURL   string    `db:"avatar_url"`
}

// UserSettings holds per-user preferences.
type UserSettings struct {
	UserID                  string    `db:"user_id"`
	Timezone                string    `db:"timezone"`
	Locale                  string    `db:"locale"`
	DateFormat              string    `db:"date_format"`
	TimeFormat              string    `db:"time_format"`
	FirstDayOfWeek          int       `db:"first_day_of_week"`
	DefaultView             string    `db:"default_view"`
	DefaultEventDurationMin int       `db:"default_event_duration_mins"`
	CreatedAt               time.Time `db:"created_at"`
	UpdatedAt               time.Time `db:"updated_at"`
}

// Calendar is a named collection of events owned by a user.
type Calendar struct {
	ID                string     `db:"id"`
	OwnerID           string     `db:"owner_id"`
	Name              string     `db:"name"`
	Description       string     `db:"description"`
	Color             string     `db:"color"`
	Timezone          string     `db:"timezone"`
	DefaultVisibility string     `db:"default_visibility"`
	DefaultBusyStatus string     `db:"default_busy_status"`
	IsDefault         bool       `db:"is_default"`
	DeletedAt         *time.Time `db:"deleted_at"`
	CreatedAt         time.Time  `db:"created_at"`
	UpdatedAt         time.Time  `db:"updated_at"`
}

// CalendarMember is a user's membership in a shared calendar.
type CalendarMember struct {
	CalendarID string    `db:"calendar_id"`
	UserID     string    `db:"user_id"`
	Role       string    `db:"role"` // owner, editor, viewer
	CreatedAt  time.Time `db:"created_at"`
}

// AppPassword is a device/CalDAV token for a user.
type AppPassword struct {
	ID          string     `db:"id"`
	UserID      string     `db:"user_id"`
	Name        string     `db:"name"`
	TokenHash   string     `db:"token_hash"`
	Permissions string     `db:"permissions"`
	LastUsedAt  *time.Time `db:"last_used_at"`
	RevokedAt   *time.Time `db:"revoked_at"`
	CreatedAt   time.Time  `db:"created_at"`
}

// Event is a single calendar event (or master of a recurring series).
type Event struct {
	ID          string     `db:"id"`
	CalendarID  string     `db:"calendar_id"`
	UID         string     `db:"uid"`
	Title       string     `db:"title"`
	Description string     `db:"description"`
	Location    string     `db:"location"`
	StartAt     time.Time  `db:"start_at"`
	EndAt       time.Time  `db:"end_at"`
	Timezone    string     `db:"timezone"`
	AllDay      bool       `db:"all_day"`
	Status      string     `db:"status"`
	Visibility  string     `db:"visibility"`
	BusyStatus  string     `db:"busy_status"`
	RRule       string     `db:"rrule"`   // RRULE string, e.g. "FREQ=WEEKLY;BYDAY=MO"
	Exdates     string     `db:"exdates"` // comma-separated UTC timestamps (RFC5545 EXDATE)
	Rdates      string     `db:"rdates"`  // comma-separated UTC timestamps (RFC5545 RDATE)
	RawICS      string     `db:"raw_ics"`
	ETag        string     `db:"etag"`
	Ownership   string     `db:"ownership"` // native, caldav_created, imported_readonly
	CreatedBy   *string    `db:"created_by"`
	UpdatedBy   *string    `db:"updated_by"`
	DeletedAt   *time.Time `db:"deleted_at"`
	CreatedAt   time.Time  `db:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at"`
}

// EventException is a per-occurrence override or cancellation within a recurring series.
type EventException struct {
	ID            string    `db:"id"`
	MasterEventID string    `db:"master_event_id"`
	RecurrenceID  time.Time `db:"recurrence_id"` // original occurrence start (RFC5545 RECURRENCE-ID)
	Title         string    `db:"title"`
	Description   string    `db:"description"`
	Location      string    `db:"location"`
	StartAt       time.Time `db:"start_at"`
	EndAt         time.Time `db:"end_at"`
	Timezone      string    `db:"timezone"`
	AllDay        bool      `db:"all_day"`
	Status        string    `db:"status"`
	IsDeleted     bool      `db:"is_deleted"` // true = this occurrence is cancelled
	ETag          string    `db:"etag"`
	CreatedAt     time.Time `db:"created_at"`
	UpdatedAt     time.Time `db:"updated_at"`
}
