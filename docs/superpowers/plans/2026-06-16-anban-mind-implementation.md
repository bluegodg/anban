# AnBan Mind Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the complete AnBan Mind layer that turns scattered proactive features into a unified companion mind with event stream, situation, memory, self-state, drives, thoughts, intentions, behavior selection, expression gate, reflection, and life loop.

**Architecture:** Keep Scheme C intact: `anban-code` owns the companion mind, while xiaozhi remains the voice/device runtime. Implement the mind as a new `server/internal/mind` module with small subpackages; existing `domains/*` become action executors and xiaozhiclient remains the only xiaozhi adapter.

**Tech Stack:** Go 1.25, GORM, glebarez/sqlite, Gin, robfig/cron, existing `xiaozhiclient`, existing `scheduler`, existing `llm` package.

---

## Scope Check

The approved spec covers several subsystems: event stream, state, drives, thoughts, behavior, expression, reflection, life loop, executor adapters, and route/domain integration. This plan keeps them in one plan set because the subsystems form one data path and each task produces working, testable software on its own. Do not start Task N+1 until Task N is committed and `go test ./...` is green.

## File Structure

Create these files:

- `server/internal/mind/types.go`: shared public mind types and the `Engine` interface.
- `server/internal/mind/store.go`: GORM persistence for events, state, thoughts, actions, feedback, reflections, and life state.
- `server/internal/mind/store_test.go`: persistence tests.
- `server/internal/mind/situation/situation.go`: deterministic situation builder from recent events.
- `server/internal/mind/situation/situation_test.go`: situation tests.
- `server/internal/mind/selfstate/selfstate.go`: default self-state and event-driven state adjustments.
- `server/internal/mind/selfstate/selfstate_test.go`: self-state tests.
- `server/internal/mind/drives/drives.go`: drive activation.
- `server/internal/mind/drives/drives_test.go`: drive tests.
- `server/internal/mind/thoughts/thoughts.go`: deterministic thought generation fallback.
- `server/internal/mind/thoughts/thoughts_test.go`: thought tests.
- `server/internal/mind/behavior/behavior.go`: action candidate scoring and controlled variability.
- `server/internal/mind/behavior/behavior_test.go`: behavior tests.
- `server/internal/mind/expression/expression.go`: expression gate and decision reasons.
- `server/internal/mind/expression/expression_test.go`: expression gate tests.
- `server/internal/mind/engine/engine.go`: orchestration pipeline implementing `mind.Engine`.
- `server/internal/mind/engine/engine_test.go`: engine integration tests with fake executors.
- `server/internal/mind/executors/types.go`: executor interfaces and action result types.
- `server/internal/mind/executors/domain_adapters.go`: adapters wrapping existing domain services.
- `server/internal/mind/executors/domain_adapters_test.go`: adapter tests.
- `server/internal/mind/reflection/reflection.go`: feedback-to-memory/state reflection logic.
- `server/internal/mind/reflection/reflection_test.go`: reflection tests.
- `server/internal/mind/life/life.go`: life loop state update logic.
- `server/internal/mind/life/life_test.go`: life loop tests.
- `server/internal/mind/simulation/day_test.go`: one-day continuity simulation.

Modify these files:

- `server/cmd/anban/main.go`: migrate mind tables, construct the mind engine, wire executors, schedule idle/reflection/life ticks.
- `server/internal/domains/message/service.go`: split queue and play so message events can pass through Mind before speaking.
- `server/internal/domains/message/service_test.go`: preserve existing direct service behavior and add Mind-oriented queue/play tests.
- `server/internal/domains/reminder/service.go`: emit `reminder_due` through Mind before speaking.
- `server/internal/domains/reminder/service_test.go`: add reminder-through-Mind tests.
- `server/internal/domains/greeting/service.go`: keep executor capability and let Mind own proactive decisions.
- `server/internal/domains/greeting/service_test.go`: add executor-facing tests.
- `server/internal/domains/vision/service.go`: emit presence observations as Mind events instead of directly deciding greeting when Mind is wired.
- `server/internal/domains/vision/service_test.go`: add Mind event sink tests.
- `server/internal/architecture/architecture_test.go`: assert `domains` do not import `mind`; orchestration must live in `cmd/anban`, childapi, scheduler, or mind adapters.
- `docs/superpowers/specs/2026-06-16-anban-mind-design.md`: append a short implementation status link after all tasks are complete.

## Shared Test Command

Every task uses this command from `server/` unless a narrower command is listed:

```powershell
$env:GOPROXY="https://goproxy.cn,direct"; $env:GOSUMDB="off"; $env:CGO_ENABLED="0"; go test ./...
```

---

### Task 1: Core Mind Types

**Files:**
- Create: `server/internal/mind/types.go`
- Test: compile through package tests in later tasks

- [ ] **Step 1: Create the shared type file**

Create `server/internal/mind/types.go` with this content:

```go
package mind

import (
	"context"
	"time"
)

type EventType string

const (
	EventElderSpoke            EventType = "elder_spoke"
	EventAssistantSpoke        EventType = "assistant_spoke"
	EventChildMessageReceived  EventType = "child_message_received"
	EventReminderCreated       EventType = "reminder_created"
	EventReminderDue           EventType = "reminder_due"
	EventReminderAcknowledged  EventType = "reminder_acknowledged"
	EventGreetingRequested     EventType = "greeting_requested"
	EventPresenceSeen          EventType = "presence_seen"
	EventPresenceAbsent        EventType = "presence_absent"
	EventVisionObservation     EventType = "vision_observation"
	EventDeviceOnline          EventType = "device_online"
	EventDeviceOffline         EventType = "device_offline"
	EventLongSilence           EventType = "long_silence"
	EventProfileUpdated        EventType = "profile_updated"
	EventMemoryDistilled       EventType = "memory_distilled"
	EventActionExecuted        EventType = "action_executed"
	EventFeedbackObserved      EventType = "feedback_observed"
	EventIdleTick              EventType = "idle_tick"
	EventReflectionTick        EventType = "reflection_tick"
	EventLifeTick              EventType = "life_tick"
)

type EventSource string

const (
	SourceXiaozhi   EventSource = "xiaozhi"
	SourceChildAPI  EventSource = "childapi"
	SourceScheduler EventSource = "scheduler"
	SourceVision    EventSource = "vision"
	SourceDomain    EventSource = "domain"
	SourceMind      EventSource = "mind"
)

type Event struct {
	ID         string
	DeviceID   string
	Type       EventType
	Source     EventSource
	At         time.Time
	Summary    string
	Payload    map[string]any
	Salience   float64
	Emotion    string
	Confidence float64
}

type Situation struct {
	DeviceID        string
	At              time.Time
	TimeOfDay       string
	ElderPresence   string
	InteractionMode string
	ActivityLevel   string
	EmotionalTone   string
	SocialContext   string
	OpenLoops       []string
	Constraints     []string
}

type MemoryKind string

const (
	MemoryProfileFact    MemoryKind = "profile_fact"
	MemoryPreference     MemoryKind = "preference"
	MemoryRoutine        MemoryKind = "routine"
	MemoryEpisodic       MemoryKind = "episodic"
	MemoryRelationship   MemoryKind = "relationship"
	MemoryEmotionalTrace MemoryKind = "emotional_trace"
	MemoryOpenLoop       MemoryKind = "open_loop"
	MemoryBehaviorLesson MemoryKind = "behavior_lesson"
)

type MemoryItem struct {
	ID               string
	DeviceID         string
	Kind             MemoryKind
	Content          string
	EvidenceEventIDs []string
	Importance       float64
	Confidence       float64
	CreatedAt        time.Time
	UpdatedAt        time.Time
	LastUsedAt        *time.Time
	DecayPolicy      string
}

type SelfState struct {
	DeviceID      string
	At            time.Time
	Warmth        float64
	Concern       float64
	Curiosity     float64
	Playfulness   float64
	Energy        float64
	Quietness     float64
	Patience      float64
	Confidence    float64
	FamilyWeight  float64
	PetWeight     float64
	StewardWeight float64
}

type Drive struct {
	Name           string
	Strength       float64
	Reason         string
	SourceEventIDs []string
}

const (
	DriveCompanionship = "companionship"
	DriveCare          = "care"
	DriveCuriosity     = "curiosity"
	DrivePlay          = "play"
	DriveStewardship   = "stewardship"
	DriveFamilyBridge  = "family_bridge"
	DriveQuietPresence = "quiet_presence"
)

type ThoughtStatus string

const (
	ThoughtPending    ThoughtStatus = "pending"
	ThoughtConverted  ThoughtStatus = "converted"
	ThoughtSuppressed ThoughtStatus = "suppressed"
	ThoughtArchived   ThoughtStatus = "archived"
)

type Thought struct {
	ID               string
	DeviceID         string
	At               time.Time
	Content          string
	SourceEventIDs   []string
	RelatedMemoryIDs []string
	DriveName        string
	EmotionalTone    string
	Urgency          float64
	CareValue        float64
	Novelty          float64
	InterruptionCost float64
	Intimacy         float64
	Status           ThoughtStatus
}

type IntentionKind string

const (
	IntentionCheckIn     IntentionKind = "check_in"
	IntentionComfort     IntentionKind = "comfort"
	IntentionShare       IntentionKind = "share"
	IntentionRemind      IntentionKind = "remind"
	IntentionAsk         IntentionKind = "ask"
	IntentionObserve     IntentionKind = "observe"
	IntentionWait        IntentionKind = "wait"
	IntentionNotifyChild IntentionKind = "notify_child"
	IntentionUpdateMemory IntentionKind = "update_memory"
	IntentionSyncPrompt  IntentionKind = "sync_prompt"
)

type Intention struct {
	ID        string
	DeviceID  string
	ThoughtID string
	Kind      IntentionKind
	Goal      string
	Priority  float64
}

type ActionType string

const (
	ActionSpeak                 ActionType = "speak"
	ActionWait                  ActionType = "wait"
	ActionListen                ActionType = "listen"
	ActionCallMCPTool           ActionType = "call_mcp_tool"
	ActionSendChildNotification ActionType = "send_child_notification"
	ActionUpdateRolePrompt      ActionType = "update_role_prompt"
	ActionArchiveMemory         ActionType = "archive_memory"
	ActionScheduleRecheck       ActionType = "schedule_recheck"
	ActionSubtleExpression      ActionType = "subtle_expression"
	ActionSilentStateUpdate     ActionType = "silent_state_update"
)

type ActionStatus string

const (
	ActionPending   ActionStatus = "pending"
	ActionExecuted  ActionStatus = "executed"
	ActionDeferred  ActionStatus = "deferred"
	ActionSuppressed ActionStatus = "suppressed"
	ActionFailed    ActionStatus = "failed"
)

type Action struct {
	ID           string
	DeviceID     string
	IntentionID  string
	Type         ActionType
	Executor     string
	Text         string
	Args         map[string]any
	ScheduledFor *time.Time
	Status       ActionStatus
	Reason       string
	Score        float64
}

type Feedback struct {
	ID            string
	DeviceID      string
	ActionID      string
	At            time.Time
	Kind          string
	Signal        string
	EffectOnState map[string]float64
	Notes         string
}

type Reflection struct {
	ID              string
	DeviceID        string
	At              time.Time
	EpisodeSummary  string
	MemoryIDs       []string
	StateAdjustments map[string]float64
	BehaviorLessons []string
}

type LifeState struct {
	DeviceID                string
	At                      time.Time
	TodayTheme              string
	LingeringThoughts       []string
	SocialEnergy            float64
	CareFocus               string
	PlayfulnessTrend        float64
	RelationshipTemperature float64
}

type TimeWindow struct {
	From time.Time
	To   time.Time
}

type Engine interface {
	Ingest(ctx context.Context, event Event) ([]Action, error)
	TickIdle(ctx context.Context, deviceID string, at time.Time) ([]Action, error)
	Reflect(ctx context.Context, deviceID string, window TimeWindow) error
}
```

- [ ] **Step 2: Run package discovery**

Run:

```powershell
Set-Location server
$env:GOPROXY="https://goproxy.cn,direct"; $env:GOSUMDB="off"; $env:CGO_ENABLED="0"; go test ./internal/mind
```

Expected: PASS with `? github.com/bluegodg/anban/server/internal/mind [no test files]`.

- [ ] **Step 3: Commit**

```powershell
git add server/internal/mind/types.go
git commit -m "feat(mind): define core mind types"
```

---

### Task 2: Mind Store Persistence

**Files:**
- Create: `server/internal/mind/store.go`
- Create: `server/internal/mind/store_test.go`

- [ ] **Step 1: Write failing persistence tests**

Create `server/internal/mind/store_test.go`:

```go
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
```

- [ ] **Step 2: Run the failing tests**

Run:

```powershell
Set-Location server
$env:GOPROXY="https://goproxy.cn,direct"; $env:GOSUMDB="off"; $env:CGO_ENABLED="0"; go test ./internal/mind
```

Expected: FAIL with errors including `undefined: Store` and `undefined: NewStore`.

- [ ] **Step 3: Implement the store**

Create `server/internal/mind/store.go`:

