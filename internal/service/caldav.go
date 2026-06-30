package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/FyrmForge/huginn/internal/repo"
)

var ErrAppPasswordNotFound = errors.New("app password not found or revoked")

type CalDAVService struct {
	store repo.Store
}

func NewCalDAVService(store repo.Store) *CalDAVService {
	return &CalDAVService{store: store}
}

// CreateAppPassword generates a new device token, stores its hash, and
// returns the plain-text token (shown once to the user).
func (s *CalDAVService) CreateAppPassword(ctx context.Context, userID, name string) (plainToken string, ap *repo.AppPassword, err error) {
	// 16 random bytes (128-bit) as base64url → 22 chars. Shorter than hex(32)=64
	// while still cryptographically strong. Existing tokens stay valid (lookup is
	// by sha256 of whatever the client sends, encoding-agnostic).
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("generate token: %w", err)
	}
	plain := base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256Hex(plain)

	now := time.Now()
	ap = &repo.AppPassword{
		ID:          uuid.New().String(),
		UserID:      userID,
		Name:        name,
		TokenHash:   hash,
		Permissions: "caldav",
		CreatedAt:   now,
	}
	if err := s.store.CreateAppPassword(ctx, ap); err != nil {
		return "", nil, fmt.Errorf("save app password: %w", err)
	}
	return plain, ap, nil
}

// Authenticate validates a plain token, updates last_used_at, and returns the
// owning user. Used by CalDAV Basic Auth middleware.
func (s *CalDAVService) Authenticate(ctx context.Context, plain string) (*repo.User, error) {
	hash := sha256Hex(plain)
	ap, err := s.store.GetAppPasswordByHash(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("lookup token: %w", err)
	}
	if ap == nil {
		return nil, ErrAppPasswordNotFound
	}
	_ = s.store.TouchAppPassword(ctx, ap.ID)
	user, err := s.store.GetUserByID(ctx, ap.UserID)
	if err != nil || user == nil {
		return nil, fmt.Errorf("user not found for token")
	}
	return user, nil
}

// ListAppPasswords returns all app passwords for a user.
func (s *CalDAVService) ListAppPasswords(ctx context.Context, userID string) ([]*repo.AppPassword, error) {
	return s.store.ListAppPasswords(ctx, userID)
}

// RevokeAppPassword revokes a token by ID.
func (s *CalDAVService) RevokeAppPassword(ctx context.Context, userID, id string) error {
	aps, err := s.store.ListAppPasswords(ctx, userID)
	if err != nil {
		return err
	}
	for _, ap := range aps {
		if ap.ID == id {
			return s.store.RevokeAppPassword(ctx, id)
		}
	}
	return ErrAppPasswordNotFound
}

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
