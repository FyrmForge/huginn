package service

import (
	"context"
	"fmt"

	"github.com/FyrmForge/huginn/internal/repo"
)

type SettingsService struct {
	store repo.Store
}

func NewSettingsService(store repo.Store) *SettingsService {
	return &SettingsService{store: store}
}

// GetOrDefault returns the user's settings, creating defaults if none exist.
func (s *SettingsService) GetOrDefault(ctx context.Context, userID string) (*repo.UserSettings, error) {
	us, err := s.store.GetUserSettings(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user settings: %w", err)
	}
	if us != nil {
		return us, nil
	}
	// First time — return defaults without persisting; they're saved on first edit.
	return &repo.UserSettings{
		UserID:                  userID,
		Timezone:                "UTC",
		Locale:                  "en",
		DateFormat:              "YYYY-MM-DD",
		TimeFormat:              "24h",
		FirstDayOfWeek:          1,
		DefaultView:             "month",
		DefaultEventDurationMin: 60,
	}, nil
}

func (s *SettingsService) Save(ctx context.Context, us *repo.UserSettings) error {
	if err := s.store.UpsertUserSettings(ctx, us); err != nil {
		return fmt.Errorf("save user settings: %w", err)
	}
	return nil
}
