package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/FyrmForge/huginn/internal/repo"
	"github.com/lib/pq"
)

const eventCols = `id, calendar_id, uid, title, description, location,
        start_at, end_at, timezone, all_day, status, visibility, busy_status,
        rrule, exdates, rdates, raw_ics, etag, ownership, created_by, updated_by,
        deleted_at, created_at, updated_at`

func (s *Store) CreateEvent(ctx context.Context, e *repo.Event) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO events
		    (id, calendar_id, uid, title, description, location,
		     start_at, end_at, timezone, all_day, status, visibility, busy_status,
		     rrule, exdates, rdates, raw_ics, etag, ownership, created_by, updated_by, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23)`,
		e.ID, e.CalendarID, e.UID, e.Title, e.Description, e.Location,
		e.StartAt, e.EndAt, e.Timezone, e.AllDay, e.Status, e.Visibility, e.BusyStatus,
		e.RRule, e.Exdates, e.Rdates, e.RawICS, e.ETag, e.Ownership, e.CreatedBy, e.UpdatedBy,
		e.CreatedAt, e.UpdatedAt,
	)
	return err
}

func (s *Store) GetEventByID(ctx context.Context, id string) (*repo.Event, error) {
	var e repo.Event
	err := s.db.GetContext(ctx, &e,
		`SELECT `+eventCols+`
		 FROM events WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &e, nil
}

func (s *Store) ListEvents(ctx context.Context, calendarID string, from, to time.Time) ([]*repo.Event, error) {
	var evs []*repo.Event
	err := s.db.SelectContext(ctx, &evs,
		`SELECT `+eventCols+`
		 FROM events
		 WHERE calendar_id = $1 AND deleted_at IS NULL
		   AND (
		     (rrule = '' AND start_at < $3 AND end_at > $2)
		     OR (rrule <> '' AND start_at < $3)
		   )
		 ORDER BY start_at`,
		// ponytail: recurring masters with start_at < to are all fetched; Go-side ExpandOccurrences filters to window.
		// UNTIL-expired series still fetched but expand to nothing — cheap for typical sizes.
		calendarID, from, to)
	return evs, err
}

func (s *Store) ListEventsByCalendars(ctx context.Context, calendarIDs []string, from, to time.Time) ([]*repo.Event, error) {
	if len(calendarIDs) == 0 {
		return nil, nil
	}
	var evs []*repo.Event
	err := s.db.SelectContext(ctx, &evs,
		`SELECT `+eventCols+`
		 FROM events
		 WHERE calendar_id = ANY($1) AND deleted_at IS NULL
		   AND (
		     (rrule = '' AND start_at < $3 AND end_at > $2)
		     OR (rrule <> '' AND start_at < $3)
		   )
		 ORDER BY start_at`,
		pq.Array(calendarIDs), from, to)
	return evs, err
}

func (s *Store) UpdateEvent(ctx context.Context, e *repo.Event) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE events SET
		    calendar_id = $1, title = $2, description = $3, location = $4,
		    start_at = $5, end_at = $6, timezone = $7, all_day = $8,
		    status = $9, visibility = $10, busy_status = $11,
		    rrule = $12, exdates = $13, rdates = $14,
		    raw_ics = $15, etag = $16, updated_by = $17, updated_at = NOW()
		 WHERE id = $18`,
		e.CalendarID, e.Title, e.Description, e.Location,
		e.StartAt, e.EndAt, e.Timezone, e.AllDay,
		e.Status, e.Visibility, e.BusyStatus,
		e.RRule, e.Exdates, e.Rdates,
		e.RawICS, e.ETag, e.UpdatedBy, e.ID,
	)
	return err
}

func (s *Store) DeleteEvent(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE events SET deleted_at = NOW(), updated_at = NOW() WHERE id = $1`, id)
	return err
}

func (s *Store) GetEventByUID(ctx context.Context, calendarID, uid string) (*repo.Event, error) {
	var e repo.Event
	err := s.db.GetContext(ctx, &e,
		`SELECT `+eventCols+`
		 FROM events WHERE calendar_id = $1 AND uid = $2 AND deleted_at IS NULL`, calendarID, uid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &e, nil
}

func (s *Store) ListEventsModifiedSince(ctx context.Context, calendarID string, since time.Time) ([]*repo.Event, error) {
	var evs []*repo.Event
	err := s.db.SelectContext(ctx, &evs,
		`SELECT `+eventCols+`
		 FROM events WHERE calendar_id = $1 AND updated_at > $2
		 ORDER BY updated_at`,
		calendarID, since)
	return evs, err
}

func (s *Store) GetCalendarSyncStamp(ctx context.Context, calendarID string) (time.Time, error) {
	var t time.Time
	err := s.db.GetContext(ctx, &t,
		`SELECT COALESCE(MAX(updated_at), NOW()) FROM events WHERE calendar_id = $1`, calendarID)
	return t, err
}

// --- Event exceptions ---

func (s *Store) UpsertEventException(ctx context.Context, ex *repo.EventException) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO event_exceptions
		    (id, master_event_id, recurrence_id, title, description, location,
		     start_at, end_at, timezone, all_day, status, is_deleted, etag, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
		 ON CONFLICT (master_event_id, recurrence_id) DO UPDATE SET
		    title=$4, description=$5, location=$6,
		    start_at=$7, end_at=$8, timezone=$9, all_day=$10, status=$11,
		    is_deleted=$12, etag=$13, updated_at=NOW()`,
		ex.ID, ex.MasterEventID, ex.RecurrenceID, ex.Title, ex.Description, ex.Location,
		ex.StartAt, ex.EndAt, ex.Timezone, ex.AllDay, ex.Status, ex.IsDeleted, ex.ETag,
		ex.CreatedAt, ex.UpdatedAt,
	)
	return err
}

func (s *Store) GetEventException(ctx context.Context, masterID string, recurrenceID time.Time) (*repo.EventException, error) {
	var ex repo.EventException
	err := s.db.GetContext(ctx, &ex,
		`SELECT id, master_event_id, recurrence_id, title, description, location,
		        start_at, end_at, timezone, all_day, status, is_deleted, etag, created_at, updated_at
		 FROM event_exceptions WHERE master_event_id = $1 AND recurrence_id = $2`, masterID, recurrenceID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &ex, nil
}

func (s *Store) ListEventExceptions(ctx context.Context, masterID string) ([]*repo.EventException, error) {
	var exs []*repo.EventException
	err := s.db.SelectContext(ctx, &exs,
		`SELECT id, master_event_id, recurrence_id, title, description, location,
		        start_at, end_at, timezone, all_day, status, is_deleted, etag, created_at, updated_at
		 FROM event_exceptions WHERE master_event_id = $1 ORDER BY recurrence_id`, masterID)
	return exs, err
}

func (s *Store) DeleteEventException(ctx context.Context, masterID string, recurrenceID time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM event_exceptions WHERE master_event_id = $1 AND recurrence_id = $2`, masterID, recurrenceID)
	return err
}
