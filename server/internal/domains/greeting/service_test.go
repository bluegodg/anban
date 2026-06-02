package greeting

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/bluegodg/anban/server/internal/store"
	"github.com/bluegodg/anban/server/internal/xiaozhiclient"
)

func newTestService(t *testing.T, xc xiaozhiclient.Client) *Service {
	t.Helper()

	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	greetingStore := NewStore(st.DB)
	if err := greetingStore.AutoMigrate(); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}

	svc := NewService(greetingStore, xc)
	svc.now = func() time.Time { return time.Date(2026, 5, 31, 15, 30, 0, 0, time.UTC) }
	return svc
}

func TestServiceTriggerInjectsGreetingAndPersistsPlayed(t *testing.T) {
	fake := &xiaozhiclient.FakeClient{}
	svc := newTestService(t, fake)

	got, err := svc.Trigger(context.Background(), TriggerRequest{
		DeviceID:   "dev-001",
		TonePreset: ToneWarm,
	})
	if err != nil {
		t.Fatalf("Trigger: %v", err)
	}
	if got.ID == 0 {
		t.Fatal("greeting ID was not assigned")
	}
	if got.Status != StatusPlayed {
		t.Fatalf("Status = %q, want %q", got.Status, StatusPlayed)
	}
	if got.PlayedAt == nil {
		t.Fatal("PlayedAt is nil")
	}
	if len(fake.InjectCalls) != 1 {
		t.Fatalf("InjectCalls = %d, want 1", len(fake.InjectCalls))
	}
	call := fake.InjectCalls[0]
	if call.DeviceID != "dev-001" {
		t.Fatalf("DeviceID = %q, want dev-001", call.DeviceID)
	}
	if call.Text != "王阿姨，下午好~ 今天精神咋样？" {
		t.Fatalf("inject text = %q", call.Text)
	}
	if !call.Opts.SkipLLM {
		t.Fatal("SkipLLM = false, want true for deterministic greeting demo")
	}

	list, err := svc.List(context.Background(), ListFilter{DeviceID: "dev-001"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0].ID != got.ID {
		t.Fatalf("List = %+v, want the created greeting", list)
	}
}

func TestServiceTriggerValidatesInputAndDefaultsTone(t *testing.T) {
	fake := &xiaozhiclient.FakeClient{}
	svc := newTestService(t, fake)

	if _, err := svc.Trigger(context.Background(), TriggerRequest{}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("missing deviceID err = %v, want ErrInvalidInput", err)
	}

	got, err := svc.Trigger(context.Background(), TriggerRequest{DeviceID: "dev-001", TonePreset: "unknown"})
	if err != nil {
		t.Fatalf("Trigger with unknown tone: %v", err)
	}
	if got.TonePreset != ToneWarm {
		t.Fatalf("TonePreset = %q, want %q", got.TonePreset, ToneWarm)
	}
}

func TestServiceGreetingScheduleDefaultsAndPersists(t *testing.T) {
	svc := newTestService(t, &xiaozhiclient.FakeClient{})
	ctx := context.Background()

	defaultSchedule, err := svc.GetSchedule(ctx, "dev-001")
	if err != nil {
		t.Fatalf("GetSchedule default: %v", err)
	}
	if defaultSchedule.DeviceID != "dev-001" {
		t.Fatalf("DeviceID = %q, want dev-001", defaultSchedule.DeviceID)
	}
	if len(defaultSchedule.Slots) != 3 {
		t.Fatalf("default slots = %+v, want 3 slots", defaultSchedule.Slots)
	}
	if defaultSchedule.Slots[0].Time != "08:00" || !defaultSchedule.Slots[0].Enabled {
		t.Fatalf("morning slot = %+v, want enabled 08:00", defaultSchedule.Slots[0])
	}

	updated, err := svc.UpdateSchedule(ctx, ScheduleRequest{
		DeviceID: " dev-001 ",
		Slots: []ScheduleSlot{
			{Label: "morning", Time: "07:30", Enabled: true, TonePreset: ToneWarm},
			{Label: "noon", Time: "12:20", Enabled: false, TonePreset: ToneCasual},
			{Label: "evening", Time: "18:10", Enabled: true},
		},
	})
	if err != nil {
		t.Fatalf("UpdateSchedule: %v", err)
	}
	if updated.DeviceID != "dev-001" {
		t.Fatalf("updated DeviceID = %q, want trimmed dev-001", updated.DeviceID)
	}
	if updated.Slots[2].TonePreset != ToneWarm {
		t.Fatalf("evening tone = %q, want default warm", updated.Slots[2].TonePreset)
	}

	got, err := svc.GetSchedule(ctx, "dev-001")
	if err != nil {
		t.Fatalf("GetSchedule after update: %v", err)
	}
	if got.Slots[0].Time != "07:30" || got.Slots[1].Enabled {
		t.Fatalf("persisted slots = %+v, want updated schedule", got.Slots)
	}
}

func TestServiceGreetingScheduleValidatesInput(t *testing.T) {
	svc := newTestService(t, &xiaozhiclient.FakeClient{})
	ctx := context.Background()

	tests := []struct {
		name string
		req  ScheduleRequest
	}{
		{name: "missing device", req: ScheduleRequest{Slots: []ScheduleSlot{{Label: "morning", Time: "08:00", Enabled: true}}}},
		{name: "missing slots", req: ScheduleRequest{DeviceID: "dev-001"}},
		{name: "bad time", req: ScheduleRequest{DeviceID: "dev-001", Slots: []ScheduleSlot{{Label: "morning", Time: "8am", Enabled: true}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := svc.UpdateSchedule(ctx, tt.req); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("err = %v, want ErrInvalidInput", err)
			}
		})
	}

	if _, err := svc.GetSchedule(ctx, " "); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("GetSchedule blank err = %v, want ErrInvalidInput", err)
	}
}

func TestServiceTriggerMarksFailedWhenInjectFails(t *testing.T) {
	svc := newTestService(t, failingClient{err: errors.New("manager unavailable")})

	got, err := svc.Trigger(context.Background(), TriggerRequest{DeviceID: "dev-001", TonePreset: ToneCasual})
	if err == nil {
		t.Fatal("expected inject error, got nil")
	}
	if got.Status != StatusFailed {
		t.Fatalf("Status = %q, want %q", got.Status, StatusFailed)
	}
	if got.ErrorMessage == "" {
		t.Fatal("ErrorMessage is empty")
	}
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
