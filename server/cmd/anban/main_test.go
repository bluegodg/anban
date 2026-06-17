package main

import (
	"context"
	"testing"
	"time"

	"github.com/bluegodg/anban/server/internal/config"
	"github.com/bluegodg/anban/server/internal/domains/profile"
	"github.com/bluegodg/anban/server/internal/mind"
	"github.com/bluegodg/anban/server/internal/mind/executors"
	"github.com/bluegodg/anban/server/internal/store"
	"github.com/bluegodg/anban/server/internal/xiaozhiclient"
)

func TestHistoryMindEventMapsConversationRoles(t *testing.T) {
	at := time.Date(2026, 6, 16, 10, 30, 0, 0, time.UTC)
	tests := []struct {
		name     string
		message  xiaozhiclient.HistoryMessage
		wantType mind.EventType
	}{
		{
			name:     "user",
			message:  xiaozhiclient.HistoryMessage{Role: "user", Text: "我今天想听会儿戏", At: at},
			wantType: mind.EventElderSpoke,
		},
		{
			name:     "assistant",
			message:  xiaozhiclient.HistoryMessage{Role: "assistant", Text: "好呀，我陪您慢慢听。", At: at},
			wantType: mind.EventAssistantSpoke,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, ok := historyMindEvent("dev-001", tt.message)
			if !ok {
				t.Fatal("historyMindEvent ok = false, want true")
			}
			if event.Type != tt.wantType || event.Source != mind.SourceXiaozhi {
				t.Fatalf("event = %+v, want xiaozhi %s", event, tt.wantType)
			}
			if event.ID == "" || event.DeviceID != "dev-001" || event.Summary == "" {
				t.Fatalf("event = %+v, want stable id, device, and summary", event)
			}
			again, ok := historyMindEvent("dev-001", tt.message)
			if !ok || again.ID != event.ID {
				t.Fatalf("second event = %+v ok=%v, want deterministic id %q", again, ok, event.ID)
			}
		})
	}
}

func TestHistoryMindEventSkipsUnusableMessages(t *testing.T) {
	at := time.Date(2026, 6, 16, 10, 30, 0, 0, time.UTC)
	tests := []xiaozhiclient.HistoryMessage{
		{Role: "tool", Text: "internal", At: at},
		{Role: "user", Text: "   ", At: at},
		{Role: "user", Text: "没有时间", At: time.Time{}},
	}
	for _, msg := range tests {
		if event, ok := historyMindEvent("dev-001", msg); ok {
			t.Fatalf("historyMindEvent(%+v) = %+v, want skipped", msg, event)
		}
	}
}

func TestMindActionExecutorDefersMissingSpeakExecutor(t *testing.T) {
	exec := mindActionExecutor{dispatcher: executors.NewDispatcher(map[string]executors.SpeakExecutor{})}

	result, err := exec.Execute(context.Background(), mind.Action{
		ID:       "action-message",
		DeviceID: "dev-001",
		Type:     mind.ActionSpeak,
		Executor: "message",
	})
	if err != nil {
		t.Fatalf("Execute error = %v, want missing executor to be a safe defer", err)
	}
	if result.Status != mind.ActionDeferred {
		t.Fatalf("result = %+v, want deferred", result)
	}
}

func TestConfigureMindEngineAppliesProactiveOutputSettings(t *testing.T) {
	loc := time.FixedZone("Asia/Shanghai", 8*60*60)
	target := &fakeMindEngineConfigTarget{}

	configureMindEngine(target, config.Config{
		TimezoneLocation:         loc,
		MindProactiveCooldown:    45 * time.Minute,
		MindProactiveDaytimeOnly: true,
	})

	if target.location != loc {
		t.Fatalf("location = %v, want configured location", target.location)
	}
	if target.cooldown != 45*time.Minute {
		t.Fatalf("cooldown = %s, want 45m", target.cooldown)
	}
	if !target.daytimeOnly {
		t.Fatal("daytimeOnly = false, want true")
	}
}