```go
package mind

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
)

var ErrNotFound = errors.New("mind: not found")

type Store struct {
	db *gorm.DB
}

func NewStore(db *gorm.DB) *Store {
	return &Store{db: db}
}

func (s *Store) AutoMigrate() error {
	return s.db.AutoMigrate(
		&eventRecord{},
		&selfStateRecord{},
		&thoughtRecord{},
		&actionRecord{},
		&feedbackRecord{},
		&reflectionRecord{},
		&lifeStateRecord{},
	)
}

type eventRecord struct {
	ID          uint      `gorm:"primaryKey"`
	EventID     string    `gorm:"uniqueIndex;size:80;not null"`
	DeviceID    string    `gorm:"index;not null"`
	Type        string    `gorm:"size:60;index;not null"`
	Source      string    `gorm:"size:60;index;not null"`
	At          time.Time `gorm:"index;not null"`
	Summary     string    `gorm:"size:500"`
	PayloadJSON string    `gorm:"type:text"`
	Salience    float64
	Emotion     string `gorm:"size:60"`
	Confidence  float64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type selfStateRecord struct {
	ID            uint   `gorm:"primaryKey"`
	DeviceID      string `gorm:"uniqueIndex;not null"`
	At            time.Time
	Warmth        float64
	Concern       float64
	Curiosity     float64
	Playfulness   float64
	Energy        float64
	Quietness     float64
	Patience      float64
	Confidence    float64
	FamilyWeight  float64
	PetWeight     float64
	StewardWeight float64
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type thoughtRecord struct {
	ID                   uint   `gorm:"primaryKey"`
	ThoughtID            string `gorm:"uniqueIndex;size:80;not null"`
	DeviceID             string `gorm:"index;not null"`
	At                   time.Time
	Content              string `gorm:"size:1000"`
	SourceEventIDsJSON   string `gorm:"type:text"`
	RelatedMemoryIDsJSON string `gorm:"type:text"`
	DriveName            string `gorm:"size:60"`
	EmotionalTone        string `gorm:"size:60"`
	Urgency              float64
	CareValue            float64
	Novelty              float64
	InterruptionCost     float64
	Intimacy             float64
	Status               string `gorm:"size:30;index"`
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type actionRecord struct {
	ID           uint   `gorm:"primaryKey"`
	ActionID     string `gorm:"uniqueIndex;size:80;not null"`
	DeviceID     string `gorm:"index;not null"`
	IntentionID  string `gorm:"size:80;index"`
	Type         string `gorm:"size:60;index"`
	Executor     string `gorm:"size:80;index"`
	Text         string `gorm:"size:1000"`
	ArgsJSON     string `gorm:"type:text"`
	ScheduledFor *time.Time
	Status       string `gorm:"size:30;index"`
	Reason       string `gorm:"size:500"`
	Score        float64
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type feedbackRecord struct {
	ID                uint   `gorm:"primaryKey"`
	FeedbackID        string `gorm:"uniqueIndex;size:80;not null"`
	DeviceID          string `gorm:"index;not null"`
	ActionID          string `gorm:"size:80;index"`
	At                time.Time
	Kind              string `gorm:"size:60"`
	Signal            string `gorm:"size:80"`
	EffectOnStateJSON string `gorm:"type:text"`
	Notes             string `gorm:"size:500"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type reflectionRecord struct {
	ID                   uint   `gorm:"primaryKey"`
	ReflectionID         string `gorm:"uniqueIndex;size:80;not null"`
	DeviceID             string `gorm:"index;not null"`
	At                   time.Time
	EpisodeSummary       string `gorm:"size:1000"`
	MemoryIDsJSON        string `gorm:"type:text"`
	StateAdjustmentsJSON string `gorm:"type:text"`
	BehaviorLessonsJSON  string `gorm:"type:text"`
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type lifeStateRecord struct {
	ID                          uint   `gorm:"primaryKey"`
	DeviceID                    string `gorm:"uniqueIndex;not null"`
	At                          time.Time
	TodayTheme                  string `gorm:"size:200"`
	LingeringThoughtsJSON       string `gorm:"type:text"`
	SocialEnergy                float64
	CareFocus                   string `gorm:"size:200"`
	PlayfulnessTrend            float64
	RelationshipTemperature     float64
	CreatedAt                   time.Time
	UpdatedAt                   time.Time
}

func (s *Store) AppendEvent(ctx context.Context, event Event) error {
	payload, err := encodeJSON(event.Payload)
	if err != nil {
		return err
	}
	rec := eventRecord{
		EventID: event.ID, DeviceID: event.DeviceID, Type: string(event.Type), Source: string(event.Source),
		At: event.At, Summary: event.Summary, PayloadJSON: payload, Salience: event.Salience,
		Emotion: event.Emotion, Confidence: event.Confidence,
	}
	return s.db.WithContext(ctx).Create(&rec).Error
}

func (s *Store) ListRecentEvents(ctx context.Context, deviceID string, limit int) ([]Event, error) {
	if limit <= 0 {
		limit = 20
	}
	var records []eventRecord
	if err := s.db.WithContext(ctx).Where("device_id = ?", deviceID).Order("at desc, id desc").Limit(limit).Find(&records).Error; err != nil {
		return nil, err
	}
	out := make([]Event, 0, len(records))
	for _, rec := range records {
		payload := map[string]any{}
		if rec.PayloadJSON != "" {
			if err := json.Unmarshal([]byte(rec.PayloadJSON), &payload); err != nil {
				return nil, err
			}
		}
		out = append(out, Event{ID: rec.EventID, DeviceID: rec.DeviceID, Type: EventType(rec.Type), Source: EventSource(rec.Source), At: rec.At, Summary: rec.Summary, Payload: payload, Salience: rec.Salience, Emotion: rec.Emotion, Confidence: rec.Confidence})
	}
	return out, nil
}

func (s *Store) SaveSelfState(ctx context.Context, state SelfState) error {
	var existing selfStateRecord
	err := s.db.WithContext(ctx).Where("device_id = ?", state.DeviceID).First(&existing).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	rec := selfStateRecord{DeviceID: state.DeviceID, At: state.At, Warmth: state.Warmth, Concern: state.Concern, Curiosity: state.Curiosity, Playfulness: state.Playfulness, Energy: state.Energy, Quietness: state.Quietness, Patience: state.Patience, Confidence: state.Confidence, FamilyWeight: state.FamilyWeight, PetWeight: state.PetWeight, StewardWeight: state.StewardWeight}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return s.db.WithContext(ctx).Create(&rec).Error
	}
	rec.ID = existing.ID
	return s.db.WithContext(ctx).Save(&rec).Error
}

func (s *Store) GetSelfState(ctx context.Context, deviceID string) (SelfState, error) {
	var rec selfStateRecord
	err := s.db.WithContext(ctx).Where("device_id = ?", deviceID).First(&rec).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return SelfState{}, ErrNotFound
	}
	if err != nil {
		return SelfState{}, err
	}
	return SelfState{DeviceID: rec.DeviceID, At: rec.At, Warmth: rec.Warmth, Concern: rec.Concern, Curiosity: rec.Curiosity, Playfulness: rec.Playfulness, Energy: rec.Energy, Quietness: rec.Quietness, Patience: rec.Patience, Confidence: rec.Confidence, FamilyWeight: rec.FamilyWeight, PetWeight: rec.PetWeight, StewardWeight: rec.StewardWeight}, nil
}

func (s *Store) SaveThought(ctx context.Context, thought Thought) error {
	sourceIDs, err := encodeJSON(thought.SourceEventIDs)
	if err != nil { return err }
	memoryIDs, err := encodeJSON(thought.RelatedMemoryIDs)
	if err != nil { return err }
	rec := thoughtRecord{ThoughtID: thought.ID, DeviceID: thought.DeviceID, At: thought.At, Content: thought.Content, SourceEventIDsJSON: sourceIDs, RelatedMemoryIDsJSON: memoryIDs, DriveName: thought.DriveName, EmotionalTone: thought.EmotionalTone, Urgency: thought.Urgency, CareValue: thought.CareValue, Novelty: thought.Novelty, InterruptionCost: thought.InterruptionCost, Intimacy: thought.Intimacy, Status: string(thought.Status)}
	return s.db.WithContext(ctx).Create(&rec).Error
}

func (s *Store) SaveAction(ctx context.Context, action Action) error {
	args, err := encodeJSON(action.Args)
	if err != nil { return err }
	rec := actionRecord{ActionID: action.ID, DeviceID: action.DeviceID, IntentionID: action.IntentionID, Type: string(action.Type), Executor: action.Executor, Text: action.Text, ArgsJSON: args, ScheduledFor: action.ScheduledFor, Status: string(action.Status), Reason: action.Reason, Score: action.Score}
	return s.db.WithContext(ctx).Create(&rec).Error
}

func (s *Store) SaveFeedback(ctx context.Context, feedback Feedback) error {
	effects, err := encodeJSON(feedback.EffectOnState)
	if err != nil { return err }
	rec := feedbackRecord{FeedbackID: feedback.ID, DeviceID: feedback.DeviceID, ActionID: feedback.ActionID, At: feedback.At, Kind: feedback.Kind, Signal: feedback.Signal, EffectOnStateJSON: effects, Notes: feedback.Notes}
	return s.db.WithContext(ctx).Create(&rec).Error
}

func (s *Store) SaveReflection(ctx context.Context, reflection Reflection) error {
	memoryIDs, err := encodeJSON(reflection.MemoryIDs)
	if err != nil { return err }
	adjustments, err := encodeJSON(reflection.StateAdjustments)
	if err != nil { return err }
	lessons, err := encodeJSON(reflection.BehaviorLessons)
	if err != nil { return err }
	rec := reflectionRecord{ReflectionID: reflection.ID, DeviceID: reflection.DeviceID, At: reflection.At, EpisodeSummary: reflection.EpisodeSummary, MemoryIDsJSON: memoryIDs, StateAdjustmentsJSON: adjustments, BehaviorLessonsJSON: lessons}
	return s.db.WithContext(ctx).Create(&rec).Error
}

func (s *Store) SaveLifeState(ctx context.Context, life LifeState) error {
	thoughts, err := encodeJSON(life.LingeringThoughts)
	if err != nil { return err }
	var existing lifeStateRecord
	err = s.db.WithContext(ctx).Where("device_id = ?", life.DeviceID).First(&existing).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) { return err }
	rec := lifeStateRecord{DeviceID: life.DeviceID, At: life.At, TodayTheme: life.TodayTheme, LingeringThoughtsJSON: thoughts, SocialEnergy: life.SocialEnergy, CareFocus: life.CareFocus, PlayfulnessTrend: life.PlayfulnessTrend, RelationshipTemperature: life.RelationshipTemperature}
	if errors.Is(err, gorm.ErrRecordNotFound) { return s.db.WithContext(ctx).Create(&rec).Error }
	rec.ID = existing.ID
	return s.db.WithContext(ctx).Save(&rec).Error
}

func (s *Store) GetLifeState(ctx context.Context, deviceID string) (LifeState, error) {
	var rec lifeStateRecord
	err := s.db.WithContext(ctx).Where("device_id = ?", deviceID).First(&rec).Error
	if errors.Is(err, gorm.ErrRecordNotFound) { return LifeState{}, ErrNotFound }
	if err != nil { return LifeState{}, err }
	var thoughts []string
	if rec.LingeringThoughtsJSON != "" {
		if err := json.Unmarshal([]byte(rec.LingeringThoughtsJSON), &thoughts); err != nil { return LifeState{}, err }
	}
	return LifeState{DeviceID: rec.DeviceID, At: rec.At, TodayTheme: rec.TodayTheme, LingeringThoughts: thoughts, SocialEnergy: rec.SocialEnergy, CareFocus: rec.CareFocus, PlayfulnessTrend: rec.PlayfulnessTrend, RelationshipTemperature: rec.RelationshipTemperature}, nil
}

func encodeJSON(value any) (string, error) {
	if value == nil {
		return "", nil
	}
	body, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(body), nil
}
```

- [ ] **Step 4: Run tests**

Run:

```powershell
Set-Location server
$env:GOPROXY="https://goproxy.cn,direct"; $env:GOSUMDB="off"; $env:CGO_ENABLED="0"; go test ./internal/mind
```

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add server/internal/mind/store.go server/internal/mind/store_test.go
git commit -m "feat(mind): persist mind event and state records"
```

---

### Task 3: Situation and SelfState Builders

**Files:**
- Create: `server/internal/mind/situation/situation.go`
- Create: `server/internal/mind/situation/situation_test.go`
- Create: `server/internal/mind/selfstate/selfstate.go`
- Create: `server/internal/mind/selfstate/selfstate_test.go`

- [ ] **Step 1: Write situation tests**

Create `server/internal/mind/situation/situation_test.go`:

```go
package situation

import (
	"testing"
	"time"

	"github.com/bluegodg/anban/server/internal/mind"
)

func TestBuildMarksNightLongSilenceAsQuietObservation(t *testing.T) {
	at := time.Date(2026, 6, 16, 22, 10, 0, 0, time.UTC)
	got := Build("dev-001", at, []mind.Event{
		{ID: "evt-1", DeviceID: "dev-001", Type: mind.EventLongSilence, At: at.Add(-time.Minute), Summary: "老人 50 分钟无互动"},
	})
	if got.TimeOfDay != "night" {
		t.Fatalf("TimeOfDay = %q, want night", got.TimeOfDay)
	}
	if got.InteractionMode != "idle" || got.ActivityLevel != "low" {
		t.Fatalf("situation = %+v, want idle low activity", got)
	}
	if !has(got.Constraints, "avoid_long_speech") || !has(got.Constraints, "prefer_observation") {
		t.Fatalf("constraints = %+v, want avoid_long_speech and prefer_observation", got.Constraints)
	}
}

func TestBuildDetectsChildMessageSocialContext(t *testing.T) {
	at := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	got := Build("dev-001", at, []mind.Event{
		{ID: "evt-1", DeviceID: "dev-001", Type: mind.EventChildMessageReceived, At: at, Summary: "小明发来留言"},
	})
	if got.SocialContext != "child_waiting_reply" {
		t.Fatalf("SocialContext = %q, want child_waiting_reply", got.SocialContext)
	}
	if !has(got.OpenLoops, "child_message_pending") {
		t.Fatalf("OpenLoops = %+v, want child_message_pending", got.OpenLoops)
	}
}

func has(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Write self-state tests**

Create `server/internal/mind/selfstate/selfstate_test.go`:

```go
package selfstate

