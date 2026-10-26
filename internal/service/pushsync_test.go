package service

import (
	"context"
	"testing"
	"time"

	"github.com/FyrmForge/huginn/internal/repo"
)

// fakeStore covers only what ApplySnapshot touches; everything else panics via the nil embed.
type fakeStore struct {
	repo.Store
	cals   map[string]*repo.Calendar // by external id
	events map[string]*repo.Event    // by uid
}

func (f *fakeStore) GetCalendarByExternalID(_ context.Context, _, ext string) (*repo.Calendar, error) {
	return f.cals[ext], nil
}
func (f *fakeStore) CreateCalendar(_ context.Context, c *repo.Calendar) error {
	f.cals[c.ExternalID] = c
	return nil
}
func (f *fakeStore) UpdateCalendar(_ context.Context, _ *repo.Calendar) error { return nil }
func (f *fakeStore) GetEventByUID(_ context.Context, _, uid string) (*repo.Event, error) {
	return f.events[uid], nil
}
func (f *fakeStore) CreateEvent(_ context.Context, e *repo.Event) error {
	f.events[e.UID] = e
	return nil
}
func (f *fakeStore) UpdateEvent(_ context.Context, e *repo.Event) error {
	f.events[e.UID] = e
	return nil
}
func (f *fakeStore) DeleteEventsNotIn(_ context.Context, _ string, keep []string) (int, error) {
	k := map[string]bool{}
	for _, u := range keep {
		k[u] = true
	}
	n := 0
	for uid := range f.events {
		if !k[uid] {
			delete(f.events, uid)
			n++
		}
	}
	return n, nil
}

func TestApplySnapshot_CreateUpdatePrune(t *testing.T) {
	fs := &fakeStore{cals: map[string]*repo.Calendar{}, events: map[string]*repo.Event{}}
	svc := NewPushSyncService(fs, NewEventService(fs))
	ctx := context.Background()
	t0 := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)

	snap := Snapshot{Name: "Work", Events: []SnapshotEvent{
		{UID: "a", Title: "A", StartAt: t0, EndAt: t0.Add(time.Hour)},
		{UID: "b", Title: "B", StartAt: t0},
		{UID: "", Title: "no uid", StartAt: t0},
	}}
	res, err := svc.ApplySnapshot(ctx, "u1", "ext-1", snap)
	if err != nil || res.Created != 2 || res.Skipped != 1 || res.Deleted != 0 {
		t.Fatalf("first sync: %+v err=%v", res, err)
	}
	if fs.events["b"].EndAt != t0.Add(time.Hour) || fs.events["a"].Ownership != "imported_readonly" {
		t.Fatalf("defaults not applied: %+v", fs.events["b"])
	}

	snap.Events = []SnapshotEvent{{UID: "a", Title: "A renamed", StartAt: t0, EndAt: t0.Add(time.Hour)}}
	res, err = svc.ApplySnapshot(ctx, "u1", "ext-1", snap)
	if err != nil || res.Created != 0 || res.Updated != 1 || res.Deleted != 1 {
		t.Fatalf("second sync: %+v err=%v", res, err)
	}
	if fs.events["a"].Title != "A renamed" || fs.events["b"] != nil || len(fs.cals) != 1 {
		t.Fatalf("state after second sync wrong")
	}

	res, _ = svc.ApplySnapshot(ctx, "u1", "ext-1", snap)
	if res.Created+res.Updated+res.Deleted != 0 {
		t.Fatalf("idempotent sync changed things: %+v", res)
	}
}
