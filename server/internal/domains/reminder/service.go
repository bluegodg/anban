package reminder

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/bluegodg/anban/server/internal/scheduler"
	"github.com/bluegodg/anban/server/internal/xiaozhiclient"
	sharedtypes "github.com/bluegodg/anban/server/pkg/types"
)

const (
	defaultAckTimeout     = 30 * time.Minute
	proactiveRetryDelay   = time.Minute
	voiceAckPollInterval  = 10 * time.Second
	voiceAckHistoryLimit  = 20
	voiceAckTimestampSkew = 2 * time.Second
	reminderInjectTimeout = 60 * time.Second
	minReminderTextRunes  = 30
	maxReminderTextRunes  = 60
)

var voiceAcknowledgementPhrases = map[string]struct{}{
	"好":    {},
	"好的":   {},
	"好啊":   {},
	"好呀":   {},
	"好嘞":   {},
	"嗯好":   {},
	"嗯好的":  {},
	"嗯嗯好":  {},
	"知道了":  {},
	"知道啦":  {},
	"我知道了": {},
	"收到":   {},
	"收到了":  {},
	"我收到了": {},
}

type OneShotScheduler interface {
	ScheduleAt(t time.Time, fn func()) (scheduler.JobID, error)
	Cancel(id scheduler.JobID)
}

type Service struct {
	store     *Store
	xc        xiaozhiclient.Client
	sch       OneShotScheduler
	now       func() time.Time
	voiceGate sharedtypes.ProactiveVoiceGate
}

func NewService(store *Store, xc xiaozhiclient.Client, sch OneShotScheduler) *Service {
	return &Service{
		store: store,
		xc:    xc,
		sch:   sch,
		now:   time.Now,
	}
}

func (s *Service) UseProactiveVoiceGate(gate sharedtypes.ProactiveVoiceGate) {
	s.voiceGate = gate
}

func (s *Service) Create(ctx context.Context, req CreateRequest) (Reminder, error) {
	deviceID := strings.TrimSpace(req.DeviceID)
	content := strings.TrimSpace(req.Content)
	scheduledAt := req.ScheduledAt.UTC()
	if deviceID == "" || content == "" || scheduledAt.IsZero() || !scheduledAt.After(s.now().UTC()) {
		return Reminder{}, ErrInvalidInput
	}

	category := normalizeCategory(req.Category)
	recurrence, customDates := normalizeRecurrence(req.Recurrence, req.CustomDates)
	rem := Reminder{
		DeviceID:    deviceID,
		ScheduledAt: scheduledAt,
		Content:     content,
		Category:    category,
		Recurrence:  recurrence,
		CustomDates: customDates,
		Important:   req.Important,
		Text:        reminderPlaybackText(content, category, req.Important),
		Status:      StatusScheduled,
	}
	if err := s.store.Create(ctx, &rem); err != nil {
		return Reminder{}, err
	}

	if err := s.scheduleReminder(ctx, &rem); err != nil {
		return Reminder{}, err
	}
	return rem, nil
}

func (s *Service) List(ctx context.Context, filter ListFilter) ([]Reminder, error) {
	filter.DeviceID = strings.TrimSpace(filter.DeviceID)
	filter.Status = Status(strings.TrimSpace(string(filter.Status)))
	return s.store.List(ctx, filter)
}

func (s *Service) Cancel(ctx context.Context, id uint) (Reminder, error) {
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
	if rem.JobID != "" {
		s.sch.Cancel(scheduler.JobID(rem.JobID))
	}
	rem.Status = StatusCanceled
	rem.JobID = ""
	if err := s.store.Update(ctx, &rem); err != nil {
		return Reminder{}, err
	}
	return rem, nil
}

func (s *Service) Acknowledge(ctx context.Context, id uint, req AckRequest) (Reminder, error) {
	if id == 0 {
		return Reminder{}, ErrInvalidInput
	}
	return s.acknowledge(ctx, id, normalizeAckKind(req.AckKind), true)
}

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

func (s *Service) RestoreScheduled(ctx context.Context) (int, error) {
	reminders, err := s.store.List(ctx, ListFilter{Status: StatusScheduled})
	if err != nil {
		return 0, err
	}

	restored := 0
	for i := range reminders {
		rem := reminders[i]
		if err := s.scheduleReminder(ctx, &rem); err != nil {
			return restored, err
		}
		restored++
	}

	played, err := s.store.List(ctx, ListFilter{Status: StatusPlayed})
	if err != nil {
		return restored, err
	}
	for i := range played {
		rem := played[i]
		if rem.PlayedAt == nil {
			continue
		}
		timeoutAt := rem.PlayedAt.UTC().Add(defaultAckTimeout)
		if !timeoutAt.After(s.now().UTC()) {
			if _, err := s.acknowledge(ctx, rem.ID, AckKindTimeout, false); err != nil {
				return restored, err
			}
			restored++
			continue
		}
		jobID, err := s.sch.ScheduleAt(timeoutAt, func() {
			s.markUnanswered(rem.ID)
		})
		if err != nil {
			rem.Status = StatusFailed
			rem.ErrorMessage = err.Error()
			_ = s.store.Update(ctx, &rem)
			return restored, err
		}
		rem.AckJobID = string(jobID)
		if err := s.store.Update(ctx, &rem); err != nil {
			return restored, err
		}
		s.scheduleVoiceAckPoll(rem.ID, s.now().UTC())
		restored++
	}
	return restored, nil
}