import (
	"testing"
	"time"

	"github.com/bluegodg/anban/server/internal/mind"
)

func TestDefaultStateUsesApprovedPersonaWeights(t *testing.T) {
	state := Default("dev-001", time.Date(2026, 6, 16, 8, 0, 0, 0, time.UTC))
	if state.FamilyWeight != 0.60 || state.PetWeight != 0.25 || state.StewardWeight != 0.15 {
		t.Fatalf("weights = family %.2f pet %.2f steward %.2f", state.FamilyWeight, state.PetWeight, state.StewardWeight)
	}
	if state.Warmth <= 0 || state.Patience <= 0 {
		t.Fatalf("state = %+v, want positive warmth and patience", state)
	}
}

func TestApplyEventsAdjustsConcernAndPlayfulness(t *testing.T) {
	at := time.Date(2026, 6, 16, 14, 0, 0, 0, time.UTC)
	state := Default("dev-001", at)
	updated := ApplyEvents(state, []mind.Event{
		{ID: "evt-1", DeviceID: "dev-001", Type: mind.EventLongSilence, At: at},
		{ID: "evt-2", DeviceID: "dev-001", Type: mind.EventPresenceSeen, At: at},
	})
	if updated.Concern <= state.Concern {
		t.Fatalf("Concern = %.2f, want greater than %.2f", updated.Concern, state.Concern)
	}
	if updated.Curiosity <= state.Curiosity {
		t.Fatalf("Curiosity = %.2f, want greater than %.2f", updated.Curiosity, state.Curiosity)
	}
}
```

- [ ] **Step 3: Run failing tests**

Run:

```powershell
Set-Location server
$env:GOPROXY="https://goproxy.cn,direct"; $env:GOSUMDB="off"; $env:CGO_ENABLED="0"; go test ./internal/mind/situation ./internal/mind/selfstate
```

Expected: FAIL with `undefined: Build`, `undefined: Default`, and `undefined: ApplyEvents`.

- [ ] **Step 4: Implement situation builder**

Create `server/internal/mind/situation/situation.go`:

```go
package situation

import (
	"time"

	"github.com/bluegodg/anban/server/internal/mind"
)

func Build(deviceID string, at time.Time, events []mind.Event) mind.Situation {
	out := mind.Situation{
		DeviceID: deviceID,
		At: at,
		TimeOfDay: timeOfDay(at),
		ElderPresence: "unknown",
		InteractionMode: "idle",
		ActivityLevel: "normal",
		EmotionalTone: "uncertain",
		SocialContext: "alone",
	}
	for _, event := range events {
		switch event.Type {
		case mind.EventPresenceSeen:
			out.ElderPresence = "present"
			out.ActivityLevel = "normal"
		case mind.EventPresenceAbsent:
			out.ElderPresence = "absent"
		case mind.EventLongSilence:
			out.InteractionMode = "idle"
			out.ActivityLevel = "low"
			out.EmotionalTone = "quiet"
			out.Constraints = addUnique(out.Constraints, "avoid_long_speech")
			out.Constraints = addUnique(out.Constraints, "prefer_observation")
		case mind.EventChildMessageReceived:
			out.SocialContext = "child_waiting_reply"
			out.OpenLoops = addUnique(out.OpenLoops, "child_message_pending")
		case mind.EventReminderDue:
			out.OpenLoops = addUnique(out.OpenLoops, "reminder_due")
		case mind.EventElderSpoke:
			out.ElderPresence = "present"
			out.InteractionMode = "conversation"
			out.ActivityLevel = "normal"
		}
	}
	if out.TimeOfDay == "night" {
		out.Constraints = addUnique(out.Constraints, "prefer_short_phrase")
	}
	return out
}

func timeOfDay(at time.Time) string {
	hour := at.In(time.FixedZone("CST", 8*3600)).Hour()
	switch {
	case hour >= 5 && hour < 11:
		return "morning"
	case hour >= 11 && hour < 14:
		return "noon"
	case hour >= 14 && hour < 18:
		return "afternoon"
	case hour >= 18 && hour < 22:
		return "evening"
	default:
		return "night"
	}
}

func addUnique(values []string, next string) []string {
	for _, value := range values {
		if value == next {
			return values
		}
	}
	return append(values, next)
}
```

- [ ] **Step 5: Implement self-state builder**

Create `server/internal/mind/selfstate/selfstate.go`:

```go
package selfstate

import (
	"math"
	"time"

	"github.com/bluegodg/anban/server/internal/mind"
)

func Default(deviceID string, at time.Time) mind.SelfState {
	return mind.SelfState{
		DeviceID: deviceID,
		At: at,
		Warmth: 0.55,
		Concern: 0.30,
		Curiosity: 0.35,
		Playfulness: 0.25,
		Energy: 0.50,
		Quietness: 0.45,
		Patience: 0.70,
		Confidence: 0.60,
		FamilyWeight: 0.60,
		PetWeight: 0.25,
		StewardWeight: 0.15,
	}
}

func ApplyEvents(state mind.SelfState, events []mind.Event) mind.SelfState {
	for _, event := range events {
		switch event.Type {
		case mind.EventLongSilence:
			state.Concern = clamp(state.Concern + 0.08)
			state.Quietness = clamp(state.Quietness + 0.04)
		case mind.EventPresenceSeen:
			state.Curiosity = clamp(state.Curiosity + 0.05)
			state.Warmth = clamp(state.Warmth + 0.02)
		case mind.EventChildMessageReceived:
			state.Warmth = clamp(state.Warmth + 0.04)
		case mind.EventReminderAcknowledged:
			state.Concern = clamp(state.Concern - 0.05)
			state.Warmth = clamp(state.Warmth + 0.02)
		case mind.EventFeedbackObserved:
			if event.Emotion == "rejected" {
				state.Quietness = clamp(state.Quietness + 0.08)
				state.Playfulness = clamp(state.Playfulness - 0.04)
			}
		}
	}
	return state
}

func clamp(value float64) float64 {
	return math.Max(0, math.Min(1, value))
}
```

- [ ] **Step 6: Run tests**

Run:

```powershell
Set-Location server
$env:GOPROXY="https://goproxy.cn,direct"; $env:GOSUMDB="off"; $env:CGO_ENABLED="0"; go test ./internal/mind/situation ./internal/mind/selfstate
```

Expected: PASS.

- [ ] **Step 7: Commit**

```powershell
git add server/internal/mind/situation server/internal/mind/selfstate
git commit -m "feat(mind): derive situation and self state"
```

---

### Task 4: Drives and Thought Generation

**Files:**
- Create: `server/internal/mind/drives/drives.go`
- Create: `server/internal/mind/drives/drives_test.go`
- Create: `server/internal/mind/thoughts/thoughts.go`
- Create: `server/internal/mind/thoughts/thoughts_test.go`

- [ ] **Step 1: Write drive tests**

Create `server/internal/mind/drives/drives_test.go`:

```go
package drives

import (
	"testing"
	"time"

	"github.com/bluegodg/anban/server/internal/mind"
)

func TestActivateReminderDueRaisesStewardshipAndCare(t *testing.T) {
	at := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	got := Activate(mind.Situation{DeviceID: "dev-001", At: at, OpenLoops: []string{"reminder_due"}}, mind.SelfState{Concern: 0.4, StewardWeight: 0.15}, []mind.Event{{ID: "evt-1", Type: mind.EventReminderDue}})
	if strength(got, mind.DriveStewardship) < 0.7 {
		t.Fatalf("drives = %+v, want stewardship >= 0.7", got)
	}
	if strength(got, mind.DriveCare) < 0.45 {
		t.Fatalf("drives = %+v, want care >= 0.45", got)
	}
}

func TestActivateLongSilenceRaisesQuietPresence(t *testing.T) {
	got := Activate(mind.Situation{DeviceID: "dev-001", ActivityLevel: "low", Constraints: []string{"prefer_observation"}}, mind.SelfState{Concern: 0.6, Quietness: 0.8}, []mind.Event{{ID: "evt-1", Type: mind.EventLongSilence}})
	if strength(got, mind.DriveQuietPresence) < 0.7 {
		t.Fatalf("drives = %+v, want quiet presence high", got)
	}
}

func strength(drives []mind.Drive, name string) float64 {
	for _, drive := range drives {
		if drive.Name == name {
			return drive.Strength
		}
	}
	return 0
}
```

- [ ] **Step 2: Write thought tests**

Create `server/internal/mind/thoughts/thoughts_test.go`:

```go
package thoughts

import (
	"testing"
	"time"

	"github.com/bluegodg/anban/server/internal/mind"
)

func TestGenerateLongSilenceThoughtPrefersWaitWhenInterruptionCostHigh(t *testing.T) {
	at := time.Date(2026, 6, 16, 22, 0, 0, 0, time.UTC)
	got := Generate(mind.Situation{DeviceID: "dev-001", At: at, TimeOfDay: "night", Constraints: []string{"prefer_observation"}}, mind.SelfState{Concern: 0.7, Quietness: 0.8}, []mind.Drive{{Name: mind.DriveCare, Strength: 0.7}, {Name: mind.DriveQuietPresence, Strength: 0.8}}, []mind.Event{{ID: "evt-1", DeviceID: "dev-001", Type: mind.EventLongSilence, At: at}})
	if len(got) == 0 {
		t.Fatal("thoughts empty")
	}
	if got[0].DriveName != mind.DriveQuietPresence {
		t.Fatalf("DriveName = %q, want quiet_presence", got[0].DriveName)
	}
	if got[0].InterruptionCost < 0.7 {
		t.Fatalf("InterruptionCost = %.2f, want high", got[0].InterruptionCost)
	}
}

func TestGenerateChildMessageThoughtUsesFamilyBridge(t *testing.T) {
	at := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	got := Generate(mind.Situation{DeviceID: "dev-001", At: at, SocialContext: "child_waiting_reply"}, mind.SelfState{Warmth: 0.6}, []mind.Drive{{Name: mind.DriveFamilyBridge, Strength: 0.8}}, []mind.Event{{ID: "evt-1", DeviceID: "dev-001", Type: mind.EventChildMessageReceived, At: at}})
	if len(got) != 1 {
		t.Fatalf("thoughts = %+v, want 1", got)
	}
	if got[0].DriveName != mind.DriveFamilyBridge || got[0].CareValue < 0.7 {
		t.Fatalf("thought = %+v, want family bridge high care", got[0])
	}
}
```

- [ ] **Step 3: Run failing tests**

Run:

```powershell
Set-Location server
$env:GOPROXY="https://goproxy.cn,direct"; $env:GOSUMDB="off"; $env:CGO_ENABLED="0"; go test ./internal/mind/drives ./internal/mind/thoughts
```

Expected: FAIL with `undefined: Activate` and `undefined: Generate`.

- [ ] **Step 4: Implement drive activation**

Create `server/internal/mind/drives/drives.go`:

```go
package drives

import (
	"math"

	"github.com/bluegodg/anban/server/internal/mind"
)

func Activate(s mind.Situation, state mind.SelfState, events []mind.Event) []mind.Drive {
	strengths := map[string]float64{}
	reasons := map[string]string{}
	sourceIDs := map[string][]string{}
	add := func(name string, amount float64, reason string, eventID string) {
		strengths[name] = clamp(strengths[name] + amount)
		if reasons[name] == "" {
			reasons[name] = reason
		}
		if eventID != "" {
			sourceIDs[name] = append(sourceIDs[name], eventID)
		}
	}
	for _, event := range events {
		switch event.Type {
		case mind.EventReminderDue:
			add(mind.DriveStewardship, 0.75+state.StewardWeight, "提醒到期，需要完成照看任务", event.ID)
			add(mind.DriveCare, 0.35+state.Concern*0.35, "提醒和老人状态有关", event.ID)
		case mind.EventChildMessageReceived:
			add(mind.DriveFamilyBridge, 0.80, "子女消息需要温和带到老人身边", event.ID)
			add(mind.DriveCompanionship, 0.25+state.Warmth*0.2, "家庭连接能增强陪伴感", event.ID)
		case mind.EventLongSilence:
			add(mind.DriveCare, 0.35+state.Concern*0.4, "长时间沉默需要留意", event.ID)
			add(mind.DriveQuietPresence, 0.45+state.Quietness*0.4, "当前更适合安静陪伴", event.ID)
		case mind.EventPresenceSeen:
			add(mind.DriveCompanionship, 0.45+state.Warmth*0.25, "看见老人出现，可以维持存在感", event.ID)
			add(mind.DriveCuriosity, 0.30+state.Curiosity*0.25, "新的 presence 值得观察", event.ID)
		}
	}
	for _, loop := range s.OpenLoops {
		if loop == "reminder_due" {
			add(mind.DriveStewardship, 0.25, "当前有未完成提醒", "")
		}
	}
	if s.ActivityLevel == "low" {
		add(mind.DriveQuietPresence, 0.20, "活动水平低时减少打扰", "")
	}
	out := make([]mind.Drive, 0, len(strengths))
	for name, strength := range strengths {
		out = append(out, mind.Drive{Name: name, Strength: clamp(strength), Reason: reasons[name], SourceEventIDs: sourceIDs[name]})
	}
	return out
}