type fakeMindEngineConfigTarget struct {
	location    *time.Location
	cooldown    time.Duration
	daytimeOnly bool
}

func (f *fakeMindEngineConfigTarget) UseLocation(location *time.Location) {
	f.location = location
}

func (f *fakeMindEngineConfigTarget) UseProactiveCooldown(cooldown time.Duration) {
	f.cooldown = cooldown
}

func (f *fakeMindEngineConfigTarget) UseProactiveDaytimeOnly(enabled bool) {
	f.daytimeOnly = enabled
}

func TestRunMindLoopsSyncsMindContextAfterLifeUpdate(t *testing.T) {
	profileStore := newProfileStoreForMainTest(t)
	ctx := context.Background()
	if err := profileStore.Upsert(ctx, &profile.Profile{DeviceID: "dev-001", Fields: profile.Fields{Name: "王秀英"}}); err != nil {
		t.Fatalf("seed profile: %v", err)
	}
	engine := &fakeLoopMindEngine{mindContext: "最近你较挂念老人，语气更关切些。"}
	syncer := &fakeMindContextSyncer{}

	runMindLoops(profileStore, engine, syncer)

	if engine.tickCalls != 1 || engine.reflectCalls != 1 || engine.lifeCalls != 1 || engine.contextCalls != 1 {
		t.Fatalf("engine calls tick=%d reflect=%d life=%d context=%d, want all 1", engine.tickCalls, engine.reflectCalls, engine.lifeCalls, engine.contextCalls)
	}
	if len(syncer.calls) != 1 {
		t.Fatalf("sync calls = %+v, want one", syncer.calls)
	}
	if syncer.calls[0].deviceID != "dev-001" || syncer.calls[0].mindContext != engine.mindContext {
		t.Fatalf("sync call = %+v, want device and generated context", syncer.calls[0])
	}
}

func TestRunMindLoopsSkipsEmptyMindContext(t *testing.T) {
	profileStore := newProfileStoreForMainTest(t)
	ctx := context.Background()
	if err := profileStore.Upsert(ctx, &profile.Profile{DeviceID: "dev-001", Fields: profile.Fields{Name: "王秀英"}}); err != nil {
		t.Fatalf("seed profile: %v", err)
	}
	engine := &fakeLoopMindEngine{}
	syncer := &fakeMindContextSyncer{}

	runMindLoops(profileStore, engine, syncer)

	if len(syncer.calls) != 0 {
		t.Fatalf("sync calls = %+v, want none for empty context", syncer.calls)
	}
}

func newProfileStoreForMainTest(t *testing.T) *profile.Store {
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
	profileStore := profile.NewStore(st.DB)
	if err := profileStore.AutoMigrate(); err != nil {
		t.Fatalf("profile AutoMigrate: %v", err)
	}
	return profileStore
}

type fakeLoopMindEngine struct {
	tickCalls    int
	reflectCalls int
	lifeCalls    int
	contextCalls int
	mindContext  string
}

func (f *fakeLoopMindEngine) Ingest(context.Context, mind.Event) ([]mind.Action, error) {
	return nil, nil
}

func (f *fakeLoopMindEngine) TickIdle(context.Context, string, time.Time) ([]mind.Action, error) {
	f.tickCalls++
	return nil, nil
}

func (f *fakeLoopMindEngine) Reflect(context.Context, string, mind.TimeWindow) error {
	f.reflectCalls++
	return nil
}

func (f *fakeLoopMindEngine) UpdateLife(context.Context, string, time.Time) error {
	f.lifeCalls++
	return nil
}

func (f *fakeLoopMindEngine) BuildMindContext(context.Context, string, time.Time) (string, error) {
	f.contextCalls++
	return f.mindContext, nil
}

type fakeMindContextSyncer struct {
	calls []struct {
		deviceID    string
		mindContext string
	}
}

func (f *fakeMindContextSyncer) SyncMindContext(_ context.Context, deviceID string, mindContext string) error {
	f.calls = append(f.calls, struct {
		deviceID    string
		mindContext string
	}{deviceID: deviceID, mindContext: mindContext})
	return nil
}
