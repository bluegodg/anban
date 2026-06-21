package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bluegodg/anban/server/internal/config"
	"github.com/bluegodg/anban/server/internal/domains/greeting"
	"github.com/bluegodg/anban/server/internal/domains/profile"
	"github.com/bluegodg/anban/server/internal/domains/vision"
	"github.com/bluegodg/anban/server/internal/mind"
	mindengine "github.com/bluegodg/anban/server/internal/mind/engine"
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

func TestMindGreetingExecutorSilentlyDefersFailedMindProactiveSpeak(t *testing.T) {
	speaker := &fakeMindGreetingSpeaker{
		greeting: greeting.Greeting{ID: 7, Status: greeting.StatusFailed, ErrorMessage: "device offline"},
		err:      errors.New("device offline"),
	}
	exec := newMindGreetingSpeakExecutor(speaker)

	result, err := exec.Speak(context.Background(), mind.Action{
		ID:       "action-proactive",
		DeviceID: "dev-001",
		Type:     mind.ActionSpeak,
		Executor: "greeting",
		Text:     "我在这儿呢。",
		Args:     map[string]any{"mindProactive": true},
	})
	if err != nil {
		t.Fatalf("Speak error = %v, want nil for silent proactive skip", err)
	}
	if result.Status != mind.ActionDeferred || result.ExecutorRef != "greeting:7" {
		t.Fatalf("result = %+v, want deferred greeting ref", result)
	}
	if !strings.Contains(result.ErrorMessage, "device offline") {
		t.Fatalf("ErrorMessage = %q, want device offline detail for action record", result.ErrorMessage)
	}
}

func TestMindGreetingExecutorReturnsErrorForNormalGreetingFailure(t *testing.T) {
	speaker := &fakeMindGreetingSpeaker{
		greeting: greeting.Greeting{ID: 8, Status: greeting.StatusFailed, ErrorMessage: "device offline"},
		err:      errors.New("device offline"),
	}
	exec := newMindGreetingSpeakExecutor(speaker)

	result, err := exec.Speak(context.Background(), mind.Action{
		ID:       "action-normal",
		DeviceID: "dev-001",
		Type:     mind.ActionSpeak,
		Executor: "greeting",
		Text:     "您好。",
	})
	if !errors.Is(err, speaker.err) {
		t.Fatalf("Speak error = %v, want normal greeting error", err)
	}
	if result.Status != mind.ActionFailed {
		t.Fatalf("result = %+v, want failed", result)
	}
}

