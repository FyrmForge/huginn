package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/FyrmForge/huginn/internal/repo"
)

func (s *Store) GetOAuthProviderConfig(ctx context.Context, provider string) (*repo.OAuthProviderConfig, error) {
	var c repo.OAuthProviderConfig
	err := s.db.GetContext(ctx, &c,
		`SELECT id, provider, client_id, client_secret, created_at, updated_at
		 FROM oauth_provider_configs WHERE provider = $1`, provider)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &c, nil
}

func (s *Store) UpsertOAuthProviderConfig(ctx context.Context, cfg *repo.OAuthProviderConfig) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO oauth_provider_configs (id, provider, client_id, client_secret, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,NOW(),NOW())
		 ON CONFLICT (provider) DO UPDATE SET
		    client_id = EXCLUDED.client_id,
		    client_secret = EXCLUDED.client_secret,
		    updated_at = NOW()`,
		cfg.ID, cfg.Provider, cfg.ClientID, cfg.ClientSecret,
	)
	return err
}

func (s *Store) CreateSyncConnection(ctx context.Context, sc *repo.SyncConnection) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sync_connections
		    (id, user_id, provider, external_email, status, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		sc.ID, sc.UserID, sc.Provider, sc.ExternalEmail, sc.Status, sc.CreatedAt, sc.UpdatedAt,
	)
	return err
}

func (s *Store) GetSyncConnectionByID(ctx context.Context, id string) (*repo.SyncConnection, error) {
	var sc repo.SyncConnection
	err := s.db.GetContext(ctx, &sc,
		`SELECT id, user_id, provider, external_email, status, last_synced_at, last_error, created_at, updated_at
		 FROM sync_connections WHERE id = $1`, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &sc, nil
}

func (s *Store) ListSyncConnectionsByUser(ctx context.Context, userID string) ([]*repo.SyncConnection, error) {
	var scs []*repo.SyncConnection
	err := s.db.SelectContext(ctx, &scs,
		`SELECT id, user_id, provider, external_email, status, last_synced_at, last_error, created_at, updated_at
		 FROM sync_connections WHERE user_id = $1 ORDER BY created_at`, userID)
	return scs, err
}

func (s *Store) UpdateSyncConnection(ctx context.Context, sc *repo.SyncConnection) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE sync_connections SET
		    status = $1, last_synced_at = $2, last_error = $3, updated_at = NOW()
		 WHERE id = $4`,
		sc.Status, sc.LastSyncedAt, sc.LastError, sc.ID,
	)
	return err
}

func (s *Store) DeleteSyncConnection(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sync_connections WHERE id = $1`, id)
	return err
}

func (s *Store) UpsertSyncConnectionToken(ctx context.Context, tok *repo.SyncConnectionToken) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sync_connection_tokens (connection_id, access_token, refresh_token, expires_at, scope, updated_at)
		 VALUES ($1,$2,$3,$4,$5,NOW())
		 ON CONFLICT (connection_id) DO UPDATE SET
		    access_token = EXCLUDED.access_token,
		    refresh_token = EXCLUDED.refresh_token,
		    expires_at = EXCLUDED.expires_at,
		    scope = EXCLUDED.scope,
		    updated_at = NOW()`,
		tok.ConnectionID, tok.AccessToken, tok.RefreshToken, tok.ExpiresAt, tok.Scope,
	)
	return err
}

func (s *Store) GetSyncConnectionToken(ctx context.Context, connectionID string) (*repo.SyncConnectionToken, error) {
	var tok repo.SyncConnectionToken
	err := s.db.GetContext(ctx, &tok,
		`SELECT connection_id, access_token, refresh_token, expires_at, scope, updated_at
		 FROM sync_connection_tokens WHERE connection_id = $1`, connectionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &tok, nil
}
