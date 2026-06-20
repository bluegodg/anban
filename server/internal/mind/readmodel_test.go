package mind

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestReadServiceSnapshotReturnsUnavailableWhenMindHasNoState(t *testing.T) {
	ms := newMindStoreForTest(t)
	svc := NewReadService(ms)

	got, err := svc.Snapshot(context.Background(), "dev-001")
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if got.Available {
		t.Fatalf("Available = true, want false for an unseen device")
	}
	if got.UpdatedAt != nil || got.SelfState != nil || got.LifeState != nil || got.LatestThought != nil || got.LatestAction != nil {
		t.Fatalf("snapshot = %+v, want empty public snapshot", got)
	}
}

func TestReadServiceSnapshotProjectsLatestPublicMindState(t *testing.T) {
	ms := newMindStoreForTest(t)
	ctx := context.Background()
	base := time.Date(2026, 6, 20, 8, 0, 0, 0, time.UTC)
	if err := ms.SaveSelfState(ctx, SelfState{
		DeviceID: "dev-001", At: base,
		Warmth: 0.81, Concern: 0.72, Curiosity: 0.46, Playfulness: 0.31,
		Energy: 0.53, Quietness: 0.64, Patience: 0.70, Confidence: 0.66,
		FamilyWeight: 0.99, PetWeight: 0.12, StewardWeight: 0.43,
		ProcessedEventIDs: []string{"secret-event"},
	}); err != nil {
		t.Fatalf("SaveSelfState: %v", err)
	}
	if err := ms.SaveLifeState(ctx, LifeState{
		DeviceID: "dev-001", At: base.Add(time.Minute),
		TodayTheme: "轻轻留意老人状态", CareFocus: "最近互动偏少，先轻轻留意",
		LingeringThoughts: []string{"晚上问候要轻一点"}, SocialEnergy: 0.51,
		PlayfulnessTrend: 0.22, RelationshipTemperature: 0.77,
	}); err != nil {
		t.Fatalf("SaveLifeState: %v", err)
	}
	if err := ms.SaveThought(ctx, Thought{
		ID: "thought-latest", DeviceID: "dev-001", At: base.Add(2 * time.Minute),
		Content: "妈妈今天说话少，我先保持安静陪着", DriveName: DriveQuietPresence,
		EmotionalTone: "quiet", Urgency: 0.2, CareValue: 0.7, Status: ThoughtPending,
		SourceEventIDs: []string{"secret-event"}, RelatedMemoryIDs: []string{"secret-memory"},
	}); err != nil {
		t.Fatalf("SaveThought: %v", err)
	}
	if err := ms.SaveAction(ctx, Action{
		ID: "action-latest", DeviceID: "dev-001", IntentionID: "intention-thought-latest",
		Type: ActionWait, Status: ActionDeferred, Executor: "internal", ExecutorRef: "secret-ref",
		Reason: "不打断当前安静状态", Args: map[string]any{"secret": true}, Score: 0.8,
	}); err != nil {
		t.Fatalf("SaveAction: %v", err)
	}
	if err := ms.SaveThought(ctx, Thought{
		ID: "thought-other", DeviceID: "dev-002", At: base.Add(10 * time.Minute),
		Content: "其他设备的新念头", DriveName: DriveCare, Status: ThoughtPending,
	}); err != nil {
		t.Fatalf("SaveThought other device: %v", err)
	}

	got, err := NewReadService(ms).Snapshot(ctx, "dev-001")
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if !got.Available {
		t.Fatal("Available = false, want true")
	}
	if got.SelfState == nil || got.SelfState.Warmth != 0.81 || got.SelfState.Confidence != 0.66 {
		t.Fatalf("SelfState = %+v, want public eight-metric view", got.SelfState)
	}
	if got.LifeState == nil || got.LifeState.TodayTheme != "轻轻留意老人状态" || got.LifeState.RelationshipTemperature != 0.77 {
		t.Fatalf("LifeState = %+v, want public life state", got.LifeState)
	}
	if got.LatestThought == nil || got.LatestThought.Content != "妈妈今天说话少，我先保持安静陪着" || got.LatestThought.Drive != DriveQuietPresence {
		t.Fatalf("LatestThought = %+v, want device-local latest thought", got.LatestThought)
	}
	if got.LatestAction == nil || got.LatestAction.Type != ActionWait || got.LatestAction.Status != ActionDeferred || got.LatestAction.Reason != "不打断当前安静状态" {
		t.Fatalf("LatestAction = %+v, want paired public action", got.LatestAction)
	}
	body := mustJSON(t, got)
	for _, forbidden := range []string{"familyWeight", "petWeight", "stewardWeight", "processedEvent", "secret-event", "secret-memory", "secret-ref", "\"secret\""} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("snapshot JSON leaked %q: %s", forbidden, body)
		}
	}
}