func clamp(value float64) float64 {
	return math.Max(0, math.Min(1, value))
}
```

- [ ] **Step 5: Implement deterministic thought generation**

Create `server/internal/mind/thoughts/thoughts.go`:

```go
package thoughts

import (
	"fmt"

	"github.com/bluegodg/anban/server/internal/mind"
)

func Generate(s mind.Situation, state mind.SelfState, drives []mind.Drive, events []mind.Event) []mind.Thought {
	out := []mind.Thought{}
	for _, event := range events {
		switch event.Type {
		case mind.EventLongSilence:
			driveName := mind.DriveCare
			if driveStrength(drives, mind.DriveQuietPresence) >= driveStrength(drives, mind.DriveCare) {
				driveName = mind.DriveQuietPresence
			}
			out = append(out, mind.Thought{
				ID: fmt.Sprintf("thought-%s-long-silence", event.ID), DeviceID: event.DeviceID, At: event.At,
				Content: "老人安静了一段时间，先判断是否适合轻声关心，或者只安静陪着。",
				SourceEventIDs: []string{event.ID}, DriveName: driveName, EmotionalTone: "quiet",
				Urgency: 0.35 + state.Concern*0.2, CareValue: 0.55 + state.Concern*0.25,
				Novelty: 0.2, InterruptionCost: 0.55 + state.Quietness*0.25, Intimacy: 0.4 + state.Warmth*0.2,
				Status: mind.ThoughtPending,
			})
		case mind.EventChildMessageReceived:
			out = append(out, mind.Thought{
				ID: fmt.Sprintf("thought-%s-child-message", event.ID), DeviceID: event.DeviceID, At: event.At,
				Content: "子女发来了消息，需要找一个不打扰的时机带给老人。",
				SourceEventIDs: []string{event.ID}, DriveName: mind.DriveFamilyBridge, EmotionalTone: "warm",
				Urgency: 0.55, CareValue: 0.78, Novelty: 0.45, InterruptionCost: 0.35, Intimacy: 0.7,
				Status: mind.ThoughtPending,
			})
		case mind.EventReminderDue:
			out = append(out, mind.Thought{
				ID: fmt.Sprintf("thought-%s-reminder", event.ID), DeviceID: event.DeviceID, At: event.At,
				Content: "提醒到期了，应该用家人式语气轻轻带到，而不是像命令。",
				SourceEventIDs: []string{event.ID}, DriveName: mind.DriveStewardship, EmotionalTone: "caring",
				Urgency: 0.75, CareValue: 0.72, Novelty: 0.15, InterruptionCost: 0.30, Intimacy: 0.55,
				Status: mind.ThoughtPending,
			})
		case mind.EventPresenceSeen:
			out = append(out, mind.Thought{
				ID: fmt.Sprintf("thought-%s-presence", event.ID), DeviceID: event.DeviceID, At: event.At,
				Content: "老人出现了，可以轻轻保持存在感，也可以先观察不打扰。",
				SourceEventIDs: []string{event.ID}, DriveName: mind.DriveCompanionship, EmotionalTone: "warm",
				Urgency: 0.35, CareValue: 0.45, Novelty: 0.35, InterruptionCost: 0.40, Intimacy: 0.55,
				Status: mind.ThoughtPending,
			})
		}
	}
	return out
}

func driveStrength(drives []mind.Drive, name string) float64 {
	for _, drive := range drives {
		if drive.Name == name {
			return drive.Strength
		}
	}
	return 0
}
```

- [ ] **Step 6: Run tests**

Run:

```powershell
Set-Location server
$env:GOPROXY="https://goproxy.cn,direct"; $env:GOSUMDB="off"; $env:CGO_ENABLED="0"; go test ./internal/mind/drives ./internal/mind/thoughts
```

Expected: PASS.

- [ ] **Step 7: Commit**

```powershell
git add server/internal/mind/drives server/internal/mind/thoughts
git commit -m "feat(mind): activate drives and generate thoughts"
```

---

### Task 5: Behavior Selection and Expression Gate

**Files:**
- Create: `server/internal/mind/behavior/behavior.go`
- Create: `server/internal/mind/behavior/behavior_test.go`
- Create: `server/internal/mind/expression/expression.go`
- Create: `server/internal/mind/expression/expression_test.go`

- [ ] **Step 1: Write behavior tests**

Create `server/internal/mind/behavior/behavior_test.go`:

```go
package behavior

import (
	"testing"
	"time"

	"github.com/bluegodg/anban/server/internal/mind"
)

func TestSelectTurnsReminderThoughtIntoSpeakAction(t *testing.T) {
	at := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	got := Select(mind.Situation{DeviceID: "dev-001", At: at, InteractionMode: "idle"}, mind.SelfState{FamilyWeight: 0.6, StewardWeight: 0.15}, []mind.Thought{{
		ID: "thought-1", DeviceID: "dev-001", At: at, DriveName: mind.DriveStewardship, Content: "提醒到期", Urgency: 0.8, CareValue: 0.7, InterruptionCost: 0.2,
	}})
	if len(got) != 1 {
		t.Fatalf("actions = %+v, want 1", got)
	}
	if got[0].Type != mind.ActionSpeak || got[0].Executor != "reminder" {
		t.Fatalf("action = %+v, want reminder speak", got[0])
	}
}

func TestSelectTurnsNightSilenceIntoWait(t *testing.T) {
	at := time.Date(2026, 6, 16, 22, 0, 0, 0, time.UTC)
	got := Select(mind.Situation{DeviceID: "dev-001", At: at, TimeOfDay: "night", Constraints: []string{"prefer_observation"}}, mind.SelfState{Quietness: 0.85, FamilyWeight: 0.6}, []mind.Thought{{
		ID: "thought-1", DeviceID: "dev-001", At: at, DriveName: mind.DriveQuietPresence, Content: "安静陪着", CareValue: 0.7, InterruptionCost: 0.8,
	}})
	if got[0].Type != mind.ActionWait {
		t.Fatalf("action = %+v, want wait", got[0])
	}
	if got[0].Score <= 0 {
		t.Fatalf("score = %.2f, want positive", got[0].Score)
	}
}
```

- [ ] **Step 2: Write expression tests**

Create `server/internal/mind/expression/expression_test.go`:

```go
package expression

import (
	"testing"

	"github.com/bluegodg/anban/server/internal/mind"
)

func TestGateSuppressesLowScoreSpeak(t *testing.T) {
	decision := Gate(mind.Action{ID: "a1", Type: mind.ActionSpeak, Score: 0.15, Reason: "低价值"}, mind.Situation{}, mind.SelfState{})
	if decision.Status != mind.ActionSuppressed {
		t.Fatalf("decision = %+v, want suppressed", decision)
	}
}

func TestGateAllowsWaitEvenWithModerateScore(t *testing.T) {
	decision := Gate(mind.Action{ID: "a1", Type: mind.ActionWait, Score: 0.3, Reason: "夜间先观察"}, mind.Situation{}, mind.SelfState{})
	if decision.Status != mind.ActionDeferred {
		t.Fatalf("decision = %+v, want deferred wait", decision)
	}
	if decision.Reason == "" {
		t.Fatal("Reason is empty")
	}
}

func TestGateExecutesHighValueSpeak(t *testing.T) {
	decision := Gate(mind.Action{ID: "a1", Type: mind.ActionSpeak, Score: 0.72, Reason: "提醒重要且空闲"}, mind.Situation{InteractionMode: "idle"}, mind.SelfState{})
	if decision.Status != mind.ActionPending {
		t.Fatalf("decision = %+v, want pending execution", decision)
	}
}
```

- [ ] **Step 3: Run failing tests**

Run:

```powershell
Set-Location server
$env:GOPROXY="https://goproxy.cn,direct"; $env:GOSUMDB="off"; $env:CGO_ENABLED="0"; go test ./internal/mind/behavior ./internal/mind/expression
```

Expected: FAIL with `undefined: Select` and `undefined: Gate`.

- [ ] **Step 4: Implement behavior selection**

Create `server/internal/mind/behavior/behavior.go`:

```go
package behavior

import (
	"fmt"
	"math"
	"time"

	"github.com/bluegodg/anban/server/internal/mind"
)

func Select(s mind.Situation, state mind.SelfState, thoughts []mind.Thought) []mind.Action {
	out := make([]mind.Action, 0, len(thoughts))
	for _, thought := range thoughts {
		actionType, executor := actionFor(thought, s)
		score := score(thought, s, state, actionType)
		action := mind.Action{
			ID: fmt.Sprintf("action-%s", thought.ID),
			DeviceID: thought.DeviceID,
			IntentionID: fmt.Sprintf("intention-%s", thought.ID),
			Type: actionType,
			Executor: executor,
			Text: defaultText(thought, actionType),
			Status: mind.ActionPending,
			Reason: reasonFor(thought, actionType),
			Score: score,
		}
		if actionType == mind.ActionScheduleRecheck {
			next := s.At.Add(20 * time.Minute)
			action.ScheduledFor = &next
		}
		out = append(out, action)
	}
	return out
}

func actionFor(thought mind.Thought, s mind.Situation) (mind.ActionType, string) {
	if thought.DriveName == mind.DriveQuietPresence || thought.InterruptionCost >= 0.75 {
		return mind.ActionWait, "mind"
	}
	switch thought.DriveName {
	case mind.DriveStewardship:
		return mind.ActionSpeak, "reminder"
	case mind.DriveFamilyBridge:
		if s.InteractionMode == "conversation" {
			return mind.ActionWait, "mind"
		}
		return mind.ActionSpeak, "message"
	case mind.DriveCompanionship:
		return mind.ActionSpeak, "greeting"
	case mind.DriveCare:
		return mind.ActionScheduleRecheck, "mind"
	default:
		return mind.ActionSilentStateUpdate, "mind"
	}
}

func score(thought mind.Thought, s mind.Situation, state mind.SelfState, actionType mind.ActionType) float64 {
	value := thought.Urgency*0.20 + thought.CareValue*0.25 + thought.Novelty*0.10 + thought.Intimacy*0.15
	personality := state.FamilyWeight*0.08 + state.PetWeight*0.03 + state.StewardWeight*0.05
	cost := thought.InterruptionCost*0.30
	if s.InteractionMode == "conversation" && actionType == mind.ActionSpeak {
		cost += 0.25
	}
	if s.TimeOfDay == "night" && actionType == mind.ActionSpeak {
		cost += 0.20
	}
	if actionType == mind.ActionWait {
		value += state.Quietness * 0.20
		cost *= 0.5
	}
	return clamp(value + personality - cost)
}

func defaultText(thought mind.Thought, actionType mind.ActionType) string {
	if actionType != mind.ActionSpeak {
		return ""
	}
	switch thought.DriveName {
	case mind.DriveStewardship:
		return "到提醒时间啦，慢慢来，我帮你记着呢。"
	case mind.DriveFamilyBridge:
		return "孩子刚发来一句话，我轻轻说给你听。"
	case mind.DriveCompanionship:
		return "我在这儿呢，慢慢来。"
	default:
		return "我在呢。"
	}
}

func reasonFor(thought mind.Thought, actionType mind.ActionType) string {
	if actionType == mind.ActionWait {
		return "当前打扰成本较高，选择等待或安静陪伴"
	}
	return fmt.Sprintf("由 %s 动机和 thought %s 选择", thought.DriveName, thought.ID)
}

func clamp(value float64) float64 {
	return math.Max(0, math.Min(1, value))
}
```

- [ ] **Step 5: Implement expression gate**

Create `server/internal/mind/expression/expression.go`:

```go
package expression

import "github.com/bluegodg/anban/server/internal/mind"

func Gate(action mind.Action, s mind.Situation, state mind.SelfState) mind.Action {
	switch action.Type {
	case mind.ActionWait, mind.ActionScheduleRecheck:
		action.Status = mind.ActionDeferred
		if action.Reason == "" {
			action.Reason = "选择等待，避免打扰"
		}
		return action
	case mind.ActionSilentStateUpdate, mind.ActionSubtleExpression:
		action.Status = mind.ActionPending
		return action
	case mind.ActionSpeak:
		if s.InteractionMode == "conversation" && action.Score < 0.85 {
			action.Status = mind.ActionDeferred
			action.Reason = "老人正在对话，主动表达延后"
			return action
		}
		if action.Score < 0.35 {
			action.Status = mind.ActionSuppressed
			if action.Reason == "" {
				action.Reason = "表达价值不足"
			}
			return action
		}
		action.Status = mind.ActionPending
		return action
	default:
		if action.Score < 0.20 {
			action.Status = mind.ActionSuppressed
			return action
		}
		action.Status = mind.ActionPending
		return action
	}
}
```

- [ ] **Step 6: Run tests**

Run:

```powershell
Set-Location server
$env:GOPROXY="https://goproxy.cn,direct"; $env:GOSUMDB="off"; $env:CGO_ENABLED="0"; go test ./internal/mind/behavior ./internal/mind/expression
```

Expected: PASS.

- [ ] **Step 7: Commit**

```powershell
git add server/internal/mind/behavior server/internal/mind/expression
git commit -m "feat(mind): select actions through expression gate"
```

---

### Task 6: Engine Pipeline

**Files:**
- Create: `server/internal/mind/engine/engine.go`
- Create: `server/internal/mind/engine/engine_test.go`

- [ ] **Step 1: Write engine tests**

Create `server/internal/mind/engine/engine_test.go`:

```go
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
	ms := mind.NewStore(st.DB)
	if err := ms.AutoMigrate(); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	return New(ms), ms
}

