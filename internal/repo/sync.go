package repo

import "time"

// OAuthProviderConfig holds admin-configured OAuth credentials for a provider.
type OAuthProviderConfig struct {
	ID           string    `db:"id"`
	Provider     string    `db:"provider"`
	ClientID     string    `db:"client_id"`
	ClientSecret string    `db:"client_secret"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
}

// SyncConnection is a user's connected external calendar account.
type SyncConnection struct {
	ID             string     `db:"id"`
	UserID         string     `db:"user_id"`
	Provider       string     `db:"provider"`
	ExternalEmail  string     `db:"external_email"`
	Status         string     `db:"status"`
	LastSyncedAt   *time.Time `db:"last_synced_at"`
	LastError      string     `db:"last_error"`
	CreatedAt      time.Time  `db:"created_at"`
	UpdatedAt      time.Time  `db:"updated_at"`
}

// SyncConnectionToken holds OAuth tokens for a sync connection.
type SyncConnectionToken struct {
	ConnectionID string    `db:"connection_id"`
	AccessToken  string    `db:"access_token"`
	RefreshToken string    `db:"refresh_token"`
	ExpiresAt    time.Time `db:"expires_at"`
	Scope        string    `db:"scope"`
	UpdatedAt    time.Time `db:"updated_at"`
}
