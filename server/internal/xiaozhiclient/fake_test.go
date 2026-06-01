package xiaozhiclient

import (
	"context"
	"testing"
)

func TestFakeClientRecordsCallsAndReturnsDefaults(t *testing.T) {
	fake := &FakeClient{}
	ctx := context.Background()

	if err := fake.InjectSpeak(ctx, "dev-001", "你好", InjectOptions{SkipLLM: true}); err != nil {
		t.Fatalf("InjectSpeak: %v", err)
	}
	if err := fake.SetRolePrompt(ctx, "dev-001", "画像 prompt"); err != nil {
		t.Fatalf("SetRolePrompt: %v", err)
	}
	status, err := fake.GetDeviceStatus(ctx, "dev-001")
	if err != nil {
		t.Fatalf("GetDeviceStatus: %v", err)
	}
	history, err := fake.GetHistory(ctx, "dev-001", 5)
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	raw, err := fake.CallDeviceMCPTool(ctx, "dev-001", "camera.capture", nil)
	if err != nil {
		t.Fatalf("CallDeviceMCPTool: %v", err)
	}

	if len(fake.InjectCalls) != 1 || fake.InjectCalls[0].Text != "你好" {
		t.Fatalf("InjectCalls = %+v, want recorded inject", fake.InjectCalls)
	}
	if len(fake.RolePromptCalls) != 1 || fake.RolePromptCalls[0].Prompt != "画像 prompt" {
		t.Fatalf("RolePromptCalls = %+v, want recorded prompt", fake.RolePromptCalls)
	}
	if status.DeviceID != "dev-001" || !status.Online {
		t.Fatalf("status = %+v, want online dev-001", status)
	}
	if history != nil {
		t.Fatalf("history = %+v, want nil", history)
	}
	if string(raw) != "{}" {
		t.Fatalf("raw = %s, want {}", raw)
	}
}