func TestIngestReminderDueProducesReminderSpeakAction(t *testing.T) {
	svc, _ := newEngineForTest(t)
	at := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	actions, err := svc.Ingest(context.Background(), mind.Event{ID: "evt-1", DeviceID: "dev-001", Type: mind.EventReminderDue, Source: mind.SourceScheduler, At: at, Summary: "吃药提醒"})
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
```

- [ ] **Step 2: Run failing tests**

Run:

```powershell
Set-Location server
$env:GOPROXY="https://goproxy.cn,direct"; $env:GOSUMDB="off"; $env:CGO_ENABLED="0"; go test ./internal/mind/engine
```

Expected: FAIL with `undefined: Service` and `undefined: New`.

- [ ] **Step 3: Implement engine**

Create `server/internal/mind/engine/engine.go`:

```go
package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/bluegodg/anban/server/internal/mind"
	"github.com/bluegodg/anban/server/internal/mind/behavior"
	"github.com/bluegodg/anban/server/internal/mind/drives"
	"github.com/bluegodg/anban/server/internal/mind/expression"
	"github.com/bluegodg/anban/server/internal/mind/selfstate"
	"github.com/bluegodg/anban/server/internal/mind/situation"
	"github.com/bluegodg/anban/server/internal/mind/thoughts"
)

type Service struct {
	store *mind.Store
}

func New(store *mind.Store) *Service {
	return &Service{store: store}
}

func (s *Service) Ingest(ctx context.Context, event mind.Event) ([]mind.Action, error) {
	if event.At.IsZero() {
		event.At = time.Now().UTC()
	}
	if event.ID == "" {
		event.ID = fmt.Sprintf("evt-%s-%d", event.DeviceID, event.At.UnixNano())
	}
	if err := s.store.AppendEvent(ctx, event); err != nil {
		return nil, err
	}
	recent, err := s.store.ListRecentEvents(ctx, event.DeviceID, 20)
	if err != nil {
		return nil, err
	}
	return s.runPipeline(ctx, event.DeviceID, event.At, recent)
}

func (s *Service) TickIdle(ctx context.Context, deviceID string, at time.Time) ([]mind.Action, error) {
	return s.Ingest(ctx, mind.Event{
		ID: fmt.Sprintf("evt-%s-idle-%d", deviceID, at.UnixNano()),
		DeviceID: deviceID,
		Type: mind.EventLongSilence,
		Source: mind.SourceMind,
		At: at,
		Summary: "空闲循环检测到一段沉默",
		Salience: 0.45,
		Emotion: "quiet",
		Confidence: 0.7,
	})
}

func (s *Service) Reflect(ctx context.Context, deviceID string, window mind.TimeWindow) error {
	reflection := mind.Reflection{
		ID: fmt.Sprintf("reflection-%s-%d", deviceID, window.To.UnixNano()),
		DeviceID: deviceID,
		At: window.To,
		EpisodeSummary: "本轮反思已记录，具体摘要由 reflection 模块补充",
		StateAdjustments: map[string]float64{},
		BehaviorLessons: []string{},
	}
	return s.store.SaveReflection(ctx, reflection)
}

func (s *Service) runPipeline(ctx context.Context, deviceID string, at time.Time, recent []mind.Event) ([]mind.Action, error) {
	sit := situation.Build(deviceID, at, recent)
	state, err := s.store.GetSelfState(ctx, deviceID)
	if err != nil {
		state = selfstate.Default(deviceID, at)
	}
	state = selfstate.ApplyEvents(state, recent)
	if err := s.store.SaveSelfState(ctx, state); err != nil {
		return nil, err
	}
	activeDrives := drives.Activate(sit, state, recent)
	generatedThoughts := thoughts.Generate(sit, state, activeDrives, recent[:1])
	for _, thought := range generatedThoughts {
		if err := s.store.SaveThought(ctx, thought); err != nil {
			return nil, err
		}
	}
	candidates := behavior.Select(sit, state, generatedThoughts)
	out := make([]mind.Action, 0, len(candidates))
	for _, candidate := range candidates {
		selected := expression.Gate(candidate, sit, state)
		if err := s.store.SaveAction(ctx, selected); err != nil {
			return nil, err
		}
		out = append(out, selected)
	}
	return out, nil
}
```

- [ ] **Step 4: Run tests**

Run:

```powershell
Set-Location server
$env:GOPROXY="https://goproxy.cn,direct"; $env:GOSUMDB="off"; $env:CGO_ENABLED="0"; go test ./internal/mind/engine
```

Expected: PASS.

- [ ] **Step 5: Run all mind tests**

Run:

```powershell
Set-Location server
$env:GOPROXY="https://goproxy.cn,direct"; $env:GOSUMDB="off"; $env:CGO_ENABLED="0"; go test ./internal/mind/...
```

Expected: PASS.

- [ ] **Step 6: Commit**

```powershell
git add server/internal/mind/engine
git commit -m "feat(mind): orchestrate event to action pipeline"
```

---

### Task 7: Executor Interfaces and Domain Adapters

**Files:**
- Create: `server/internal/mind/executors/types.go`
- Create: `server/internal/mind/executors/domain_adapters.go`
- Create: `server/internal/mind/executors/domain_adapters_test.go`

- [ ] **Step 1: Write adapter tests**

Create `server/internal/mind/executors/domain_adapters_test.go`:

```go
package executors

import (
	"context"
	"testing"

	"github.com/bluegodg/anban/server/internal/mind"
)

