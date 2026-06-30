package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/FyrmForge/huginn/internal/repo"
)

var (
	ErrCalendarNotFound    = errors.New("calendar not found")
	ErrCalendarForbidden   = errors.New("calendar access denied")
	ErrCannotDeleteDefault = errors.New("cannot delete the default calendar")
	ErrShareTargetNotFound = errors.New("no user found with that email")
	ErrShareSelf           = errors.New("cannot share with yourself")
	ErrNotMember           = errors.New("you are not a member of this calendar")
	ErrOwnerCannotLeave    = errors.New("owners cannot leave — delete the calendar instead")
)

type CalendarService struct {
	store repo.Store
}

func NewCalendarService(store repo.Store) *CalendarService {
	return &CalendarService{store: store}
}

// GetByID returns a calendar if the user has at least viewer access.
func (s *CalendarService) GetByID(ctx context.Context, userID, calendarID string) (*repo.Calendar, error) {
	return s.get(ctx, userID, calendarID, "viewer")
}

// ListForUser returns all calendars visible to the given user.
func (s *CalendarService) ListForUser(ctx context.Context, userID string) ([]*repo.Calendar, error) {
	return s.store.ListCalendarsByUser(ctx, userID)
}

// ListEditableForUser returns calendars the user can create/edit events in (owner or editor role).
func (s *CalendarService) ListEditableForUser(ctx context.Context, userID string) ([]*repo.Calendar, error) {
	all, err := s.store.ListCalendarsByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	var out []*repo.Calendar
	for _, cal := range all {
		if cal.OwnerID == userID {
			out = append(out, cal)
			continue
		}
		m, err := s.store.GetCalendarMember(ctx, cal.ID, userID)
		if err == nil && m != nil && m.Role == "editor" {
			out = append(out, cal)
		}
	}
	return out, nil
}

