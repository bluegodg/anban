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
	ID                      uint   `gorm:"primaryKey"`
	DeviceID                string `gorm:"uniqueIndex;not null"`
	At                      time.Time
	TodayTheme              string `gorm:"size:200"`
	LingeringThoughtsJSON   string `gorm:"type:text"`
	SocialEnergy            float64
	CareFocus               string `gorm:"size:200"`
	PlayfulnessTrend        float64
	RelationshipTemperature float64
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

func (s *Store) AppendEvent(ctx context.Context, event Event) error {
	payload, err := encodeJSON(event.Payload)
	if err != nil {
		return err
	}

	rec := eventRecord{
		EventID:     event.ID,
		DeviceID:    event.DeviceID,
		Type:        string(event.Type),
		Source:      string(event.Source),
		At:          event.At,
		Summary:     event.Summary,
		PayloadJSON: payload,
		Salience:    event.Salience,
		Emotion:     event.Emotion,
		Confidence:  event.Confidence,
	}
	return s.db.WithContext(ctx).Create(&rec).Error
}

func (s *Store) ListRecentEvents(ctx context.Context, deviceID string, limit int) ([]Event, error) {
	if limit <= 0 {
		limit = 20
	}

	var records []eventRecord
	err := s.db.WithContext(ctx).
		Where("device_id = ?", deviceID).
		Order("at desc, id desc").
		Limit(limit).
		Find(&records).Error
	if err != nil {
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
		out = append(out, Event{
			ID:         rec.EventID,
			DeviceID:   rec.DeviceID,
			Type:       EventType(rec.Type),
			Source:     EventSource(rec.Source),
			At:         rec.At,
			Summary:    rec.Summary,
			Payload:    payload,
			Salience:   rec.Salience,
			Emotion:    rec.Emotion,
			Confidence: rec.Confidence,
		})
	}
	return out, nil
}

func (s *Store) SaveSelfState(ctx context.Context, state SelfState) error {
	rec := selfStateRecord{
		DeviceID:      state.DeviceID,
		At:            state.At,
		Warmth:        state.Warmth,
		Concern:       state.Concern,
		Curiosity:     state.Curiosity,
		Playfulness:   state.Playfulness,
		Energy:        state.Energy,
		Quietness:     state.Quietness,
		Patience:      state.Patience,
		Confidence:    state.Confidence,
		FamilyWeight:  state.FamilyWeight,
		PetWeight:     state.PetWeight,
		StewardWeight: state.StewardWeight,
	}

	var existing selfStateRecord
	err := s.db.WithContext(ctx).Where("device_id = ?", state.DeviceID).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return s.db.WithContext(ctx).Create(&rec).Error
	}
	if err != nil {
		return err
	}

	rec.ID = existing.ID
	rec.CreatedAt = existing.CreatedAt
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

	return SelfState{
		DeviceID:      rec.DeviceID,
		At:            rec.At,
		Warmth:        rec.Warmth,
		Concern:       rec.Concern,
		Curiosity:     rec.Curiosity,
		Playfulness:   rec.Playfulness,
		Energy:        rec.Energy,
		Quietness:     rec.Quietness,
		Patience:      rec.Patience,
		Confidence:    rec.Confidence,
		FamilyWeight:  rec.FamilyWeight,
		PetWeight:     rec.PetWeight,
		StewardWeight: rec.StewardWeight,
	}, nil
}

func (s *Store) SaveThought(ctx context.Context, thought Thought) error {
	sourceIDs, err := encodeJSON(thought.SourceEventIDs)
	if err != nil {
		return err
	}
	memoryIDs, err := encodeJSON(thought.RelatedMemoryIDs)
	if err != nil {
		return err
	}

	rec := thoughtRecord{
		ThoughtID:            thought.ID,
		DeviceID:             thought.DeviceID,
		At:                   thought.At,
		Content:              thought.Content,
		SourceEventIDsJSON:   sourceIDs,
		RelatedMemoryIDsJSON: memoryIDs,
		DriveName:            thought.DriveName,
		EmotionalTone:        thought.EmotionalTone,
		Urgency:              thought.Urgency,
		CareValue:            thought.CareValue,
		Novelty:              thought.Novelty,
		InterruptionCost:     thought.InterruptionCost,
		Intimacy:             thought.Intimacy,
		Status:               string(thought.Status),
	}
	return s.db.WithContext(ctx).Create(&rec).Error
}

