package service

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"

	"github.com/FyrmForge/huginn/internal/ics"
	"github.com/FyrmForge/huginn/internal/repo"
)

type ImportExportService struct {
	store repo.Store
}

func NewImportExportService(store repo.Store) *ImportExportService {
	return &ImportExportService{store: store}
}

// ImportICS parses an ICS stream and inserts events into calendarID.
// Returns the number of events imported.
func (s *ImportExportService) ImportICS(ctx context.Context, userID, calendarID string, r io.Reader) (int, error) {
	parsed, err := ics.Parse(r)
	if err != nil {
		return 0, fmt.Errorf("parse ics: %w", err)
	}

	now := time.Now()
	count := 0
	for _, p := range parsed {
		if p.StartAt.IsZero() {
			continue // skip malformed events
		}
		endAt := p.EndAt
		if endAt.IsZero() {
			endAt = p.StartAt.Add(time.Hour)
		}
		status := "confirmed"
		if p.Status != "" {
			status = p.Status
		}
		uid := p.UID
		if uid == "" {
			uid = uuid.New().String() + "@huginn-import"
		}
		e := &repo.Event{
			ID:          uuid.New().String(),
			CalendarID:  calendarID,
			UID:         uid,
			Title:       p.Summary,
			Description: p.Description,
			Location:    p.Location,
			StartAt:     p.StartAt,
			EndAt:       endAt,
			Timezone:    "UTC",
			AllDay:      p.AllDay,
			Status:      status,
			Visibility:  "private",
			BusyStatus:  "busy",
			RawICS:      p.RawBlock,
			Ownership:   "native",
			CreatedBy:   &userID,
			UpdatedBy:   &userID,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := s.store.CreateEvent(ctx, e); err != nil {
			return count, fmt.Errorf("insert event %q: %w", uid, err)
		}
		count++
	}
	return count, nil
}

// ExportICS writes events from calendarID as an ICS stream.
func (s *ImportExportService) ExportICS(ctx context.Context, calendarID string, w io.Writer) error {
	cal, err := s.store.GetCalendarByID(ctx, calendarID)
	if err != nil || cal == nil {
		return fmt.Errorf("calendar not found")
	}
	from := time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)
	events, err := s.store.ListEvents(ctx, calendarID, from, to)
	if err != nil {
		return fmt.Errorf("list events: %w", err)
	}
	return ics.Write(w, cal.Name, events)
}
