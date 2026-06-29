package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/FyrmForge/huginn/internal/repo"
)

func (s *Store) CreateAppPassword(ctx context.Context, ap *repo.AppPassword) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO app_passwords (id, user_id, name, token_hash, permissions, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6)`,
		ap.ID, ap.UserID, ap.Name, ap.TokenHash, ap.Permissions, ap.CreatedAt,
	)
	return err
}

func (s *Store) GetAppPasswordByHash(ctx context.Context, hash string) (*repo.AppPassword, error) {
	var ap repo.AppPassword
	err := s.db.GetContext(ctx, &ap,
		`SELECT id, user_id, name, token_hash, permissions, last_used_at, revoked_at, created_at
		 FROM app_passwords WHERE token_hash = $1 AND revoked_at IS NULL`, hash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &ap, nil
}

func (s *Store) ListAppPasswords(ctx context.Context, userID string) ([]*repo.AppPassword, error) {
	var aps []*repo.AppPassword
	err := s.db.SelectContext(ctx, &aps,
		`SELECT id, user_id, name, token_hash, permissions, last_used_at, revoked_at, created_at
		 FROM app_passwords WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	return aps, err
}

func (s *Store) RevokeAppPassword(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE app_passwords SET revoked_at = NOW() WHERE id = $1`, id)
	return err
}

func (s *Store) TouchAppPassword(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE app_passwords SET last_used_at = NOW() WHERE id = $1`, id)
	return err
}
