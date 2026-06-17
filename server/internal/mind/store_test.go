package mind

import (
	"context"
	"testing"
	"time"

	"github.com/bluegodg/anban/server/internal/store"
)

func newMindStoreForTest(t *testing.T) *Store {
	t.Helper()
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	ms := NewStore(st.DB)
	if err := ms.AutoMigrate(); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	return ms
}

func TestStoreAppendsAndListsRecentEvents(t *testing.T) {
	ms := newMindStoreForTest(t)
	ctx := context.Background()
	base := time.Date(2026, 6, 16, 8, 0, 0, 0, time.UTC)

	first := Event{
		ID: "evt-1", DeviceID: "dev-001", Type: EventLongSilence, Source: SourceScheduler,
		At: base, Summary: "老人 40 分钟无互动", Payload: map[string]any{"minutes": float64(40)},
		Salience: 0.7, Emotion: "quiet", Confidence: 0.9,
	}
	second := Event{
		ID: "evt-2", DeviceID: "dev-001", Type: EventReminderDue, Source: SourceScheduler,
		At: base.Add(time.Minute), Summary: "午间提醒到期", Payload: map[string]any{"reminderId": float64(7)},
		Salience: 0.8, Emotion: "neutral", Confidence: 1,
	}
	if err := ms.AppendEvent(ctx, first); err != nil {
		t.Fatalf("AppendEvent first: %v", err)
	}
	if err := ms.AppendEvent(ctx, second); err != nil {
		t.Fatalf("AppendEvent second: %v", err)
	}

	got, err := ms.ListRecentEvents(ctx, "dev-001", 10)
	if err != nil {
		t.Fatalf("ListRecentEvents: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("events = %+v, want 2", got)
	}
	if got[0].ID != "evt-2" || got[1].ID != "evt-1" {
		t.Fatalf("events order = %+v, want newest first", got)
	}
	if got[0].Payload["reminderId"] != float64(7) {
		t.Fatalf("payload = %+v, want reminderId 7", got[0].Payload)
	}
}

func TestStoreUpsertsSelfStateAndLifeState(t *testing.T) {
	ms := newMindStoreForTest(t)
	ctx := context.Background()
	at := time.Date(2026, 6, 16, 9, 0, 0, 0, time.UTC)

	state := SelfState{DeviceID: "dev-001", At: at, Warmth: 0.6, Concern: 0.4, FamilyWeight: 0.6, PetWeight: 0.25, StewardWeight: 0.15}
	if err := ms.SaveSelfState(ctx, state); err != nil {
		t.Fatalf("SaveSelfState: %v", err)
	}
	state.Concern = 0.7
	if err := ms.SaveSelfState(ctx, state); err != nil {
		t.Fatalf("SaveSelfState update: %v", err)
	}
	got, err := ms.GetSelfState(ctx, "dev-001")
	if err != nil {
		t.Fatalf("GetSelfState: %v", err)
	}
	if got.Concern != 0.7 || got.Warmth != 0.6 {
		t.Fatalf("state = %+v, want concern 0.7 warmth 0.6", got)
	}

	life := LifeState{DeviceID: "dev-001", At: at, TodayTheme: "让今天轻一点", LingeringThoughts: []string{"昨天聊到老歌"}, SocialEnergy: 0.5, CareFocus: "上午互动少", PlayfulnessTrend: 0.2, RelationshipTemperature: 0.6}
	if err := ms.SaveLifeState(ctx, life); err != nil {
		t.Fatalf("SaveLifeState: %v", err)
	}
	gotLife, err := ms.GetLifeState(ctx, "dev-001")
	if err != nil {
		t.Fatalf("GetLifeState: %v", err)
	}
	if gotLife.TodayTheme != "让今天轻一点" || len(gotLife.LingeringThoughts) != 1 {
		t.Fatalf("life = %+v, want saved theme and lingering thought", gotLife)
	}
}

func TestStorePersistsThoughtActionFeedbackReflection(t *testing.T) {
	ms := newMindStoreForTest(t)
	ctx := context.Background()
	at := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)

	thought := Thought{ID: "thought-1", DeviceID: "dev-001", At: at, Content: "他今天比平时安静", SourceEventIDs: []string{"evt-1"}, DriveName: DriveCare, CareValue: 0.8, InterruptionCost: 0.7, Status: ThoughtPending}
	if err := ms.SaveThought(ctx, thought); err != nil {
		t.Fatalf("SaveThought: %v", err)
	}
	action := Action{ID: "action-1", DeviceID: "dev-001", IntentionID: "intent-1", Type: ActionWait, Executor: "mind", Status: ActionDeferred, Reason: "夜晚打扰成本高", Score: 0.64}
	if err := ms.SaveAction(ctx, action); err != nil {
		t.Fatalf("SaveAction: %v", err)
	}
	feedback := Feedback{ID: "feedback-1", DeviceID: "dev-001", ActionID: "action-1", At: at, Kind: "implicit", Signal: "waited", EffectOnState: map[string]float64{"quietness": 0.02}, Notes: "选择等待"}
	if err := ms.SaveFeedback(ctx, feedback); err != nil {
		t.Fatalf("SaveFeedback: %v", err)
	}
	reflection := Reflection{ID: "reflection-1", DeviceID: "dev-001", At: at, EpisodeSummary: "夜晚保持安静", StateAdjustments: map[string]float64{"quietness": 0.02}, BehaviorLessons: []string{"夜晚长沉默先观察"}}
	if err := ms.SaveReflection(ctx, reflection); err != nil {
		t.Fatalf("SaveReflection: %v", err)
	}
}
