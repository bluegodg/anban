package mind

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
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
	sqlDB, err := st.DB.DB()
	if err != nil {
		t.Fatalf("st.DB.DB: %v", err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("close test db: %v", err)
		}
	})

	ms := NewStore(st.DB)
	if err := ms.AutoMigrate(); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	return ms
}

func TestStoreAutoMigrateCreatesMindTables(t *testing.T) {
	ms := newMindStoreForTest(t)

	tables := []string{
		"mind_events",
		"mind_situations",
		"mind_memories",
		"mind_self_states",
		"mind_thoughts",
		"mind_intentions",
		"mind_actions",
		"mind_feedback",
		"mind_reflections",
		"mind_life_states",
	}
	expected := make(map[string]bool, len(tables))
	for _, table := range tables {
		expected[table] = true
		t.Run(table, func(t *testing.T) {
			if !ms.db.Migrator().HasTable(table) {
				t.Fatalf("expected table %q to exist", table)
			}
		})
	}

	var actualTables []string
	if err := ms.db.Raw("SELECT name FROM sqlite_master WHERE type = 'table' AND name LIKE 'mind_%'").Scan(&actualTables).Error; err != nil {
		t.Fatalf("list mind tables: %v", err)
	}
	if len(actualTables) != len(expected) {
		t.Fatalf("mind tables = %+v, want exactly %+v", actualTables, tables)
	}
	for _, table := range actualTables {
		if !expected[table] {
			t.Fatalf("unexpected mind table %q among %+v", table, actualTables)
		}
	}
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

func TestStoreAppendEventDuplicateReturnsErrDuplicateEvent(t *testing.T) {
	ms := newMindStoreForTest(t)
	ctx := context.Background()
	event := Event{
		ID:         "evt-duplicate",
		DeviceID:   "dev-001",
		Type:       EventReminderDue,
		Source:     SourceScheduler,
		At:         time.Date(2026, 6, 16, 8, 0, 0, 0, time.UTC),
		Summary:    "早间提醒",
		Salience:   0.8,
		Emotion:    "neutral",
		Confidence: 1,
	}

	if err := ms.AppendEvent(ctx, event); err != nil {
		t.Fatalf("AppendEvent first: %v", err)
	}
	if err := ms.AppendEvent(ctx, event); !errors.Is(err, ErrDuplicateEvent) {
		t.Fatalf("AppendEvent duplicate error = %v, want ErrDuplicateEvent", err)
	}
	var count int64
	if err := ms.db.Model(&eventRecord{}).Where("event_id = ?", "evt-duplicate").Count(&count).Error; err != nil {
		t.Fatalf("count duplicate event rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("event rows = %d, want 1", count)
	}
}

func TestStoreWithinTransactionRollsBackOnError(t *testing.T) {
	ms := newMindStoreForTest(t)
	ctx := context.Background()
	sentinel := errors.New("force rollback after append")

	err := ms.WithinTransaction(ctx, func(txStore *Store) error {
		if err := txStore.AppendEvent(ctx, Event{
			ID:         "evt-rollback",
			DeviceID:   "dev-001",
			Type:       EventReminderDue,
			Source:     SourceScheduler,
			At:         time.Date(2026, 6, 16, 8, 0, 0, 0, time.UTC),
			Summary:    "事务内写入后失败",
			Salience:   0.8,
			Emotion:    "neutral",
			Confidence: 1,
		}); err != nil {
			return err
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("WithinTransaction error = %v, want sentinel", err)
	}

	events, err := ms.ListRecentEvents(ctx, "dev-001", 10)
	if err != nil {
		t.Fatalf("ListRecentEvents: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("events after rollback = %+v, want none", events)
	}
}

func TestStoreUpsertsSelfStateAndLifeState(t *testing.T) {
	ms := newMindStoreForTest(t)
	ctx := context.Background()
	at := time.Date(2026, 6, 16, 9, 0, 0, 0, time.UTC)

	state := SelfState{
		DeviceID:          "dev-001",
		At:                at,
		Warmth:            0.6,
		Concern:           0.4,
		FamilyWeight:      0.6,
		PetWeight:         0.25,
		StewardWeight:     0.15,
		ProcessedEventIDs: []string{"evt-1", "evt-2"},
	}
	if err := ms.SaveSelfState(ctx, state); err != nil {
		t.Fatalf("SaveSelfState: %v", err)
	}
	var selfStateRec selfStateRecord
	if err := ms.db.Where("device_id = ?", "dev-001").First(&selfStateRec).Error; err != nil {
		t.Fatalf("read self state record: %v", err)
	}
	selfStateCreatedAt := selfStateRec.CreatedAt

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
	var selfStateCount int64
	if err := ms.db.Model(&selfStateRecord{}).Where("device_id = ?", "dev-001").Count(&selfStateCount).Error; err != nil {
		t.Fatalf("count self state records: %v", err)
	}
	if selfStateCount != 1 {
		t.Fatalf("self state rows = %d, want 1", selfStateCount)
	}
	if err := ms.db.Where("device_id = ?", "dev-001").First(&selfStateRec).Error; err != nil {
		t.Fatalf("read updated self state record: %v", err)
	}
	if !selfStateRec.CreatedAt.Equal(selfStateCreatedAt) {
		t.Fatalf("self state created_at = %v, want preserved %v", selfStateRec.CreatedAt, selfStateCreatedAt)
	}
	if !slices.Equal(got.ProcessedEventIDs, []string{"evt-1", "evt-2"}) {
		t.Fatalf("processed event IDs = %+v, want initial IDs", got.ProcessedEventIDs)
	}

	state.ProcessedEventIDs = []string{"evt-3", "evt-4"}
	if err := ms.SaveSelfState(ctx, state); err != nil {
		t.Fatalf("SaveSelfState watermark update: %v", err)
	}
	got, err = ms.GetSelfState(ctx, "dev-001")
	if err != nil {
		t.Fatalf("GetSelfState watermark update: %v", err)
	}
	if !slices.Equal(got.ProcessedEventIDs, []string{"evt-3", "evt-4"}) {
		t.Fatalf("processed event IDs = %+v, want updated IDs", got.ProcessedEventIDs)
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

	life.At = at.Add(time.Hour)
	life.TodayTheme = "慢慢把精神养回来"
	life.LingeringThoughts = []string{"午后想起老朋友", "晚饭后再聊老歌"}
	life.SocialEnergy = 0.8
	if err := ms.SaveLifeState(ctx, life); err != nil {
		t.Fatalf("SaveLifeState update: %v", err)
	}
	gotLife, err = ms.GetLifeState(ctx, "dev-001")
	if err != nil {
		t.Fatalf("GetLifeState updated: %v", err)
	}
	if gotLife.TodayTheme != "慢慢把精神养回来" || gotLife.SocialEnergy != 0.8 || !slices.Equal(gotLife.LingeringThoughts, []string{"午后想起老朋友", "晚饭后再聊老歌"}) {
		t.Fatalf("updated life = %+v, want updated theme, social energy, and lingering thoughts", gotLife)
	}
	var lifeCount int64
	if err := ms.db.Model(&lifeStateRecord{}).Where("device_id = ?", "dev-001").Count(&lifeCount).Error; err != nil {
		t.Fatalf("count life state records: %v", err)
	}
	if lifeCount != 1 {
		t.Fatalf("life state rows = %d, want 1", lifeCount)
	}
}

func TestStoreGetStateReturnsNotFoundForMissingDevice(t *testing.T) {
	ms := newMindStoreForTest(t)
	ctx := context.Background()

	if _, err := ms.GetSelfState(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetSelfState error = %v, want ErrNotFound", err)
	}
	if _, err := ms.GetLifeState(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetLifeState error = %v, want ErrNotFound", err)
	}
}

func TestStorePersistsSituationMemoryAndIntention(t *testing.T) {
	ms := newMindStoreForTest(t)
	ctx := context.Background()
	at := time.Date(2026, 6, 16, 9, 30, 0, 0, time.UTC)
	lastUsedAt := at.Add(time.Hour)

	situation := Situation{
		DeviceID:        "dev-001",
		At:              at,
		TimeOfDay:       "morning",
		ElderPresence:   "present",
		InteractionMode: "quiet_presence",
		ActivityLevel:   "low",
		EmotionalTone:   "calm",
		SocialContext:   "alone",
		OpenLoops:       []string{"ask about breakfast", "follow up on sleep"},
		Constraints:     []string{"do not interrupt music"},
	}
	if err := ms.SaveSituation(ctx, situation); err != nil {
		t.Fatalf("SaveSituation: %v", err)
	}

	memory := MemoryItem{
		ID:               "memory-1",
		DeviceID:         "dev-001",
		Kind:             MemoryPreference,
		Content:          "喜欢早上听老歌",
		EvidenceEventIDs: []string{"evt-1", "evt-2"},
		Importance:       0.8,
		Confidence:       0.9,
		CreatedAt:        at,
		UpdatedAt:        at,
		LastUsedAt:       &lastUsedAt,
		DecayPolicy:      "keep",
	}
	if err := ms.SaveMemory(ctx, memory); err != nil {
		t.Fatalf("SaveMemory: %v", err)
	}

	intention := Intention{
		ID:        "intention-1",
		DeviceID:  "dev-001",
		ThoughtID: "thought-1",
		Kind:      IntentionCheckIn,
		Goal:      "确认老人今天状态",
		Priority:  0.7,
	}
	if err := ms.SaveIntention(ctx, intention); err != nil {
		t.Fatalf("SaveIntention: %v", err)
	}

	var situationRec situationRecord
	if err := ms.db.Where("device_id = ?", "dev-001").First(&situationRec).Error; err != nil {
		t.Fatalf("read situation record: %v", err)
	}
	var openLoops []string
	if err := json.Unmarshal([]byte(situationRec.OpenLoopsJSON), &openLoops); err != nil {
		t.Fatalf("decode open loops: %v", err)
	}
	if len(openLoops) != 2 || openLoops[0] != "ask about breakfast" {
		t.Fatalf("open loops = %+v, want saved slice", openLoops)
	}
	var constraints []string
	if err := json.Unmarshal([]byte(situationRec.ConstraintsJSON), &constraints); err != nil {
		t.Fatalf("decode constraints: %v", err)
	}
	if len(constraints) != 1 || constraints[0] != "do not interrupt music" {
		t.Fatalf("constraints = %+v, want saved slice", constraints)
	}

	var memoryRec memoryRecord
	if err := ms.db.Where("memory_id = ?", "memory-1").First(&memoryRec).Error; err != nil {
		t.Fatalf("read memory record: %v", err)
	}
	var evidenceIDs []string
	if err := json.Unmarshal([]byte(memoryRec.EvidenceEventIDsJSON), &evidenceIDs); err != nil {
		t.Fatalf("decode evidence ids: %v", err)
	}
	if len(evidenceIDs) != 2 || evidenceIDs[1] != "evt-2" {
		t.Fatalf("evidence ids = %+v, want saved slice", evidenceIDs)
	}
	if memoryRec.LastUsedAt == nil || !memoryRec.LastUsedAt.Equal(lastUsedAt) {
		t.Fatalf("last used at = %+v, want %v", memoryRec.LastUsedAt, lastUsedAt)
	}

	var intentionRec intentionRecord
	if err := ms.db.Where("intention_id = ?", "intention-1").First(&intentionRec).Error; err != nil {
		t.Fatalf("read intention record: %v", err)
	}
	if intentionRec.Kind != string(IntentionCheckIn) || intentionRec.Priority != 0.7 {
		t.Fatalf("intention = %+v, want saved kind and priority", intentionRec)
	}
}

func TestStorePersistsThoughtActionFeedbackReflection(t *testing.T) {
	ms := newMindStoreForTest(t)
	ctx := context.Background()
	at := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	scheduledFor := at.Add(15 * time.Minute)

	thought := Thought{
		ID: "thought-1", DeviceID: "dev-001", At: at, Content: "他今天比平时安静",
		SourceEventIDs: []string{"evt-1", "evt-2"}, RelatedMemoryIDs: []string{"memory-1", "memory-2"},
		DriveName: DriveCare, EmotionalTone: "quiet", Urgency: 0.3, CareValue: 0.8, Novelty: 0.2,
		InterruptionCost: 0.7, Intimacy: 0.6, Status: ThoughtPending,
	}
	if err := ms.SaveThought(ctx, thought); err != nil {
		t.Fatalf("SaveThought: %v", err)
	}
	action := Action{
		ID: "action-1", DeviceID: "dev-001", IntentionID: "intent-1", Type: ActionSpeak,
		Executor: "xiaozhi", Text: "我们听首老歌好吗", Args: map[string]any{"skipLLM": true, "volume": 0.4},
		ScheduledFor: &scheduledFor, Status: ActionPending, Reason: "适合轻声问候", Score: 0.64,
	}
	if err := ms.SaveAction(ctx, action); err != nil {
		t.Fatalf("SaveAction: %v", err)
	}
	feedback := Feedback{ID: "feedback-1", DeviceID: "dev-001", ActionID: "action-1", At: at, Kind: "implicit", Signal: "waited", EffectOnState: map[string]float64{"quietness": 0.02}, Notes: "选择等待"}
	if err := ms.SaveFeedback(ctx, feedback); err != nil {
		t.Fatalf("SaveFeedback: %v", err)
	}
	reflection := Reflection{
		ID: "reflection-1", DeviceID: "dev-001", At: at, EpisodeSummary: "夜晚保持安静",
		MemoryIDs: []string{"memory-1", "memory-2"}, StateAdjustments: map[string]float64{"quietness": 0.02, "patience": 0.01},
		BehaviorLessons: []string{"夜晚长沉默先观察", "先低打扰确认"},
	}
	if err := ms.SaveReflection(ctx, reflection); err != nil {
		t.Fatalf("SaveReflection: %v", err)
	}

	var thoughtRec thoughtRecord
	if err := ms.db.Where("thought_id = ?", "thought-1").First(&thoughtRec).Error; err != nil {
		t.Fatalf("read thought record: %v", err)
	}
	if thoughtRec.Content != "他今天比平时安静" || thoughtRec.DriveName != DriveCare || thoughtRec.Status != string(ThoughtPending) {
		t.Fatalf("thought = %+v, want saved content drive and status", thoughtRec)
	}
	var sourceEventIDs []string
	if err := json.Unmarshal([]byte(thoughtRec.SourceEventIDsJSON), &sourceEventIDs); err != nil {
		t.Fatalf("decode thought source event ids: %v", err)
	}
	if !slices.Equal(sourceEventIDs, []string{"evt-1", "evt-2"}) {
		t.Fatalf("source event ids = %+v, want saved slice", sourceEventIDs)
	}
	var relatedMemoryIDs []string
	if err := json.Unmarshal([]byte(thoughtRec.RelatedMemoryIDsJSON), &relatedMemoryIDs); err != nil {
		t.Fatalf("decode thought related memory ids: %v", err)
	}
	if !slices.Equal(relatedMemoryIDs, []string{"memory-1", "memory-2"}) {
		t.Fatalf("related memory ids = %+v, want saved slice", relatedMemoryIDs)
	}

	var actionRec actionRecord
	if err := ms.db.Where("action_id = ?", "action-1").First(&actionRec).Error; err != nil {
		t.Fatalf("read action record: %v", err)
	}
	if actionRec.Executor != "xiaozhi" || actionRec.Text != "我们听首老歌好吗" || actionRec.Status != string(ActionPending) {
		t.Fatalf("action = %+v, want saved executor text and status", actionRec)
	}
	if actionRec.ScheduledFor == nil || !actionRec.ScheduledFor.Equal(scheduledFor) {
		t.Fatalf("action scheduled for = %+v, want %v", actionRec.ScheduledFor, scheduledFor)
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(actionRec.ArgsJSON), &args); err != nil {
		t.Fatalf("decode action args: %v", err)
	}
	if args["skipLLM"] != true || args["volume"] != 0.4 {
		t.Fatalf("action args = %+v, want skipLLM true and volume 0.4", args)
	}
	createdAt := actionRec.CreatedAt

	statusOnlyAction := Action{
		ID:          "action-1",
		Status:      ActionExecuted,
		Reason:      "老人回应了老歌",
		ExecutorRef: "message:12",
	}
	if err := ms.SaveAction(ctx, statusOnlyAction); err != nil {
		t.Fatalf("SaveAction update: %v", err)
	}
	var actionCount int64
	if err := ms.db.Model(&actionRecord{}).Where("action_id = ?", "action-1").Count(&actionCount).Error; err != nil {
		t.Fatalf("count action records: %v", err)
	}
	if actionCount != 1 {
		t.Fatalf("action rows = %d, want 1", actionCount)
	}
	if err := ms.db.Where("action_id = ?", "action-1").First(&actionRec).Error; err != nil {
		t.Fatalf("read updated action record: %v", err)
	}
	if !actionRec.CreatedAt.Equal(createdAt) {
		t.Fatalf("action created_at = %v, want preserved %v", actionRec.CreatedAt, createdAt)
	}
	if actionRec.DeviceID != "dev-001" || actionRec.IntentionID != "intent-1" || actionRec.Type != string(ActionSpeak) || actionRec.Executor != "xiaozhi" {
		t.Fatalf("updated action identity = %+v, want original device, intention, type, and executor", actionRec)
	}
	if actionRec.Text != "我们听首老歌好吗" || actionRec.ArgsJSON == "" || actionRec.ScheduledFor == nil || !actionRec.ScheduledFor.Equal(scheduledFor) {
		t.Fatalf("updated action plan = %+v, want original text, args, and schedule", actionRec)
	}
	if actionRec.Score != 0.64 {
		t.Fatalf("updated action score = %v, want preserved 0.64", actionRec.Score)
	}
	if actionRec.Status != string(ActionExecuted) || actionRec.Reason != "老人回应了老歌" || actionRec.ExecutorRef != "message:12" {
		t.Fatalf("updated action = %+v, want executed reason and executor ref", actionRec)
	}

	var feedbackRec feedbackRecord
	if err := ms.db.Where("feedback_id = ?", "feedback-1").First(&feedbackRec).Error; err != nil {
		t.Fatalf("read feedback record: %v", err)
	}
	if feedbackRec.Kind != "implicit" || feedbackRec.Signal != "waited" || feedbackRec.Notes != "选择等待" {
		t.Fatalf("feedback = %+v, want saved kind signal and notes", feedbackRec)
	}
	var effects map[string]float64
	if err := json.Unmarshal([]byte(feedbackRec.EffectOnStateJSON), &effects); err != nil {
		t.Fatalf("decode feedback effects: %v", err)
	}
	if effects["quietness"] != 0.02 {
		t.Fatalf("feedback effects = %+v, want quietness 0.02", effects)
	}

	var reflectionRec reflectionRecord
	if err := ms.db.Where("reflection_id = ?", "reflection-1").First(&reflectionRec).Error; err != nil {
		t.Fatalf("read reflection record: %v", err)
	}
	var reflectionMemoryIDs []string
	if err := json.Unmarshal([]byte(reflectionRec.MemoryIDsJSON), &reflectionMemoryIDs); err != nil {
		t.Fatalf("decode reflection memory ids: %v", err)
	}
	if !slices.Equal(reflectionMemoryIDs, []string{"memory-1", "memory-2"}) {
		t.Fatalf("reflection memory ids = %+v, want saved slice", reflectionMemoryIDs)
	}
	var stateAdjustments map[string]float64
	if err := json.Unmarshal([]byte(reflectionRec.StateAdjustmentsJSON), &stateAdjustments); err != nil {
		t.Fatalf("decode reflection state adjustments: %v", err)
	}
	if stateAdjustments["quietness"] != 0.02 || stateAdjustments["patience"] != 0.01 {
		t.Fatalf("state adjustments = %+v, want saved map", stateAdjustments)
	}
	var behaviorLessons []string
	if err := json.Unmarshal([]byte(reflectionRec.BehaviorLessonsJSON), &behaviorLessons); err != nil {
		t.Fatalf("decode reflection behavior lessons: %v", err)
	}
	if !slices.Equal(behaviorLessons, []string{"夜晚长沉默先观察", "先低打扰确认"}) {
		t.Fatalf("behavior lessons = %+v, want saved slice", behaviorLessons)
	}
}

func TestStoreSaveReflectionIsIdempotent(t *testing.T) {
	ms := newMindStoreForTest(t)
	ctx := context.Background()
	at := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	reflection := Reflection{
		ID: "reflection-duplicate", DeviceID: "dev-001", At: at, EpisodeSummary: "保持安静",
		MemoryIDs: []string{"memory-1"}, StateAdjustments: map[string]float64{"quietness": 0.02},
		BehaviorLessons: []string{"先观察"},
	}

	if err := ms.SaveReflection(ctx, reflection); err != nil {
		t.Fatalf("SaveReflection first: %v", err)
	}
	reflection.EpisodeSummary = "重复调用不覆盖"
	if err := ms.SaveReflection(ctx, reflection); err != nil {
		t.Fatalf("SaveReflection duplicate: %v", err)
	}

	var count int64
	if err := ms.db.Model(&reflectionRecord{}).Where("reflection_id = ?", "reflection-duplicate").Count(&count).Error; err != nil {
		t.Fatalf("count reflection rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("reflection rows = %d, want 1", count)
	}
	var rec reflectionRecord
	if err := ms.db.Where("reflection_id = ?", "reflection-duplicate").First(&rec).Error; err != nil {
		t.Fatalf("read reflection: %v", err)
	}
	if rec.EpisodeSummary != "保持安静" {
		t.Fatalf("EpisodeSummary = %q, want first insert preserved", rec.EpisodeSummary)
	}
}
