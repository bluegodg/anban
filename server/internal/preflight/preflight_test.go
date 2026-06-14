package preflight

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bluegodg/anban/server/internal/xiaozhiclient"
)

func TestRunPassesDeviceStatusButKeepsManualXiaozhiGate(t *testing.T) {
	lastActive := time.Date(2026, 6, 8, 9, 30, 0, 0, time.UTC)
	client := &statusReader{
		status: xiaozhiclient.DeviceStatus{
			DeviceID:     "dev-001",
			Online:       true,
			LastActiveAt: lastActive,
		},
	}

	report := Run(context.Background(), client, " dev-001 ")

	if report.Failed() {
		t.Fatalf("report should not fail: %+v", report)
	}
	if !client.accessChecked {
		t.Fatal("manager access was not checked before device status")
	}
	if client.deviceID != "dev-001" {
		t.Fatalf("deviceID = %q, want trimmed dev-001", client.deviceID)
	}
	assertCheck(t, report, "xiaozhi-only voice loop", StatusManual)
	assertCheck(t, report, "xiaozhi manager OpenAPI access", StatusPass)
	assertCheck(t, report, "xiaozhi manager device status", StatusPass)
	assertCheck(t, report, "anban optionality smoke", StatusManual)

	formatted := FormatReport(report)
	if !strings.Contains(formatted, "Gate A") || !strings.Contains(formatted, "Gate D") || !strings.Contains(formatted, "last_active_at=2026-06-08T09:30:00Z") {
		t.Fatalf("formatted report missing gate/status detail:\n%s", formatted)
	}
}

func TestRunIncludesManualAnbanOptionalityGate(t *testing.T) {
	client := &statusReader{}

	report := Run(context.Background(), client, "")

	assertCheck(t, report, "anban optionality smoke", StatusManual)
	formatted := FormatReport(report)
	if !strings.Contains(formatted, "停掉 anban") || !strings.Contains(formatted, "原版小智") {
		t.Fatalf("formatted report missing optionality guidance:\n%s", formatted)
	}
}

func TestRunSkipsDeviceStatusWhenDeviceIDMissing(t *testing.T) {
	client := &statusReader{}

	report := Run(context.Background(), client, "")

	if report.Failed() {
		t.Fatalf("report should not fail when optional device check is skipped: %+v", report)
	}
	assertCheck(t, report, "xiaozhi manager OpenAPI access", StatusPass)
	assertCheck(t, report, "xiaozhi manager device status", StatusSkip)
	if !client.accessChecked {
		t.Fatal("manager access should be checked even when device ID is empty")
	}
	if client.called {
		t.Fatal("GetDeviceStatus should not be called when device ID is empty")
	}
}

func TestRunFailsWhenManagerAccessCannotBeChecked(t *testing.T) {
	client := &statusReader{accessErr: errors.New("401 unauthorized")}

	report := Run(context.Background(), client, "")

	if !report.Failed() {
		t.Fatalf("report should fail: %+v", report)
	}
	assertCheck(t, report, "xiaozhi manager OpenAPI access", StatusFail)
	if client.called {
		t.Fatal("GetDeviceStatus should not be called when manager access fails")
	}
}

func TestRunFailsWhenDeviceStatusCannotBeRead(t *testing.T) {
	client := &statusReader{err: errors.New("401 unauthorized")}

	report := Run(context.Background(), client, "dev-001")

	if !report.Failed() {
		t.Fatalf("report should fail: %+v", report)
	}
	assertCheck(t, report, "xiaozhi manager device status", StatusFail)
}

func TestRunFailsWhenDeviceIsOffline(t *testing.T) {
	client := &statusReader{
		status: xiaozhiclient.DeviceStatus{
			DeviceID: "dev-001",
			Online:   false,
		},
	}

	report := Run(context.Background(), client, "dev-001")

	if !report.Failed() {
		t.Fatalf("report should fail for offline device: %+v", report)
	}
	assertCheck(t, report, "xiaozhi manager device status", StatusFail)
}

func assertCheck(t *testing.T, report Report, name string, status Status) {
	t.Helper()
	for _, check := range report.Checks {
		if check.Name == name {
			if check.Status != status {
				t.Fatalf("%s status = %s, want %s", name, check.Status, status)
			}
			return
		}
	}
	t.Fatalf("missing check %q in %+v", name, report.Checks)
}

type statusReader struct {
	status        xiaozhiclient.DeviceStatus
	accessErr     error
	err           error
	accessChecked bool
	called        bool
	deviceID      string
}

func (s *statusReader) CheckManagerAccess(ctx context.Context) error {
	s.accessChecked = true
	return s.accessErr
}

func (s *statusReader) GetDeviceStatus(ctx context.Context, deviceID string) (xiaozhiclient.DeviceStatus, error) {
	s.called = true
	s.deviceID = deviceID
	if s.err != nil {
		return xiaozhiclient.DeviceStatus{}, s.err
	}
	return s.status, nil
}
