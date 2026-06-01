package xiaozhiclient

import (
	"context"
	"encoding/json"
)

// FakeClient 实现 Client，把调用记录在内存里，供各域并行开发与单测使用。
type FakeClient struct {
	InjectCalls     []InjectCall
	RolePromptCalls []RolePromptCall
}

type InjectCall struct {
	DeviceID string
	Text     string
	Opts     InjectOptions
}

type RolePromptCall struct {
	DeviceID string
	Prompt   string
}

var _ Client = (*FakeClient)(nil)

func (f *FakeClient) InjectSpeak(ctx context.Context, deviceID, text string, opts InjectOptions) error {
	f.InjectCalls = append(f.InjectCalls, InjectCall{DeviceID: deviceID, Text: text, Opts: opts})
	return nil
}
func (f *FakeClient) GetDeviceStatus(ctx context.Context, deviceID string) (DeviceStatus, error) {
	return DeviceStatus{DeviceID: deviceID, Online: true}, nil
}
func (f *FakeClient) GetHistory(ctx context.Context, deviceID string, limit int) ([]HistoryMessage, error) {
	return nil, nil
}
func (f *FakeClient) SetRolePrompt(ctx context.Context, deviceID, prompt string) error {
	f.RolePromptCalls = append(f.RolePromptCalls, RolePromptCall{DeviceID: deviceID, Prompt: prompt})
	return nil
}
func (f *FakeClient) CallDeviceMCPTool(ctx context.Context, deviceID, tool string, args map[string]any) (json.RawMessage, error) {
	return json.RawMessage(`{}`), nil
}