func TestReadServiceTimelineMergesPublicItemsAndNormalizesTimezones(t *testing.T) {
	ms := newMindStoreForTest(t)
	ctx := context.Background()
	cst := time.FixedZone("CST", 8*60*60)
	thoughtAt := time.Date(2026, 6, 20, 3, 0, 0, 0, time.UTC)
	eventAt := time.Date(2026, 6, 20, 10, 0, 0, 0, cst) // 02:00 UTC; should sort after the 03:00 UTC thought.
	reflectionAt := time.Date(2026, 6, 20, 1, 0, 0, 0, time.UTC)

	mustCreateMindRecord(t, ms, &thoughtRecord{
		ID: 11, ThoughtID: "thought-new", DeviceID: "dev-001", At: thoughtAt,
		Content: "先听一听妈妈是不是需要空间", DriveName: DriveQuietPresence,
		EmotionalTone: "quiet", Status: string(ThoughtPending),
		SourceEventIDsJSON: "[\"secret-event\"]", RelatedMemoryIDsJSON: "[\"secret-memory\"]",
	})
	mustCreateMindRecord(t, ms, &actionRecord{
		ID: 21, ActionID: "action-new", DeviceID: "dev-001", IntentionID: "intention-thought-new",
		Type: string(ActionWait), Status: string(ActionDeferred), Reason: "先不打断", ExecutorRef: "secret-ref",
		ArgsJSON: "{\"secret\":true}", CreatedAt: thoughtAt.Add(30 * time.Second),
	})
	mustCreateMindRecord(t, ms, &eventRecord{
		ID: 31, EventID: "event-old", DeviceID: "dev-001", Type: string(EventElderSpoke),
		Source: string(SourceXiaozhi), At: eventAt, Summary: "妈妈说刚刚在看电视", PayloadJSON: "{\"raw\":\"secret-payload\"}",
	})
	mustCreateMindRecord(t, ms, &reflectionRecord{
		ID: 41, ReflectionID: "reflection-old", DeviceID: "dev-001", At: reflectionAt,
		EpisodeSummary: "今天安静等待更合适", BehaviorLessonsJSON: "[\"先观察\"]",
		StateAdjustmentsJSON: "{\"quietness\":0.1}",
	})
	mustCreateMindRecord(t, ms, &eventRecord{
		ID: 99, EventID: "event-other", DeviceID: "dev-002", Type: string(EventDeviceOnline),
		Source: string(SourceDomain), At: thoughtAt.Add(time.Hour), Summary: "其他设备事件",
	})

	got, err := NewReadService(ms).Timeline(ctx, TimelineQuery{DeviceID: "dev-001", Kind: TimelineKindAll, Limit: 10})
	if err != nil {
		t.Fatalf("Timeline all: %v", err)
	}
	if got.HasMore || got.NextCursor != "" {
		t.Fatalf("pagination = hasMore %v cursor %q, want no more data", got.HasMore, got.NextCursor)
	}
	if len(got.Items) != 3 {
		t.Fatalf("items = %+v, want thought+event+reflection with action merged into thought", got.Items)
	}
	if got.Items[0].Kind != TimelineKindThought || got.Items[0].Text != "先听一听妈妈是不是需要空间" {
		t.Fatalf("first item = %+v, want newest thought by real instant", got.Items[0])
	}
	if got.Items[0].Decision == nil || got.Items[0].Decision.Type != ActionWait || got.Items[0].Decision.Reason != "先不打断" {
		t.Fatalf("thought decision = %+v, want paired action decision", got.Items[0].Decision)
	}
	if got.Items[1].Kind != TimelineKindEvent || got.Items[1].Text != "妈妈说刚刚在看电视" {
		t.Fatalf("second item = %+v, want +08 event after newer UTC thought", got.Items[1])
	}
	if got.Items[2].Kind != TimelineKindReflection || got.Items[2].Text != "今天安静等待更合适" {
		t.Fatalf("third item = %+v, want reflection", got.Items[2])
	}
	body := mustJSON(t, got)
	for _, forbidden := range []string{"secret-payload", "secret-event", "secret-memory", "secret-ref", "stateAdjustments", "raw"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("timeline JSON leaked %q: %s", forbidden, body)
		}
	}
}

