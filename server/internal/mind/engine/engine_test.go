package engine

import (
	"context"
	"errors"
	"strings"
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

func TestIngestUsesConfiguredLocationForSituationTimeOfDay(t *testing.T) {
	tests := []struct {
		name      string
		eventType mind.EventType
		at        time.Time
	}{
		{name: "beijing morning child message", eventType: mind.EventChildMessageReceived, at: time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC)},
		{name: "beijing noon reminder", eventType: mind.EventReminderDue, at: time.Date(2026, 6, 16, 4, 30, 0, 0, time.UTC)},
		{name: "beijing evening reminder", eventType: mind.EventReminderDue, at: time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, _ := newEngineForTest(t)
			svc.UseLocation(time.FixedZone("Asia/Shanghai", 8*60*60))
			actions, err := svc.Ingest(context.Background(), mind.Event{
				ID:         "evt-" + tt.name,
				DeviceID:   "dev-001",
				Type:       tt.eventType,
				Source:     mind.SourceScheduler,
				At:         tt.at,
				Summary:    "进入心智",
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
			if actions[0].Status == mind.ActionSuppressed {
				t.Fatalf("action = %+v, want local daytime/evening not to be suppressed as UTC night", actions[0])
			}
		})
	}
}

func TestIngestExecutesPendingActionsWhenExecutorConfigured(t *testing.T) {
	svc, st := newEngineForTest(t)
	ctx := context.Background()
	exec := &fakeActionExecutor{
		result: ExecutionResult{
			Status:      mind.ActionExecuted,
			ExecutorRef: "fake:action-1",
		},
	}
	svc.UseExecutor(exec)
	at := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)

	actions, err := svc.Ingest(ctx, mind.Event{
		ID:         "evt-exec",
		DeviceID:   "dev-001",
		Type:       mind.EventReminderDue,
		Source:     mind.SourceScheduler,
		At:         at,
		Summary:    "吃药提醒",
		Payload:    map[string]any{"reminderId": float64(12)},
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
	if exec.calls != 1 {
		t.Fatalf("executor calls = %d, want 1", exec.calls)
	}
	if exec.lastAction.Args["reminderId"] != float64(12) {
		t.Fatalf("executor action args = %+v, want reminderId payload", exec.lastAction.Args)
	}
	if actions[0].Status != mind.ActionExecuted || actions[0].ExecutorRef != "fake:action-1" {
		t.Fatalf("returned action = %+v, want executed fake ref", actions[0])
	}

	var status, executorRef string
	if err := st.DB.Raw(
		"SELECT status, executor_ref FROM mind_actions WHERE action_id = ?",
		actions[0].ID,
	).Row().Scan(&status, &executorRef); err != nil {
		t.Fatalf("query action: %v", err)
	}
	if status != string(mind.ActionExecuted) || executorRef != "fake:action-1" {
		t.Fatalf("persisted status=%q executor_ref=%q, want executed fake ref", status, executorRef)
	}

	events, err := mind.NewStore(st.DB).ListRecentEvents(ctx, "dev-001", 10)
	if err != nil {
		t.Fatalf("ListRecentEvents: %v", err)
	}
	executionEvent := findEvent(events, mind.EventActionExecuted)
	if executionEvent == nil {
		t.Fatalf("events = %+v, want action_executed event", events)
	}
	if executionEvent.Payload["actionId"] != actions[0].ID ||
		executionEvent.Payload["status"] != string(mind.ActionExecuted) ||
		executionEvent.Payload["executorRef"] != "fake:action-1" {
		t.Fatalf("execution event payload = %+v, want action id, executed status, and executor ref", executionEvent.Payload)
	}
}

func TestIngestMarksActionFailedWhenExecutorFails(t *testing.T) {
	svc, st := newEngineForTest(t)
	exec := &fakeActionExecutor{
		result: ExecutionResult{
			Status:       mind.ActionFailed,
			ErrorMessage: "speaker unavailable",
		},
		err: errors.New("speaker unavailable"),
	}
	svc.UseExecutor(exec)
	at := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)

	actions, err := svc.Ingest(context.Background(), mind.Event{
		ID:         "evt-exec-fails",
		DeviceID:   "dev-001",
		Type:       mind.EventReminderDue,
		Source:     mind.SourceScheduler,
		At:         at,
		Summary:    "吃药提醒",
		Salience:   0.8,
		Emotion:    "neutral",
		Confidence: 1,
	})
	if !errors.Is(err, exec.err) {
		t.Fatalf("Ingest error = %v, want executor error", err)
	}
	if len(actions) != 1 {
		t.Fatalf("actions = %+v, want returned failed action", actions)
	}
	if exec.calls != 1 {
		t.Fatalf("executor calls = %d, want 1", exec.calls)
	}
	if actions[0].Status != mind.ActionFailed || actions[0].Reason != "speaker unavailable" {
		t.Fatalf("returned action = %+v, want failed with reason", actions[0])
	}

	var status, reason string
	if err := st.DB.Raw(
		"SELECT status, reason FROM mind_actions WHERE action_id = ?",
		actions[0].ID,
	).Row().Scan(&status, &reason); err != nil {
		t.Fatalf("query action: %v", err)
	}
	if status != string(mind.ActionFailed) || reason != "speaker unavailable" {
		t.Fatalf("persisted status=%q reason=%q, want failed speaker unavailable", status, reason)
	}
}

func TestIngestSafelyDefersActionWhenExecutorIsMissing(t *testing.T) {
	svc, st := newEngineForTest(t)
	exec := &fakeActionExecutor{
		result: ExecutionResult{
			Status:       mind.ActionDeferred,
			ErrorMessage: "executor not found",
		},
		err: errors.New("executor not found"),
	}
	svc.UseExecutor(exec)
	at := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)

	actions, err := svc.Ingest(context.Background(), mind.Event{
		ID:         "evt-missing-executor",
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
		t.Fatalf("Ingest error = %v, want safe skip for missing executor", err)
	}
	if len(actions) != 1 {
		t.Fatalf("actions = %+v, want returned deferred action", actions)
	}
	if actions[0].Status != mind.ActionDeferred || actions[0].Reason != "executor not found" {
		t.Fatalf("returned action = %+v, want deferred with reason", actions[0])
	}

	var status, reason string
	if err := st.DB.Raw(
		"SELECT status, reason FROM mind_actions WHERE action_id = ?",
		actions[0].ID,
	).Row().Scan(&status, &reason); err != nil {
		t.Fatalf("query action: %v", err)
	}
	if status != string(mind.ActionDeferred) || reason != "executor not found" {
		t.Fatalf("persisted status=%q reason=%q, want deferred executor not found", status, reason)
	}
}

type fakeActionExecutor struct {
	calls      int
	lastAction mind.Action
	result     ExecutionResult
	err        error
}

func (f *fakeActionExecutor) Execute(ctx context.Context, action mind.Action) (ExecutionResult, error) {
	f.calls++
	f.lastAction = action
	return f.result, f.err
}

func findEvent(events []mind.Event, eventType mind.EventType) *mind.Event {
	for i := range events {
		if events[i].Type == eventType {
			return &events[i]
		}
	}
	return nil
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

func TestTickIdleHighConcernSilenceProducesAutonomousGreetingSpeak(t *testing.T) {
	svc, st := newEngineForTest(t)
	ctx := context.Background()
	deviceID := "dev-001"
	at := time.Date(2026, 6, 16, 9, 0, 0, 0, time.UTC)
	ms := mind.NewStore(st.DB)
	if err := ms.SaveSelfState(ctx, mind.SelfState{
		DeviceID:      deviceID,
		At:            at.Add(-time.Hour),
		Concern:       0.82,
		Warmth:        0.55,
		Quietness:     0.20,
		FamilyWeight:  0.60,
		PetWeight:     0.25,
		StewardWeight: 0.15,
	}); err != nil {
		t.Fatalf("SaveSelfState: %v", err)
	}

	actions, err := svc.TickIdle(ctx, deviceID, at)
	if err != nil {
		t.Fatalf("TickIdle: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("actions = %+v, want 1", actions)
	}
	action := actions[0]
	if action.Type != mind.ActionSpeak || action.Executor != "greeting" || action.Status != mind.ActionPending {
		t.Fatalf("action = %+v, want pending greeting speak", action)
	}
	if action.Args["mindProactive"] != true {
		t.Fatalf("Args = %+v, want mindProactive=true", action.Args)
	}
	if action.Text == "" {
		t.Fatal("Text is empty, want deterministic gentle check-in")
	}
}

func TestTickIdleAutonomousGreetingWaitsDuringCooldown(t *testing.T) {
	svc, st := newEngineForTest(t)
	ctx := context.Background()
	deviceID := "dev-001"
	at := time.Date(2026, 6, 16, 9, 0, 0, 0, time.UTC)
	ms := mind.NewStore(st.DB)
	if err := ms.SaveSelfState(ctx, mind.SelfState{
		DeviceID:      deviceID,
		At:            at.Add(-time.Hour),
		Concern:       0.85,
		Warmth:        0.55,
		Quietness:     0.20,
		FamilyWeight:  0.60,
		PetWeight:     0.25,
		StewardWeight: 0.15,
	}); err != nil {
		t.Fatalf("SaveSelfState: %v", err)
	}
	if err := ms.AppendEvent(ctx, mind.Event{
		ID:       "evt-last-autonomous-greeting",
		DeviceID: deviceID,
		Type:     mind.EventActionExecuted,
		Source:   mind.SourceMind,
		At:       at.Add(-20 * time.Minute),
		Summary:  "上一轮自主问候已执行",
		Payload: map[string]any{
			"actionType":    string(mind.ActionSpeak),
			"executor":      "greeting",
			"status":        string(mind.ActionExecuted),
			"mindProactive": true,
		},
	}); err != nil {
		t.Fatalf("AppendEvent: %v", err)
	}

	actions, err := svc.TickIdle(ctx, deviceID, at)
	if err != nil {
		t.Fatalf("TickIdle: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("actions = %+v, want 1", actions)
	}
	if actions[0].Type != mind.ActionWait || actions[0].Status != mind.ActionDeferred {
		t.Fatalf("action = %+v, want deferred wait during cooldown", actions[0])
	}
	if !strings.Contains(actions[0].Reason, "冷却") {
		t.Fatalf("Reason = %q, want cooldown reason", actions[0].Reason)
	}
}

func TestTickIdleAutonomousGreetingWaitsWhenDaytimeOnlyAtNight(t *testing.T) {
	svc, st := newEngineForTest(t)
	ctx := context.Background()
	deviceID := "dev-001"
	at := time.Date(2026, 6, 16, 22, 0, 0, 0, time.UTC)
	ms := mind.NewStore(st.DB)
	if err := ms.SaveSelfState(ctx, mind.SelfState{
		DeviceID:      deviceID,
		At:            at.Add(-time.Hour),
		Concern:       0.88,
		Warmth:        0.55,
		Quietness:     0.20,
		FamilyWeight:  0.60,
		PetWeight:     0.25,
		StewardWeight: 0.15,
	}); err != nil {
		t.Fatalf("SaveSelfState: %v", err)
	}

	actions, err := svc.TickIdle(ctx, deviceID, at)
	if err != nil {
		t.Fatalf("TickIdle: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("actions = %+v, want 1", actions)
	}
	if actions[0].Type != mind.ActionWait || actions[0].Status != mind.ActionDeferred {
		t.Fatalf("action = %+v, want deferred wait for daytime-only night", actions[0])
	}
	if !strings.Contains(actions[0].Reason, "仅白天") {
		t.Fatalf("Reason = %q, want daytime-only reason", actions[0].Reason)
	}
}

func TestUpdateLifePersistsLifeStateFromRecentEvents(t *testing.T) {
	svc, st := newEngineForTest(t)
	ctx := context.Background()
	deviceID := "dev-001"
	at := time.Date(2026, 6, 16, 8, 0, 0, 0, time.UTC)

	if _, err := svc.Ingest(ctx, mind.Event{
		ID:         "evt-life-long-silence",
		DeviceID:   deviceID,
		Type:       mind.EventLongSilence,
		Source:     mind.SourceMind,
		At:         at,
		Summary:    "上午互动少",
		Salience:   0.7,
		Emotion:    "quiet",
		Confidence: 0.9,
	}); err != nil {
		t.Fatalf("Ingest long silence: %v", err)
	}

	if err := svc.UpdateLife(ctx, deviceID, at.Add(5*time.Minute)); err != nil {
		t.Fatalf("UpdateLife: %v", err)
	}
	ms := mind.NewStore(st.DB)
	lifeState, err := ms.GetLifeState(ctx, deviceID)
	if err != nil {
		t.Fatalf("GetLifeState: %v", err)
	}
	if lifeState.TodayTheme == "" || lifeState.CareFocus == "" {
		t.Fatalf("lifeState = %+v, want theme and care focus", lifeState)
	}
	if len(lifeState.LingeringThoughts) != 1 {
		t.Fatalf("LingeringThoughts = %+v, want one recent trace", lifeState.LingeringThoughts)
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
	if err := svc.UpdateLife(ctx, " \t ", time.Date(2026, 6, 16, 22, 0, 0, 0, time.UTC)); !errors.Is(err, mind.ErrInvalidInput) {
		t.Fatalf("UpdateLife error = %v, want ErrInvalidInput", err)
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

func TestReflectSummarizesFeedbackAndAdjustsSelfState(t *testing.T) {
	svc, st := newEngineForTest(t)
	ctx := context.Background()
	deviceID := "dev-001"
	window := mind.TimeWindow{
		From: time.Date(2026, 6, 16, 9, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC),
	}
	ms := mind.NewStore(st.DB)
	if err := ms.SaveSelfState(ctx, mind.SelfState{DeviceID: deviceID, At: window.From, Warmth: 0.50, Quietness: 0.50}); err != nil {
		t.Fatalf("SaveSelfState: %v", err)
	}
	for _, feedback := range []mind.Feedback{
		{ID: "feedback-replied", DeviceID: deviceID, ActionID: "action-1", At: window.From.Add(5 * time.Minute), Kind: "implicit", Signal: "user_replied"},
		{ID: "feedback-ignored", DeviceID: deviceID, ActionID: "action-2", At: window.From.Add(10 * time.Minute), Kind: "implicit", Signal: "user_ignored"},
	} {
		if err := ms.SaveFeedback(ctx, feedback); err != nil {
			t.Fatalf("SaveFeedback %s: %v", feedback.ID, err)
		}
	}

	if err := svc.Reflect(ctx, deviceID, window); err != nil {
		t.Fatalf("Reflect: %v", err)
	}

	state, err := ms.GetSelfState(ctx, deviceID)
	if err != nil {
		t.Fatalf("GetSelfState: %v", err)
	}
	if state.Warmth <= 0.50 || state.Quietness <= 0.50 {
		t.Fatalf("state = %+v, want feedback adjustments applied", state)
	}

	var summary string
	if err := st.DB.Raw(
		"SELECT episode_summary FROM mind_reflections WHERE device_id = ? ORDER BY id DESC LIMIT 1",
		deviceID,
	).Row().Scan(&summary); err != nil {
		t.Fatalf("query reflection summary: %v", err)
	}
	if summary == "" || summary == "本轮反思已记录，具体摘要由 reflection 模块补充" {
		t.Fatalf("summary = %q, want summarized feedback", summary)
	}
}
