package service

import (
	"context"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/FyrmForge/huginn/internal/repo"
)

// RoutingService manages routing rules and applies them to events.
type RoutingService struct {
	store repo.Store
}

func NewRoutingService(store repo.Store) *RoutingService {
	return &RoutingService{store: store}
}

// RuleInput is the editable fields for a routing rule.
type RuleInput struct {
	Name             string
	RuleType         string
	MatchValue       string
	CaseSensitive    bool
	TargetCalendarID string
	Priority         int
}

func (s *RoutingService) Create(ctx context.Context, userID string, in RuleInput) (*repo.RoutingRule, error) {
	now := time.Now()
	r := &repo.RoutingRule{
		ID:               uuid.New().String(),
		UserID:           userID,
		Name:             in.Name,
		RuleType:         in.RuleType,
		MatchValue:       in.MatchValue,
		CaseSensitive:    in.CaseSensitive,
		TargetCalendarID: in.TargetCalendarID,
		Priority:         in.Priority,
		Enabled:          true,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := s.store.CreateRoutingRule(ctx, r); err != nil {
		return nil, err
	}
	return r, nil
}

func (s *RoutingService) ListForUser(ctx context.Context, userID string) ([]*repo.RoutingRule, error) {
	return s.store.ListRoutingRulesByUser(ctx, userID)
}

func (s *RoutingService) Update(ctx context.Context, userID, ruleID string, in RuleInput) error {
	r, err := s.store.GetRoutingRuleByID(ctx, ruleID)
	if err != nil || r == nil || r.UserID != userID {
		return fmt.Errorf("rule not found")
	}
	r.Name             = in.Name
	r.RuleType         = in.RuleType
	r.MatchValue       = in.MatchValue
	r.CaseSensitive    = in.CaseSensitive
	r.TargetCalendarID = in.TargetCalendarID
	r.Priority         = in.Priority
	return s.store.UpdateRoutingRule(ctx, r)
}

func (s *RoutingService) Delete(ctx context.Context, userID, ruleID string) error {
	r, err := s.store.GetRoutingRuleByID(ctx, ruleID)
	if err != nil || r == nil || r.UserID != userID {
		return fmt.Errorf("rule not found")
	}
	return s.store.DeleteRoutingRule(ctx, ruleID)
}

// Route applies enabled rules to an event. Returns the matched target calendar
// ID (or "" if no rule matched) and records an audit entry.
func (s *RoutingService) Route(ctx context.Context, userID string, event *repo.Event) (string, error) {
	rules, err := s.store.ListRoutingRulesByUser(ctx, userID)
	if err != nil {
		return "", err
	}

	for _, r := range rules {
		if !r.Enabled {
			continue
		}
		matched, reason := matchRule(r, event)
		if !matched {
			continue
		}
		id := r.ID
		audit := &repo.RoutingAudit{
			ID:       uuid.New().String(),
			EventID:  event.ID,
			RuleID:   &id,
			Reason:   reason,
			RoutedAt: time.Now(),
		}
		_ = s.store.CreateRoutingAudit(ctx, audit)
		return r.TargetCalendarID, nil
	}

	// No rule matched — record unrouted audit.
	audit := &repo.RoutingAudit{
		ID:       uuid.New().String(),
		EventID:  event.ID,
		RuleID:   nil,
		Reason:   "no rule matched",
		RoutedAt: time.Now(),
	}
	_ = s.store.CreateRoutingAudit(ctx, audit)
	return "", nil
}

// matchRule returns true and a reason string if the rule matches the event.
func matchRule(r *repo.RoutingRule, e *repo.Event) (bool, string) {
	switch r.RuleType {
	case "source_domain":
		domain := extractDomain(e.Description + " " + e.Location)
		if domain == "" {
			return false, ""
		}
		if strings.EqualFold(domain, r.MatchValue) {
			return true, fmt.Sprintf("source_domain matched %q in event text", r.MatchValue)
		}

	case "keyword":
		haystack := e.Title + " " + e.Description
		needle := r.MatchValue
		if !r.CaseSensitive {
			haystack = strings.ToLower(haystack)
			needle   = strings.ToLower(needle)
		}
		if strings.Contains(haystack, needle) {
			return true, fmt.Sprintf("keyword %q matched in title/description", r.MatchValue)
		}

	case "source_calendar":
		// Match the calendar name (for external imports the calendar name carries the provider label).
		if strings.EqualFold(e.CalendarID, r.MatchValue) {
			return true, fmt.Sprintf("source_calendar matched %q", r.MatchValue)
		}
	}
	return false, ""
}

// extractDomain pulls the first recognizable email domain from a string.
func extractDomain(s string) string {
	for _, word := range strings.Fields(s) {
		addr, err := mail.ParseAddress(word)
		if err == nil {
			parts := strings.SplitN(addr.Address, "@", 2)
			if len(parts) == 2 {
				return parts[1]
			}
		}
		if strings.Contains(word, "@") {
			parts := strings.SplitN(word, "@", 2)
			if len(parts) == 2 && strings.Contains(parts[1], ".") {
				return parts[1]
			}
		}
	}
	return ""
}