func (s *Store) SaveAction(ctx context.Context, action Action) error {
	args, err := encodeJSON(action.Args)
	if err != nil {
		return err
	}

	rec := actionRecord{
		ActionID:     action.ID,
		DeviceID:     action.DeviceID,
		IntentionID:  action.IntentionID,
		Type:         string(action.Type),
		Executor:     action.Executor,
		Text:         action.Text,
		ArgsJSON:     args,
		ScheduledFor: action.ScheduledFor,
		Status:       string(action.Status),
		Reason:       action.Reason,
		Score:        action.Score,
	}
	return s.db.WithContext(ctx).Create(&rec).Error
}

func (s *Store) SaveFeedback(ctx context.Context, feedback Feedback) error {
	effects, err := encodeJSON(feedback.EffectOnState)
	if err != nil {
		return err
	}

	rec := feedbackRecord{
		FeedbackID:        feedback.ID,
		DeviceID:          feedback.DeviceID,
		ActionID:          feedback.ActionID,
		At:                feedback.At,
		Kind:              feedback.Kind,
		Signal:            feedback.Signal,
		EffectOnStateJSON: effects,
		Notes:             feedback.Notes,
	}
	return s.db.WithContext(ctx).Create(&rec).Error
}

func (s *Store) SaveReflection(ctx context.Context, reflection Reflection) error {
	memoryIDs, err := encodeJSON(reflection.MemoryIDs)
	if err != nil {
		return err
	}
	adjustments, err := encodeJSON(reflection.StateAdjustments)
	if err != nil {
		return err
	}
	lessons, err := encodeJSON(reflection.BehaviorLessons)
	if err != nil {
		return err
	}

	rec := reflectionRecord{
		ReflectionID:         reflection.ID,
		DeviceID:             reflection.DeviceID,
		At:                   reflection.At,
		EpisodeSummary:       reflection.EpisodeSummary,
		MemoryIDsJSON:        memoryIDs,
		StateAdjustmentsJSON: adjustments,
		BehaviorLessonsJSON:  lessons,
	}
	return s.db.WithContext(ctx).Create(&rec).Error
}

func (s *Store) SaveLifeState(ctx context.Context, life LifeState) error {
	thoughts, err := encodeJSON(life.LingeringThoughts)
	if err != nil {
		return err
	}

	rec := lifeStateRecord{
		DeviceID:                life.DeviceID,
		At:                      life.At,
		TodayTheme:              life.TodayTheme,
		LingeringThoughtsJSON:   thoughts,
		SocialEnergy:            life.SocialEnergy,
		CareFocus:               life.CareFocus,
		PlayfulnessTrend:        life.PlayfulnessTrend,
		RelationshipTemperature: life.RelationshipTemperature,
	}

	var existing lifeStateRecord
	err = s.db.WithContext(ctx).Where("device_id = ?", life.DeviceID).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return s.db.WithContext(ctx).Create(&rec).Error
	}
	if err != nil {
		return err
	}

	rec.ID = existing.ID
	rec.CreatedAt = existing.CreatedAt
	return s.db.WithContext(ctx).Save(&rec).Error
}

func (s *Store) GetLifeState(ctx context.Context, deviceID string) (LifeState, error) {
	var rec lifeStateRecord
	err := s.db.WithContext(ctx).Where("device_id = ?", deviceID).First(&rec).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return LifeState{}, ErrNotFound
	}
	if err != nil {
		return LifeState{}, err
	}

	var thoughts []string
	if rec.LingeringThoughtsJSON != "" {
		if err := json.Unmarshal([]byte(rec.LingeringThoughtsJSON), &thoughts); err != nil {
			return LifeState{}, err
		}
	}

	return LifeState{
		DeviceID:                rec.DeviceID,
		At:                      rec.At,
		TodayTheme:              rec.TodayTheme,
		LingeringThoughts:       thoughts,
		SocialEnergy:            rec.SocialEnergy,
		CareFocus:               rec.CareFocus,
		PlayfulnessTrend:        rec.PlayfulnessTrend,
		RelationshipTemperature: rec.RelationshipTemperature,
	}, nil
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
