package reminder

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bluegodg/anban/server/internal/scheduler"
	"github.com/bluegodg/anban/server/internal/store"
	"github.com/bluegodg/anban/server/internal/xiaozhiclient"
	sharedtypes "github.com/bluegodg/anban/server/pkg/types"
)

func newTestService(t *testing.T, xc xiaozhiclient.Client, sch *fakeScheduler) *Service {
	t.Helper()

	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	reminderStore := NewStore(st.DB)
	if err := reminderStore.AutoMigrate(); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}

	svc := NewService(reminderStore, xc, sch)
	svc.now = func() time.Time { return time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC) }
	return svc
}

func TestServiceCreateSchedulesAndFiresReminder(t *testing.T) {
	fakeXC := &xiaozhiclient.FakeClient{}
	fakeSch := &fakeScheduler{}
	svc := newTestService(t, fakeXC, fakeSch)

	scheduledAt := time.Date(2026, 6, 1, 9, 1, 30, 0, time.UTC)
	got, err := svc.Create(context.Background(), CreateRequest{
		DeviceID:    "dev-001",
		ScheduledAt: scheduledAt,
		Content:     "测血压",
		Category:    CategoryMed,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.ID == 0 {
		t.Fatal("reminder ID was not assigned")
	}
	if got.Status != StatusScheduled {
		t.Fatalf("Status = %q, want %q", got.Status, StatusScheduled)
	}
	if got.JobID != "job-1" {
		t.Fatalf("JobID = %q, want job-1", got.JobID)
	}
	if len(fakeSch.jobs) != 1 {
		t.Fatalf("scheduled jobs = %d, want 1", len(fakeSch.jobs))
	}
	if !fakeSch.jobs[0].at.Equal(scheduledAt) {
		t.Fatalf("scheduled at = %s, want %s", fakeSch.jobs[0].at, scheduledAt)
	}
	if len(fakeXC.InjectCalls) != 0 {
		t.Fatalf("InjectCalls before fire = %d, want 0", len(fakeXC.InjectCalls))
	}

	fakeSch.fire(0)
	if len(fakeXC.InjectCalls) != 1 {
		t.Fatalf("InjectCalls after fire = %d, want 1", len(fakeXC.InjectCalls))
	}
	call := fakeXC.InjectCalls[0]
	if call.DeviceID != "dev-001" {
		t.Fatalf("DeviceID = %q, want dev-001", call.DeviceID)
	}
	if !strings.Contains(call.Text, "测血压") {
		t.Fatalf("inject text = %q, want to include reminder content", call.Text)
	}
	assertReminderTextLength(t, call.Text)
	if !call.Opts.SkipLLM {
		t.Fatal("SkipLLM = false, want true for deterministic reminder playback")
	}
	if call.Opts.AutoListen == nil || !*call.Opts.AutoListen {
		t.Fatal("AutoListen is not true; reminder playback should allow the elder to answer through xiaozhi")
	}

	list, err := svc.List(context.Background(), ListFilter{DeviceID: "dev-001"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0].Status != StatusPlayed || list[0].PlayedAt == nil {
		t.Fatalf("List = %+v, want played reminder", list)
	}
}

func TestServicePlayScheduledSupportsMindExecutor(t *testing.T) {
	fakeXC := &xiaozhiclient.FakeClient{}
	svc := newTestService(t, fakeXC, &fakeScheduler{})
	ctx := context.Background()
	rem, err := svc.Create(ctx, CreateRequest{
		DeviceID:    "dev-001",
		Content:     "吃药",
		ScheduledAt: svc.now().Add(time.Hour),
		Category:    CategoryMed,
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

func TestServiceFireEmitsMindEventWhenSinkConfigured(t *testing.T) {
	sink := &fakeReminderMindSink{}
	fakeXC := &xiaozhiclient.FakeClient{}
	svc := newTestService(t, fakeXC, &fakeScheduler{})
	svc.UseMindSink(sink)
	ctx := context.Background()
	rem, err := svc.Create(ctx, CreateRequest{DeviceID: "dev-001", Content: "吃药", ScheduledAt: svc.now().Add(time.Hour), Category: CategoryMed})
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
	if len(fakeXC.InjectCalls) != 0 {
		t.Fatalf("InjectCalls = %d, want Mind to choose playback", len(fakeXC.InjectCalls))
	}
}

func TestServiceFireFallsBackToDirectPlayWhenMindSinkFails(t *testing.T) {
	fakeXC := &xiaozhiclient.FakeClient{}
	svc := newTestService(t, fakeXC, &fakeScheduler{})
	svc.UseMindSink(failingReminderMindSink{err: errors.New("mind unavailable")})
	ctx := context.Background()
	rem, err := svc.Create(ctx, CreateRequest{DeviceID: "dev-001", Content: "吃药", ScheduledAt: svc.now().Add(time.Hour), Category: CategoryMed})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	svc.fire(rem.ID)
	if len(fakeXC.InjectCalls) != 1 {
		t.Fatalf("InjectCalls = %d, want fallback direct playback", len(fakeXC.InjectCalls))
	}
}

type fakeReminderMindSink struct{ events []MindEvent }

func (f *fakeReminderMindSink) IngestMindEvent(ctx context.Context, event MindEvent) error {
	f.events = append(f.events, event)
	return nil
}

type failingReminderMindSink struct {
	err error
}

func (f failingReminderMindSink) IngestMindEvent(context.Context, MindEvent) error {
	return f.err
}

func TestServiceCreateStoresRecurrenceAndImportant(t *testing.T) {
	fakeSch := &fakeScheduler{}
	svc := newTestService(t, &xiaozhiclient.FakeClient{}, fakeSch)

	got, err := svc.Create(context.Background(), CreateRequest{
		DeviceID:    "dev-001",
		ScheduledAt: time.Date(2026, 6, 1, 9, 1, 30, 0, time.UTC),
		Content:     "测血压",
		Category:    CategoryMed,
		Recurrence:  RecurrenceDaily,
		Important:   true,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.Recurrence != RecurrenceDaily {
		t.Fatalf("Recurrence = %q, want %q", got.Recurrence, RecurrenceDaily)
	}
	if !got.Important {
		t.Fatal("Important = false, want true")
	}
	if len(fakeSch.jobs) != 1 {
		t.Fatalf("scheduled jobs = %d, want 1", len(fakeSch.jobs))
	}
}

func TestServiceFireRecurringReminderSchedulesNextOccurrence(t *testing.T) {
	fakeXC := &xiaozhiclient.FakeClient{}
	fakeSch := &fakeScheduler{}
	svc := newTestService(t, fakeXC, fakeSch)
	ctx := context.Background()

	firstAt := time.Date(2026, 6, 1, 9, 1, 30, 0, time.UTC)
	created, err := svc.Create(ctx, CreateRequest{
		DeviceID:    "dev-001",
		ScheduledAt: firstAt,
		Content:     "测血压",
		Category:    CategoryMed,
		Recurrence:  RecurrenceDaily,
		Important:   true,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	fakeSch.fire(0)
	if len(fakeXC.InjectCalls) != 1 {
		t.Fatalf("InjectCalls = %d, want 1", len(fakeXC.InjectCalls))
	}

	played, err := svc.store.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get played: %v", err)
	}
	if played.Status != StatusPlayed {
		t.Fatalf("played status = %q, want %q", played.Status, StatusPlayed)
	}

	scheduled, err := svc.List(ctx, ListFilter{DeviceID: "dev-001", Status: StatusScheduled})
	if err != nil {
		t.Fatalf("List scheduled: %v", err)
	}
	if len(scheduled) != 1 {
		t.Fatalf("scheduled reminders = %+v, want one next occurrence", scheduled)
	}
	next := scheduled[0]
	if !next.ScheduledAt.Equal(firstAt.AddDate(0, 0, 1)) {
		t.Fatalf("next scheduledAt = %s, want %s", next.ScheduledAt, firstAt.AddDate(0, 0, 1))
	}
	if next.Recurrence != RecurrenceDaily || !next.Important {
		t.Fatalf("next recurrence/important = %q/%v, want daily/true", next.Recurrence, next.Important)
	}
	if next.ID == created.ID {
		t.Fatal("next occurrence reused played row; want separate scheduled occurrence for history")
	}
}

func TestNextRecurringScheduledAt(t *testing.T) {
	base := time.Date(2026, 6, 5, 8, 0, 0, 0, time.UTC) // Friday
	after := time.Date(2026, 6, 5, 9, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		recurrence Recurrence
		dates      []string
		want       time.Time
	}{
		{name: "daily", recurrence: RecurrenceDaily, want: time.Date(2026, 6, 6, 8, 0, 0, 0, time.UTC)},
		{name: "weekdays skips weekend", recurrence: RecurrenceWeekdays, want: time.Date(2026, 6, 8, 8, 0, 0, 0, time.UTC)},
		{name: "weekends", recurrence: RecurrenceWeekends, want: time.Date(2026, 6, 6, 8, 0, 0, 0, time.UTC)},
		{name: "custom dates", recurrence: RecurrenceCustomDates, dates: []string{"2026-06-07", "2026-06-09"}, want: time.Date(2026, 6, 7, 8, 0, 0, 0, time.UTC)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := nextRecurringScheduledAt(base, tt.recurrence, tt.dates, after)
			if !ok {
				t.Fatal("ok = false, want true")
			}
			if !got.Equal(tt.want) {
				t.Fatalf("next = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestServiceFireBoundsInjectForPRDDeliveryWindow(t *testing.T) {
	fakeXC := &deadlineReminderClient{}
	fakeSch := &fakeScheduler{}
	svc := newTestService(t, fakeXC, fakeSch)

	_, err := svc.Create(context.Background(), CreateRequest{
		DeviceID:    "dev-001",
		ScheduledAt: time.Date(2026, 6, 1, 9, 1, 30, 0, time.UTC),
		Content:     "测血压",
		Category:    CategoryMed,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	fakeSch.fire(0)
	if !fakeXC.gotDeadline {
		t.Fatal("InjectSpeak context has no deadline; PRD #6 reminder delivery should stay within 60s")
	}
	if remaining := time.Until(fakeXC.deadline); remaining <= 0 || remaining > 60*time.Second {
		t.Fatalf("InjectSpeak deadline remaining = %s, want within 60s", remaining)
	}
}

func TestServiceCreateValidatesAndNormalizesInput(t *testing.T) {
	svc := newTestService(t, &xiaozhiclient.FakeClient{}, &fakeScheduler{})

	validTime := time.Date(2026, 6, 1, 9, 1, 0, 0, time.UTC)
	tests := []struct {
		name string
		req  CreateRequest
	}{
		{name: "missing device", req: CreateRequest{ScheduledAt: validTime, Content: "测血压"}},
		{name: "missing content", req: CreateRequest{DeviceID: "dev-001", ScheduledAt: validTime}},
		{name: "missing time", req: CreateRequest{DeviceID: "dev-001", Content: "测血压"}},
		{name: "past time", req: CreateRequest{DeviceID: "dev-001", ScheduledAt: svc.now().Add(-time.Minute), Content: "测血压"}},
		{name: "current time", req: CreateRequest{DeviceID: "dev-001", ScheduledAt: svc.now(), Content: "测血压"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := svc.Create(context.Background(), tt.req); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("err = %v, want ErrInvalidInput", err)
			}
		})
	}

	got, err := svc.Create(context.Background(), CreateRequest{
		DeviceID:    "dev-001",
		ScheduledAt: validTime,
		Content:     "  记得喝水  ",
		Category:    "unknown",
	})
	if err != nil {
		t.Fatalf("Create with unknown category: %v", err)
	}
	if got.Category != CategoryCustom {
		t.Fatalf("Category = %q, want %q", got.Category, CategoryCustom)
	}
	if got.Content != "记得喝水" {
		t.Fatalf("Content = %q, want trimmed content", got.Content)
	}
}

func TestServiceListNormalizesFilters(t *testing.T) {
	svc := newTestService(t, &xiaozhiclient.FakeClient{}, &fakeScheduler{})
	ctx := context.Background()

	created, err := svc.Create(ctx, CreateRequest{
		DeviceID:    "dev-001",
		ScheduledAt: time.Date(2026, 6, 1, 9, 1, 0, 0, time.UTC),
		Content:     "测血压",
		Category:    CategoryMed,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := svc.List(ctx, ListFilter{DeviceID: "  dev-001  ", Status: "  scheduled  "})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 || got[0].ID != created.ID {
		t.Fatalf("List = %+v, want reminder %d after trimmed filters", got, created.ID)
	}
}

func TestReminderTextFitsPRDLength(t *testing.T) {
	tests := []struct {
		name         string
		content      string
		category     Category
		wantContains string
	}{
		{name: "short medicine reminder", content: "测血压", category: CategoryMed, wantContains: "该"},
		{name: "birthday reminder", content: "小宝七岁", category: CategoryBirthday, wantContains: "生日"},
		{name: "festival reminder", content: "端午包粽子", category: CategoryFestival, wantContains: "节日"},
		{name: "long custom reminder", content: strings.Repeat("记得测血压", 20), category: CategoryCustom, wantContains: "提醒"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text := reminderText(tt.content, tt.category)
			assertReminderTextLength(t, text)
			if !strings.Contains(text, "您") {
				t.Fatalf("text = %q, want elder-facing salutation", text)
			}
			if !strings.Contains(text, tt.wantContains) {
				t.Fatalf("text = %q, want category cue %q", text, tt.wantContains)
			}
		})
	}
}

func TestReminderTextAvoidsDuplicatedPromptParticles(t *testing.T) {
	text := reminderText("该测血压啦", CategoryMed)

	assertReminderTextLength(t, text)
	if strings.Contains(text, "该该") || strings.Contains(text, "啦啦") {
		t.Fatalf("text = %q, want no duplicated prompt particles", text)
	}
	if !strings.Contains(text, "测血压") {
		t.Fatalf("text = %q, want preserve reminder content", text)
	}
}

func TestServiceCreateMarksFailedWhenInjectFailsOnFire(t *testing.T) {
	fakeSch := &fakeScheduler{}
	svc := newTestService(t, failingClient{err: errors.New("manager unavailable")}, fakeSch)

	got, err := svc.Create(context.Background(), CreateRequest{
		DeviceID:    "dev-001",
		ScheduledAt: time.Date(2026, 6, 1, 9, 1, 0, 0, time.UTC),
		Content:     "吃药",
		Category:    CategoryMed,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	fakeSch.fire(0)
	list, err := svc.List(context.Background(), ListFilter{Status: StatusFailed})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(list) != 1 || list[0].ID != got.ID || list[0].ErrorMessage == "" {
		t.Fatalf("failed reminders = %+v, want failed reminder with error", list)
	}
}

func TestServiceRequeuesReminderWhenProactiveVoiceQuotaUsed(t *testing.T) {
	fakeXC := &xiaozhiclient.FakeClient{}
	fakeSch := &fakeScheduler{}
	svc := newTestService(t, fakeXC, fakeSch)
	svc.UseProactiveVoiceGate(throttledVoiceGate{})

	got, err := svc.Create(context.Background(), CreateRequest{
		DeviceID:    "dev-001",
		ScheduledAt: time.Date(2026, 6, 1, 9, 1, 0, 0, time.UTC),
		Content:     "测血压",
		Category:    CategoryMed,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	fakeSch.fire(0)
	list, err := svc.List(context.Background(), ListFilter{Status: StatusScheduled})
	if err != nil {
		t.Fatalf("List scheduled: %v", err)
	}
	if len(list) != 1 || list[0].ID != got.ID {
		t.Fatalf("scheduled reminders = %+v, want requeued reminder %d", list, got.ID)
	}
	if list[0].JobID != "job-2" {
		t.Fatalf("JobID = %q, want retry job id job-2", list[0].JobID)
	}
	if want := svc.now().UTC().Add(proactiveRetryDelay); !list[0].ScheduledAt.Equal(want) {
		t.Fatalf("ScheduledAt = %s, want retry at %s", list[0].ScheduledAt, want)
	}
	if list[0].ErrorMessage == "" {
		t.Fatal("ErrorMessage is empty")
	}
	if len(fakeXC.InjectCalls) != 0 {
		t.Fatalf("InjectCalls = %d, want no xiaozhi injection when quota is used", len(fakeXC.InjectCalls))
	}
	if len(fakeSch.jobs) != 2 {
		t.Fatalf("scheduled jobs = %d, want original reminder job plus retry job", len(fakeSch.jobs))
	}
	if want := svc.now().UTC().Add(proactiveRetryDelay); !fakeSch.jobs[1].at.Equal(want) {
		t.Fatalf("retry scheduled at = %s, want %s", fakeSch.jobs[1].at, want)
	}
}

func TestServiceRestoreScheduledRehydratesPendingReminders(t *testing.T) {
	fakeXC := &xiaozhiclient.FakeClient{}
	fakeSch := &fakeScheduler{}
	svc := newTestService(t, fakeXC, fakeSch)
	ctx := context.Background()

	futureAt := time.Date(2026, 6, 1, 9, 5, 0, 0, time.UTC)
	pending := Reminder{
		DeviceID:    "dev-001",
		ScheduledAt: futureAt,
		Content:     "测血压",
		Category:    CategoryMed,
		Text:        reminderText("测血压", CategoryMed),
		Status:      StatusScheduled,
		JobID:       "stale-job",
	}
	if err := svc.store.Create(ctx, &pending); err != nil {
		t.Fatalf("create pending: %v", err)
	}
	played := Reminder{
		DeviceID:    "dev-001",
		ScheduledAt: futureAt,
		Content:     "已播过",
		Category:    CategoryCustom,
		Text:        reminderText("已播过", CategoryCustom),
		Status:      StatusPlayed,
	}
	if err := svc.store.Create(ctx, &played); err != nil {
		t.Fatalf("create played: %v", err)
	}

	count, err := svc.RestoreScheduled(ctx)
	if err != nil {
		t.Fatalf("RestoreScheduled: %v", err)
	}
	if count != 1 {
		t.Fatalf("restored count = %d, want 1", count)
	}
	if len(fakeSch.jobs) != 1 {
		t.Fatalf("scheduled jobs = %d, want 1", len(fakeSch.jobs))
	}
	if !fakeSch.jobs[0].at.Equal(futureAt) {
		t.Fatalf("scheduled at = %s, want %s", fakeSch.jobs[0].at, futureAt)
	}

	list, err := svc.List(ctx, ListFilter{Status: StatusScheduled})
	if err != nil {
		t.Fatalf("List scheduled: %v", err)
	}
	if len(list) != 1 || list[0].ID != pending.ID || list[0].JobID != "job-1" {
		t.Fatalf("scheduled list = %+v, want restored job ID on pending reminder", list)
	}

	fakeSch.fire(0)
	if len(fakeXC.InjectCalls) != 1 {
		t.Fatalf("InjectCalls = %d, want restored reminder to fire once", len(fakeXC.InjectCalls))
	}
}

func TestServiceRestoreScheduledRehydratesPlayedAckTimeouts(t *testing.T) {
	fakeSch := &fakeScheduler{}
	svc := newTestService(t, &xiaozhiclient.FakeClient{}, fakeSch)
	ctx := context.Background()
	playedAt := svc.now().UTC().Add(-5 * time.Minute)

	played := Reminder{
		DeviceID:    "dev-001",
		ScheduledAt: time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC),
		Content:     "测血压",
		Category:    CategoryMed,
		Text:        reminderText("测血压", CategoryMed),
		Status:      StatusPlayed,
		PlayedAt:    &playedAt,
		AckJobID:    "stale-ack-job",
	}
	if err := svc.store.Create(ctx, &played); err != nil {
		t.Fatalf("create played: %v", err)
	}

	count, err := svc.RestoreScheduled(ctx)
	if err != nil {
		t.Fatalf("RestoreScheduled: %v", err)
	}
	if count != 1 {
		t.Fatalf("restored count = %d, want 1 played ack timeout", count)
	}
	if len(fakeSch.jobs) != 2 {
		t.Fatalf("scheduled jobs = %d, want ack timeout plus voice ack poll", len(fakeSch.jobs))
	}
	if want := playedAt.Add(defaultAckTimeout); !fakeSch.jobs[0].at.Equal(want) {
		t.Fatalf("ack timeout at = %s, want %s", fakeSch.jobs[0].at, want)
	}
	if want := svc.now().UTC(); !fakeSch.jobs[1].at.Equal(want) {
		t.Fatalf("voice ack poll at = %s, want %s", fakeSch.jobs[1].at, want)
	}

	list, err := svc.List(ctx, ListFilter{Status: StatusPlayed})
	if err != nil {
		t.Fatalf("List played: %v", err)
	}
	if len(list) != 1 || list[0].ID != played.ID || list[0].AckJobID != "job-1" {
		t.Fatalf("played list = %+v, want restored ack job ID on played reminder", list)
	}

	fakeSch.fire(0)
	unanswered, err := svc.List(ctx, ListFilter{Status: StatusUnanswered})
	if err != nil {
		t.Fatalf("List unanswered: %v", err)
	}
	if len(unanswered) != 1 || unanswered[0].ID != played.ID || unanswered[0].AckKind != AckKindTimeout {
		t.Fatalf("unanswered reminders = %+v, want restored timeout reminder", unanswered)
	}
}

func TestServiceRestoreScheduledMarksOverduePlayedReminderUnanswered(t *testing.T) {
	fakeSch := &fakeScheduler{}
	svc := newTestService(t, &xiaozhiclient.FakeClient{}, fakeSch)
	ctx := context.Background()
	playedAt := svc.now().UTC().Add(-defaultAckTimeout - time.Minute)

	played := Reminder{
		DeviceID:    "dev-001",
		ScheduledAt: svc.now().UTC().Add(-2 * defaultAckTimeout),
		Content:     "测血压",
		Category:    CategoryMed,
		Text:        reminderText("测血压", CategoryMed),
		Status:      StatusPlayed,
		PlayedAt:    &playedAt,
		AckJobID:    "stale-ack-job",
	}
	if err := svc.store.Create(ctx, &played); err != nil {
		t.Fatalf("create played: %v", err)
	}

	count, err := svc.RestoreScheduled(ctx)
	if err != nil {
		t.Fatalf("RestoreScheduled: %v", err)
	}
	if count != 1 {
		t.Fatalf("restored count = %d, want 1 overdue ack timeout", count)
	}
	if len(fakeSch.jobs) != 0 {
		t.Fatalf("scheduled jobs = %d, want no past ack timeout job", len(fakeSch.jobs))
	}

	unanswered, err := svc.List(ctx, ListFilter{Status: StatusUnanswered})
	if err != nil {
		t.Fatalf("List unanswered: %v", err)
	}
	if len(unanswered) != 1 || unanswered[0].ID != played.ID || unanswered[0].AckKind != AckKindTimeout {
		t.Fatalf("unanswered reminders = %+v, want overdue played reminder marked unanswered", unanswered)
	}
	if unanswered[0].AckJobID != "" {
		t.Fatalf("AckJobID = %q, want cleared after overdue timeout", unanswered[0].AckJobID)
	}
	if unanswered[0].AcknowledgedAt == nil {
		t.Fatal("AcknowledgedAt = nil, want overdue timeout timestamp")
	}
}

func TestServiceCancelScheduledReminder(t *testing.T) {
	fakeSch := &fakeScheduler{}
	svc := newTestService(t, &xiaozhiclient.FakeClient{}, fakeSch)
	ctx := context.Background()

	got, err := svc.Create(ctx, CreateRequest{
		DeviceID:    "dev-001",
		ScheduledAt: time.Date(2026, 6, 1, 9, 10, 0, 0, time.UTC),
		Content:     "测血压",
		Category:    CategoryMed,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	canceled, err := svc.Cancel(ctx, got.ID)
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if canceled.Status != StatusCanceled {
		t.Fatalf("Status = %q, want %q", canceled.Status, StatusCanceled)
	}
	if canceled.JobID != "" {
		t.Fatalf("JobID = %q, want cleared job id", canceled.JobID)
	}
	if len(fakeSch.canceled) != 1 || fakeSch.canceled[0] != "job-1" {
		t.Fatalf("canceled jobs = %+v, want job-1", fakeSch.canceled)
	}

	list, err := svc.List(ctx, ListFilter{Status: StatusCanceled})
	if err != nil {
		t.Fatalf("List canceled: %v", err)
	}
	if len(list) != 1 || list[0].ID != got.ID {
		t.Fatalf("canceled reminders = %+v, want canceled reminder", list)
	}
}

func TestServiceMarksReminderUnansweredAfterAckTimeout(t *testing.T) {
	fakeSch := &fakeScheduler{}
	svc := newTestService(t, &xiaozhiclient.FakeClient{}, fakeSch)
	ctx := context.Background()

	got, err := svc.Create(ctx, CreateRequest{
		DeviceID:    "dev-001",
		ScheduledAt: time.Date(2026, 6, 1, 9, 10, 0, 0, time.UTC),
		Content:     "测血压",
		Category:    CategoryMed,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	fakeSch.fire(0)
	played, err := svc.store.Get(ctx, got.ID)
	if err != nil {
		t.Fatalf("Get played: %v", err)
	}
	if played.Status != StatusPlayed {
		t.Fatalf("Status after play = %q, want %q", played.Status, StatusPlayed)
	}
	if played.AckJobID != "job-2" {
		t.Fatalf("AckJobID = %q, want job-2", played.AckJobID)
	}
	if len(fakeSch.jobs) != 3 {
		t.Fatalf("scheduled jobs = %d, want reminder + ack timeout + voice ack poll", len(fakeSch.jobs))
	}
	wantTimeoutAt := svc.now().UTC().Add(30 * time.Minute)
	if !fakeSch.jobs[1].at.Equal(wantTimeoutAt) {
		t.Fatalf("ack timeout at = %s, want %s", fakeSch.jobs[1].at, wantTimeoutAt)
	}
	if want := svc.now().UTC().Add(voiceAckPollInterval); !fakeSch.jobs[2].at.Equal(want) {
		t.Fatalf("voice ack poll at = %s, want %s", fakeSch.jobs[2].at, want)
	}

	fakeSch.fire(1)
	list, err := svc.List(ctx, ListFilter{Status: StatusUnanswered})
	if err != nil {
		t.Fatalf("List unanswered: %v", err)
	}
	if len(list) != 1 || list[0].ID != got.ID || list[0].AckKind != AckKindTimeout {
		t.Fatalf("unanswered reminders = %+v, want timeout reminder", list)
	}
	if list[0].AckJobID != "" {
		t.Fatalf("AckJobID = %q, want cleared after timeout", list[0].AckJobID)
	}
}

func TestServiceAcknowledgePlayedReminderCompletesAndCancelsTimeout(t *testing.T) {
	fakeSch := &fakeScheduler{}
	svc := newTestService(t, &xiaozhiclient.FakeClient{}, fakeSch)
	ctx := context.Background()

	got, err := svc.Create(ctx, CreateRequest{
		DeviceID:    "dev-001",
		ScheduledAt: time.Date(2026, 6, 1, 9, 10, 0, 0, time.UTC),
		Content:     "测血压",
		Category:    CategoryMed,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	fakeSch.fire(0)

	completed, err := svc.Acknowledge(ctx, got.ID, AckRequest{AckKind: AckKindVoice})
	if err != nil {
		t.Fatalf("Acknowledge: %v", err)
	}
	if completed.Status != StatusCompleted {
		t.Fatalf("Status = %q, want %q", completed.Status, StatusCompleted)
	}
	if completed.AckKind != AckKindVoice {
		t.Fatalf("AckKind = %q, want %q", completed.AckKind, AckKindVoice)
	}
	if completed.AcknowledgedAt == nil {
		t.Fatal("AcknowledgedAt = nil, want timestamp")
	}
	if completed.AckJobID != "" {
		t.Fatalf("AckJobID = %q, want cleared after acknowledge", completed.AckJobID)
	}
	if len(fakeSch.canceled) != 1 || fakeSch.canceled[0] != "job-2" {
		t.Fatalf("canceled jobs = %+v, want ack timeout job-2", fakeSch.canceled)
	}

	fakeSch.fire(1)
	list, err := svc.List(ctx, ListFilter{Status: StatusUnanswered})
	if err != nil {
		t.Fatalf("List unanswered: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("unanswered reminders = %+v, want none after voice ack", list)
	}
}

func TestServiceAcknowledgeEmitsMindEventWhenSinkConfigured(t *testing.T) {
	fakeSch := &fakeScheduler{}
	svc := newTestService(t, &xiaozhiclient.FakeClient{}, fakeSch)
	ctx := context.Background()

	got, err := svc.Create(ctx, CreateRequest{
		DeviceID:    "dev-001",
		ScheduledAt: time.Date(2026, 6, 1, 9, 10, 0, 0, time.UTC),
		Content:     "测血压",
		Category:    CategoryMed,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	fakeSch.fire(0)

	sink := &fakeReminderMindSink{}
	svc.UseMindSink(sink)
	if _, err := svc.Acknowledge(ctx, got.ID, AckRequest{AckKind: AckKindVoice}); err != nil {
		t.Fatalf("Acknowledge: %v", err)
	}
	if len(sink.events) != 1 {
		t.Fatalf("events = %+v, want 1", sink.events)
	}
	event := sink.events[0]
	if event.Type != "reminder_acknowledged" || event.SourceID != got.ID {
		t.Fatalf("event = %+v, want reminder_acknowledged for %d", event, got.ID)
	}
	if event.Payload["ackKind"] != string(AckKindVoice) || event.Payload["status"] != string(StatusCompleted) {
		t.Fatalf("payload = %+v, want voice completed", event.Payload)
	}
}

func TestServiceCompletesReminderFromVoiceAcknowledgementHistory(t *testing.T) {
	now := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	xc := &historyClient{}
	fakeSch := &fakeScheduler{}
	svc := newTestService(t, xc, fakeSch)
	svc.now = func() time.Time { return now }
	ctx := context.Background()

	got, err := svc.Create(ctx, CreateRequest{
		DeviceID:    "dev-001",
		ScheduledAt: now.Add(time.Minute),
		Content:     "测血压",
		Category:    CategoryMed,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	fakeSch.fire(0)

	if len(fakeSch.jobs) != 3 {
		t.Fatalf("scheduled jobs = %d, want reminder + ack timeout + voice ack poll", len(fakeSch.jobs))
	}
	if want := now.Add(voiceAckPollInterval); !fakeSch.jobs[2].at.Equal(want) {
		t.Fatalf("voice ack poll at = %s, want %s", fakeSch.jobs[2].at, want)
	}

	xc.history = []xiaozhiclient.HistoryMessage{
		{Role: "assistant", Text: "您该测血压啦", At: now},
		{Role: "user", Text: "好的", At: now.Add(5 * time.Second)},
	}
	now = now.Add(voiceAckPollInterval)
	fakeSch.fire(2)

	completed, err := svc.store.Get(ctx, got.ID)
	if err != nil {
		t.Fatalf("Get completed: %v", err)
	}
	if completed.Status != StatusCompleted || completed.AckKind != AckKindVoice {
		t.Fatalf("completed reminder = %+v, want voice-completed", completed)
	}
	if completed.AcknowledgedAt == nil || !completed.AcknowledgedAt.Equal(now) {
		t.Fatalf("AcknowledgedAt = %v, want %s", completed.AcknowledgedAt, now)
	}
	if xc.gotDeviceID != "dev-001" || xc.gotLimit != voiceAckHistoryLimit {
		t.Fatalf("GetHistory device=%q limit=%d, want dev-001 limit=%d", xc.gotDeviceID, xc.gotLimit, voiceAckHistoryLimit)
	}
	if len(fakeSch.canceled) != 1 || fakeSch.canceled[0] != "job-2" {
		t.Fatalf("canceled jobs = %+v, want ack timeout job-2", fakeSch.canceled)
	}
}

func TestServiceCompletesReminderFromNaturalVoiceAcknowledgement(t *testing.T) {
	now := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	xc := &historyClient{}
	fakeSch := &fakeScheduler{}
	svc := newTestService(t, xc, fakeSch)
	svc.now = func() time.Time { return now }
	ctx := context.Background()

	got, err := svc.Create(ctx, CreateRequest{
		DeviceID:    "dev-001",
		ScheduledAt: now.Add(time.Minute),
		Content:     "测血压",
		Category:    CategoryMed,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	fakeSch.fire(0)

	xc.history = []xiaozhiclient.HistoryMessage{
		{Role: "user", Text: "好的，我这就去测", At: now.Add(5 * time.Second)},
	}
	now = now.Add(voiceAckPollInterval)
	fakeSch.fire(2)

	completed, err := svc.store.Get(ctx, got.ID)
	if err != nil {
		t.Fatalf("Get completed: %v", err)
	}
	if completed.Status != StatusCompleted || completed.AckKind != AckKindVoice {
		t.Fatalf("completed reminder = %+v, want natural voice-completed", completed)
	}
}

func TestServiceIgnoresOldOrNegativeVoiceHistoryAndKeepsPolling(t *testing.T) {
	now := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	xc := &historyClient{}
	fakeSch := &fakeScheduler{}
	svc := newTestService(t, xc, fakeSch)
	svc.now = func() time.Time { return now }
	ctx := context.Background()

	got, err := svc.Create(ctx, CreateRequest{
		DeviceID:    "dev-001",
		ScheduledAt: now.Add(time.Minute),
		Content:     "测血压",
		Category:    CategoryMed,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	fakeSch.fire(0)
	if len(fakeSch.jobs) != 3 {
		t.Fatalf("scheduled jobs = %d, want reminder + ack timeout + voice ack poll", len(fakeSch.jobs))
	}

	xc.history = []xiaozhiclient.HistoryMessage{
		{Role: "user", Text: "好的", At: now.Add(-time.Minute)},
		{Role: "user", Text: "我不好", At: now.Add(5 * time.Second)},
	}
	now = now.Add(voiceAckPollInterval)
	fakeSch.fire(2)

	played, err := svc.store.Get(ctx, got.ID)
	if err != nil {
		t.Fatalf("Get played: %v", err)
	}
	if played.Status != StatusPlayed {
		t.Fatalf("Status = %q, want %q", played.Status, StatusPlayed)
	}
	if len(fakeSch.jobs) != 4 {
		t.Fatalf("scheduled jobs = %d, want next voice ack poll", len(fakeSch.jobs))
	}
	if want := now.Add(voiceAckPollInterval); !fakeSch.jobs[3].at.Equal(want) {
		t.Fatalf("next voice ack poll at = %s, want %s", fakeSch.jobs[3].at, want)
	}
}

func TestVoiceAcknowledgementPhraseRecognition(t *testing.T) {
	for _, text := range []string{"好", "好的", "知道了", "收到", "嗯，好的！"} {
		if !isVoiceAcknowledgement(text) {
			t.Fatalf("isVoiceAcknowledgement(%q) = false, want true", text)
		}
	}
	for _, text := range []string{"不好", "我不知道", "没收到", "好像还没完成"} {
		if isVoiceAcknowledgement(text) {
			t.Fatalf("isVoiceAcknowledgement(%q) = true, want false", text)
		}
	}
}

type fakeScheduler struct {
	jobs     []fakeJob
	canceled []scheduler.JobID
}

type fakeJob struct {
	at time.Time
	fn func()
}

func (f *fakeScheduler) ScheduleAt(t time.Time, fn func()) (scheduler.JobID, error) {
	f.jobs = append(f.jobs, fakeJob{at: t, fn: fn})
	return scheduler.JobID("job-" + string(rune('0'+len(f.jobs)))), nil
}

func (f *fakeScheduler) Cancel(id scheduler.JobID) {
	f.canceled = append(f.canceled, id)
}

func (f *fakeScheduler) fire(i int) {
	f.jobs[i].fn()
}

type historyClient struct {
	xiaozhiclient.FakeClient
	history     []xiaozhiclient.HistoryMessage
	err         error
	gotDeviceID string
	gotLimit    int
}

func (c *historyClient) GetHistory(_ context.Context, deviceID string, limit int) ([]xiaozhiclient.HistoryMessage, error) {
	c.gotDeviceID = deviceID
	c.gotLimit = limit
	return c.history, c.err
}

type deadlineReminderClient struct {
	xiaozhiclient.FakeClient
	gotDeadline bool
	deadline    time.Time
}

func (c *deadlineReminderClient) InjectSpeak(ctx context.Context, deviceID, text string, opts xiaozhiclient.InjectOptions) error {
	c.deadline, c.gotDeadline = ctx.Deadline()
	return nil
}

type failingClient struct {
	err error
}

func assertReminderTextLength(t *testing.T, text string) {
	t.Helper()

	n := len([]rune(text))
	if n < 30 || n > 60 {
		t.Fatalf("reminder text length = %d, want 30..60, text=%q", n, text)
	}
}

func (f failingClient) InjectSpeak(ctx context.Context, deviceID, text string, opts xiaozhiclient.InjectOptions) error {
	return f.err
}

func (f failingClient) GetDeviceStatus(ctx context.Context, deviceID string) (xiaozhiclient.DeviceStatus, error) {
	return xiaozhiclient.DeviceStatus{}, nil
}

func (f failingClient) GetHistory(ctx context.Context, deviceID string, limit int) ([]xiaozhiclient.HistoryMessage, error) {
	return nil, nil
}

func (f failingClient) SetRolePrompt(ctx context.Context, deviceID, prompt string) error {
	return nil
}

func (f failingClient) CallDeviceMCPTool(ctx context.Context, deviceID, tool string, args map[string]any) (json.RawMessage, error) {
	return json.RawMessage(`{}`), nil
}

type throttledVoiceGate struct{}

func (throttledVoiceGate) TryAcquireProactiveVoice(context.Context, string, time.Time) (sharedtypes.ProactiveVoiceLease, error) {
	return nil, sharedtypes.ErrProactiveVoiceThrottled
}
