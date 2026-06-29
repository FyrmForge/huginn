package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/FyrmForge/huginn/internal/repo"
)

func (s *Store) CreateCalendar(ctx context.Context, cal *repo.Calendar) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO calendars
		    (id, owner_id, name, description, color, timezone,
		     default_visibility, default_busy_status, is_default, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		cal.ID, cal.OwnerID, cal.Name, cal.Description, cal.Color, cal.Timezone,
		cal.DefaultVisibility, cal.DefaultBusyStatus, cal.IsDefault,
		cal.CreatedAt, cal.UpdatedAt,
	)
	return err
}

func (s *Store) GetCalendarByID(ctx context.Context, id string) (*repo.Calendar, error) {
	var c repo.Calendar
	err := s.db.GetContext(ctx, &c,
		`SELECT id, owner_id, name, description, color, timezone,
		        default_visibility, default_busy_status, is_default,
		        deleted_at, created_at, updated_at
		 FROM calendars WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &c, nil
}

func (s *Store) ListCalendarsByUser(ctx context.Context, userID string) ([]*repo.Calendar, error) {
	var cals []*repo.Calendar
	err := s.db.SelectContext(ctx, &cals,
		`SELECT c.id, c.owner_id, c.name, c.description, c.color, c.timezone,
		        c.default_visibility, c.default_busy_status, c.is_default,
		        c.deleted_at, c.created_at, c.updated_at
		 FROM calendars c
		 LEFT JOIN calendar_members cm ON cm.calendar_id = c.id AND cm.user_id = $1
		 WHERE c.deleted_at IS NULL
		   AND (c.owner_id = $1 OR cm.user_id = $1)
		 ORDER BY c.is_default DESC, c.name`, userID)
	return cals, err
}

func (s *Store) UpdateCalendar(ctx context.Context, cal *repo.Calendar) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE calendars SET
		    name = $1, description = $2, color = $3, timezone = $4,
		    default_visibility = $5, default_busy_status = $6, updated_at = NOW()
		 WHERE id = $7`,
		cal.Name, cal.Description, cal.Color, cal.Timezone,
		cal.DefaultVisibility, cal.DefaultBusyStatus, cal.ID,
	)
	return err
}

func (s *Store) DeleteCalendar(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE calendars SET deleted_at = NOW(), updated_at = NOW() WHERE id = $1`, id)
	return err
}

func (s *Store) AddCalendarMember(ctx context.Context, m *repo.CalendarMember) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO calendar_members (calendar_id, user_id, role, created_at)
		 VALUES ($1,$2,$3,$4)
		 ON CONFLICT (calendar_id, user_id) DO UPDATE SET role = EXCLUDED.role`,
		m.CalendarID, m.UserID, m.Role, m.CreatedAt,
	)
	return err
}

func (s *Store) RemoveCalendarMember(ctx context.Context, calendarID, userID string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM calendar_members WHERE calendar_id = $1 AND user_id = $2`,
		calendarID, userID,
	)
	return err
}

func (s *Store) ListCalendarMembersWithUsers(ctx context.Context, calendarID string) ([]*repo.CalendarMemberInfo, error) {
	var out []*repo.CalendarMemberInfo
	err := s.db.SelectContext(ctx, &out, `
		SELECT cm.calendar_id, cm.user_id, cm.role, cm.created_at,
		       u.email, u.name, u.avatar_url
		FROM calendar_members cm
		JOIN users u ON u.id = cm.user_id
		WHERE cm.calendar_id = $1
		ORDER BY cm.created_at`, calendarID)
	return out, err
}

func (s *Store) ListCalendarMembers(ctx context.Context, calendarID string) ([]*repo.CalendarMember, error) {
	var members []*repo.CalendarMember
	err := s.db.SelectContext(ctx, &members,
		`SELECT calendar_id, user_id, role, created_at
		 FROM calendar_members WHERE calendar_id = $1`, calendarID)
	return members, err
}

func (s *Store) GetCalendarMember(ctx context.Context, calendarID, userID string) (*repo.CalendarMember, error) {
	var m repo.CalendarMember
	err := s.db.GetContext(ctx, &m,
		`SELECT calendar_id, user_id, role, created_at
		 FROM calendar_members WHERE calendar_id = $1 AND user_id = $2`,
		calendarID, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &m, nil
}