func TestReadServiceTimelineSupportsActionFilterAndOpaqueCursor(t *testing.T) {
	ms := newMindStoreForTest(t)
	ctx := context.Background()
	base := time.Date(2026, 6, 20, 8, 0, 0, 0, time.UTC)
	for i, label := range []string{"一", "二", "三"} {
		thoughtID := "thought-page-" + label
		mustCreateMindRecord(t, ms, &thoughtRecord{
			ID: uint(100 + i), ThoughtID: thoughtID, DeviceID: "dev-001", At: base.Add(time.Duration(i) * time.Minute),
			Content: "念头" + label, DriveName: DriveCare, Status: string(ThoughtPending),
		})
		mustCreateMindRecord(t, ms, &actionRecord{
			ID: uint(200 + i), ActionID: "action-page-" + label, DeviceID: "dev-001",
			IntentionID: "intention-" + thoughtID, Type: string(ActionSpeak), Status: string(ActionExecuted),
			Reason: "选择" + label, Text: "说话" + label, CreatedAt: base.Add(time.Duration(i)*time.Minute + 10*time.Second),
		})
	}
	svc := NewReadService(ms)
	first, err := svc.Timeline(ctx, TimelineQuery{DeviceID: "dev-001", Kind: TimelineKindAction, Limit: 2})
	if err != nil {
		t.Fatalf("Timeline first: %v", err)
	}
	if !first.HasMore || first.NextCursor == "" || len(first.Items) != 2 {
		t.Fatalf("first page = %+v, want two action items and a cursor", first)
	}
	if first.Items[0].Kind != TimelineKindAction || first.Items[0].RelatedThought != "念头三" || first.Items[0].Text != "说话三" {
		t.Fatalf("first action = %+v, want newest action with related thought", first.Items[0])
	}

	second, err := svc.Timeline(ctx, TimelineQuery{DeviceID: "dev-001", Kind: TimelineKindAction, Limit: 2, Cursor: first.NextCursor})
	if err != nil {
		t.Fatalf("Timeline second: %v", err)
	}
	if second.HasMore || second.NextCursor != "" || len(second.Items) != 1 {
		t.Fatalf("second page = %+v, want final single action", second)
	}
	seen := map[string]bool{}
	for _, item := range append(first.Items, second.Items...) {
		if seen[item.Text] {
			t.Fatalf("duplicate timeline item text %q across cursor pages", item.Text)
		}
		seen[item.Text] = true
	}

	_, err = svc.Timeline(ctx, TimelineQuery{DeviceID: "dev-001", Kind: TimelineKindAction, Cursor: "not-a-valid-cursor"})
	if !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("invalid cursor error = %v, want ErrInvalidCursor", err)
	}
}

func TestReadServiceTimelineCapsLimitAndRejectsUnknownKind(t *testing.T) {
	ms := newMindStoreForTest(t)
	ctx := context.Background()
	base := time.Date(2026, 6, 20, 8, 0, 0, 0, time.UTC)
	for i := 0; i < 55; i++ {
		if err := ms.AppendEvent(ctx, Event{
			ID: fmt.Sprintf("event-limit-%02d", i), DeviceID: "dev-limit",
			Type: EventIdleTick, Source: SourceMind, At: base.Add(time.Duration(i) * time.Second),
			Summary: fmt.Sprintf("心智事件 %02d", i),
		}); err != nil {
			t.Fatalf("AppendEvent %d: %v", i, err)
		}
	}

	page, err := NewReadService(ms).Timeline(ctx, TimelineQuery{
		DeviceID: "dev-limit", Kind: TimelineKindEvent, Limit: 100,
	})
	if err != nil {
		t.Fatalf("Timeline: %v", err)
	}
	if len(page.Items) != 50 || !page.HasMore || page.NextCursor == "" {
		t.Fatalf("page = len %d hasMore %v cursor %q, want capped 50 with more data", len(page.Items), page.HasMore, page.NextCursor)
	}

	_, err = NewReadService(ms).Timeline(ctx, TimelineQuery{
		DeviceID: "dev-limit", Kind: TimelineKind("raw-table"),
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("unknown kind error = %v, want ErrInvalidInput", err)
	}
}

func mustCreateMindRecord(t *testing.T, ms *Store, value any) {
	t.Helper()
	if err := ms.db.Create(value).Error; err != nil {
		t.Fatalf("create %+v: %v", value, err)
	}
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json marshal: %v", err)
	}
	return string(body)
}
