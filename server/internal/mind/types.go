package mind

import (
	"context"
	"time"
)

type EventType string

const (
	EventElderSpoke           EventType = "elder_spoke"
	EventAssistantSpoke       EventType = "assistant_spoke"
	EventChildMessageReceived EventType = "child_message_received"
	EventReminderCreated      EventType = "reminder_created"
	EventReminderDue          EventType = "reminder_due"
	EventReminderAcknowledged EventType = "reminder_acknowledged"
	EventGreetingRequested    EventType = "greeting_requested"
	EventPresenceSeen         EventType = "presence_seen"
	EventPresenceAbsent       EventType = "presence_absent"
	EventVisionObservation    EventType = "vision_observation"
	EventDeviceOnline         EventType = "device_online"
	EventDeviceOffline        EventType = "device_offline"
	EventLongSilence          EventType = "long_silence"
	EventProfileUpdated       EventType = "profile_updated"
	EventMemoryDistilled      EventType = "memory_distilled"
	EventActionExecuted       EventType = "action_executed"
	EventFeedbackObserved     EventType = "feedback_observed"
	EventIdleTick             EventType = "idle_tick"
	EventReflectionTick       EventType = "reflection_tick"
	EventLifeTick             EventType = "life_tick"
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
	LastUsedAt       *time.Time
	DecayPolicy      string
}

type SelfState struct {
	DeviceID          string
	At                time.Time
	Warmth            float64
	Concern           float64
	Curiosity         float64
	Playfulness       float64
	Energy            float64
	Quietness         float64
	Patience          float64
	Confidence        float64
	FamilyWeight      float64
	PetWeight         float64
	StewardWeight     float64
	ProcessedEventIDs []string
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
	IntentionCheckIn      IntentionKind = "check_in"
	IntentionComfort      IntentionKind = "comfort"
	IntentionShare        IntentionKind = "share"
	IntentionRemind       IntentionKind = "remind"
	IntentionAsk          IntentionKind = "ask"
	IntentionObserve      IntentionKind = "observe"
	IntentionWait         IntentionKind = "wait"
	IntentionNotifyChild  IntentionKind = "notify_child"
	IntentionUpdateMemory IntentionKind = "update_memory"
	IntentionSyncPrompt   IntentionKind = "sync_prompt"
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
	ActionPending    ActionStatus = "pending"
	ActionExecuted   ActionStatus = "executed"
	ActionDeferred   ActionStatus = "deferred"
	ActionSuppressed ActionStatus = "suppressed"
	ActionFailed     ActionStatus = "failed"
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
	ExecutorRef  string
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
	ID               string
	DeviceID         string
	At               time.Time
	EpisodeSummary   string
	MemoryIDs        []string
	StateAdjustments map[string]float64
	BehaviorLessons  []string
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
	UpdateLife(ctx context.Context, deviceID string, at time.Time) error
}
