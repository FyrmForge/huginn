package repo

import "time"

// RoutingRule is a user-defined rule for auto-filing imported events.
type RoutingRule struct {
	ID               string    `db:"id"`
	UserID           string    `db:"user_id"`
	Name             string    `db:"name"`
	RuleType         string    `db:"rule_type"` // source_domain, keyword, source_calendar
	MatchValue       string    `db:"match_value"`
	CaseSensitive    bool      `db:"case_sensitive"`
	TargetCalendarID string    `db:"target_calendar_id"`
	Priority         int       `db:"priority"`
	Enabled          bool      `db:"enabled"`
	CreatedAt        time.Time `db:"created_at"`
	UpdatedAt        time.Time `db:"updated_at"`
}

// RoutingAudit records which rule routed a specific event.
type RoutingAudit struct {
	ID       string    `db:"id"`
	EventID  string    `db:"event_id"`
	RuleID   *string   `db:"rule_id"`
	Reason   string    `db:"reason"`
	RoutedAt time.Time `db:"routed_at"`
}
