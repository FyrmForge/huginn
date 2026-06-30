package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	rrule "github.com/teambition/rrule-go"

	"github.com/FyrmForge/hamr/pkg/websocket"

	"github.com/FyrmForge/huginn/internal/repo"
)

var (
	ErrEventNotFound  = errors.New("event not found")
	ErrEventForbidden = errors.New("event access denied")
	ErrEventReadOnly  = errors.New("imported events are read-only")
)

// Occurrence is a single materialised instance of an event (or a recurring series occurrence).
// It carries instance-level times so views and week-grid calculations are correct.
type Occurrence struct {
	MasterEvent  *repo.Event
	RecurrenceID time.Time // original occurrence start (zero for non-recurring)
	Start        time.Time
	End          time.Time
	Title        string
	Description  string
	Location     string
	AllDay       bool
	IsException  bool   // occurrence was individually modified
	ExceptionID  string // ID of the EventException row if IsException
}

// CalendarID returns the calendar ID of the underlying master event.
func (o *Occurrence) CalendarID() string { return o.MasterEvent.CalendarID }

// EventID returns the master event ID.
func (o *Occurrence) EventID() string { return o.MasterEvent.ID }

// IsRecurring reports whether this occurrence comes from a recurring series.
func (o *Occurrence) IsRecurring() bool { return !o.RecurrenceID.IsZero() }

type EventService struct {
	store   repo.Store
	emitter *websocket.Emitter
}

func NewEventService(store repo.Store) *EventService {
	return &EventService{store: store}
}

// SetEmitter wires the WebSocket emitter used to push live calendar updates.
// Optional: when unset (e.g. in tests) notifications are no-ops.
func (s *EventService) SetEmitter(e *websocket.Emitter) { s.emitter = e }

// notifyCalendarChange pushes a refresh trigger to every user who can see the
// calendar, so their open calendar views re-fetch. Best-effort: errors are
// swallowed since the mutation itself already succeeded.
func (s *EventService) notifyCalendarChange(ctx context.Context, calendarID string) {
	if s.emitter == nil {
		return
	}
	// Recipients = owner + any share members, deduped. The owner is taken from
	// the calendar row directly: seeded calendars may have no calendar_members
	// row for the owner, so ListCalendarMembers alone can miss them.
	recipients := map[string]bool{}
	if cal, err := s.store.GetCalendarByID(ctx, calendarID); err == nil && cal != nil {
		recipients[cal.OwnerID] = true
	}
	if members, err := s.store.ListCalendarMembers(ctx, calendarID); err == nil {
		for _, m := range members {
			recipients[m.UserID] = true
		}
	}
	// Reuse the existing client refresh convention: the calendar grids listen
	// for "huginn:refresh-calendar from:body" (see calendar.templ) and re-fetch.
	ev := websocket.NewTriggerEvent("huginn:refresh-calendar", "body", "huginn:refresh-calendar")
	for uid := range recipients {
		s.emitter.ToSubject(uid, ev)
	}
}

// notifyEventChange resolves the calendar for an event ID and notifies its
// members. Used by paths that only have the event/master ID at hand.
func (s *EventService) notifyEventChange(ctx context.Context, eventID string) {
	if s.emitter == nil {
		return
	}
	if e, err := s.store.GetEventByID(ctx, eventID); err == nil && e != nil {
		s.notifyCalendarChange(ctx, e.CalendarID)
	}
}

func (s *EventService) GetByID(ctx context.Context, id string) (*repo.Event, error) {
	return s.store.GetEventByID(ctx, id)
}

// GetUserDisplay returns "Name (email)" for display, or email alone, or "" if not found.
func (s *EventService) GetUserDisplay(ctx context.Context, userID string) string {
	u, err := s.store.GetUserByID(ctx, userID)
	if err != nil || u == nil {
		return ""
	}
	if u.Name != "" && u.Name != u.Email {
		return u.Name + " (" + u.Email + ")"
	}
	return u.Email
}

func (s *EventService) GetByUID(ctx context.Context, calendarID, uid string) (*repo.Event, error) {
	return s.store.GetEventByUID(ctx, calendarID, uid)
}

