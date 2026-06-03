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

	list, err := svc.List(context.Background(), ListFilter{DeviceID: "dev-001"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0].Status != StatusPlayed || list[0].PlayedAt == nil {
		t.Fatalf("List = %+v, want played reminder", list)
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

func TestReminderTextFitsPRDLength(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		category Category
	}{
		{name: "short medicine reminder", content: "测血压", category: CategoryMed},
		{name: "long custom reminder", content: strings.Repeat("记得测血压", 20), category: CategoryCustom},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text := reminderText(tt.content, tt.category)
			assertReminderTextLength(t, text)
			if !strings.Contains(text, "王阿姨") {
				t.Fatalf("text = %q, want elder-facing salutation", text)
			}
		})
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

func TestServiceSkipsReminderWhenProactiveVoiceQuotaUsed(t *testing.T) {
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
	list, err := svc.List(context.Background(), ListFilter{Status: StatusSkipped})
	if err != nil {
		t.Fatalf("List skipped: %v", err)
	}
	if len(list) != 1 || list[0].ID != got.ID {
		t.Fatalf("skipped reminders = %+v, want skipped reminder %d", list, got.ID)
	}
	if list[0].JobID != "" {
		t.Fatalf("JobID = %q, want cleared after skipped proactive voice", list[0].JobID)
	}
	if list[0].ErrorMessage == "" {
		t.Fatal("ErrorMessage is empty")
	}
	if len(fakeXC.InjectCalls) != 0 {
		t.Fatalf("InjectCalls = %d, want no xiaozhi injection when quota is used", len(fakeXC.InjectCalls))
	}
	if len(fakeSch.jobs) != 1 {
		t.Fatalf("scheduled jobs = %d, want no ack timeout job after skipped reminder", len(fakeSch.jobs))
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
	playedAt := time.Date(2026, 6, 1, 9, 5, 0, 0, time.UTC)

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
	if len(fakeSch.jobs) != 1 {
		t.Fatalf("scheduled jobs = %d, want 1 ack timeout", len(fakeSch.jobs))
	}
	if want := playedAt.Add(defaultAckTimeout); !fakeSch.jobs[0].at.Equal(want) {
		t.Fatalf("ack timeout at = %s, want %s", fakeSch.jobs[0].at, want)
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
	if len(fakeSch.jobs) != 2 {
		t.Fatalf("scheduled jobs = %d, want reminder + ack timeout", len(fakeSch.jobs))
	}
	wantTimeoutAt := svc.now().UTC().Add(30 * time.Minute)
	if !fakeSch.jobs[1].at.Equal(wantTimeoutAt) {
		t.Fatalf("ack timeout at = %s, want %s", fakeSch.jobs[1].at, wantTimeoutAt)
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
