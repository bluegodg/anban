package mind

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"
)

var ErrInvalidCursor = errors.New("mind: invalid cursor")

type ReadService struct {
	store *Store
}

func NewReadService(store *Store) *ReadService {
	return &ReadService{store: store}
}

type PublicSelfState struct {
	Warmth      float64 `json:"warmth"`
	Concern     float64 `json:"concern"`
	Curiosity   float64 `json:"curiosity"`
	Playfulness float64 `json:"playfulness"`
	Energy      float64 `json:"energy"`
	Quietness   float64 `json:"quietness"`
	Patience    float64 `json:"patience"`
	Confidence  float64 `json:"confidence"`
}

type PublicLifeState struct {
	TodayTheme              string   `json:"todayTheme,omitempty"`
	LingeringThoughts       []string `json:"lingeringThoughts,omitempty"`
	SocialEnergy            float64  `json:"socialEnergy"`
	CareFocus               string   `json:"careFocus,omitempty"`
	PlayfulnessTrend        float64  `json:"playfulnessTrend"`
	RelationshipTemperature float64  `json:"relationshipTemperature"`
}

type PublicThought struct {
	At            time.Time `json:"at"`
	Content       string    `json:"content"`
	Drive         string    `json:"drive,omitempty"`
	EmotionalTone string    `json:"emotionalTone,omitempty"`
}

type PublicAction struct {
	At     time.Time    `json:"at"`
	Type   ActionType   `json:"type,omitempty"`
	Status ActionStatus `json:"status,omitempty"`
	Reason string       `json:"reason,omitempty"`
	Text   string       `json:"text,omitempty"`
}

type MindSnapshot struct {
	Available     bool             `json:"available"`
	UpdatedAt     *time.Time       `json:"updatedAt"`
	SelfState     *PublicSelfState `json:"selfState"`
	LifeState     *PublicLifeState `json:"lifeState"`
	LatestThought *PublicThought   `json:"latestThought"`
	LatestAction  *PublicAction    `json:"latestAction"`
}

type TimelineKind string

const (
	TimelineKindAll        TimelineKind = "all"
	TimelineKindThought    TimelineKind = "thought"
	TimelineKindAction     TimelineKind = "action"
	TimelineKindEvent      TimelineKind = "event"
	TimelineKindReflection TimelineKind = "reflection"
)

type TimelineDecision struct {
	At     time.Time    `json:"at"`
	Type   ActionType   `json:"type,omitempty"`
	Status ActionStatus `json:"status,omitempty"`
	Reason string       `json:"reason,omitempty"`
	Text   string       `json:"text,omitempty"`
}

type TimelineItem struct {
	Kind           TimelineKind      `json:"kind"`
	At             time.Time         `json:"at"`
	Text           string            `json:"text"`
	Category       string            `json:"category,omitempty"`
	Emotion        string            `json:"emotion,omitempty"`
	Status         string            `json:"status,omitempty"`
	Reason         string            `json:"reason,omitempty"`
	Decision       *TimelineDecision `json:"decision,omitempty"`
	RelatedThought string            `json:"relatedThought,omitempty"`
	Lessons        []string          `json:"lessons,omitempty"`

	sortKind int
	sortKey  uint
}

type TimelineQuery struct {
	DeviceID string
	Kind     TimelineKind
	Limit    int
	Cursor   string
}

type TimelinePage struct {
	Items      []TimelineItem `json:"items"`
	NextCursor string         `json:"nextCursor"`
	HasMore    bool           `json:"hasMore"`
}