func (s *EventService) GetSyncStamp(ctx context.Context, calendarID string) (time.Time, error) {
	return s.store.GetCalendarSyncStamp(ctx, calendarID)
}

func (s *EventService) ListModifiedSince(ctx context.Context, calendarID string, since time.Time) ([]*repo.Event, error) {
	return s.store.ListEventsModifiedSince(ctx, calendarID, since)
}

// ListForRange returns master events (for CalDAV use). Recurring masters have rrule set.
func (s *EventService) ListForRange(ctx context.Context, calendarIDs []string, from, to time.Time) ([]*repo.Event, error) {
	return s.store.ListEventsByCalendars(ctx, calendarIDs, from, to)
}

// ExpandForRange returns expanded occurrences for the views layer.
// Non-recurring events produce one Occurrence each; recurring events expand via RRULE.
func (s *EventService) ExpandForRange(ctx context.Context, calendarIDs []string, from, to time.Time) ([]Occurrence, error) {
	events, err := s.store.ListEventsByCalendars(ctx, calendarIDs, from, to)
	if err != nil {
		return nil, err
	}

	var out []Occurrence
	for _, e := range events {
		if e.RRule == "" {
			// Non-recurring: emit single occurrence only if it overlaps the window.
			if e.StartAt.Before(to) && e.EndAt.After(from) {
				out = append(out, Occurrence{
					MasterEvent: e,
					Start:       e.StartAt,
					End:         e.EndAt,
					Title:       e.Title,
					Description: e.Description,
					Location:    e.Location,
					AllDay:      e.AllDay,
				})
			}
			continue
		}

		exs, err := s.store.ListEventExceptions(ctx, e.ID)
		if err != nil {
			return nil, fmt.Errorf("load exceptions for %s: %w", e.ID, err)
		}

		occs := ExpandOccurrences(e, exs, from, to)
		out = append(out, occs...)
	}
	return out, nil
}

// ExpandOccurrences expands a recurring master event into individual occurrences within [from, to).
// Exceptions (modified or deleted occurrences) are applied.
func ExpandOccurrences(e *repo.Event, exceptions []*repo.EventException, from, to time.Time) []Occurrence {
	// Build a map of exceptions keyed by recurrence_id (truncated to second for robustness).
	exMap := make(map[time.Time]*repo.EventException, len(exceptions))
	for _, ex := range exceptions {
		exMap[ex.RecurrenceID.UTC().Truncate(time.Second)] = ex
	}

	loc := loadLocation(e.Timezone)
	duration := e.EndAt.Sub(e.StartAt)

	rset, err := buildRSet(e, loc)
	if err != nil {
		return nil
	}

	// Get all occurrences in [from, to). The library's Between is inclusive on start; we use
	// inc=true and filter out the exclusive end ourselves.
	times := rset.Between(from.Add(-duration), to, true)

	var out []Occurrence
	for _, t := range times {
		occStart := t.In(loc)
		occEnd := occStart.Add(duration)

		// Filter: occurrence must overlap [from, to).
		if !occStart.Before(to) || !occEnd.After(from) {
			continue
		}

		key := occStart.UTC().Truncate(time.Second)
		if ex, ok := exMap[key]; ok {
			if ex.IsDeleted {
				continue // EXDATE — skip this occurrence
			}
			// Modified occurrence.
			out = append(out, Occurrence{
				MasterEvent:  e,
				RecurrenceID: occStart.UTC(),
				Start:        ex.StartAt,
				End:          ex.EndAt,
				Title:        ex.Title,
				Description:  ex.Description,
				Location:     ex.Location,
				AllDay:       ex.AllDay,
				IsException:  true,
				ExceptionID:  ex.ID,
			})
			continue
		}

		out = append(out, Occurrence{
			MasterEvent:  e,
			RecurrenceID: occStart.UTC(),
			Start:        occStart.UTC(),
			End:          occEnd.UTC(),
			Title:        e.Title,
			Description:  e.Description,
			Location:     e.Location,
			AllDay:       e.AllDay,
		})
	}
	return out
}

