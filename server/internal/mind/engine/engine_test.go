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
		ID:         "evt-elder-spoke-later",
		DeviceID:   "dev-001",
		Type:       mind.EventElderSpoke,
		Source:     mind.SourceXiaozhi,
		At:         later,
		Summary:    "老人稍后正在聊天",
		Salience:   0.5,
		Emotion:    "calm",
		Confidence: 0.9,
	}); err != nil {
		t.Fatalf("Ingest later elder spoke: %v", err)
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
	if actions[0].Status != mind.ActionPending {
		t.Fatalf("action status = %s, want pending so future conversation does not defer it", actions[0].Status)
	}
}

func TestIngestLateOutOfOrderEventDoesNotUseFutureSelfState(t *testing.T) {
	svc, st := newEngineForTest(t)
	ctx := context.Background()
	deviceID := "dev-001"
	later := time.Date(2026, 6, 16, 22, 0, 0, 0, time.UTC)
	earlier := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)

	if _, err := svc.Ingest(ctx, mind.Event{
		ID:         "evt-long-silence-future",
		DeviceID:   deviceID,
		Type:       mind.EventLongSilence,
		Source:     mind.SourceScheduler,
		At:         later,
		Summary:    "晚上长时间没有互动",
		Salience:   0.7,
		Emotion:    "quiet",
		Confidence: 0.9,
	}); err != nil {
		t.Fatalf("Ingest future long silence: %v", err)
	}

	ms := mind.NewStore(st.DB)
	futureState, err := ms.GetSelfState(ctx, deviceID)
	if err != nil {
		t.Fatalf("GetSelfState after future event: %v", err)
	}

	actions, err := svc.Ingest(ctx, mind.Event{
		ID:         "evt-reminder-earlier-than-silence",
		DeviceID:   deviceID,
		Type:       mind.EventReminderDue,
		Source:     mind.SourceScheduler,
		At:         earlier,
		Summary:    "中午吃药提醒",
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
		t.Fatalf("action = %+v, want reminder speak from as-of self-state", actions[0])
	}
	if actions[0].Status != mind.ActionPending {
		t.Fatalf("action status = %s, want pending so future quietness does not defer it", actions[0].Status)
	}

	persistedState, err := ms.GetSelfState(ctx, deviceID)
	if err != nil {
		t.Fatalf("GetSelfState after late event: %v", err)
	}
	if persistedState.Concern < futureState.Concern || persistedState.Quietness < futureState.Quietness {
		t.Fatalf(
			"persisted state rewound after late event: concern %.2f quietness %.2f, want at least future concern %.2f quietness %.2f",
			persistedState.Concern,
			persistedState.Quietness,
			futureState.Concern,
			futureState.Quietness,
		)
	}
	if containsString(persistedState.ProcessedEventIDs, "evt-reminder-earlier-than-silence") {
		t.Fatalf("persisted state processed late event ID, want late as-of state not saved: %+v", persistedState.ProcessedEventIDs)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
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

func TestIngestRollsBackEventWhenPipelinePersistenceFails(t *testing.T) {
	svc, st := newEngineForTest(t)
	ctx := context.Background()
	at := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)

	if err := st.DB.Exec(
		"INSERT INTO mind_thoughts (thought_id, device_id, at, status) VALUES (?, ?, ?, ?)",
		"thought-evt-post-append-failure-reminder",
		"dev-001",
		at,
		string(mind.ThoughtPending),
	).Error; err != nil {
		t.Fatalf("seed duplicate thought: %v", err)
	}

	actions, err := svc.Ingest(ctx, mind.Event{
		ID:         "evt-post-append-failure",
		DeviceID:   "dev-001",
		Type:       mind.EventReminderDue,
		Source:     mind.SourceScheduler,
		At:         at,
		Summary:    "吃药提醒",
		Salience:   0.8,
		Emotion:    "neutral",
		Confidence: 1,
	})
	if err == nil {
		t.Fatal("Ingest error = nil, want duplicate thought persistence error")
	}
	if len(actions) != 0 {
		t.Fatalf("actions = %+v, want none on failed transaction", actions)
	}

	var count int64
	if err := st.DB.Raw("SELECT COUNT(*) FROM mind_events WHERE event_id = ?", "evt-post-append-failure").Scan(&count).Error; err != nil {
		t.Fatalf("count failed event: %v", err)
	}
	if count != 0 {
		t.Fatalf("event rows after failed pipeline = %d, want 0", count)
	}
}

func TestIngestBlankEventIDsGenerateDistinctPersistentIDs(t *testing.T) {
	svc, st := newEngineForTest(t)
	ctx := context.Background()
	at := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	event := mind.Event{
		DeviceID:   "dev-001",
		Type:       mind.EventReminderDue,
		Source:     mind.SourceScheduler,
		At:         at,
		Summary:    "吃药提醒",
		Salience:   0.8,
		Emotion:    "neutral",
		Confidence: 1,
	}

	for i := 0; i < 2; i++ {
		actions, err := svc.Ingest(ctx, event)
		if err != nil {
			t.Fatalf("Ingest blank ID #%d: %v", i+1, err)
		}
		if len(actions) != 1 || actions[0].Type != mind.ActionSpeak || actions[0].Executor != "reminder" {
			t.Fatalf("actions #%d = %+v, want reminder speak", i+1, actions)
		}
	}

	if got := countRows(t, st, "mind_events"); got != 2 {
		t.Fatalf("event rows = %d, want 2", got)
	}
	if got := countRows(t, st, "mind_thoughts"); got != 2 {
		t.Fatalf("thought rows = %d, want 2", got)
	}
	if got := countRows(t, st, "mind_actions"); got != 2 {
		t.Fatalf("action rows = %d, want 2", got)
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

	adjacentWindow := mind.TimeWindow{
		From: time.Date(2026, 6, 16, 9, 0, 0, 0, time.UTC),
		To:   window.To,
	}
	if err := svc.Reflect(ctx, "dev-001", adjacentWindow); err != nil {
		t.Fatalf("Reflect adjacent window with same To: %v", err)
	}
	if got := countRows(t, st, "mind_reflections"); got != 2 {
		t.Fatalf("reflection rows after distinct same-To window = %d, want 2", got)
	}

	if err := svc.Reflect(ctx, "  ", window); !errors.Is(err, mind.ErrInvalidInput) {
		t.Fatalf("Reflect blank device error = %v, want ErrInvalidInput", err)
	}
	if err := svc.Reflect(ctx, "dev-001", mind.TimeWindow{}); !errors.Is(err, mind.ErrInvalidInput) {
		t.Fatalf("Reflect zero window error = %v, want ErrInvalidInput", err)
	}
	if err := svc.Reflect(ctx, "dev-001", mind.TimeWindow{From: window.From}); !errors.Is(err, mind.ErrInvalidInput) {
		t.Fatalf("Reflect zero To error = %v, want ErrInvalidInput", err)
	}
	if err := svc.Reflect(ctx, "dev-001", mind.TimeWindow{To: window.To}); !errors.Is(err, mind.ErrInvalidInput) {
		t.Fatalf("Reflect zero From error = %v, want ErrInvalidInput", err)
	}
	if err := svc.Reflect(ctx, "dev-001", mind.TimeWindow{From: window.To, To: window.From}); !errors.Is(err, mind.ErrInvalidInput) {
		t.Fatalf("Reflect reversed window error = %v, want ErrInvalidInput", err)
	}
	if got := countRows(t, st, "mind_reflections"); got != 2 {
		t.Fatalf("reflection rows after invalid calls = %d, want 2", got)
	}
}
