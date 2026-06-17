package simulation

import (
	"context"
	"testing"
	"time"

	"github.com/bluegodg/anban/server/internal/mind"
	"github.com/bluegodg/anban/server/internal/mind/engine"
	"github.com/bluegodg/anban/server/internal/store"
)

func TestOneDayMindShowsContinuityAndRestraint(t *testing.T) {
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
	svc := engine.New(ms)
	ctx := context.Background()
	deviceID := "dev-001"

	morning := time.Date(2026, 6, 16, 8, 10, 0, 0, time.UTC)
	if _, err := svc.Ingest(ctx, mind.Event{
		ID:       "evt-presence",
		DeviceID: deviceID,
		Type:     mind.EventPresenceSeen,
		Source:   mind.SourceVision,
		At:       morning,
		Summary:  "老人早晨出现",
	}); err != nil {
		t.Fatalf("presence ingest: %v", err)
	}

	noon := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	actions, err := svc.Ingest(ctx, mind.Event{
		ID:       "evt-reminder",
		DeviceID: deviceID,
		Type:     mind.EventReminderDue,
		Source:   mind.SourceScheduler,
		At:       noon,
		Summary:  "午间吃药提醒",
	})
	if err != nil {
		t.Fatalf("reminder ingest: %v", err)
	}
	if len(actions) == 0 || actions[0].Type != mind.ActionSpeak {
		t.Fatalf("reminder actions = %+v, want speak", actions)
	}

	night := time.Date(2026, 6, 16, 22, 30, 0, 0, time.UTC)
	actions, err = svc.TickIdle(ctx, deviceID, night)
	if err != nil {
		t.Fatalf("night idle: %v", err)
	}
	if len(actions) == 0 || actions[0].Type == mind.ActionSpeak {
		t.Fatalf("night actions = %+v, want non-speech restraint", actions)
	}

	if err := svc.UpdateLife(ctx, deviceID, night.Add(time.Minute)); err != nil {
		t.Fatalf("UpdateLife: %v", err)
	}

	state, err := ms.GetSelfState(ctx, deviceID)
	if err != nil {
		t.Fatalf("GetSelfState: %v", err)
	}
	if state.Concern <= 0.30 {
		t.Fatalf("state = %+v, want concern above default after day events", state)
	}

	lifeState, err := ms.GetLifeState(ctx, deviceID)
	if err != nil {
		t.Fatalf("GetLifeState: %v", err)
	}
	if lifeState.TodayTheme == "" || lifeState.RelationshipTemperature <= 0 {
		t.Fatalf("lifeState = %+v, want continuity metadata", lifeState)
	}
}