func (s *Service) fire(id uint) {
	ctx := context.Background()
	rem, err := s.store.Get(ctx, id)
	if err != nil {
		return
	}
	if rem.Status != StatusScheduled {
		return
	}
	s.play(ctx, &rem)
}

func (s *Service) play(ctx context.Context, rem *Reminder) {
	playedAt := s.now().UTC()
	lease, err := s.tryAcquireProactiveVoice(ctx, rem.DeviceID, playedAt)
	if err != nil {
		if errors.Is(err, sharedtypes.ErrProactiveVoiceThrottled) {
			s.requeueProactiveVoice(ctx, rem, playedAt, err)
			return
		}
		rem.Status = StatusSkipped
		rem.ErrorMessage = err.Error()
		rem.JobID = ""
		_ = s.store.Update(ctx, rem)
		return
	}

	injectCtx, cancel := withReminderInjectTimeout(ctx)
	defer cancel()

	if err := s.xc.InjectSpeak(injectCtx, rem.DeviceID, rem.Text, proactiveSpeakOptions()); err != nil {
		if lease != nil {
			_ = lease.Rollback(ctx)
		}
		rem.Status = StatusFailed
		rem.ErrorMessage = err.Error()
		_ = s.store.Update(ctx, rem)
		return
	}
	if lease != nil {
		_ = lease.Commit(ctx)
	}

	rem.Status = StatusPlayed
	rem.PlayedAt = &playedAt
	rem.JobID = ""
	jobID, err := s.sch.ScheduleAt(playedAt.Add(defaultAckTimeout), func() {
		s.markUnanswered(rem.ID)
	})
	if err != nil {
		rem.Status = StatusFailed
		rem.ErrorMessage = err.Error()
		_ = s.store.Update(ctx, rem)
		return
	}
	rem.AckJobID = string(jobID)
	if err := s.store.Update(ctx, rem); err != nil {
		return
	}
	_ = s.scheduleNextOccurrence(ctx, rem, playedAt)
	s.scheduleVoiceAckPoll(rem.ID, playedAt.Add(voiceAckPollInterval))
}

func (s *Service) scheduleReminder(ctx context.Context, rem *Reminder) error {
	if s.sch == nil {
		return nil
	}
	jobID, err := s.sch.ScheduleAt(rem.ScheduledAt, func() {
		s.fire(rem.ID)
	})
	if err != nil {
		rem.Status = StatusFailed
		rem.ErrorMessage = err.Error()
		rem.JobID = ""
		_ = s.store.Update(ctx, rem)
		return err
	}
	rem.JobID = string(jobID)
	return s.store.Update(ctx, rem)
}

func (s *Service) scheduleNextOccurrence(ctx context.Context, rem *Reminder, after time.Time) error {
	nextAt, ok := nextRecurringScheduledAt(rem.ScheduledAt, rem.Recurrence, rem.CustomDates, after)
	if !ok {
		return nil
	}
	next := Reminder{
		DeviceID:    rem.DeviceID,
		ScheduledAt: nextAt,
		Content:     rem.Content,
		Category:    rem.Category,
		Recurrence:  rem.Recurrence,
		CustomDates: append([]string(nil), rem.CustomDates...),
		Important:   rem.Important,
		Text:        reminderPlaybackText(rem.Content, rem.Category, rem.Important),
		Status:      StatusScheduled,
	}
	if err := s.store.Create(ctx, &next); err != nil {
		return err
	}
	return s.scheduleReminder(ctx, &next)
}

func (s *Service) requeueProactiveVoice(ctx context.Context, rem *Reminder, at time.Time, cause error) {
	retryAt := at.UTC().Add(proactiveRetryDelay)
	if s.sch == nil {
		rem.Status = StatusFailed
		rem.ErrorMessage = cause.Error()
		rem.JobID = ""
		_ = s.store.Update(ctx, rem)
		return
	}

	jobID, err := s.sch.ScheduleAt(retryAt, func() {
		s.fire(rem.ID)
	})
	if err != nil {
		rem.Status = StatusFailed
		rem.ErrorMessage = err.Error()
		rem.JobID = ""
		_ = s.store.Update(ctx, rem)
		return
	}

	rem.Status = StatusScheduled
	rem.ScheduledAt = retryAt
	rem.JobID = string(jobID)
	rem.ErrorMessage = cause.Error()
	_ = s.store.Update(ctx, rem)
}