func buildRSet(e *repo.Event, loc *time.Location) (*rrule.Set, error) {
	// Parse the master RRULE.
	ruleStr := "RRULE:" + e.RRule
	r, err := rrule.StrToRRule(ruleStr)
	if err != nil {
		return nil, err
	}
	// DTSTART must be in the event's timezone for DST-correct expansion.
	dtstart := e.StartAt.In(loc)
	r.DTStart(dtstart)

	set := &rrule.Set{}
	set.RRule(r)

	// Apply EXDATEs stored as comma-separated UTC timestamps.
	if e.Exdates != "" {
		for _, raw := range strings.Split(e.Exdates, ",") {
			raw = strings.TrimSpace(raw)
			if raw == "" {
				continue
			}
			t, err := time.Parse(time.RFC3339, raw)
			if err != nil {
				continue
			}
			set.ExDate(t.In(loc))
		}
	}

	// Apply RDATEs.
	if e.Rdates != "" {
		for _, raw := range strings.Split(e.Rdates, ",") {
			raw = strings.TrimSpace(raw)
			if raw == "" {
				continue
			}
			t, err := time.Parse(time.RFC3339, raw)
			if err != nil {
				continue
			}
			set.RDate(t.In(loc))
		}
	}

	return set, nil
}

func loadLocation(tz string) *time.Location {
	if tz == "" {
		return time.UTC
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return time.UTC
	}
	return loc
}

