package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunCommandCanExplicitlyCheckManagerAccessWithoutDeviceID(t *testing.T) {
	var gotPath, gotToken string
	manager := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotToken = r.Header.Get("X-API-Token")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":[]}`))
	}))
	defer manager.Close()

	t.Setenv("ANBAN_MANAGER_BASE_URL", manager.URL)
	t.Setenv("ANBAN_MANAGER_API_TOKEN", "tok_abc")
	t.Setenv("ANBAN_ACCESS_CODE", "")
	t.Setenv("ANBAN_PREFLIGHT_DEVICE_ID", "")

	var stdout, stderr bytes.Buffer
	exitCode := run([]string{"--xiaozhi-gate-passed", "--allow-missing-device-id"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0; stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
	}
	if gotPath != "/api/open/v1/devices" {
		t.Fatalf("path = %q, want devices endpoint", gotPath)
	}
	if gotToken != "tok_abc" {
		t.Fatalf("X-API-Token = %q, want tok_abc", gotToken)
	}
	if !strings.Contains(stdout.String(), "[PASS] xiaozhi manager OpenAPI access") {
		t.Fatalf("stdout missing manager PASS:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "[SKIP] xiaozhi manager device status") {
		t.Fatalf("stdout missing device SKIP:\n%s", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunCommandRequiresDeviceIDByDefault(t *testing.T) {
	manager := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":[]}`))
	}))
	defer manager.Close()

	t.Setenv("ANBAN_MANAGER_BASE_URL", manager.URL)
	t.Setenv("ANBAN_MANAGER_API_TOKEN", "tok_abc")
	t.Setenv("ANBAN_ACCESS_CODE", "")
	t.Setenv("ANBAN_PREFLIGHT_DEVICE_ID", "")
	t.Setenv("ANBAN_PREFLIGHT_XIAOZHI_GATE_PASSED", "true")

	var stdout, stderr bytes.Buffer
	exitCode := run([]string{}, &stdout, &stderr)
	if exitCode == 0 {
		t.Fatalf("exitCode = 0, want non-zero when device ID is missing by default; stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "device ID") || !strings.Contains(stderr.String(), "allow-missing-device-id") {
		t.Fatalf("stderr missing device ID guidance:\n%s", stderr.String())
	}
	if !strings.Contains(stdout.String(), "[SKIP] xiaozhi manager device status") {
		t.Fatalf("stdout should still show skipped manager device check:\n%s", stdout.String())
	}
}

func TestRunCommandPassesWithConfirmedGateAndOnlineDevice(t *testing.T) {
	var deviceChecks int
	manager := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		deviceChecks++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":[{"device_name":"dev-001","online":true,"last_active_at":"2026-06-08T09:30:00Z"}]}`))
	}))
	defer manager.Close()

	t.Setenv("ANBAN_MANAGER_BASE_URL", manager.URL)
	t.Setenv("ANBAN_MANAGER_API_TOKEN", "tok_abc")
	t.Setenv("ANBAN_ACCESS_CODE", "")
	t.Setenv("ANBAN_PREFLIGHT_DEVICE_ID", "dev-001")
	t.Setenv("ANBAN_PREFLIGHT_XIAOZHI_GATE_PASSED", "true")

	var stdout, stderr bytes.Buffer
	exitCode := run([]string{}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 with confirmed Gate A and online device; stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
	}
	if deviceChecks < 2 {
		t.Fatalf("manager device endpoint checks = %d, want access check and device status check", deviceChecks)
	}
	if !strings.Contains(stdout.String(), "[PASS] xiaozhi manager device status") || !strings.Contains(stdout.String(), "dev-001 在线") {
		t.Fatalf("stdout missing online device pass:\n%s", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunCommandRequiresExplicitXiaozhiGateConfirmation(t *testing.T) {
	manager := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":[]}`))
	}))
	defer manager.Close()

	t.Setenv("ANBAN_MANAGER_BASE_URL", manager.URL)
	t.Setenv("ANBAN_MANAGER_API_TOKEN", "tok_abc")
	t.Setenv("ANBAN_ACCESS_CODE", "")
	t.Setenv("ANBAN_PREFLIGHT_DEVICE_ID", "")
	t.Setenv("ANBAN_PREFLIGHT_XIAOZHI_GATE_PASSED", "")

	var stdout, stderr bytes.Buffer
	exitCode := run([]string{}, &stdout, &stderr)
	if exitCode == 0 {
		t.Fatalf("exitCode = 0, want non-zero until Gate A is explicitly confirmed; stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "Gate A") {
		t.Fatalf("stderr missing Gate A guidance:\n%s", stderr.String())
	}
}

func TestRunCommandAcceptsGateAndManagerOnlyConfirmationFromEnv(t *testing.T) {
	manager := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":[]}`))
	}))
	defer manager.Close()

	t.Setenv("ANBAN_MANAGER_BASE_URL", manager.URL)
	t.Setenv("ANBAN_MANAGER_API_TOKEN", "tok_abc")
	t.Setenv("ANBAN_ACCESS_CODE", "")
	t.Setenv("ANBAN_PREFLIGHT_DEVICE_ID", "")
	t.Setenv("ANBAN_PREFLIGHT_XIAOZHI_GATE_PASSED", "true")
	t.Setenv("ANBAN_PREFLIGHT_ALLOW_MISSING_DEVICE_ID", "true")

	var stdout, stderr bytes.Buffer
	exitCode := run([]string{}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 with env gate and manager-only confirmation; stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
	}
}