func (s *ReadService) Snapshot(ctx context.Context, deviceID string) (MindSnapshot, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" || s == nil || s.store == nil {
		return MindSnapshot{}, ErrInvalidInput
	}

	var snapshot MindSnapshot
	if state, err := s.store.GetSelfState(ctx, deviceID); err == nil {
		snapshot.Available = true
		snapshot.SelfState = &PublicSelfState{
			Warmth: state.Warmth, Concern: state.Concern, Curiosity: state.Curiosity, Playfulness: state.Playfulness,
			Energy: state.Energy, Quietness: state.Quietness, Patience: state.Patience, Confidence: state.Confidence,
		}
		snapshot.UpdatedAt = maxPublicTime(snapshot.UpdatedAt, state.At)
	} else if !errors.Is(err, ErrNotFound) {
		return MindSnapshot{}, err
	}

	if life, err := s.store.GetLifeState(ctx, deviceID); err == nil {
		snapshot.Available = true
		snapshot.LifeState = &PublicLifeState{
			TodayTheme: life.TodayTheme, LingeringThoughts: life.LingeringThoughts, SocialEnergy: life.SocialEnergy,
			CareFocus: life.CareFocus, PlayfulnessTrend: life.PlayfulnessTrend, RelationshipTemperature: life.RelationshipTemperature,
		}
		snapshot.UpdatedAt = maxPublicTime(snapshot.UpdatedAt, life.At)
	} else if !errors.Is(err, ErrNotFound) {
		return MindSnapshot{}, err
	}

	thoughts, err := s.listThoughtRecords(ctx, deviceID)
	if err != nil {
		return MindSnapshot{}, err
	}
	sort.SliceStable(thoughts, func(i, j int) bool {
		return compareSort(thoughts[i].At, thoughtSortKind(), thoughts[i].ID, thoughts[j].At, thoughtSortKind(), thoughts[j].ID) < 0
	})
	if len(thoughts) > 0 {
		latest := thoughts[0]
		snapshot.Available = true
		snapshot.LatestThought = &PublicThought{At: latest.At.UTC(), Content: latest.Content, Drive: latest.DriveName, EmotionalTone: latest.EmotionalTone}
		snapshot.UpdatedAt = maxPublicTime(snapshot.UpdatedAt, latest.At)
		if action, ok, err := s.findActionForThought(ctx, deviceID, latest.ThoughtID); err != nil {
			return MindSnapshot{}, err
		} else if ok {
			snapshot.LatestAction = publicActionFromRecord(action)
			snapshot.UpdatedAt = maxPublicTime(snapshot.UpdatedAt, action.CreatedAt)
		}
	}

	return snapshot, nil
}