// Create creates a new calendar owned by userID.
func (s *CalendarService) Create(ctx context.Context, userID, name, description, color, timezone string) (*repo.Calendar, error) {
	now := time.Now()
	cal := &repo.Calendar{
		ID:                uuid.New().String(),
		OwnerID:           userID,
		Name:              name,
		Description:       description,
		Color:             color,
		Timezone:          timezone,
		DefaultVisibility: "private",
		DefaultBusyStatus: "busy",
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := s.store.CreateCalendar(ctx, cal); err != nil {
		return nil, fmt.Errorf("create calendar: %w", err)
	}
	return cal, nil
}

// CreateWithID creates a calendar with a caller-supplied ID (used by CalDAV MKCALENDAR).
func (s *CalendarService) CreateWithID(ctx context.Context, userID, id, name, color string) (*repo.Calendar, error) {
	now := time.Now()
	cal := &repo.Calendar{
		ID:                id,
		OwnerID:           userID,
		Name:              name,
		Color:             color,
		Timezone:          "UTC",
		DefaultVisibility: "private",
		DefaultBusyStatus: "busy",
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := s.store.CreateCalendar(ctx, cal); err != nil {
		return nil, fmt.Errorf("create calendar: %w", err)
	}
	if err := s.store.AddCalendarMember(ctx, &repo.CalendarMember{
		CalendarID: id,
		UserID:     userID,
		Role:       "owner",
		CreatedAt:  now,
	}); err != nil {
		return nil, fmt.Errorf("add member: %w", err)
	}
	return cal, nil
}

// EnsureDefaults creates the standard default calendars for a new user
// if they don't already have any.
func (s *CalendarService) EnsureDefaults(ctx context.Context, userID string) error {
	existing, err := s.store.ListCalendarsByUser(ctx, userID)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		return nil
	}

	defaults := []struct{ name, color string }{
		{"Personal", "#4f8ef7"},
		{"Work", "#7dd181"},
		{"Other Events", "#f5a623"},
	}
	now := time.Now()
	for i, d := range defaults {
		cal := &repo.Calendar{
			ID:                uuid.New().String(),
			OwnerID:           userID,
			Name:              d.name,
			Color:             d.color,
			Timezone:          "UTC",
			DefaultVisibility: "private",
			DefaultBusyStatus: "busy",
			IsDefault:         i == 0,
			CreatedAt:         now,
			UpdatedAt:         now,
		}
		if err := s.store.CreateCalendar(ctx, cal); err != nil {
			return fmt.Errorf("create default calendar %q: %w", d.name, err)
		}
	}
	return nil
}

// Update updates calendar metadata. Only the owner may update.
func (s *CalendarService) Update(ctx context.Context, userID, calendarID, name, description, color, timezone string) error {
	cal, err := s.get(ctx, userID, calendarID, "owner")
	if err != nil {
		return err
	}
	cal.Name = name
	cal.Description = description
	cal.Color = color
	cal.Timezone = timezone
	return s.store.UpdateCalendar(ctx, cal)
}

// Delete soft-deletes a calendar. Only the owner may delete; default calendars are protected.
func (s *CalendarService) Delete(ctx context.Context, userID, calendarID string) error {
	cal, err := s.get(ctx, userID, calendarID, "owner")
	if err != nil {
		return err
	}
	if cal.IsDefault {
		return ErrCannotDeleteDefault
	}
	return s.store.DeleteCalendar(ctx, calendarID)
}

// GetUserRole returns the effective role of userID on calendarID:
// "owner", "editor", "viewer", or "" (no access).
func (s *CalendarService) GetUserRole(ctx context.Context, calendarID, userID string) (string, error) {
	cal, err := s.store.GetCalendarByID(ctx, calendarID)
	if err != nil || cal == nil {
		return "", err
	}
	if cal.OwnerID == userID {
		return "owner", nil
	}
	m, err := s.store.GetCalendarMember(ctx, calendarID, userID)
	if err != nil {
		return "", err
	}
	if m == nil {
		return "", nil
	}
	return m.Role, nil
}

// ListMembersWithUsers returns members of a calendar with user info.
// Only accessible to owners.
func (s *CalendarService) ListMembersWithUsers(ctx context.Context, calendarID, ownerID string) ([]*repo.CalendarMemberInfo, error) {
	cal, err := s.store.GetCalendarByID(ctx, calendarID)
	if err != nil || cal == nil {
		return nil, ErrCalendarNotFound
	}
	if cal.OwnerID != ownerID {
		return nil, ErrCalendarForbidden
	}
	return s.store.ListCalendarMembersWithUsers(ctx, calendarID)
}

// ShareWith grants targetEmail access to calendarID with the given role.
// Only the owner may share.
func (s *CalendarService) ShareWith(ctx context.Context, calendarID, ownerID, targetEmail, role string) error {
	if role != "editor" && role != "viewer" {
		return fmt.Errorf("invalid role %q", role)
	}
	cal, err := s.store.GetCalendarByID(ctx, calendarID)
	if err != nil || cal == nil {
		return ErrCalendarNotFound
	}
	if cal.OwnerID != ownerID {
		return ErrCalendarForbidden
	}
	target, err := s.store.GetUserByEmail(ctx, targetEmail)
	if err != nil {
		return fmt.Errorf("look up user: %w", err)
	}
	if target == nil {
		return ErrShareTargetNotFound
	}
	if target.ID == ownerID {
		return ErrShareSelf
	}
	return s.store.AddCalendarMember(ctx, &repo.CalendarMember{
		CalendarID: calendarID,
		UserID:     target.ID,
		Role:       role,
		CreatedAt:  time.Now(),
	})
}

// Unshare removes a member from a calendar. Only the owner may do this.
func (s *CalendarService) Unshare(ctx context.Context, calendarID, ownerID, targetUserID string) error {
	cal, err := s.store.GetCalendarByID(ctx, calendarID)
	if err != nil || cal == nil {
		return ErrCalendarNotFound
	}
	if cal.OwnerID != ownerID {
		return ErrCalendarForbidden
	}
	if targetUserID == ownerID {
		return ErrOwnerCannotLeave
	}
	return s.store.RemoveCalendarMember(ctx, calendarID, targetUserID)
}

// Leave removes the requesting user from a shared calendar.
// Owners cannot leave — they must delete the calendar.
func (s *CalendarService) Leave(ctx context.Context, calendarID, userID string) error {
	cal, err := s.store.GetCalendarByID(ctx, calendarID)
	if err != nil || cal == nil {
		return ErrCalendarNotFound
	}
	if cal.OwnerID == userID {
		return ErrOwnerCannotLeave
	}
	m, err := s.store.GetCalendarMember(ctx, calendarID, userID)
	if err != nil || m == nil {
		return ErrNotMember
	}
	return s.store.RemoveCalendarMember(ctx, calendarID, userID)
}

// get fetches a calendar and checks access. role: "owner", "editor", "viewer", "any".
func (s *CalendarService) get(ctx context.Context, userID, calendarID, minRole string) (*repo.Calendar, error) {
	cal, err := s.store.GetCalendarByID(ctx, calendarID)
	if err != nil {
		return nil, fmt.Errorf("get calendar: %w", err)
	}
	if cal == nil {
		return nil, ErrCalendarNotFound
	}
	if minRole == "owner" && cal.OwnerID != userID {
		return nil, ErrCalendarForbidden
	}
	if minRole == "any" {
		return cal, nil
	}
	// For non-owner access check membership.
	if cal.OwnerID != userID {
		m, err := s.store.GetCalendarMember(ctx, calendarID, userID)
		if err != nil {
			return nil, err
		}
		if m == nil {
			return nil, ErrCalendarForbidden
		}
	}
	return cal, nil
}