func TestRunCommandAcceptsTrimmedGateAndManagerOnlyConfirmationFromEnv(t *testing.T) {
	manager := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":[]}`))
	}))
	defer manager.Close()

	t.Setenv("ANBAN_MANAGER_BASE_URL", manager.URL)
	t.Setenv("ANBAN_MANAGER_API_TOKEN", "tok_abc")
	t.Setenv("ANBAN_ACCESS_CODE", "")
	t.Setenv("ANBAN_PREFLIGHT_DEVICE_ID", "")
	t.Setenv("ANBAN_PREFLIGHT_XIAOZHI_GATE_PASSED", " true ")
	t.Setenv("ANBAN_PREFLIGHT_ALLOW_MISSING_DEVICE_ID", " true ")

	var stdout, stderr bytes.Buffer
	exitCode := run([]string{}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 with trimmed env gate and manager-only confirmation; stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
	}
}

func TestRunCommandReturnsNonZeroWhenManagerCheckFails(t *testing.T) {
	manager := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"bad token"}`))
	}))
	defer manager.Close()

	t.Setenv("ANBAN_MANAGER_BASE_URL", manager.URL)
	t.Setenv("ANBAN_MANAGER_API_TOKEN", "bad")
	t.Setenv("ANBAN_ACCESS_CODE", "")
	t.Setenv("ANBAN_PREFLIGHT_DEVICE_ID", "")
	t.Setenv("ANBAN_PREFLIGHT_XIAOZHI_GATE_PASSED", "true")

	var stdout, stderr bytes.Buffer
	exitCode := run([]string{}, &stdout, &stderr)
	if exitCode == 0 {
		t.Fatalf("exitCode = 0, want non-zero; stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "[FAIL] xiaozhi manager OpenAPI access") {
		t.Fatalf("stdout missing manager FAIL:\n%s", stdout.String())
	}
}

func TestLoadPreflightConfigDoesNotRequireChildAccessCode(t *testing.T) {
	t.Setenv("ANBAN_MANAGER_BASE_URL", "http://localhost:8080")
	t.Setenv("ANBAN_MANAGER_API_TOKEN", "tok_123")
	t.Setenv("ANBAN_ACCESS_CODE", "")

	cfg, err := loadPreflightConfig()
	if err != nil {
		t.Fatalf("loadPreflightConfig: %v", err)
	}
	if cfg.ManagerBaseURL != "http://localhost:8080" {
		t.Fatalf("ManagerBaseURL = %q", cfg.ManagerBaseURL)
	}
	if cfg.ManagerAPIToken != "tok_123" {
		t.Fatalf("ManagerAPIToken = %q", cfg.ManagerAPIToken)
	}
}

func TestLoadPreflightConfigRejectsExampleManagerTokenPlaceholder(t *testing.T) {
	t.Setenv("ANBAN_MANAGER_BASE_URL", "http://localhost:8080")
	t.Setenv("ANBAN_MANAGER_API_TOKEN", "请填_manager签发的APIToken")

	if _, err := loadPreflightConfig(); err == nil {
		t.Fatal("expected example manager token placeholder to be rejected")
	}
}

func TestLoadPreflightConfigTrimsManagerAccess(t *testing.T) {
	t.Setenv("ANBAN_MANAGER_BASE_URL", " http://localhost:8080/ ")
	t.Setenv("ANBAN_MANAGER_API_TOKEN", " tok_123 ")

	cfg, err := loadPreflightConfig()
	if err != nil {
		t.Fatalf("loadPreflightConfig: %v", err)
	}
	if cfg.ManagerBaseURL != "http://localhost:8080/" {
		t.Fatalf("ManagerBaseURL = %q, want trimmed URL", cfg.ManagerBaseURL)
	}
	if cfg.ManagerAPIToken != "tok_123" {
		t.Fatalf("ManagerAPIToken = %q, want trimmed token", cfg.ManagerAPIToken)
	}
}

func TestLoadPreflightConfigRequiresManagerAccess(t *testing.T) {
	t.Setenv("ANBAN_MANAGER_BASE_URL", "")
	t.Setenv("ANBAN_MANAGER_API_TOKEN", "tok_123")
	if _, err := loadPreflightConfig(); err == nil {
		t.Fatal("expected missing manager URL error, got nil")
	}

	t.Setenv("ANBAN_MANAGER_BASE_URL", "http://localhost:8080")
	t.Setenv("ANBAN_MANAGER_API_TOKEN", "")
	if _, err := loadPreflightConfig(); err == nil {
		t.Fatal("expected missing manager token error, got nil")
	}

	t.Setenv("ANBAN_MANAGER_BASE_URL", "   ")
	t.Setenv("ANBAN_MANAGER_API_TOKEN", "tok_123")
	if _, err := loadPreflightConfig(); err == nil {
		t.Fatal("expected whitespace manager URL error, got nil")
	}
}
