package calendar

import (
	"strings"
	"testing"
	"time"

	"github.com/FyrmForge/huginn/internal/repo"
	"github.com/FyrmForge/huginn/internal/service"
)

func TestOccDayRangeExclusiveEnd(t *testing.T) {
	d := func(day, h int) time.Time { return time.Date(2026, 10, day, h, 0, 0, 0, time.UTC) }
	cases := []struct {
		name       string
		start, end time.Time
		wantEndDay int
	}{
		{"all-day one day", d(19, 0), d(20, 0), 19},
		{"all-day two days", d(19, 0), d(21, 0), 20},
		{"legacy start==end", d(19, 0), d(19, 0), 19},
		{"timed to midnight", d(19, 22), d(20, 0), 19},
		{"timed past midnight", d(19, 22), d(20, 1), 20},
	}
	for _, c := range cases {
		s, e := occDayRange(service.Occurrence{Start: c.start, End: c.end})
		if s.Day() != 19 || e.Day() != c.wantEndDay {
			t.Errorf("%s: got %d..%d, want 19..%d", c.name, s.Day(), e.Day(), c.wantEndDay)
		}
	}
}

func TestCalendarColorCSSRejectsInjection(t *testing.T) {
	css := calendarColorCSS([]*repo.Calendar{
		{ID: "ok-1", Color: "#fff}</style><script>alert(1)</script>"},
		{ID: `x"]}</style><script>`, Color: "#123456"},
		{ID: "ok-2", Color: "#4f8ef7"},
	})
	if strings.ContainsAny(css, "<>") || strings.Contains(css, "script") {
		t.Fatalf("unsafe CSS emitted: %s", css)
	}
	if !strings.Contains(css, "#888888") || !strings.Contains(css, "#4f8ef7") {
		t.Fatalf("expected fallback and valid colour: %s", css)
	}
}