func (s *Service) tryAcquireProactiveVoice(ctx context.Context, deviceID string, at time.Time) (sharedtypes.ProactiveVoiceLease, error) {
	if s.voiceGate == nil {
		return nil, nil
	}
	return s.voiceGate.TryAcquireProactiveVoice(ctx, deviceID, at)
}

func proactiveSpeakOptions() xiaozhiclient.InjectOptions {
	autoListen := true
	return xiaozhiclient.InjectOptions{SkipLLM: true, AutoListen: &autoListen}
}

func withReminderInjectTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) <= reminderInjectTimeout {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, reminderInjectTimeout)
}

func (s *Service) markUnanswered(id uint) {
	_, _ = s.acknowledge(context.Background(), id, AckKindTimeout, false)
}

func (s *Service) scheduleVoiceAckPoll(id uint, at time.Time) {
	if s.sch == nil {
		return
	}
	_, _ = s.sch.ScheduleAt(at, func() {
		s.pollVoiceAck(id)
	})
}

func (s *Service) pollVoiceAck(id uint) {
	ctx := context.Background()
	rem, err := s.store.Get(ctx, id)
	if err != nil || rem.Status != StatusPlayed || rem.PlayedAt == nil {
		return
	}

	history, historyErr := s.xc.GetHistory(ctx, rem.DeviceID, voiceAckHistoryLimit)
	if historyErr == nil && historyContainsVoiceAcknowledgement(history, rem.PlayedAt.UTC()) {
		if _, err := s.acknowledge(ctx, rem.ID, AckKindVoice, true); err == nil {
			return
		}
	}

	nextPollAt := s.now().UTC().Add(voiceAckPollInterval)
	if nextPollAt.Before(rem.PlayedAt.UTC().Add(defaultAckTimeout)) {
		s.scheduleVoiceAckPoll(rem.ID, nextPollAt)
	}
}

func historyContainsVoiceAcknowledgement(history []xiaozhiclient.HistoryMessage, playedAt time.Time) bool {
	cutoff := playedAt.UTC().Add(-voiceAckTimestampSkew)
	for _, message := range history {
		if !strings.EqualFold(strings.TrimSpace(message.Role), "user") || message.At.IsZero() {
			continue
		}
		if message.At.UTC().Before(cutoff) {
			continue
		}
		if isVoiceAcknowledgement(message.Text) {
			return true
		}
	}
	return false
}

func isVoiceAcknowledgement(text string) bool {
	normalized := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || unicode.IsPunct(r) {
			return -1
		}
		return r
	}, strings.TrimSpace(text))
	if _, ok := voiceAcknowledgementPhrases[normalized]; ok {
		return true
	}
	for phrase := range voiceAcknowledgementPhrases {
		if phrase == "好" {
			continue
		}
		if strings.HasPrefix(normalized, phrase) {
			return true
		}
	}
	return false
}

func (s *Service) acknowledge(ctx context.Context, id uint, kind AckKind, cancelJob bool) (Reminder, error) {
	rem, err := s.store.Get(ctx, id)
	if err != nil {
		return Reminder{}, err
	}

	switch rem.Status {
	case StatusPlayed:
	case StatusCompleted, StatusUnanswered:
		return rem, nil
	default:
		return Reminder{}, ErrInvalidInput
	}

	if cancelJob && rem.AckJobID != "" {
		s.sch.Cancel(scheduler.JobID(rem.AckJobID))
	}

	ackAt := s.now().UTC()
	if kind == AckKindTimeout {
		rem.Status = StatusUnanswered
	} else {
		rem.Status = StatusCompleted
	}
	rem.AckKind = kind
	rem.AcknowledgedAt = &ackAt
	rem.AckJobID = ""
	if err := s.store.Update(ctx, &rem); err != nil {
		return Reminder{}, err
	}
	return rem, nil
}

func normalizeAckKind(kind AckKind) AckKind {
	if kind == AckKindTimeout {
		return AckKindTimeout
	}
	return AckKindVoice
}

func normalizeCategory(category Category) Category {
	switch category {
	case CategoryMed, CategoryBirthday, CategoryFestival:
		return category
	default:
		return CategoryCustom
	}
}