func (s *ReadService) Timeline(ctx context.Context, query TimelineQuery) (TimelinePage, error) {
	deviceID := strings.TrimSpace(query.DeviceID)
	if deviceID == "" || s == nil || s.store == nil {
		return TimelinePage{}, ErrInvalidInput
	}
	kind, err := normalizeTimelineKind(query.Kind)
	if err != nil {
		return TimelinePage{}, err
	}
	limit := query.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}
	cursor, err := decodeTimelineCursor(query.Cursor)
	if err != nil {
		return TimelinePage{}, err
	}

	items, err := s.timelineItems(ctx, deviceID, kind)
	if err != nil {
		return TimelinePage{}, err
	}
	sort.SliceStable(items, func(i, j int) bool {
		return compareTimelineItems(items[i], items[j]) < 0
	})
	if cursor != nil {
		filtered := items[:0]
		for _, item := range items {
			if compareItemToCursor(item, *cursor) > 0 {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}

	page := TimelinePage{}
	if len(items) > limit {
		page.HasMore = true
		items = items[:limit]
	}
	page.Items = stripSortFields(items)
	if page.HasMore && len(page.Items) > 0 {
		page.NextCursor = encodeTimelineCursor(items[len(items)-1])
	}
	return page, nil
}

func (s *ReadService) timelineItems(ctx context.Context, deviceID string, kind TimelineKind) ([]TimelineItem, error) {
	actions, err := s.listActionRecords(ctx, deviceID)
	if err != nil {
		return nil, err
	}
	actionByIntention := latestActionsByIntention(actions)

	switch kind {
	case TimelineKindAction:
		thoughts, err := s.listThoughtRecords(ctx, deviceID)
		if err != nil {
			return nil, err
		}
		thoughtByID := map[string]string{}
		for _, thought := range thoughts {
			thoughtByID[thought.ThoughtID] = thought.Content
		}
		items := make([]TimelineItem, 0, len(actions))
		for _, action := range actions {
			items = append(items, TimelineItem{
				Kind: TimelineKindAction, At: action.CreatedAt.UTC(), Text: firstNonEmpty(action.Text, action.Reason),
				Category: action.Type, Status: action.Status, Reason: action.Reason,
				RelatedThought: thoughtByID[strings.TrimPrefix(action.IntentionID, "intention-")],
				sortKind:       actionSortKind(), sortKey: action.ID,
			})
		}
		return items, nil
	case TimelineKindThought, TimelineKindAll:
		thoughts, err := s.listThoughtRecords(ctx, deviceID)
		if err != nil {
			return nil, err
		}
		items := make([]TimelineItem, 0, len(thoughts))
		for _, thought := range thoughts {
			item := TimelineItem{
				Kind: TimelineKindThought, At: thought.At.UTC(), Text: thought.Content, Category: thought.DriveName,
				Emotion: thought.EmotionalTone, sortKind: thoughtSortKind(), sortKey: thought.ID,
			}
			if action, ok := actionByIntention["intention-"+thought.ThoughtID]; ok {
				item.Decision = timelineDecisionFromRecord(action)
			}
			items = append(items, item)
		}
		if kind == TimelineKindThought {
			return items, nil
		}
		events, err := s.eventTimelineItems(ctx, deviceID)
		if err != nil {
			return nil, err
		}
		reflections, err := s.reflectionTimelineItems(ctx, deviceID)
		if err != nil {
			return nil, err
		}
		items = append(items, events...)
		items = append(items, reflections...)
		return items, nil
	case TimelineKindEvent:
		return s.eventTimelineItems(ctx, deviceID)
	case TimelineKindReflection:
		return s.reflectionTimelineItems(ctx, deviceID)
	default:
		return nil, ErrInvalidInput
	}
}

func (s *ReadService) eventTimelineItems(ctx context.Context, deviceID string) ([]TimelineItem, error) {
	var records []eventRecord
	if err := s.store.db.WithContext(ctx).Where("device_id = ?", deviceID).Find(&records).Error; err != nil {
		return nil, err
	}
	items := make([]TimelineItem, 0, len(records))
	for _, rec := range records {
		items = append(items, TimelineItem{
			Kind: TimelineKindEvent, At: rec.At.UTC(), Text: rec.Summary, Category: rec.Type,
			Emotion: rec.Emotion, sortKind: eventSortKind(), sortKey: rec.ID,
		})
	}
	return items, nil
}

func (s *ReadService) reflectionTimelineItems(ctx context.Context, deviceID string) ([]TimelineItem, error) {
	var records []reflectionRecord
	if err := s.store.db.WithContext(ctx).Where("device_id = ?", deviceID).Find(&records).Error; err != nil {
		return nil, err
	}
	items := make([]TimelineItem, 0, len(records))
	for _, rec := range records {
		item := TimelineItem{Kind: TimelineKindReflection, At: rec.At.UTC(), Text: rec.EpisodeSummary, Category: "reflection", sortKind: reflectionSortKind(), sortKey: rec.ID}
		if rec.BehaviorLessonsJSON != "" {
			_ = json.Unmarshal([]byte(rec.BehaviorLessonsJSON), &item.Lessons)
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *ReadService) listThoughtRecords(ctx context.Context, deviceID string) ([]thoughtRecord, error) {
	var records []thoughtRecord
	err := s.store.db.WithContext(ctx).Where("device_id = ?", deviceID).Find(&records).Error
	return records, err
}

func (s *ReadService) listActionRecords(ctx context.Context, deviceID string) ([]actionRecord, error) {
	var records []actionRecord
	err := s.store.db.WithContext(ctx).Where("device_id = ?", deviceID).Find(&records).Error
	return records, err
}

func (s *ReadService) findActionForThought(ctx context.Context, deviceID, thoughtID string) (actionRecord, bool, error) {
	var records []actionRecord
	err := s.store.db.WithContext(ctx).Where("device_id = ? AND intention_id = ?", deviceID, "intention-"+thoughtID).Find(&records).Error
	if err != nil {
		return actionRecord{}, false, err
	}
	if len(records) == 0 {
		return actionRecord{}, false, nil
	}
	sort.SliceStable(records, func(i, j int) bool {
		return compareSort(records[i].CreatedAt, actionSortKind(), records[i].ID, records[j].CreatedAt, actionSortKind(), records[j].ID) < 0
	})
	return records[0], true, nil
}

func latestActionsByIntention(records []actionRecord) map[string]actionRecord {
	out := map[string]actionRecord{}
	for _, rec := range records {
		if rec.IntentionID == "" {
			continue
		}
		current, ok := out[rec.IntentionID]
		if !ok || compareSort(rec.CreatedAt, actionSortKind(), rec.ID, current.CreatedAt, actionSortKind(), current.ID) < 0 {
			out[rec.IntentionID] = rec
		}
	}
	return out
}

func publicActionFromRecord(rec actionRecord) *PublicAction {
	return &PublicAction{At: rec.CreatedAt.UTC(), Type: ActionType(rec.Type), Status: ActionStatus(rec.Status), Reason: rec.Reason, Text: rec.Text}
}

func timelineDecisionFromRecord(rec actionRecord) *TimelineDecision {
	return &TimelineDecision{At: rec.CreatedAt.UTC(), Type: ActionType(rec.Type), Status: ActionStatus(rec.Status), Reason: rec.Reason, Text: rec.Text}
}

func normalizeTimelineKind(kind TimelineKind) (TimelineKind, error) {
	switch kind {
	case "", TimelineKindAll:
		return TimelineKindAll, nil
	case TimelineKindThought, TimelineKindAction, TimelineKindEvent, TimelineKindReflection:
		return kind, nil
	default:
		return "", ErrInvalidInput
	}
}

type timelineCursor struct {
	At       time.Time    `json:"at"`
	Kind     TimelineKind `json:"kind"`
	SortKind int          `json:"sortKind"`
	SortKey  uint         `json:"sortKey"`
}

func encodeTimelineCursor(item TimelineItem) string {
	body, err := json.Marshal(timelineCursor{At: item.At.UTC(), Kind: item.Kind, SortKind: item.sortKind, SortKey: item.sortKey})
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(body)
}

func decodeTimelineCursor(raw string) (*timelineCursor, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	body, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, ErrInvalidCursor
	}
	var cursor timelineCursor
	if err := json.Unmarshal(body, &cursor); err != nil || cursor.At.IsZero() || cursor.SortKey == 0 {
		return nil, ErrInvalidCursor
	}
	return &cursor, nil
}

func compareTimelineItems(a, b TimelineItem) int {
	return compareSort(a.At, a.sortKind, a.sortKey, b.At, b.sortKind, b.sortKey)
}

func compareItemToCursor(item TimelineItem, cursor timelineCursor) int {
	return compareSort(item.At, item.sortKind, item.sortKey, cursor.At, cursor.SortKind, cursor.SortKey)
}

func compareSort(aAt time.Time, aKind int, aKey uint, bAt time.Time, bKind int, bKey uint) int {
	a := aAt.UTC()
	b := bAt.UTC()
	if !a.Equal(b) {
		if a.After(b) {
			return -1
		}
		return 1
	}
	if aKind != bKind {
		if aKind < bKind {
			return -1
		}
		return 1
	}
	if aKey == bKey {
		return 0
	}
	if aKey > bKey {
		return -1
	}
	return 1
}

func stripSortFields(items []TimelineItem) []TimelineItem {
	out := make([]TimelineItem, len(items))
	copy(out, items)
	for i := range out {
		out[i].sortKind = 0
		out[i].sortKey = 0
	}
	return out
}

func maxPublicTime(current *time.Time, candidate time.Time) *time.Time {
	if candidate.IsZero() {
		return current
	}
	normalized := candidate.UTC()
	if current == nil || normalized.After((*current).UTC()) {
		return &normalized
	}
	return current
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func thoughtSortKind() int    { return 0 }
func eventSortKind() int      { return 1 }
func reflectionSortKind() int { return 2 }
func actionSortKind() int     { return 3 }