func TestDispatcherRoutesSpeakActionToNamedExecutor(t *testing.T) {
	rem := &fakeSpeakExecutor{}
	msg := &fakeSpeakExecutor{}
	dispatcher := NewDispatcher(map[string]SpeakExecutor{"reminder": rem, "message": msg})

	result, err := dispatcher.Execute(context.Background(), mind.Action{ID: "action-1", Type: mind.ActionSpeak, Executor: "reminder", DeviceID: "dev-001", Text: "到提醒时间啦"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Status != mind.ActionExecuted {
		t.Fatalf("result = %+v, want executed", result)
	}
	if rem.calls != 1 || msg.calls != 0 {
		t.Fatalf("rem calls=%d msg calls=%d, want reminder only", rem.calls, msg.calls)
	}
}

func TestDispatcherDefersWaitActionWithoutDomainCall(t *testing.T) {
	dispatcher := NewDispatcher(map[string]SpeakExecutor{"reminder": &fakeSpeakExecutor{}})
	result, err := dispatcher.Execute(context.Background(), mind.Action{ID: "action-1", Type: mind.ActionWait, Executor: "mind", DeviceID: "dev-001"})
	if err != nil {
		t.Fatalf("Execute wait: %v", err)
	}
	if result.Status != mind.ActionDeferred {
		t.Fatalf("result = %+v, want deferred", result)
	}
}

type fakeSpeakExecutor struct{ calls int }

func (f *fakeSpeakExecutor) Speak(ctx context.Context, action mind.Action) (Result, error) {
	f.calls++
	return Result{ActionID: action.ID, Status: mind.ActionExecuted, ExecutorRef: "fake-ref"}, nil
}
```

- [ ] **Step 2: Run failing tests**

Run:

```powershell
Set-Location server
$env:GOPROXY="https://goproxy.cn,direct"; $env:GOSUMDB="off"; $env:CGO_ENABLED="0"; go test ./internal/mind/executors
```

Expected: FAIL with `undefined: NewDispatcher`, `undefined: SpeakExecutor`, and `undefined: Result`.

- [ ] **Step 3: Implement executor interfaces**

Create `server/internal/mind/executors/types.go`:

```go
package executors

import (
	"context"
	"errors"

	"github.com/bluegodg/anban/server/internal/mind"
)

var ErrExecutorNotFound = errors.New("mind executors: executor not found")

type Result struct {
	ActionID    string
	Status      mind.ActionStatus
	ExecutorRef string
	ErrorMessage string
}

type SpeakExecutor interface {
	Speak(ctx context.Context, action mind.Action) (Result, error)
}

type VisionExecutor interface {
	Observe(ctx context.Context, action mind.Action) (Result, error)
}

type PromptExecutor interface {
	SyncPrompt(ctx context.Context, action mind.Action) (Result, error)
}

type Dispatcher struct {
	speakers map[string]SpeakExecutor
}

func NewDispatcher(speakers map[string]SpeakExecutor) *Dispatcher {
	return &Dispatcher{speakers: speakers}
}

func (d *Dispatcher) Execute(ctx context.Context, action mind.Action) (Result, error) {
	switch action.Type {
	case mind.ActionWait, mind.ActionScheduleRecheck:
		return Result{ActionID: action.ID, Status: mind.ActionDeferred, ExecutorRef: "mind"}, nil
	case mind.ActionSilentStateUpdate:
		return Result{ActionID: action.ID, Status: mind.ActionExecuted, ExecutorRef: "mind"}, nil
	case mind.ActionSpeak:
		exec, ok := d.speakers[action.Executor]
		if !ok {
			return Result{ActionID: action.ID, Status: mind.ActionFailed, ErrorMessage: ErrExecutorNotFound.Error()}, ErrExecutorNotFound
		}
		return exec.Speak(ctx, action)
	default:
		return Result{ActionID: action.ID, Status: mind.ActionFailed, ErrorMessage: ErrExecutorNotFound.Error()}, ErrExecutorNotFound
	}
}
```

- [ ] **Step 4: Add domain adapter file with explicit adapter contracts**

Create `server/internal/mind/executors/domain_adapters.go`:

```go
package executors

import (
	"context"

	"github.com/bluegodg/anban/server/internal/mind"
)

type SpeakFunc func(ctx context.Context, action mind.Action) (Result, error)

func (fn SpeakFunc) Speak(ctx context.Context, action mind.Action) (Result, error) {
	return fn(ctx, action)
}
```

The concrete adapters for `message`, `reminder`, and `greeting` are added after those domains expose executor-oriented methods in Tasks 8-10. This file is still useful now because it lets `cmd/anban` and tests wrap domain methods without creating package cycles.

- [ ] **Step 5: Run tests**

Run:

```powershell
Set-Location server
$env:GOPROXY="https://goproxy.cn,direct"; $env:GOSUMDB="off"; $env:CGO_ENABLED="0"; go test ./internal/mind/executors
```

Expected: PASS.

- [ ] **Step 6: Commit**

```powershell
git add server/internal/mind/executors
git commit -m "feat(mind): define action executor dispatcher"
```

---

### Task 8: Message Domain Through Mind

**Files:**
- Modify: `server/internal/domains/message/service.go`
- Modify: `server/internal/domains/message/service_test.go`
- Modify later wiring in `server/cmd/anban/main.go` in Task 12

- [ ] **Step 1: Add failing queue/play test**

Append this test to `server/internal/domains/message/service_test.go`:

```go
func TestServiceQueueAndPlaySupportMindOrchestration(t *testing.T) {
	fake := &xiaozhiclient.FakeClient{}
	svc := newTestService(t, fake)
	ctx := context.Background()

	queued, err := svc.Queue(ctx, SendRequest{DeviceID: "dev-001", Text: "妈，我今天晚点到家", FromName: "小明"})
	if err != nil {
		t.Fatalf("Queue: %v", err)
	}
	if queued.Status != StatusPending {
		t.Fatalf("queued status = %q, want pending", queued.Status)
	}
	if len(fake.InjectCalls) != 0 {
		t.Fatalf("InjectCalls = %d, want no speech before Mind selects action", len(fake.InjectCalls))
	}

	played, err := svc.PlayQueued(ctx, queued.ID)
	if err != nil {
		t.Fatalf("PlayQueued: %v", err)
	}
	if played.Status != StatusPlayed {
		t.Fatalf("played status = %q, want played", played.Status)
	}
	if len(fake.InjectCalls) != 1 {
		t.Fatalf("InjectCalls = %d, want one after PlayQueued", len(fake.InjectCalls))
	}
}
```

- [ ] **Step 2: Run failing message tests**

Run:

```powershell
Set-Location server
$env:GOPROXY="https://goproxy.cn,direct"; $env:GOSUMDB="off"; $env:CGO_ENABLED="0"; go test ./internal/domains/message
```

Expected: FAIL with `svc.Queue undefined` and `svc.PlayQueued undefined`.

- [ ] **Step 3: Refactor message service**

Modify `server/internal/domains/message/service.go` so `Send` delegates to new methods:

```go
func (s *Service) Send(ctx context.Context, req SendRequest) (Message, error) {
	msg, err := s.Queue(ctx, req)
	if err != nil {
		return Message{}, err
	}
	return s.PlayQueued(ctx, msg.ID)
}

func (s *Service) Queue(ctx context.Context, req SendRequest) (Message, error) {
	deviceID := strings.TrimSpace(req.DeviceID)
	text := trimAndLimit(req.Text, MaxTextRunes)
	if deviceID == "" || text == "" {
		return Message{}, ErrInvalidInput
	}
	now := s.now().UTC()
	msg := Message{
		DeviceID: deviceID,
		Text: text,
		FromName: strings.TrimSpace(req.FromName),
		Status: StatusPending,
		QueuedAt: now,
	}
	if err := s.store.Create(ctx, &msg); err != nil {
		return Message{}, err
	}
	return msg, nil
}

func (s *Service) PlayQueued(ctx context.Context, id uint) (Message, error) {
	if id == 0 {
		return Message{}, ErrInvalidInput
	}
	msg, err := s.store.Get(ctx, id)
	if err != nil {
		return Message{}, err
	}
	if msg.Status == StatusPlayed {
		return msg, nil
	}
	if err := s.play(ctx, &msg); err != nil {
		return msg, err
	}
	return msg, nil
}
```

Keep the existing `play`, `withMessageInjectTimeout`, and `messageSpeakOptions` functions unchanged.

- [ ] **Step 4: Run message tests**

Run:

```powershell
Set-Location server
$env:GOPROXY="https://goproxy.cn,direct"; $env:GOSUMDB="off"; $env:CGO_ENABLED="0"; go test ./internal/domains/message
```

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add server/internal/domains/message/service.go server/internal/domains/message/service_test.go
git commit -m "feat(message): expose queue and play for mind orchestration"
```

---

### Task 9: Reminder and Greeting Executor Entrypoints

**Files:**
- Modify: `server/internal/domains/reminder/service.go`
- Modify: `server/internal/domains/reminder/service_test.go`
- Modify: `server/internal/domains/greeting/service.go`
- Modify: `server/internal/domains/greeting/service_test.go`

- [ ] **Step 1: Add reminder executor test**

Append to `server/internal/domains/reminder/service_test.go`:

```go
func TestServicePlayScheduledSupportsMindExecutor(t *testing.T) {
	fakeXC := &xiaozhiclient.FakeClient{}
	svc := newTestService(t, fakeXC, &fakeScheduler{})
	ctx := context.Background()
	rem, err := svc.Create(ctx, CreateRequest{
		DeviceID: "dev-001",
		Content: "吃药",
		ScheduledAt: svc.now().Add(time.Hour),
		Category: CategoryMedicine,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	played, err := svc.PlayScheduled(ctx, rem.ID)
	if err != nil {
		t.Fatalf("PlayScheduled: %v", err)
	}
	if played.Status != StatusPlayed {
		t.Fatalf("status = %q, want played", played.Status)
	}
	if len(fakeXC.InjectCalls) != 1 {
		t.Fatalf("InjectCalls = %d, want 1", len(fakeXC.InjectCalls))
	}
}
```

- [ ] **Step 2: Add greeting executor test**

Append to `server/internal/domains/greeting/service_test.go`:

```go
func TestServiceSpeakTextSupportsMindExecutor(t *testing.T) {
	fake := &xiaozhiclient.FakeClient{}
	svc := newTestService(t, fake)
	ctx := context.Background()
	got, err := svc.SpeakText(ctx, "dev-001", "我在这儿呢，慢慢来。")
	if err != nil {
		t.Fatalf("SpeakText: %v", err)
	}
	if got.Status != StatusPlayed {
		t.Fatalf("status = %q, want played", got.Status)
	}
	if len(fake.InjectCalls) != 1 || fake.InjectCalls[0].Text != "我在这儿呢，慢慢来。" {
		t.Fatalf("InjectCalls = %+v", fake.InjectCalls)
	}
}
```

- [ ] **Step 3: Run failing tests**

Run:

```powershell
Set-Location server
$env:GOPROXY="https://goproxy.cn,direct"; $env:GOSUMDB="off"; $env:CGO_ENABLED="0"; go test ./internal/domains/reminder ./internal/domains/greeting
```

Expected: FAIL with `PlayScheduled undefined` and `SpeakText undefined`.

- [ ] **Step 4: Implement reminder executor method**

Add to `server/internal/domains/reminder/service.go`:

```go
func (s *Service) PlayScheduled(ctx context.Context, id uint) (Reminder, error) {
	if id == 0 {
		return Reminder{}, ErrInvalidInput
	}
	rem, err := s.store.Get(ctx, id)
	if err != nil {
		return Reminder{}, err
	}
	if rem.Status != StatusScheduled {
		return rem, nil
	}
	s.play(ctx, &rem)
	return rem, nil
}
```

- [ ] **Step 5: Implement greeting executor method**

Add to `server/internal/domains/greeting/service.go`:

```go
func (s *Service) SpeakText(ctx context.Context, deviceID, text string) (Greeting, error) {
	deviceID = strings.TrimSpace(deviceID)
	text = strings.TrimSpace(text)
	if deviceID == "" || text == "" {
		return Greeting{}, ErrInvalidInput
	}
	now := s.now().UTC()
	greeting := Greeting{
		DeviceID: deviceID,
		TonePreset: ToneCasual,
		Text: text,
		Status: StatusPending,
		TriggeredAt: now,
	}
	if err := s.store.Create(ctx, &greeting); err != nil {
		return Greeting{}, err
	}
	if err := s.play(ctx, &greeting, now); err != nil {
		return greeting, err
	}
	return greeting, nil
}
```

- [ ] **Step 6: Run tests**

Run:

```powershell
Set-Location server
$env:GOPROXY="https://goproxy.cn,direct"; $env:GOSUMDB="off"; $env:CGO_ENABLED="0"; go test ./internal/domains/reminder ./internal/domains/greeting
```

Expected: PASS.

- [ ] **Step 7: Commit**

```powershell
git add server/internal/domains/reminder/service.go server/internal/domains/reminder/service_test.go server/internal/domains/greeting/service.go server/internal/domains/greeting/service_test.go
git commit -m "feat(domains): expose reminder and greeting executors"
```

---

### Task 10: Mind Domain Adapters

**Files:**
- Modify: `server/internal/mind/executors/domain_adapters.go`
- Modify: `server/internal/mind/executors/domain_adapters_test.go`

- [ ] **Step 1: Add tests for adapter constructors**

Append to `server/internal/mind/executors/domain_adapters_test.go`:

```go
func TestSpeakFuncReturnsDomainReference(t *testing.T) {
	exec := SpeakFunc(func(ctx context.Context, action mind.Action) (Result, error) {
		return Result{ActionID: action.ID, Status: mind.ActionExecuted, ExecutorRef: "message:12"}, nil
	})
	got, err := exec.Speak(context.Background(), mind.Action{ID: "action-1", Type: mind.ActionSpeak})
	if err != nil {
		t.Fatalf("Speak: %v", err)
	}
	if got.ExecutorRef != "message:12" {
		t.Fatalf("ExecutorRef = %q, want message:12", got.ExecutorRef)
	}
}
```

- [ ] **Step 2: Run tests**

Run:

```powershell
Set-Location server
$env:GOPROXY="https://goproxy.cn,direct"; $env:GOSUMDB="off"; $env:CGO_ENABLED="0"; go test ./internal/mind/executors
```

Expected: PASS because `SpeakFunc` was added in Task 7.

- [ ] **Step 3: Commit if tests required any imports or formatting changes**

```powershell
git add server/internal/mind/executors/domain_adapters.go server/internal/mind/executors/domain_adapters_test.go
git commit -m "test(mind): cover speak executor adapter"
```

---

### Task 11: Reflection and Life Loop

**Files:**
- Create: `server/internal/mind/reflection/reflection.go`
- Create: `server/internal/mind/reflection/reflection_test.go`
- Create: `server/internal/mind/life/life.go`
- Create: `server/internal/mind/life/life_test.go`

- [ ] **Step 1: Write reflection tests**

Create `server/internal/mind/reflection/reflection_test.go`:

```go
package reflection

import (
	"testing"
	"time"

	"github.com/bluegodg/anban/server/internal/mind"
)

func TestSummarizeAcceptedInteractionRaisesWarmth(t *testing.T) {
	at := time.Date(2026, 6, 16, 20, 0, 0, 0, time.UTC)
	got := Summarize("dev-001", at, []mind.Feedback{{ID: "fb-1", Signal: "user_replied", Notes: "老人接着聊了三轮"}})
	if got.StateAdjustments["warmth"] <= 0 {
		t.Fatalf("StateAdjustments = %+v, want warmth increase", got.StateAdjustments)
	}
	if len(got.BehaviorLessons) == 0 {
		t.Fatalf("BehaviorLessons empty")
	}
}
```

- [ ] **Step 2: Write life tests**

Create `server/internal/mind/life/life_test.go`:

```go
package life

import (
	"testing"
	"time"

	"github.com/bluegodg/anban/server/internal/mind"
)

func TestUpdateCreatesThemeFromCareFocus(t *testing.T) {
	at := time.Date(2026, 6, 16, 8, 0, 0, 0, time.UTC)
	got := Update("dev-001", at, mind.SelfState{Concern: 0.75, Playfulness: 0.2}, []mind.Event{{ID: "evt-1", Type: mind.EventLongSilence, Summary: "上午互动少"}})
	if got.TodayTheme == "" {
		t.Fatal("TodayTheme is empty")
	}
	if got.CareFocus == "" {
		t.Fatal("CareFocus is empty")
	}
	if got.PlayfulnessTrend != 0.2 {
		t.Fatalf("PlayfulnessTrend = %.2f, want 0.2", got.PlayfulnessTrend)
	}
}
```

- [ ] **Step 3: Run failing tests**

Run:

```powershell
Set-Location server
$env:GOPROXY="https://goproxy.cn,direct"; $env:GOSUMDB="off"; $env:CGO_ENABLED="0"; go test ./internal/mind/reflection ./internal/mind/life
```

Expected: FAIL with `undefined: Summarize` and `undefined: Update`.

- [ ] **Step 4: Implement reflection**

Create `server/internal/mind/reflection/reflection.go`:

```go
package reflection

import (
	"fmt"
	"time"

	"github.com/bluegodg/anban/server/internal/mind"
)

func Summarize(deviceID string, at time.Time, feedback []mind.Feedback) mind.Reflection {
	adjustments := map[string]float64{}
	lessons := []string{}
	summary := "本轮互动没有明显反馈"
	for _, item := range feedback {
		switch item.Signal {
		case "user_replied", "conversation_continued":
			adjustments["warmth"] += 0.03
			adjustments["quietness"] -= 0.01
			lessons = append(lessons, "当前表达被接受，类似时机可保持轻声互动")
			summary = "老人接受了本轮互动"
		case "user_ignored":
			adjustments["quietness"] += 0.03
			lessons = append(lessons, "类似场景减少追问，优先等待")
			summary = "老人没有回应本轮互动"
		case "user_rejected":
			adjustments["quietness"] += 0.06
			adjustments["playfulness"] -= 0.03
			lessons = append(lessons, "类似表达需要更克制")
			summary = "老人拒绝了本轮互动"
		}
	}
	return mind.Reflection{
		ID: fmt.Sprintf("reflection-%s-%d", deviceID, at.UnixNano()),
		DeviceID: deviceID,
		At: at,
		EpisodeSummary: summary,
		StateAdjustments: adjustments,
		BehaviorLessons: lessons,
	}
}
```

- [ ] **Step 5: Implement life loop**

Create `server/internal/mind/life/life.go`:

```go
package life

import (
	"fmt"
	"time"

	"github.com/bluegodg/anban/server/internal/mind"
)

func Update(deviceID string, at time.Time, state mind.SelfState, events []mind.Event) mind.LifeState {
	careFocus := ""
	lingering := []string{}
	for _, event := range events {
		if event.Type == mind.EventLongSilence {
			careFocus = "最近互动偏少，先轻轻留意"
			lingering = append(lingering, event.Summary)
		}
		if event.Type == mind.EventChildMessageReceived {
			lingering = append(lingering, "子女消息等待合适时机带到")
		}
	}
	theme := "温和陪伴，少打扰"
	if state.Concern >= 0.7 {
		theme = "轻轻留意老人状态"
	}
	if state.Playfulness >= 0.45 {
		theme = "用更轻松的方式陪着"
	}
	return mind.LifeState{
		DeviceID: deviceID,
		At: at,
		TodayTheme: theme,
		LingeringThoughts: lingering,
		SocialEnergy: clamp(0.45 + state.Energy*0.3 - state.Quietness*0.2),
		CareFocus: careFocus,
		PlayfulnessTrend: state.Playfulness,
		RelationshipTemperature: clamp(0.4 + state.Warmth*0.4),
	}
}

func clamp(value float64) float64 {
	if value < 0 { return 0 }
	if value > 1 { return 1 }
	return value
}
```

- [ ] **Step 6: Run tests**

Run:

```powershell
Set-Location server
$env:GOPROXY="https://goproxy.cn,direct"; $env:GOSUMDB="off"; $env:CGO_ENABLED="0"; go test ./internal/mind/reflection ./internal/mind/life
```

Expected: PASS.

- [ ] **Step 7: Commit**

```powershell
git add server/internal/mind/reflection server/internal/mind/life
git commit -m "feat(mind): add reflection and life loop logic"
```

---

### Task 12: Main Wiring and Migrations

**Files:**
- Modify: `server/cmd/anban/main.go`

- [ ] **Step 1: Add imports**

Modify `server/cmd/anban/main.go` imports to include:

```go
	"github.com/bluegodg/anban/server/internal/mind"
	"github.com/bluegodg/anban/server/internal/mind/engine"
	"github.com/bluegodg/anban/server/internal/mind/executors"
```

- [ ] **Step 2: Add mind store migration after memory migration**

After memory service setup, add:

```go
	mindStore := mind.NewStore(st.DB)
	if err := mindStore.AutoMigrate(); err != nil {
		log.Fatalf("mind 表迁移失败: %v", err)
	}
	mindEngine := engine.New(mindStore)
	_ = mindEngine
```

Expected intermediate compile: PASS. The `_ = mindEngine` line is temporary within this task and removed in Step 4.

- [ ] **Step 3: Run full tests**

Run:

```powershell
Set-Location server
$env:GOPROXY="https://goproxy.cn,direct"; $env:GOSUMDB="off"; $env:CGO_ENABLED="0"; go test ./...
```

Expected: PASS.

- [ ] **Step 4: Wire executor dispatcher**

Still in `server/cmd/anban/main.go`, replace `_ = mindEngine` with this block after all domain services exist:

```go
	mindDispatcher := executors.NewDispatcher(map[string]executors.SpeakExecutor{
		"message": executors.SpeakFunc(func(ctx context.Context, action mind.Action) (executors.Result, error) {
			id, _ := action.Args["messageId"].(float64)
			if id <= 0 {
				return executors.Result{ActionID: action.ID, Status: mind.ActionFailed, ErrorMessage: "messageId missing"}, nil
			}
			msg, err := messageService.PlayQueued(ctx, uint(id))
			if err != nil {
				return executors.Result{ActionID: action.ID, Status: mind.ActionFailed, ErrorMessage: err.Error()}, err
			}
			return executors.Result{ActionID: action.ID, Status: mind.ActionExecuted, ExecutorRef: fmt.Sprintf("message:%d", msg.ID)}, nil
		}),
		"greeting": executors.SpeakFunc(func(ctx context.Context, action mind.Action) (executors.Result, error) {
			greeting, err := greetingService.SpeakText(ctx, action.DeviceID, action.Text)
			if err != nil {
				return executors.Result{ActionID: action.ID, Status: mind.ActionFailed, ErrorMessage: err.Error()}, err
			}
			return executors.Result{ActionID: action.ID, Status: mind.ActionExecuted, ExecutorRef: fmt.Sprintf("greeting:%d", greeting.ID)}, nil
		}),
	})
	_ = mindDispatcher
```

Add `fmt` to the import list. Keep `_ = mindDispatcher` until Task 13 executes actions through the dispatcher.

- [ ] **Step 5: Run full tests**

Run:

```powershell
Set-Location server
$env:GOPROXY="https://goproxy.cn,direct"; $env:GOSUMDB="off"; $env:CGO_ENABLED="0"; go test ./...
```

Expected: PASS.

- [ ] **Step 6: Commit**

```powershell
git add server/cmd/anban/main.go
git commit -m "feat(mind): wire mind migrations and executors"
```

---

### Task 13: Message Route Event Path

**Files:**
- Modify: `server/internal/domains/message/service.go`
- Modify: `server/internal/domains/message/types.go`
- Modify: `server/internal/domains/message/service_test.go`
- Modify: `server/cmd/anban/main.go`

- [ ] **Step 1: Add MindSink interface and service hook test**

Append to `server/internal/domains/message/service_test.go`:

```go
func TestServiceSendEmitsMindEventWhenSinkConfigured(t *testing.T) {
	sink := &fakeMindSink{}
	svc := newTestService(t, &xiaozhiclient.FakeClient{})
	svc.UseMindSink(sink)

	if _, err := svc.Send(context.Background(), SendRequest{DeviceID: "dev-001", Text: "今晚早点休息", FromName: "小明"}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if len(sink.events) != 1 {
		t.Fatalf("events = %+v, want 1", sink.events)
	}
	if sink.events[0].Type != "child_message_received" {
		t.Fatalf("event type = %q, want child_message_received", sink.events[0].Type)
	}
}

type fakeMindSink struct {
	events []MindEvent
}

func (f *fakeMindSink) IngestMindEvent(ctx context.Context, event MindEvent) error {
	f.events = append(f.events, event)
	return nil
}
```

- [ ] **Step 2: Run failing message test**

Run:

```powershell
Set-Location server
$env:GOPROXY="https://goproxy.cn,direct"; $env:GOSUMDB="off"; $env:CGO_ENABLED="0"; go test ./internal/domains/message
```

Expected: FAIL with `undefined: MindEvent`, `UseMindSink undefined`, and `MindSink undefined`.

- [ ] **Step 3: Add domain-level event sink types**

Append to `server/internal/domains/message/types.go`:

```go
type MindEvent struct {
	DeviceID string
	Type     string
	SourceID uint
	Summary  string
	Payload  map[string]any
}

type MindSink interface {
	IngestMindEvent(ctx context.Context, event MindEvent) error
}
```

Add `context` to `types.go` imports:

```go
import (
	"context"
	"errors"
	"time"
)
```

- [ ] **Step 4: Wire sink in service**

Modify `Service` in `server/internal/domains/message/service.go`:

```go
type Service struct {
	store *Store
	xc    xiaozhiclient.Client
	now   func() time.Time
	mindSink MindSink
}

func (s *Service) UseMindSink(sink MindSink) {
	s.mindSink = sink
}
```

At the end of `Queue`, before `return msg, nil`, add:

```go
	if s.mindSink != nil {
		_ = s.mindSink.IngestMindEvent(ctx, MindEvent{
			DeviceID: msg.DeviceID,
			Type: "child_message_received",
			SourceID: msg.ID,
			Summary: "子女留言已进入安伴心智",
			Payload: map[string]any{"messageId": float64(msg.ID), "fromName": msg.FromName},
		})
	}
```

- [ ] **Step 5: Run tests**

Run:

```powershell
Set-Location server
$env:GOPROXY="https://goproxy.cn,direct"; $env:GOSUMDB="off"; $env:CGO_ENABLED="0"; go test ./internal/domains/message
```

Expected: PASS.

- [ ] **Step 6: Bridge message MindSink in main**

In `server/cmd/anban/main.go`, after `mindEngine` is created and after `messageService` is created, call:

```go
	messageService.UseMindSink(messageMindSink{engine: mindEngine})
```

Add this type near the bottom of `main.go`:

```go
type messageMindSink struct {
	engine mind.Engine
}

func (s messageMindSink) IngestMindEvent(ctx context.Context, event message.MindEvent) error {
	_, err := s.engine.Ingest(ctx, mind.Event{
		DeviceID: event.DeviceID,
		Type: mind.EventType(event.Type),
		Source: mind.SourceDomain,
		At: time.Now().UTC(),
		Summary: event.Summary,
		Payload: event.Payload,
		Salience: 0.75,
		Emotion: "warm",
		Confidence: 0.9,
	})
	return err
}
```

This keeps the dependency direction clean: message domain sees only its local `MindSink` interface, while `cmd/anban` adapts it to `mind.Engine`.

- [ ] **Step 7: Run full tests**

Run:

```powershell
Set-Location server
$env:GOPROXY="https://goproxy.cn,direct"; $env:GOSUMDB="off"; $env:CGO_ENABLED="0"; go test ./...
```

Expected: PASS.

- [ ] **Step 8: Commit**

```powershell
git add server/internal/domains/message server/cmd/anban/main.go
git commit -m "feat(message): emit child messages into mind"
```

---

### Task 14: Reminder Due Event Path

**Files:**
- Modify: `server/internal/domains/reminder/types.go`
- Modify: `server/internal/domains/reminder/service.go`
- Modify: `server/internal/domains/reminder/service_test.go`
- Modify: `server/cmd/anban/main.go`

- [ ] **Step 1: Add reminder MindSink test**

Append to `server/internal/domains/reminder/service_test.go`:

```go
func TestServiceFireEmitsMindEventWhenSinkConfigured(t *testing.T) {
	sink := &fakeReminderMindSink{}
	svc := newTestService(t, &xiaozhiclient.FakeClient{}, &fakeScheduler{})
	svc.UseMindSink(sink)
	ctx := context.Background()
	rem, err := svc.Create(ctx, CreateRequest{DeviceID: "dev-001", Content: "吃药", ScheduledAt: svc.now().Add(time.Hour), Category: CategoryMedicine})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	svc.fire(rem.ID)
	if len(sink.events) != 1 {
		t.Fatalf("events = %+v, want 1", sink.events)
	}
	if sink.events[0].Type != "reminder_due" || sink.events[0].SourceID != rem.ID {
		t.Fatalf("event = %+v, want reminder_due for %d", sink.events[0], rem.ID)
	}
}

type fakeReminderMindSink struct{ events []MindEvent }

func (f *fakeReminderMindSink) IngestMindEvent(ctx context.Context, event MindEvent) error {
	f.events = append(f.events, event)
	return nil
}
```

- [ ] **Step 2: Run failing reminder test**

Run:

```powershell
Set-Location server
$env:GOPROXY="https://goproxy.cn,direct"; $env:GOSUMDB="off"; $env:CGO_ENABLED="0"; go test ./internal/domains/reminder
```

Expected: FAIL with undefined MindSink symbols.

- [ ] **Step 3: Add reminder MindSink types**

Append to `server/internal/domains/reminder/types.go`:

```go
type MindEvent struct {
	DeviceID string
	Type     string
	SourceID uint
	Summary  string
	Payload  map[string]any
}

type MindSink interface {
	IngestMindEvent(ctx context.Context, event MindEvent) error
}
```

Add `context` to the imports in `types.go`.

- [ ] **Step 4: Emit due event before direct play**

Modify `Service` in `server/internal/domains/reminder/service.go`:

```go
	mindSink MindSink
```

Add:

```go
func (s *Service) UseMindSink(sink MindSink) {
	s.mindSink = sink
}
```

Modify `fire` before `s.play(ctx, &rem)`:

```go
	if s.mindSink != nil {
		_ = s.mindSink.IngestMindEvent(ctx, MindEvent{
			DeviceID: rem.DeviceID,
			Type: "reminder_due",
			SourceID: rem.ID,
			Summary: "提醒到期，交给安伴心智决定表达方式",
			Payload: map[string]any{"reminderId": float64(rem.ID), "category": string(rem.Category)},
		})
		return
	}
```

This preserves existing behavior when Mind is not wired, and makes Mind the owner of proactive behavior when it is wired.

- [ ] **Step 5: Run reminder tests**

Run:

```powershell
Set-Location server
$env:GOPROXY="https://goproxy.cn,direct"; $env:GOSUMDB="off"; $env:CGO_ENABLED="0"; go test ./internal/domains/reminder
```

Expected: PASS.

- [ ] **Step 6: Bridge reminder sink in main**

In `server/cmd/anban/main.go`, after `reminderService` is created:

```go
	reminderService.UseMindSink(reminderMindSink{engine: mindEngine})
```

Add near the bottom:

```go
type reminderMindSink struct {
	engine mind.Engine
}

func (s reminderMindSink) IngestMindEvent(ctx context.Context, event reminder.MindEvent) error {
	_, err := s.engine.Ingest(ctx, mind.Event{
		DeviceID: event.DeviceID,
		Type: mind.EventType(event.Type),
		Source: mind.SourceDomain,
		At: time.Now().UTC(),
		Summary: event.Summary,
		Payload: event.Payload,
		Salience: 0.85,
		Emotion: "caring",
		Confidence: 0.95,
	})
	return err
}
```

- [ ] **Step 7: Run full tests**

Run:

```powershell
Set-Location server
$env:GOPROXY="https://goproxy.cn,direct"; $env:GOSUMDB="off"; $env:CGO_ENABLED="0"; go test ./...
```

Expected: PASS.

- [ ] **Step 8: Commit**

```powershell
git add server/internal/domains/reminder server/cmd/anban/main.go
git commit -m "feat(reminder): route due reminders through mind"
```

---

### Task 15: Vision Presence Event Path

**Files:**
- Modify: `server/internal/domains/vision/types.go`
- Modify: `server/internal/domains/vision/service.go`
- Modify: `server/internal/domains/vision/service_test.go`
- Modify: `server/cmd/anban/main.go`

- [ ] **Step 1: Add vision MindSink test**

Append to `server/internal/domains/vision/service_test.go`:

```go
func TestServiceObservePresenceEmitsMindEventWhenSinkConfigured(t *testing.T) {
	sink := &fakeVisionMindSink{}
	svc := NewService(&visionClient{}, &fakeGreetingTrigger{})
	svc.UseMindSink(sink)
	_, err := svc.ObservePresence(context.Background(), PresenceObservationRequest{DeviceID: "dev-001", Presence: PresenceSomeone})
	if err != nil {
		t.Fatalf("ObservePresence: %v", err)
	}
	if len(sink.events) != 1 {
		t.Fatalf("events = %+v, want 1", sink.events)
	}
	if sink.events[0].Type != "presence_seen" {
		t.Fatalf("event = %+v, want presence_seen", sink.events[0])
	}
}

type fakeVisionMindSink struct{ events []MindEvent }

func (f *fakeVisionMindSink) IngestMindEvent(ctx context.Context, event MindEvent) error {
	f.events = append(f.events, event)
	return nil
}
```

- [ ] **Step 2: Run failing vision test**

Run:

```powershell
Set-Location server
$env:GOPROXY="https://goproxy.cn,direct"; $env:GOSUMDB="off"; $env:CGO_ENABLED="0"; go test ./internal/domains/vision
```

Expected: FAIL with undefined MindSink symbols.

- [ ] **Step 3: Add vision MindSink types**

Append to `server/internal/domains/vision/types.go`:

```go
type MindEvent struct {
	DeviceID string
	Type     string
	Summary  string
	Payload  map[string]any
}

type MindSink interface {
	IngestMindEvent(ctx context.Context, event MindEvent) error
}
```

Add `context` to the imports in `types.go`.

- [ ] **Step 4: Emit presence events**

Modify `Service` in `server/internal/domains/vision/service.go`:

```go
	mindSink MindSink
```

Add:

```go
func (s *Service) UseMindSink(sink MindSink) {
	s.mindSink = sink
}
```

Inside `ObservePresence`, after presence is normalized and before triggering greeting, add:

```go
	if s.mindSink != nil {
		eventType := "presence_absent"
		if presence == PresenceSomeone {
			eventType = "presence_seen"
		}
		_ = s.mindSink.IngestMindEvent(ctx, MindEvent{
			DeviceID: deviceID,
			Type: eventType,
			Summary: "视觉 presence 进入安伴心智",
			Payload: map[string]any{"presence": string(presence)},
		})
		return PresenceObservationResult{DeviceID: deviceID, Presence: presence}, nil
	}
```

This makes Mind own the decision to greet when wired. Existing direct greeting behavior stays active when Mind is not wired.

- [ ] **Step 5: Run vision tests**

Run:

```powershell
Set-Location server
$env:GOPROXY="https://goproxy.cn,direct"; $env:GOSUMDB="off"; $env:CGO_ENABLED="0"; go test ./internal/domains/vision
```

Expected: PASS.

- [ ] **Step 6: Bridge vision sink in main**

In `server/cmd/anban/main.go`, after `visionService` is created:

```go
	visionService.UseMindSink(visionMindSink{engine: mindEngine})
```

Add:

```go
type visionMindSink struct {
	engine mind.Engine
}

func (s visionMindSink) IngestMindEvent(ctx context.Context, event vision.MindEvent) error {
	_, err := s.engine.Ingest(ctx, mind.Event{
		DeviceID: event.DeviceID,
		Type: mind.EventType(event.Type),
		Source: mind.SourceVision,
		At: time.Now().UTC(),
		Summary: event.Summary,
		Payload: event.Payload,
		Salience: 0.55,
		Emotion: "warm",
		Confidence: 0.8,
	})
	return err
}
```

- [ ] **Step 7: Run full tests**

Run:

```powershell
Set-Location server
$env:GOPROXY="https://goproxy.cn,direct"; $env:GOSUMDB="off"; $env:CGO_ENABLED="0"; go test ./...
```

Expected: PASS.

- [ ] **Step 8: Commit**

```powershell
git add server/internal/domains/vision server/cmd/anban/main.go
git commit -m "feat(vision): emit presence events into mind"
```

---

### Task 16: Execute Mind Actions in Main

**Files:**
- Modify: `server/internal/mind/engine/engine.go`
- Modify: `server/internal/mind/engine/engine_test.go`
- Modify: `server/cmd/anban/main.go`

- [ ] **Step 1: Extend engine with execution hook test**

Append to `server/internal/mind/engine/engine_test.go`:

```go
func TestIngestExecutesPendingActionsWhenExecutorConfigured(t *testing.T) {
	svc, _ := newEngineForTest(t)
	exec := &fakeExecutor{}
	svc.UseExecutor(exec)
	at := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	actions, err := svc.Ingest(context.Background(), mind.Event{ID: "evt-1", DeviceID: "dev-001", Type: mind.EventReminderDue, Source: mind.SourceScheduler, At: at})
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("actions = %+v, want 1", actions)
	}
	if exec.calls != 1 {
		t.Fatalf("executor calls = %d, want 1", exec.calls)
	}
}

type fakeExecutor struct{ calls int }

func (f *fakeExecutor) Execute(ctx context.Context, action mind.Action) error {
	f.calls++
	return nil
}
```

- [ ] **Step 2: Run failing engine test**

Run:

```powershell
Set-Location server
$env:GOPROXY="https://goproxy.cn,direct"; $env:GOSUMDB="off"; $env:CGO_ENABLED="0"; go test ./internal/mind/engine
```

Expected: FAIL with `UseExecutor undefined`.

- [ ] **Step 3: Implement engine executor hook**

Modify `server/internal/mind/engine/engine.go`:

```go
type ActionExecutor interface {
	Execute(ctx context.Context, action mind.Action) error
}

type Service struct {
	store *mind.Store
	executor ActionExecutor
}

func (s *Service) UseExecutor(executor ActionExecutor) {
	s.executor = executor
}
```

Inside `runPipeline`, after `s.store.SaveAction(ctx, selected)` succeeds, add:

```go
		if s.executor != nil && selected.Status == mind.ActionPending {
			if err := s.executor.Execute(ctx, selected); err != nil {
				selected.Status = mind.ActionFailed
				selected.Reason = err.Error()
				_ = s.store.SaveAction(ctx, selected)
				return nil, err
			}
		}
```

- [ ] **Step 4: Run engine tests**

Run:

```powershell
Set-Location server
$env:GOPROXY="https://goproxy.cn,direct"; $env:GOSUMDB="off"; $env:CGO_ENABLED="0"; go test ./internal/mind/engine
```

Expected: PASS.

- [ ] **Step 5: Wire dispatcher into engine in main**

In `server/cmd/anban/main.go`, after `mindDispatcher` is constructed, add:

```go
	mindEngine.UseExecutor(mindActionExecutor{dispatcher: mindDispatcher})
```

Add:

```go
type mindActionExecutor struct {
	dispatcher *executors.Dispatcher
}

func (e mindActionExecutor) Execute(ctx context.Context, action mind.Action) error {
	_, err := e.dispatcher.Execute(ctx, action)
	return err
}
```

Remove `_ = mindDispatcher`.

- [ ] **Step 6: Run full tests**

Run:

```powershell
Set-Location server
$env:GOPROXY="https://goproxy.cn,direct"; $env:GOSUMDB="off"; $env:CGO_ENABLED="0"; go test ./...
```

Expected: PASS.

- [ ] **Step 7: Commit**

```powershell
git add server/internal/mind/engine server/cmd/anban/main.go
git commit -m "feat(mind): execute selected actions"
```

---

### Task 17: Idle, Reflection, and Life Scheduling

**Files:**
- Modify: `server/cmd/anban/main.go`

- [ ] **Step 1: Add scheduling helper functions**

Add below `startVisionPresencePoller` in `server/cmd/anban/main.go`:

```go
func startMindLoops(sch *scheduler.Scheduler, profileStore *profile.Store, mindEngine mind.Engine, interval time.Duration) {
	if interval <= 0 {
		interval = 15 * time.Minute
	}
	var scheduleNext func()
	scheduleNext = func() {
		if _, err := sch.ScheduleAt(time.Now().Add(interval), func() {
			runMindLoops(profileStore, mindEngine)
			scheduleNext()
		}); err != nil {
			log.Printf("mind loops 调度失败: %v", err)
		}
	}
	scheduleNext()
	log.Printf("mind loops enabled: interval=%s", interval)
}

func runMindLoops(profileStore *profile.Store, mindEngine mind.Engine) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	deviceIDs, err := profileStore.ListDeviceIDs(ctx)
	if err != nil {
		log.Printf("mind loops 获取设备列表失败: %v", err)
		return
	}
	now := time.Now().UTC()
	for _, deviceID := range deviceIDs {
		if _, err := mindEngine.TickIdle(ctx, deviceID, now); err != nil {
			log.Printf("mind idle tick 失败 device=%s: %v", deviceID, err)
		}
		window := mind.TimeWindow{From: now.Add(-30 * time.Minute), To: now}
		if err := mindEngine.Reflect(ctx, deviceID, window); err != nil {
			log.Printf("mind reflection 失败 device=%s: %v", deviceID, err)
		}
	}
}
```

- [ ] **Step 2: Start loops in main**

After `startVisionPresencePoller(...)`, add:

```go
	startMindLoops(sch, profileStore, mindEngine, 15*time.Minute)
```

- [ ] **Step 3: Run full tests**

Run:

```powershell
Set-Location server
$env:GOPROXY="https://goproxy.cn,direct"; $env:GOSUMDB="off"; $env:CGO_ENABLED="0"; go test ./...
```

Expected: PASS.

- [ ] **Step 4: Commit**

```powershell
git add server/cmd/anban/main.go
git commit -m "feat(mind): schedule idle and reflection loops"
```

---

### Task 18: Architecture Guardrails

**Files:**
- Modify: `server/internal/architecture/architecture_test.go`

- [ ] **Step 1: Add architecture test**

Append to `server/internal/architecture/architecture_test.go`:

```go
func TestDomainsDoNotImportMind(t *testing.T) {
	serverRoot := mustServerRoot(t)
	domainRoot := filepath.Join(serverRoot, "internal", "domains")
	for _, file := range goProductionFiles(t, domainRoot) {
		for _, importPath := range importsOf(t, file) {
			if strings.Contains(importPath, "/server/internal/mind") {
				t.Fatalf("%s imports mind; domains must remain executors and emit local events only", rel(t, serverRoot, file))
			}
		}
	}
}
```

- [ ] **Step 2: Run architecture test**

Run:

```powershell
Set-Location server
$env:GOPROXY="https://goproxy.cn,direct"; $env:GOSUMDB="off"; $env:CGO_ENABLED="0"; go test ./internal/architecture
```

Expected: PASS.

- [ ] **Step 3: Run full tests**

Run:

```powershell
Set-Location server
$env:GOPROXY="https://goproxy.cn,direct"; $env:GOSUMDB="off"; $env:CGO_ENABLED="0"; go test ./...
```

Expected: PASS.

- [ ] **Step 4: Commit**

```powershell
git add server/internal/architecture/architecture_test.go
git commit -m "test(architecture): keep domains independent from mind"
```

---

### Task 19: One-Day Simulation Test

**Files:**
- Create: `server/internal/mind/simulation/day_test.go`

- [ ] **Step 1: Write simulation test**

Create `server/internal/mind/simulation/day_test.go`:

```go
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
	ms := mind.NewStore(st.DB)
	if err := ms.AutoMigrate(); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	svc := engine.New(ms)
	ctx := context.Background()
	deviceID := "dev-001"

	morning := time.Date(2026, 6, 16, 8, 10, 0, 0, time.UTC)
	if _, err := svc.Ingest(ctx, mind.Event{ID: "evt-presence", DeviceID: deviceID, Type: mind.EventPresenceSeen, Source: mind.SourceVision, At: morning, Summary: "老人早晨出现"}); err != nil {
		t.Fatalf("presence ingest: %v", err)
	}
	noon := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	actions, err := svc.Ingest(ctx, mind.Event{ID: "evt-reminder", DeviceID: deviceID, Type: mind.EventReminderDue, Source: mind.SourceScheduler, At: noon, Summary: "午间吃药提醒"})
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
	state, err := ms.GetSelfState(ctx, deviceID)
	if err != nil {
		t.Fatalf("GetSelfState: %v", err)
	}
	if state.Concern <= 0.30 {
		t.Fatalf("state = %+v, want concern above default after day events", state)
	}
}
```

- [ ] **Step 2: Run simulation test**

Run:

```powershell
Set-Location server
$env:GOPROXY="https://goproxy.cn,direct"; $env:GOSUMDB="off"; $env:CGO_ENABLED="0"; go test ./internal/mind/simulation
```

Expected: PASS.

- [ ] **Step 3: Run full tests**

Run:

```powershell
Set-Location server
$env:GOPROXY="https://goproxy.cn,direct"; $env:GOSUMDB="off"; $env:CGO_ENABLED="0"; go test ./...
```

Expected: PASS.

- [ ] **Step 4: Commit**

```powershell
git add server/internal/mind/simulation/day_test.go
git commit -m "test(mind): simulate one day of companion continuity"
```

---

### Task 20: Documentation Status Update

**Files:**
- Modify: `docs/superpowers/specs/2026-06-16-anban-mind-design.md`
- Modify: `docs/REALTIME_CHANGELOG.md`

- [ ] **Step 1: Append implementation status to spec**

Append to `docs/superpowers/specs/2026-06-16-anban-mind-design.md`:

```markdown

## 17. 实施状态

- 实施计划：`docs/superpowers/plans/2026-06-16-anban-mind-implementation.md`
- 代码入口：`server/internal/mind`
- 装配入口：`server/cmd/anban/main.go`
- 验证命令：在 `server/` 下运行 `$env:GOPROXY="https://goproxy.cn,direct"; $env:GOSUMDB="off"; $env:CGO_ENABLED="0"; go test ./...`
```

- [ ] **Step 2: Append changelog entry**

Append to the top of `docs/REALTIME_CHANGELOG.md` below the title:

```markdown

### AnBan Mind 统一心智层

- 文件：`server/internal/mind/`、`server/cmd/anban/main.go`、`server/internal/domains/{message,reminder,vision,greeting}/`
- 内容：新增 AnBan Mind 心智层，统一事件流、处境、自我状态、动机、内心思流、行为选择、表达闸门、反思和生活流；现有 domain 逐步归位为 action executor。
- 边界：继续保持方案 C；xiaozhi 仍负责语音运行时、设备连接、打断、MCP 和对话历史，安伴负责为什么说、何时说、说什么和是否不说。
- 验证：`go test ./...`
```

- [ ] **Step 3: Run full tests**

Run:

```powershell
Set-Location server
$env:GOPROXY="https://goproxy.cn,direct"; $env:GOSUMDB="off"; $env:CGO_ENABLED="0"; go test ./...
```

Expected: PASS.

- [ ] **Step 4: Commit**

```powershell
git add docs/superpowers/specs/2026-06-16-anban-mind-design.md docs/REALTIME_CHANGELOG.md
git commit -m "docs(mind): record implementation status"
```

---

## Self-Review Checklist

Spec coverage:

- Central orchestration: Tasks 6, 12, 16.
- Event stream: Tasks 1, 2, 13, 14, 15.
- Situation: Task 3.
- SelfState: Task 3.
- Drives: Task 4.
- Thoughts: Task 4.
- Behavior and expression: Task 5.
- Executors and domain归位: Tasks 7-10, 13-16.
- Reflection: Task 11 and Task 17.
- Life loop: Task 11 and Task 17.
- Scheme C boundary: Tasks 12, 18.
- Testing strategy: Tasks 2-19.

Placeholder scan:

- This plan avoids placeholder markers and scope-shrinking labels from the banned list.
- Every task has exact paths, commands, and expected outcomes.

Type consistency:

- Shared public types are defined in Task 1.
- Store APIs used in later tasks are defined in Task 2.
- Engine APIs used in wiring tasks are defined in Task 6.
- Executor APIs used in wiring tasks are defined in Task 7.
