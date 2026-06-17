package engine

import (
	"context"
	"testing"
	"time"

	"github.com/bluegodg/anban/server/internal/mind"
	"github.com/bluegodg/anban/server/internal/store"
)

func newEngineForTest(t *testing.T) (*Service, *mind.Store) {
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
	return New(ms), ms
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
