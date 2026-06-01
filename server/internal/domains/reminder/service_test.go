package reminder

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/bluegodg/anban/server/internal/scheduler"
	"github.com/bluegodg/anban/server/internal/store"
	"github.com/bluegodg/anban/server/internal/xiaozhiclient"
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
	if call.Text != "王阿姨，该测血压啦~ 小宝昨天还问起您有没有按时测呢。" {
		t.Fatalf("inject text = %q", call.Text)
	}
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