func TestConfigureMindEngineAppliesProactiveOutputSettings(t *testing.T) {
	loc := time.FixedZone("Asia/Shanghai", 8*60*60)
	target := &fakeMindEngineConfigTarget{}

	configureMindEngine(target, config.Config{
		TimezoneLocation:             loc,
		MindProactiveCooldown:        45 * time.Minute,
		MindProactiveDaytimeOnly:     true,
		MindAutonomousVisionEnabled:  true,
		MindAutonomousVisionCooldown: 12 * time.Minute,
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
	if !target.autonomousVisionEnabled || target.autonomousVisionCooldown != 12*time.Minute {
		t.Fatalf("autonomous vision = enabled:%v cooldown:%s, want true/12m", target.autonomousVisionEnabled, target.autonomousVisionCooldown)
	}
}

type fakeMindEngineConfigTarget struct {
	location                 *time.Location
	cooldown                 time.Duration
	daytimeOnly              bool
	autonomousVisionEnabled  bool
	autonomousVisionCooldown time.Duration
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

func (f *fakeMindEngineConfigTarget) UseAutonomousVisionEnabled(enabled bool) {
	f.autonomousVisionEnabled = enabled
}

func (f *fakeMindEngineConfigTarget) UseAutonomousVisionCooldown(cooldown time.Duration) {
	f.autonomousVisionCooldown = cooldown
}

func TestMindVisionExecutorTurnsLookFailuresIntoRecordedFailure(t *testing.T) {
	looker := &fakeMindVisionLooker{
		capture: vision.CaptureDTO{CaptureID: "capture-7", Status: vision.CaptureStatusFailed, FailureMessage: "设备离线"},
		err:     errors.New("device offline"),
	}
	exec := newMindVisionExecutor(looker)
	result, err := exec.Observe(context.Background(), mind.Action{
		ID:       "action-look",
		DeviceID: "dev-001",
		Type:     mind.ActionCallMCPTool,
		Executor: "vision",
		Args:     map[string]any{"mindAutonomousVision": true},
	})
	if err != nil {
		t.Fatalf("Observe error = %v, want graceful recorded failure", err)
	}
	if result.Status != mind.ActionFailed || result.ExecutorRef != "vision:capture-7" {
		t.Fatalf("result = %+v, want failed capture ref", result)
	}
	if !strings.Contains(result.ErrorMessage, "设备离线") {
		t.Fatalf("ErrorMessage = %q, want safe failure detail", result.ErrorMessage)
	}
}

func TestMindVisionExecutorMarksSuccessfulLookExecuted(t *testing.T) {
	looker := &fakeMindVisionLooker{
		capture: vision.CaptureDTO{
			CaptureID: "capture-success",
			Status:    vision.CaptureStatusSucceeded,
			Analysis:  vision.CaptureAnalysis{Summary: "老人正在沙发上休息", Presence: vision.PresenceSomeone},
		},
	}
	exec := newMindVisionExecutor(looker)
	result, err := exec.Observe(context.Background(), mind.Action{
		ID:       "action-look-success",
		DeviceID: "dev-001",
		Type:     mind.ActionCallMCPTool,
		Executor: "vision",
	})
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}
	if result.Status != mind.ActionExecuted || result.ExecutorRef != "vision:capture-success" || result.ErrorMessage != "" {
		t.Fatalf("result = %+v, want executed successful capture", result)
	}
}

func TestVisionMindSinkCreatesFollowUpThoughtAndWaitAction(t *testing.T) {
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	mindStore := mind.NewStore(st.DB)
	if err := mindStore.AutoMigrate(); err != nil {
		t.Fatalf("mind AutoMigrate: %v", err)
	}
	sink := visionMindSink{engine: mindengine.New(mindStore)}

	err = sink.IngestMindEvent(context.Background(), vision.MindEvent{
		DeviceID: "dev-001",
		Type:     string(mind.EventVisionObservation),
		Summary:  "老人正在沙发上休息",
		Payload: map[string]any{
			"captureId": "capture-success",
			"presence":  string(vision.PresenceSomeone),
		},
	})
	if err != nil {
		t.Fatalf("IngestMindEvent: %v", err)
	}

	var thoughtContent, driveName string
	if err := st.DB.Raw(
		"SELECT content, drive_name FROM mind_thoughts WHERE device_id = ? ORDER BY id DESC LIMIT 1",
		"dev-001",
	).Row().Scan(&thoughtContent, &driveName); err != nil {
		t.Fatalf("query follow-up thought: %v", err)
	}
	if thoughtContent != "老人正在沙发上休息" || driveName != mind.DriveQuietPresence {
		t.Fatalf("thought content=%q drive=%q, want quiet vision follow-up", thoughtContent, driveName)
	}

	var actionType, actionStatus string
	if err := st.DB.Raw(
		"SELECT type, status FROM mind_actions WHERE device_id = ? ORDER BY id DESC LIMIT 1",
		"dev-001",
	).Row().Scan(&actionType, &actionStatus); err != nil {
		t.Fatalf("query follow-up action: %v", err)
	}
	if actionType != string(mind.ActionWait) || actionStatus != string(mind.ActionDeferred) {
		t.Fatalf("action type=%q status=%q, want deferred wait without recursive capture", actionType, actionStatus)
	}
}

type fakeMindVisionLooker struct {
	capture vision.CaptureDTO
	err     error
}

func (f *fakeMindVisionLooker) Look(context.Context, vision.LookRequest) (vision.CaptureDTO, error) {
	return f.capture, f.err
}

type fakeMindGreetingSpeaker struct {
	greeting greeting.Greeting
	err      error
}

func (f *fakeMindGreetingSpeaker) SpeakText(context.Context, string, string) (greeting.Greeting, error) {
	return f.greeting, f.err
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

func TestRunMindContextSyncRebuildsContextWithoutRunningMindActions(t *testing.T) {
	profileStore := newProfileStoreForMainTest(t)
	ctx := context.Background()
	if err := profileStore.Upsert(ctx, &profile.Profile{DeviceID: "dev-001", Fields: profile.Fields{Name: "蓝"}}); err != nil {
		t.Fatalf("seed profile: %v", err)
	}
	engine := &fakeLoopMindEngine{mindContext: "陪伴对象：蓝；记忆重点：老人喜欢养花。"}
	syncer := &fakeMindContextSyncer{}

	runMindContextSync(profileStore, engine, syncer)

	if engine.tickCalls != 0 || engine.reflectCalls != 0 || engine.lifeCalls != 0 {
		t.Fatalf("mind action calls tick=%d reflect=%d life=%d, want all 0", engine.tickCalls, engine.reflectCalls, engine.lifeCalls)
	}
	if engine.contextCalls != 1 {
		t.Fatalf("context calls = %d, want 1", engine.contextCalls)
	}
	if len(syncer.calls) != 1 || syncer.calls[0].deviceID != "dev-001" || syncer.calls[0].mindContext != engine.mindContext {
		t.Fatalf("sync calls = %+v, want rebuilt context", syncer.calls)
	}
}

func TestProfileSummariesIncludeAICognitivePortraitForMind(t *testing.T) {
	summaries := profileSummaries(profile.Fields{
		Name:       "蓝",
		AIPortrait: "重视家人，喜欢养花，交流时偏好温和直接的表达。",
	})
	joined := strings.Join(summaries, "\n")
	if !strings.Contains(joined, "AI认知画像：重视家人，喜欢养花") {
		t.Fatalf("summaries = %#v, want AI cognitive portrait for Mind", summaries)
	}
}

func TestRunAIPortraitRefreshRefreshesExistingProfiles(t *testing.T) {
	profileStore := newProfileStoreForMainTest(t)
	ctx := context.Background()
	if err := profileStore.Upsert(ctx, &profile.Profile{
		DeviceID: "dev-001",
		Fields: profile.Fields{
			Name:           "蓝",
			Hobbies:        []string{"养花"},
			AIPortraitMode: profile.PortraitModeAuto,
		},
		MemoryFacts: []string{"老人关注世界杯足球赛事。"},
	}); err != nil {
		t.Fatalf("seed profile: %v", err)
	}
	generator := &fakeMainPortraitGenerator{result: "喜欢养花，也关注世界杯，交流时适合温和直接。"}
	service := profile.NewService(profileStore, generator)

	runAIPortraitRefresh(profileStore, service)

	if len(generator.calls) != 1 {
		t.Fatalf("portrait calls = %d, want 1", len(generator.calls))
	}
	saved, err := service.Get(ctx, "dev-001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if saved.Fields.AIPortrait != generator.result {
		t.Fatalf("portrait = %q, want refreshed portrait", saved.Fields.AIPortrait)
	}
}

func TestRunAIPortraitRefreshThenMindSyncRebuildsContext(t *testing.T) {
	profileStore := newProfileStoreForMainTest(t)
	ctx := context.Background()
	if err := profileStore.Upsert(ctx, &profile.Profile{
		DeviceID: "dev-001",
		Fields: profile.Fields{
			Name:           "蓝",
			AIPortraitMode: profile.PortraitModeAuto,
		},
	}); err != nil {
		t.Fatalf("seed profile: %v", err)
	}
	generator := &fakeMainPortraitGenerator{result: "陪伴对象喜欢养花。"}
	service := profile.NewService(profileStore, generator)
	engine := &fakeLoopMindEngine{mindContext: "画像重点：AI认知画像：陪伴对象喜欢养花。"}
	syncer := &fakeMindContextSyncer{}

	runAIPortraitRefreshThenMindSync(profileStore, service, engine, syncer)

	if len(generator.calls) != 1 {
		t.Fatalf("portrait calls = %d, want 1", len(generator.calls))
	}
	if engine.contextCalls != 1 || len(syncer.calls) != 1 {
		t.Fatalf("contextCalls=%d syncCalls=%d, want one mind rebuild after portrait refresh", engine.contextCalls, len(syncer.calls))
	}
	if syncer.calls[0].mindContext != engine.mindContext {
		t.Fatalf("sync context = %q, want rebuilt context", syncer.calls[0].mindContext)
	}
}

func TestRunVisionCaptureMaintenanceFinalizesTimeoutsAndExpiresCaptures(t *testing.T) {
	now := time.Date(2026, 6, 18, 11, 30, 0, 0, time.UTC)
	maintainer := &fakeVisionCaptureMaintainer{}

	runVisionCaptureMaintenance(maintainer, now)

	if maintainer.finalizeCalls != 1 || maintainer.expireCalls != 1 || maintainer.pruneCalls != 1 {
		t.Fatalf("maintenance calls finalize=%d expire=%d prune=%d, want all once", maintainer.finalizeCalls, maintainer.expireCalls, maintainer.pruneCalls)
	}
	if !maintainer.finalizeAt.Equal(now) || !maintainer.expireAt.Equal(now) {
		t.Fatalf("maintenance times finalize=%s expire=%s, want %s", maintainer.finalizeAt, maintainer.expireAt, now)
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

type fakeMainPortraitGenerator struct {
	result string
	calls  []profile.PortraitInput
}

func (f *fakeMainPortraitGenerator) GeneratePortrait(_ context.Context, input profile.PortraitInput) (string, error) {
	f.calls = append(f.calls, input)
	return f.result, nil
}

type fakeVisionCaptureMaintainer struct {
	finalizeCalls int
	expireCalls   int
	pruneCalls    int
	finalizeAt    time.Time
	expireAt      time.Time
}

func (f *fakeVisionCaptureMaintainer) FinalizeTimedOutCaptures(_ context.Context, now time.Time) (int, error) {
	f.finalizeCalls++
	f.finalizeAt = now
	return 1, nil
}

func (f *fakeVisionCaptureMaintainer) ExpireCaptures(_ context.Context, now time.Time) (int, error) {
	f.expireCalls++
	f.expireAt = now
	return 1, nil
}

func (f *fakeVisionCaptureMaintainer) PruneExcessCaptures(_ context.Context) (int, error) {
	f.pruneCalls++
	return 1, nil
}
