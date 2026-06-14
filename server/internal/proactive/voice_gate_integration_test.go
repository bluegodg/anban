package proactive_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/bluegodg/anban/server/internal/domains/greeting"
	"github.com/bluegodg/anban/server/internal/domains/reminder"
	"github.com/bluegodg/anban/server/internal/proactive"
	"github.com/bluegodg/anban/server/internal/scheduler"
	"github.com/bluegodg/anban/server/internal/store"
	"github.com/bluegodg/anban/server/internal/xiaozhiclient"
)

func TestSharedVoiceGateBlocksReminderAfterGreetingForSameDevice(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}

	greetingStore := greeting.NewStore(st.DB)
	if err := greetingStore.AutoMigrate(); err != nil {
		t.Fatalf("greeting AutoMigrate: %v", err)
	}
	reminderStore := reminder.NewStore(st.DB)
	if err := reminderStore.AutoMigrate(); err != nil {
		t.Fatalf("reminder AutoMigrate: %v", err)
	}

	fakeXC := &xiaozhiclient.FakeClient{}
	gate := proactive.NewVoiceGate(10 * time.Minute)
	greetingSvc := greeting.NewService(greetingStore, fakeXC)
	greetingSvc.UseProactiveVoiceGate(gate)
	fakeSch := &integrationOneShotScheduler{}
	reminderSvc := reminder.NewService(reminderStore, fakeXC, fakeSch)
	reminderSvc.UseProactiveVoiceGate(gate)

	played, err := greetingSvc.Trigger(ctx, greeting.TriggerRequest{DeviceID: "dev-001", TonePreset: greeting.ToneWarm})
	if err != nil {
		t.Fatalf("Trigger greeting: %v", err)
	}
	if played.Status != greeting.StatusPlayed {
		t.Fatalf("greeting status = %q, want %q", played.Status, greeting.StatusPlayed)
	}

	created, err := reminderSvc.Create(ctx, reminder.CreateRequest{
		DeviceID:    "dev-001",
		ScheduledAt: time.Now().UTC().Add(time.Minute),
		Content:     "测血压",
		Category:    reminder.CategoryMed,
	})
	if err != nil {
		t.Fatalf("Create reminder: %v", err)
	}
	fakeSch.fire(0)

	scheduled, err := reminderSvc.List(ctx, reminder.ListFilter{Status: reminder.StatusScheduled})
	if err != nil {
		t.Fatalf("List scheduled reminders: %v", err)
	}
	if len(scheduled) != 1 || scheduled[0].ID != created.ID {
		t.Fatalf("scheduled reminders = %+v, want requeued reminder %d", scheduled, created.ID)
	}
	if scheduled[0].JobID != "job-2" {
		t.Fatalf("JobID = %q, want retry job id job-2", scheduled[0].JobID)
	}
	if !strings.Contains(scheduled[0].ErrorMessage, "proactive voice") {
		t.Fatalf("ErrorMessage = %q, want proactive voice throttle detail", scheduled[0].ErrorMessage)
	}
	if len(fakeXC.InjectCalls) != 1 {
		t.Fatalf("InjectCalls = %d, want only greeting to inject", len(fakeXC.InjectCalls))
	}
	if len(fakeSch.jobs) != 2 {
		t.Fatalf("scheduled jobs = %d, want original reminder job plus retry job", len(fakeSch.jobs))
	}
}

type integrationOneShotScheduler struct {
	jobs []func()
}

func (s *integrationOneShotScheduler) ScheduleAt(_ time.Time, fn func()) (scheduler.JobID, error) {
	s.jobs = append(s.jobs, fn)
	return scheduler.JobID("job-" + string(rune('0'+len(s.jobs)))), nil
}

func (s *integrationOneShotScheduler) Cancel(scheduler.JobID) {}

func (s *integrationOneShotScheduler) fire(index int) {
	s.jobs[index]()
}