func normalizeRecurrence(recurrence Recurrence, customDates []string) (Recurrence, []string) {
	switch recurrence {
	case RecurrenceDaily, RecurrenceWeekdays, RecurrenceWeekends:
		return recurrence, nil
	case RecurrenceCustomDates:
		dates := normalizeCustomDates(customDates)
		if len(dates) == 0 {
			return RecurrenceNone, nil
		}
		return RecurrenceCustomDates, dates
	default:
		return RecurrenceNone, nil
	}
}

func normalizeCustomDates(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		date := strings.TrimSpace(value)
		if _, err := time.Parse("2006-01-02", date); err != nil {
			continue
		}
		if _, ok := seen[date]; ok {
			continue
		}
		seen[date] = struct{}{}
		out = append(out, date)
	}
	sort.Strings(out)
	return out
}

func nextRecurringScheduledAt(scheduledAt time.Time, recurrence Recurrence, customDates []string, after time.Time) (time.Time, bool) {
	scheduledAt = scheduledAt.UTC()
	after = after.UTC()
	switch recurrence {
	case RecurrenceDaily:
		return nextMatchingDay(scheduledAt, after, func(time.Time) bool { return true })
	case RecurrenceWeekdays:
		return nextMatchingDay(scheduledAt, after, func(t time.Time) bool {
			weekday := t.Weekday()
			return weekday >= time.Monday && weekday <= time.Friday
		})
	case RecurrenceWeekends:
		return nextMatchingDay(scheduledAt, after, func(t time.Time) bool {
			weekday := t.Weekday()
			return weekday == time.Saturday || weekday == time.Sunday
		})
	case RecurrenceCustomDates:
		hour, minute, second := scheduledAt.Clock()
		for _, date := range normalizeCustomDates(customDates) {
			day, err := time.ParseInLocation("2006-01-02", date, time.UTC)
			if err != nil {
				continue
			}
			candidate := time.Date(day.Year(), day.Month(), day.Day(), hour, minute, second, scheduledAt.Nanosecond(), time.UTC)
			if candidate.After(after) {
				return candidate, true
			}
		}
	}
	return time.Time{}, false
}

func nextMatchingDay(scheduledAt time.Time, after time.Time, matches func(time.Time) bool) (time.Time, bool) {
	candidate := scheduledAt.AddDate(0, 0, 1)
	for i := 0; i < 370; i++ {
		if candidate.After(after) && matches(candidate) {
			return candidate, true
		}
		candidate = candidate.AddDate(0, 0, 1)
	}
	return time.Time{}, false
}

func reminderPlaybackText(content string, category Category, important bool) string {
	text := reminderText(content, category)
	if !important {
		return text
	}
	return truncateRunes("重要提醒，"+text, maxReminderTextRunes)
}

func reminderText(content string, category Category) string {
	content = strings.TrimSpace(content)
	if category == CategoryMed {
		content = normalizeMedicineReminderContent(content)
		return buildReminderText(
			"您该",
			content,
			"啦，记得按时完成哦。完成了跟安伴说一声，我也就放心啦。",
		)
	}
	if category == CategoryBirthday {
		return buildReminderText(
			"您好，生日提醒：",
			content,
			"。记得送上祝福，安伴会陪您记着。",
		)
	}
	if category == CategoryFestival {
		return buildReminderText(
			"您好，节日提醒：",
			content,
			"。安伴陪您一起记着这个日子。",
		)
	}
	return buildReminderText(
		"您好，提醒您：",
		content,
		"。完成后跟安伴说一声，我也就放心啦。",
	)
}

func normalizeMedicineReminderContent(content string) string {
	original := strings.TrimSpace(content)
	cleaned := trimReminderSpeechPunctuation(original)
	cleaned = strings.TrimSpace(strings.TrimPrefix(cleaned, "该"))
	cleaned = trimReminderSpeechPunctuation(cleaned)
	for _, suffix := range []string{"啦", "了"} {
		cleaned = strings.TrimSpace(strings.TrimSuffix(cleaned, suffix))
		cleaned = trimReminderSpeechPunctuation(cleaned)
	}
	if cleaned == "" {
		return original
	}
	return cleaned
}

func trimReminderSpeechPunctuation(value string) string {
	return strings.Trim(value, " \t\r\n，,。.!！?？~～")
}

func buildReminderText(prefix, content, suffix string) string {
	maxContentRunes := maxReminderTextRunes - runeLen(prefix) - runeLen(suffix)
	if maxContentRunes < 0 {
		return truncateRunes(prefix+suffix, maxReminderTextRunes)
	}

	text := prefix + truncateRunes(content, maxContentRunes) + suffix
	if runeLen(text) < minReminderTextRunes {
		text += "安伴会陪着您。"
	}
	return truncateRunes(text, maxReminderTextRunes)
}

func truncateRunes(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

func runeLen(value string) int {
	return len([]rune(value))
}
