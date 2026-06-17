package engine

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bluegodg/anban/server/internal/mind"
	"github.com/bluegodg/anban/server/internal/store"
)

func newEngineForTest(t *testing.T) (*Service, *store.Store) {
	t.Helper()

	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	sqlDB, err := st.DB.DB()
	if err != nil {
		t.Fatalf("DB: %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	ms := mind.NewStore(st.DB)
	if err := ms.AutoMigrate(); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	return New(ms), st
}

func countRows(t *testing.T, st *store.Store, table string) int64 {
	t.Helper()

	var count int64
	if err := st.DB.Raw("SELECT COUNT(*) FROM " + table).Scan(&count).Error; err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return count
}

func TestIngestReminderDueProducesReminderSpeakAction(t *testing.T) {
	svc, _ := newEngineForTest(t)
	at := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)

	actions, err := svc.Ingest(context.Background(), mind.Event{
		ID:         "evt-1",
		DeviceID:   "dev-001",
		Type:       mind.EventReminderDue,
		Source:     mind.SourceScheduler,
		At:         at,
		Summary:    "吃药提醒",
		Salience:   0.8,
		Emotion:    "neutral",
		Confidence: 1,
	})
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("actions = %+v, want 1", actions)
	}
	if actions[0].Type != mind.ActionSpeak || actions[0].Executor != "reminder" {
		t.Fatalf("action = %+v, want reminder speak", actions[0])
	}
}

func TestIngestLateOutOfOrderEventUsesCurrentEventForThoughts(t *testing.T) {
	svc, _ := newEngineForTest(t)
	later := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	earlier := later.Add(-time.Hour)

	if _, err := svc.Ingest(context.Background(), mind.Event{
		ID:         "evt-presence-later",
		DeviceID:   "dev-001",
		Type:       mind.EventPresenceSeen,
		Source:     mind.SourceVision,
		At:         later,
		Summary:    "老人出现在客厅",
		Salience:   0.5,
		Emotion:    "calm",
		Confidence: 0.9,
	}); err != nil {
		t.Fatalf("Ingest later presence: %v", err)
	}

	actions, err := svc.Ingest(context.Background(), mind.Event{
		ID:         "evt-reminder-earlier",
		DeviceID:   "dev-001",
		Type:       mind.EventReminderDue,
		Source:     mind.SourceScheduler,
		At:         earlier,
		Summary:    "吃药提醒",
		Salience:   0.8,
		Emotion:    "neutral",
		Confidence: 1,
	})
	if err != nil {
		t.Fatalf("Ingest earlier reminder: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("actions = %+v, want 1", actions)
	}
	if actions[0].Type != mind.ActionSpeak || actions[0].Executor != "reminder" {
		t.Fatalf("action = %+v, want reminder speak from current event", actions[0])
	}
}

func TestIngestDuplicateEventIDIsIdempotent(t *testing.T) {
	svc, st := newEngineForTest(t)
	event := mind.Event{
		ID:         "evt-duplicate",
		DeviceID:   "dev-001",
		Type:       mind.EventReminderDue,
		Source:     mind.SourceScheduler,
		At:         time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC),
		Summary:    "吃药提醒",
		Salience:   0.8,
		Emotion:    "neutral",
		Confidence: 1,
	}

	if actions, err := svc.Ingest(context.Background(), event); err != nil {
		t.Fatalf("first Ingest: %v", err)
	} else if len(actions) != 1 {
		t.Fatalf("first actions = %+v, want 1", actions)
	}
	actions, err := svc.Ingest(context.Background(), event)
	if err != nil {
		t.Fatalf("duplicate Ingest: %v", err)
	}
	if len(actions) != 0 {
		t.Fatalf("duplicate actions = %+v, want none", actions)
	}
	if got := countRows(t, st, "mind_events"); got != 1 {
		t.Fatalf("event rows = %d, want 1", got)
	}
	if got := countRows(t, st, "mind_thoughts"); got != 1 {
		t.Fatalf("thought rows = %d, want 1", got)
	}
	if got := countRows(t, st, "mind_actions"); got != 1 {
		t.Fatalf("action rows = %d, want 1", got)
	}
}

func TestTickIdleCreatesLongSilenceEventAndWaitAction(t *testing.T) {
	svc, _ := newEngineForTest(t)
	at := time.Date(2026, 6, 16, 22, 0, 0, 0, time.UTC)

	actions, err := svc.TickIdle(context.Background(), "dev-001", at)
	if err != nil {
		t.Fatalf("TickIdle: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("actions = %+v, want 1", actions)
	}
	if actions[0].Type != mind.ActionWait {
		t.Fatalf("action = %+v, want wait", actions[0])
	}
}

func TestEngineRejectsBlankDeviceIDs(t *testing.T) {
	svc, st := newEngineForTest(t)
	ctx := context.Background()

	actions, err := svc.Ingest(ctx, mind.Event{
		ID:       "evt-blank-device",
		DeviceID: "   ",
		Type:     mind.EventReminderDue,
		Source:   mind.SourceScheduler,
		At:       time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC),
	})
	if !errors.Is(err, mind.ErrInvalidInput) {
		t.Fatalf("Ingest error = %v, want ErrInvalidInput", err)
	}
	if len(actions) != 0 {
		t.Fatalf("Ingest actions = %+v, want none", actions)
	}

	actions, err = svc.TickIdle(ctx, " \t ", time.Date(2026, 6, 16, 22, 0, 0, 0, time.UTC))
	if !errors.Is(err, mind.ErrInvalidInput) {
		t.Fatalf("TickIdle error = %v, want ErrInvalidInput", err)
	}
	if len(actions) != 0 {
		t.Fatalf("TickIdle actions = %+v, want none", actions)
	}
	if got := countRows(t, st, "mind_events"); got != 0 {
		t.Fatalf("event rows = %d, want 0", got)
	}
}

func TestReflectValidatesInputsAndIsIdempotent(t *testing.T) {
	svc, st := newEngineForTest(t)
	ctx := context.Background()
	window := mind.TimeWindow{
		From: time.Date(2026, 6, 16, 8, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC),
	}

	if err := svc.Reflect(ctx, " dev-001 ", window); err != nil {
		t.Fatalf("Reflect: %v", err)
	}
	if err := svc.Reflect(ctx, "dev-001", window); err != nil {
		t.Fatalf("Reflect repeat: %v", err)
	}
	if got := countRows(t, st, "mind_reflections"); got != 1 {
		t.Fatalf("reflection rows = %d, want 1", got)
	}

	if err := svc.Reflect(ctx, "  ", window); !errors.Is(err, mind.ErrInvalidInput) {
		t.Fatalf("Reflect blank device error = %v, want ErrInvalidInput", err)
	}
	if err := svc.Reflect(ctx, "dev-001", mind.TimeWindow{}); !errors.Is(err, mind.ErrInvalidInput) {
		t.Fatalf("Reflect zero window error = %v, want ErrInvalidInput", err)
	}
	if got := countRows(t, st, "mind_reflections"); got != 1 {
		t.Fatalf("reflection rows after invalid calls = %d, want 1", got)
	}
}
