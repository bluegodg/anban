package profile

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/bluegodg/anban/server/internal/store"
	"github.com/bluegodg/anban/server/internal/xiaozhiclient"
)

func newTestService(t *testing.T, xc xiaozhiclient.Client) *Service {
	t.Helper()

	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	profileStore := NewStore(st.DB)
	if err := profileStore.AutoMigrate(); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	return NewService(profileStore, xc)
}

func TestServiceUpdatePersistsProfileAndSyncsPrompt(t *testing.T) {
	xc := &profileClient{}
	svc := newTestService(t, xc)
	ctx := context.Background()

	got, err := svc.Update(ctx, UpdateRequest{
		DeviceID: " dev-001 ",
		Fields: Fields{
			Name:          "王秀英",
			Nickname:      "妈",
			Children:      []string{"小明", "小红"},
			Grandchildren: []string{"小宝（7岁）"},
			Hobbies:       []string{"豫剧", "下棋"},
			Schedule:      "早 6 点起，晚 9 点睡",
			Health:        "高血压、轻度糖尿病",
			Taboos:        []string{"甜食"},
		},
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got.DeviceID != "dev-001" || got.Fields.Name != "王秀英" {
		t.Fatalf("profile = %+v, want trimmed device and stored fields", got)
	}
	if xc.gotDeviceID != "dev-001" {
		t.Fatalf("SetRolePrompt deviceID = %q, want dev-001", xc.gotDeviceID)
	}
	for _, want := range []string{"王秀英", "小宝", "豫剧", "高血压"} {
		if !strings.Contains(xc.gotPrompt, want) {
			t.Fatalf("prompt = %q, want contains %q", xc.gotPrompt, want)
		}
	}

	saved, err := svc.Get(ctx, "dev-001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if saved.ID != got.ID || saved.Prompt == "" {
		t.Fatalf("saved profile = %+v, want persisted profile with prompt", saved)
	}
}

func TestServiceUpdateRejectsMissingDeviceID(t *testing.T) {
	svc := newTestService(t, &profileClient{})

	_, err := svc.Update(context.Background(), UpdateRequest{DeviceID: " "})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}

func TestServiceGetRejectsMissingDeviceID(t *testing.T) {
	svc := newTestService(t, &profileClient{})

	_, err := svc.Get(context.Background(), " ")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}

func TestServiceUpdateReturnsSyncErrorAfterPersisting(t *testing.T) {
	xc := &profileClient{err: errors.New("manager unavailable")}
	svc := newTestService(t, xc)
	ctx := context.Background()

	got, err := svc.Update(ctx, UpdateRequest{
		DeviceID: "dev-001",
		Fields: Fields{
			Name:     "王秀英",
			Nickname: "妈",
			Hobbies:  []string{"豫剧"},
		},
	})
	if err == nil {
		t.Fatal("expected sync error, got nil")
	}
	if got.DeviceID != "dev-001" || got.ID == 0 {
		t.Fatalf("profile = %+v, want persisted profile returned with error", got)
	}

	saved, getErr := svc.Get(ctx, "dev-001")
	if getErr != nil {
		t.Fatalf("Get after sync error: %v", getErr)
	}
	if saved.Prompt == "" {
		t.Fatal("saved prompt is empty")
	}
}

type profileClient struct {
	xiaozhiclient.FakeClient
	gotDeviceID string
	gotPrompt   string
	err         error
}

func (c *profileClient) SetRolePrompt(ctx context.Context, deviceID, prompt string) error {
	c.gotDeviceID = deviceID
	c.gotPrompt = prompt
	return c.err
}

func (c *profileClient) CallDeviceMCPTool(ctx context.Context, deviceID, tool string, args map[string]any) (json.RawMessage, error) {
	return json.RawMessage(`{}`), nil
}
