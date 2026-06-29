package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/FyrmForge/huginn/internal/repo"
)

func (s *Store) GetUserByID(ctx context.Context, id string) (*repo.User, error) {
	var u repo.User
	err := s.db.GetContext(ctx, &u,
		`SELECT id, email, password_hash, name, role, avatar_url, active, created_at, updated_at
		 FROM users WHERE id = $1`, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (*repo.User, error) {
	var u repo.User
	err := s.db.GetContext(ctx, &u,
		`SELECT id, email, password_hash, name, role, avatar_url, active, created_at, updated_at
		 FROM users WHERE email = $1`, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

func (s *Store) CreateUser(ctx context.Context, user *repo.User) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO users (id, email, password_hash, name, role, avatar_url, active, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		user.ID, user.Email, user.PasswordHash, user.Name, user.Role, user.AvatarURL,
		user.Active, user.CreatedAt, user.UpdatedAt,
	)
	return err
}

func (s *Store) ListAllUsers(ctx context.Context) ([]*repo.User, error) {
	var users []*repo.User
	err := s.db.SelectContext(ctx, &users,
		`SELECT id, email, password_hash, name, role, avatar_url, active, created_at, updated_at
		 FROM users ORDER BY created_at DESC`)
	return users, err
}

func (s *Store) AdminStats(ctx context.Context) (*repo.Stats, error) {
	var st repo.Stats
	err := s.db.QueryRowContext(ctx, `
		SELECT
			(SELECT COUNT(*) FROM users)                              AS total_users,
			(SELECT COUNT(*) FROM users WHERE role = 'admin')        AS admin_users,
			(SELECT COUNT(*) FROM calendars)                         AS total_calendars,
			(SELECT COUNT(*) FROM events)                            AS total_events,
			(SELECT COUNT(*) FROM sync_connections)                  AS total_connections,
			(SELECT COUNT(*) FROM app_passwords WHERE revoked_at IS NULL) AS total_devices
	`).Scan(&st.TotalUsers, &st.AdminUsers, &st.TotalCalendars, &st.TotalEvents, &st.TotalConnections, &st.TotalDevices)
	return &st, err
}

func (s *Store) UpdateUser(ctx context.Context, user *repo.User) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET name=$1, role=$2, avatar_url=$3, updated_at=$4 WHERE id=$5`,
		user.Name, user.Role, user.AvatarURL, user.UpdatedAt, user.ID,
	)
	return err
}
