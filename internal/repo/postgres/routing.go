package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/FyrmForge/huginn/internal/repo"
)

func (s *Store) CreateRoutingRule(ctx context.Context, r *repo.RoutingRule) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO routing_rules
		    (id, user_id, name, rule_type, match_value, case_sensitive, target_calendar_id, priority, enabled, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		r.ID, r.UserID, r.Name, r.RuleType, r.MatchValue, r.CaseSensitive,
		r.TargetCalendarID, r.Priority, r.Enabled, r.CreatedAt, r.UpdatedAt,
	)
	return err
}

func (s *Store) GetRoutingRuleByID(ctx context.Context, id string) (*repo.RoutingRule, error) {
	var r repo.RoutingRule
	err := s.db.GetContext(ctx, &r,
		`SELECT id, user_id, name, rule_type, match_value, case_sensitive, target_calendar_id, priority, enabled, created_at, updated_at
		 FROM routing_rules WHERE id = $1`, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &r, nil
}

func (s *Store) ListRoutingRulesByUser(ctx context.Context, userID string) ([]*repo.RoutingRule, error) {
	var rules []*repo.RoutingRule
	err := s.db.SelectContext(ctx, &rules,
		`SELECT id, user_id, name, rule_type, match_value, case_sensitive, target_calendar_id, priority, enabled, created_at, updated_at
		 FROM routing_rules WHERE user_id = $1 ORDER BY priority, created_at`, userID)
	return rules, err
}

func (s *Store) UpdateRoutingRule(ctx context.Context, r *repo.RoutingRule) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE routing_rules SET
		    name = $1, rule_type = $2, match_value = $3, case_sensitive = $4,
		    target_calendar_id = $5, priority = $6, enabled = $7, updated_at = NOW()
		 WHERE id = $8`,
		r.Name, r.RuleType, r.MatchValue, r.CaseSensitive,
		r.TargetCalendarID, r.Priority, r.Enabled, r.ID,
	)
	return err
}

func (s *Store) DeleteRoutingRule(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM routing_rules WHERE id = $1`, id)
	return err
}

func (s *Store) CreateRoutingAudit(ctx context.Context, a *repo.RoutingAudit) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO routing_audit (id, event_id, rule_id, reason, routed_at)
		 VALUES ($1,$2,$3,$4,$5)`,
		a.ID, a.EventID, a.RuleID, a.Reason, a.RoutedAt,
	)
	return err
}