// Create creates a new native event.
func (s *EventService) Create(ctx context.Context, userID, calendarID string, in EventInput) (*repo.Event, error) {
	now := time.Now()
	id := in.ID
	if id == "" {
		id = uuid.New().String()
	}
	e := &repo.Event{
		ID:          id,
		CalendarID:  calendarID,
		UID:         uuid.New().String() + "@huginn",
		Title:       in.Title,
		Description: in.Description,
		Location:    in.Location,
		StartAt:     in.StartAt,
		EndAt:       in.EndAt,
		Timezone:    in.Timezone,
		AllDay:      in.AllDay,
		RRule:       in.RRule,
		Status:      "confirmed",
		Visibility:  "private",
		BusyStatus:  "busy",
		ETag:        uuid.New().String(),
		Ownership: func() string {
			if in.Ownership != "" {
				return in.Ownership
			}
			return "native"
		}(),
		CreatedBy: &userID,
		UpdatedBy: &userID,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.store.CreateEvent(ctx, e); err != nil {
		return nil, fmt.Errorf("create event: %w", err)
	}
	s.notifyCalendarChange(ctx, calendarID)
	return e, nil
}

// Update updates a native event (all occurrences — replaces the master).
func (s *EventService) Update(ctx context.Context, userID, eventID string, in EventInput) error {
	e, err := s.store.GetEventByID(ctx, eventID)
	if err != nil {
		return fmt.Errorf("get event: %w", err)
	}
	if e == nil {
		return ErrEventNotFound
	}
	if e.Ownership == "imported_readonly" {
		return ErrEventReadOnly
	}
	e.Title = in.Title
	e.Description = in.Description
	e.Location = in.Location
	e.StartAt = in.StartAt
	e.EndAt = in.EndAt
	e.Timezone = in.Timezone
	e.AllDay = in.AllDay
	e.RRule = in.RRule
	e.ETag = uuid.New().String()
	e.UpdatedBy = &userID
	if err := s.store.UpdateEvent(ctx, e); err != nil {
		return err
	}
	s.notifyCalendarChange(ctx, e.CalendarID)
	return nil
}

// UpdateOccurrence saves an override for a single occurrence of a recurring series.
func (s *EventService) UpdateOccurrence(ctx context.Context, userID, masterID string, recurrenceID time.Time, in EventInput) error {
	master, err := s.store.GetEventByID(ctx, masterID)
	if err != nil || master == nil {
		return ErrEventNotFound
	}
	if master.Ownership == "imported_readonly" {
		return ErrEventReadOnly
	}

	ex := &repo.EventException{
		ID:            uuid.New().String(),
		MasterEventID: masterID,
		RecurrenceID:  recurrenceID,
		Title:         in.Title,
		Description:   in.Description,
		Location:      in.Location,
		StartAt:       in.StartAt,
		EndAt:         in.EndAt,
		Timezone:      in.Timezone,
		AllDay:        in.AllDay,
		Status:        "confirmed",
		IsDeleted:     false,
		ETag:          uuid.New().String(),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	// Bump master ETag so CalDAV sync notices.
	master.ETag = uuid.New().String()
	master.UpdatedBy = &userID
	if err := s.store.UpdateEvent(ctx, master); err != nil {
		return err
	}
	if err := s.store.UpsertEventException(ctx, ex); err != nil {
		return err
	}
	s.notifyCalendarChange(ctx, master.CalendarID)
	return nil
}

// DeleteOccurrence cancels a single occurrence by marking it deleted in event_exceptions.
func (s *EventService) DeleteOccurrence(ctx context.Context, userID, masterID string, recurrenceID time.Time) error {
	master, err := s.store.GetEventByID(ctx, masterID)
	if err != nil || master == nil {
		return ErrEventNotFound
	}
	if master.Ownership == "imported_readonly" {
		return ErrEventReadOnly
	}
	ex := &repo.EventException{
		ID:            uuid.New().String(),
		MasterEventID: masterID,
		RecurrenceID:  recurrenceID,
		Title:         master.Title,
		Description:   master.Description,
		Location:      master.Location,
		StartAt:       recurrenceID,
		EndAt:         recurrenceID.Add(master.EndAt.Sub(master.StartAt)),
		Timezone:      master.Timezone,
		AllDay:        master.AllDay,
		Status:        "cancelled",
		IsDeleted:     true,
		ETag:          uuid.New().String(),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	master.ETag = uuid.New().String()
	master.UpdatedBy = &userID
	if err := s.store.UpdateEvent(ctx, master); err != nil {
		return err
	}
	if err := s.store.UpsertEventException(ctx, ex); err != nil {
		return err
	}
	s.notifyCalendarChange(ctx, master.CalendarID)
	return nil
}

// UpdateThisAndFuture splits the recurring series at recurrenceID:
// - The original series gets UNTIL set to (recurrenceID - 1 nanosecond).
// - A new master is created from recurrenceID onward with the updated fields.
func (s *EventService) UpdateThisAndFuture(ctx context.Context, userID, masterID string, recurrenceID time.Time, in EventInput) error {
	master, err := s.store.GetEventByID(ctx, masterID)
	if err != nil || master == nil {
		return ErrEventNotFound
	}
	if master.Ownership == "imported_readonly" {
		return ErrEventReadOnly
	}

	// Truncate original series: set UNTIL to one second before the split point.
	until := recurrenceID.Add(-time.Second).UTC()
	newRRule := appendUntil(master.RRule, until)
	master.RRule = newRRule
	master.ETag = uuid.New().String()
	master.UpdatedBy = &userID
	if err := s.store.UpdateEvent(ctx, master); err != nil {
		return err
	}

	// New series starts at recurrenceID with updated fields.
	newID := uuid.New().String()
	duration := master.EndAt.Sub(master.StartAt)
	newEnd := recurrenceID.Add(duration)
	if !in.EndAt.IsZero() {
		newEnd = in.EndAt
	}
	newMaster := &repo.Event{
		ID:          newID,
		CalendarID:  master.CalendarID,
		UID:         uuid.New().String() + "@huginn",
		Title:       in.Title,
		Description: in.Description,
		Location:    in.Location,
		StartAt:     recurrenceID,
		EndAt:       newEnd,
		Timezone:    in.Timezone,
		AllDay:      in.AllDay,
		RRule:       in.RRule,
		Status:      "confirmed",
		Visibility:  master.Visibility,
		BusyStatus:  master.BusyStatus,
		ETag:        uuid.New().String(),
		Ownership:   master.Ownership,
		CreatedBy:   &userID,
		UpdatedBy:   &userID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := s.store.CreateEvent(ctx, newMaster); err != nil {
		return err
	}
	s.notifyCalendarChange(ctx, master.CalendarID)
	return nil
}

// appendUntil appends or replaces UNTIL in an RRULE string.
func appendUntil(ruleStr string, until time.Time) string {
	untilStr := "UNTIL=" + until.UTC().Format("20060102T150405Z")
	parts := strings.Split(ruleStr, ";")
	for i, p := range parts {
		if strings.HasPrefix(p, "UNTIL=") || strings.HasPrefix(p, "COUNT=") {
			parts[i] = untilStr
			return strings.Join(parts, ";")
		}
	}
	return ruleStr + ";" + untilStr
}

// GetException loads a single occurrence exception by master ID and recurrence timestamp.
func (s *EventService) GetException(ctx context.Context, masterID string, recurrenceID time.Time) (*repo.EventException, error) {
	return s.store.GetEventException(ctx, masterID, recurrenceID)
}

// LoadExceptions loads all exceptions for a recurring event master.
func (s *EventService) LoadExceptions(ctx context.Context, masterID string) ([]*repo.EventException, error) {
	return s.store.ListEventExceptions(ctx, masterID)
}

// SetExdatesRdates updates the exdates and rdates fields on a master event directly.
// Used by CalDAV PutEvent to persist ICS-level EXDATE/RDATE lines.
func (s *EventService) SetExdatesRdates(ctx context.Context, eventID, exdates, rdates string) error {
	e, err := s.store.GetEventByID(ctx, eventID)
	if err != nil || e == nil {
		return ErrEventNotFound
	}
	e.Exdates = exdates
	e.Rdates = rdates
	if err := s.store.UpdateEvent(ctx, e); err != nil {
		return err
	}
	s.notifyCalendarChange(ctx, e.CalendarID)
	return nil
}

// UpsertCalDAVException stores or updates a per-occurrence exception received via CalDAV PUT.
func (s *EventService) UpsertCalDAVException(ctx context.Context, userID, masterID string, recurrenceID time.Time, in EventInput, isDeleted bool) error {
	ex := &repo.EventException{
		ID:            uuid.New().String(),
		MasterEventID: masterID,
		RecurrenceID:  recurrenceID,
		Title:         in.Title,
		Description:   in.Description,
		Location:      in.Location,
		StartAt:       in.StartAt,
		EndAt:         in.EndAt,
		Timezone:      in.Timezone,
		AllDay:        in.AllDay,
		Status:        "confirmed",
		IsDeleted:     isDeleted,
		ETag:          uuid.New().String(),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	if isDeleted {
		ex.Status = "cancelled"
	}
	if err := s.store.UpsertEventException(ctx, ex); err != nil {
		return err
	}
	s.notifyEventChange(ctx, masterID)
	return nil
}

// DeleteThisAndFuture truncates a recurring series at recurrenceID by setting UNTIL.
// All occurrences from recurrenceID onward are removed without creating a new series.
func (s *EventService) DeleteThisAndFuture(ctx context.Context, userID, masterID string, recurrenceID time.Time) error {
	master, err := s.store.GetEventByID(ctx, masterID)
	if err != nil || master == nil {
		return ErrEventNotFound
	}
	if master.Ownership == "imported_readonly" {
		return ErrEventReadOnly
	}
	master.RRule = appendUntil(master.RRule, recurrenceID.Add(-time.Second).UTC())
	master.ETag = uuid.New().String()
	master.UpdatedBy = &userID
	if err := s.store.UpdateEvent(ctx, master); err != nil {
		return err
	}
	s.notifyCalendarChange(ctx, master.CalendarID)
	return nil
}

// Delete soft-deletes a native event (all occurrences).
func (s *EventService) Delete(ctx context.Context, userID, eventID string) error {
	e, err := s.store.GetEventByID(ctx, eventID)
	if err != nil {
		return fmt.Errorf("get event: %w", err)
	}
	if e == nil {
		return ErrEventNotFound
	}
	if e.Ownership == "imported_readonly" {
		return ErrEventReadOnly
	}
	if err := s.store.DeleteEvent(ctx, eventID); err != nil {
		return err
	}
	s.notifyCalendarChange(ctx, e.CalendarID)
	return nil
}

// EventInput holds the editable fields for create/update.
type EventInput struct {
	ID          string // optional; if empty a UUID is generated
	Title       string
	Description string
	Location    string
	StartAt     time.Time
	EndAt       time.Time
	Timezone    string
	AllDay      bool
	RRule       string // RRULE string without the "RRULE:" prefix; empty = not recurring
	Ownership   string // defaults to "native" if empty
}
