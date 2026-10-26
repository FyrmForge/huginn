package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/FyrmForge/huginn/internal/repo"
)

// PushSyncService applies full calendar snapshots pushed by a device agent
// (e.g. cmd/huginn-sync-mac). One-way: the device is the source of truth,
// events land as imported_readonly, and anything missing from a snapshot is
// pruned from the mirrored calendar.
type PushSyncService struct {
	store repo.Store
	event *EventService
}

func NewPushSyncService(store repo.Store, event *EventService) *PushSyncService {
	return &PushSyncService{store: store, event: event}
}

// Snapshot is the wire format for one device calendar.
type Snapshot struct {
	Name   string          `json:"name"`
	Color  string          `json:"color"`
	Events []SnapshotEvent `json:"events"`
}

type SnapshotEvent struct {
	UID         string    `json:"uid"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Location    string    `json:"location"`
	StartAt     time.Time `json:"start"`
	EndAt       time.Time `json:"end"`
	AllDay      bool      `json:"all_day"`
}

// SyncResult reports what a snapshot changed.
type SyncResult struct {
	CalendarID string `json:"calendar_id"`
	Created    int    `json:"created"`
	Updated    int    `json:"updated"`
	Deleted    int    `json:"deleted"`
	Skipped    int    `json:"skipped"`
}

// ApplySnapshot upserts the snapshot's events by UID into the calendar
// identified by (userID, externalID), creating the calendar on first sight,
// and soft-deletes events no longer present in the snapshot.
func (s *PushSyncService) ApplySnapshot(ctx context.Context, userID, externalID string, snap Snapshot) (*SyncResult, error) {
	cal, err := s.ensureCalendar(ctx, userID, externalID, snap)
	if err != nil {
		return nil, err
	}
	res := &SyncResult{CalendarID: cal.ID}
	now := time.Now()
	keep := make([]string, 0, len(snap.Events))

	for _, in := range snap.Events {
		if in.UID == "" || in.StartAt.IsZero() {
			res.Skipped++
			continue
		}
		end := in.EndAt
		if end.IsZero() {
			end = in.StartAt.Add(time.Hour)
		}
		keep = append(keep, in.UID)

		existing, err := s.store.GetEventByUID(ctx, cal.ID, in.UID)
		if err != nil {
			return res, fmt.Errorf("lookup %q: %w", in.UID, err)
		}
		if existing == nil {
			e := &repo.Event{
				ID: uuid.New().String(), CalendarID: cal.ID, UID: in.UID,
				Title: in.Title, Description: in.Description, Location: in.Location,
				StartAt: in.StartAt, EndAt: end, Timezone: "UTC", AllDay: in.AllDay,
				Status: "confirmed", Visibility: "private", BusyStatus: "busy",
				ETag: uuid.New().String(), Ownership: "imported_readonly",
				CreatedBy: &userID, UpdatedBy: &userID, CreatedAt: now, UpdatedAt: now,
			}
			if err := s.store.CreateEvent(ctx, e); err != nil {
				return res, fmt.Errorf("create %q: %w", in.UID, err)
			}
			res.Created++
			continue
		}
		if existing.Title == in.Title && existing.Description == in.Description &&
			existing.Location == in.Location && existing.AllDay == in.AllDay &&
			existing.StartAt.Equal(in.StartAt) && existing.EndAt.Equal(end) {
			continue // unchanged
		}
		existing.Title, existing.Description, existing.Location = in.Title, in.Description, in.Location
		existing.StartAt, existing.EndAt, existing.AllDay = in.StartAt, end, in.AllDay
		existing.ETag = uuid.New().String()
		existing.UpdatedBy = &userID
		if err := s.store.UpdateEvent(ctx, existing); err != nil {
			return res, fmt.Errorf("update %q: %w", in.UID, err)
		}
		res.Updated++
	}

	// ponytail: full-snapshot prune; add a `since` window on the agent if a
	// calendar ever grows past tens of thousands of events.
	deleted, err := s.store.DeleteEventsNotIn(ctx, cal.ID, keep)
	if err != nil {
		return res, fmt.Errorf("prune: %w", err)
	}
	res.Deleted = deleted

	if res.Created+res.Updated+res.Deleted > 0 {
		s.event.NotifyCalendar(ctx, cal.ID)
	}
	return res, nil
}

func (s *PushSyncService) ensureCalendar(ctx context.Context, userID, externalID string, snap Snapshot) (*repo.Calendar, error) {
	cal, err := s.store.GetCalendarByExternalID(ctx, userID, externalID)
	if err != nil {
		return nil, fmt.Errorf("lookup calendar: %w", err)
	}
	name := snap.Name
	if name == "" {
		name = externalID
	}
	color := snap.Color
	if color == "" {
		color = "#f5a623"
	}
	if cal != nil {
		if cal.Name != name || cal.Color != color {
			cal.Name, cal.Color = name, color
			if err := s.store.UpdateCalendar(ctx, cal); err != nil {
				return nil, fmt.Errorf("rename calendar: %w", err)
			}
		}
		return cal, nil
	}
	now := time.Now()
	cal = &repo.Calendar{
		ID: uuid.New().String(), OwnerID: userID, Name: name, Color: color,
		Timezone: "UTC", DefaultVisibility: "private", DefaultBusyStatus: "busy",
		ExternalID: externalID, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.store.CreateCalendar(ctx, cal); err != nil {
		return nil, fmt.Errorf("create calendar: %w", err)
	}
	return cal, nil
}
