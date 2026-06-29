package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/FyrmForge/huginn/internal/repo"
)

func (s *Store) GetUserSettings(ctx context.Context, userID string) (*repo.UserSettings, error) {
	var us repo.UserSettings
	err := s.db.GetContext(ctx, &us,
		`SELECT user_id, timezone, locale, date_format, time_format,
		        first_day_of_week, default_view, default_event_duration_mins,
		        created_at, updated_at
		 FROM user_settings WHERE user_id = $1`, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &us, nil
}

func (s *Store) UpsertUserSettings(ctx context.Context, us *repo.UserSettings) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO user_settings
		    (user_id, timezone, locale, date_format, time_format,
		     first_day_of_week, default_view, default_event_duration_mins,
		     created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,NOW(),NOW())
		 ON CONFLICT (user_id) DO UPDATE SET
		    timezone                    = EXCLUDED.timezone,
		    locale                      = EXCLUDED.locale,
		    date_format                 = EXCLUDED.date_format,
		    time_format                 = EXCLUDED.time_format,
		    first_day_of_week           = EXCLUDED.first_day_of_week,
		    default_view                = EXCLUDED.default_view,
		    default_event_duration_mins = EXCLUDED.default_event_duration_mins,
		    updated_at                  = NOW()`,
		us.UserID, us.Timezone, us.Locale, us.DateFormat, us.TimeFormat,
		us.FirstDayOfWeek, us.DefaultView, us.DefaultEventDurationMin,
	)
	return err
}
