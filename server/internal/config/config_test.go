package config

import (
	"reflect"
	"testing"
)

func TestLoadFailsWhenManagerTokenMissing(t *testing.T) {
	t.Setenv("ANBAN_MANAGER_BASE_URL", "http://localhost:8080")
	t.Setenv("ANBAN_MANAGER_API_TOKEN", "")
	t.Setenv("ANBAN_ACCESS_CODE", "demo")
	if _, err := Load(); err == nil {
		t.Fatal("expected error when ANBAN_MANAGER_API_TOKEN missing, got nil")
	}
}

func TestLoadRejectsExampleManagerTokenPlaceholder(t *testing.T) {
	t.Setenv("ANBAN_MANAGER_BASE_URL", "http://localhost:8080")
	t.Setenv("ANBAN_MANAGER_API_TOKEN", "请填_manager签发的APIToken")
	t.Setenv("ANBAN_ACCESS_CODE", "demo")
	if _, err := Load(); err == nil {
		t.Fatal("expected example manager token placeholder to be rejected")
	}
}

func TestLoadOKWithDefaults(t *testing.T) {
	t.Setenv("ANBAN_MANAGER_BASE_URL", "http://localhost:8080")
	t.Setenv("ANBAN_MANAGER_API_TOKEN", "tok_123")
	t.Setenv("ANBAN_ACCESS_CODE", "demo")
	c, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.ListenAddr != ":8090" {
		t.Fatalf("ListenAddr default = %q, want :8090", c.ListenAddr)
	}
	if c.DBDSN != "anban.db" {
		t.Fatalf("DBDSN default = %q, want anban.db", c.DBDSN)
	}
	wantOrigins := []string{"http://127.0.0.1:5173", "http://localhost:5173"}
	if !reflect.DeepEqual(c.AllowedOrigins, wantOrigins) {
		t.Fatalf("AllowedOrigins = %#v, want %#v", c.AllowedOrigins, wantOrigins)
	}
}

func TestLoadParsesAllowedOrigins(t *testing.T) {
	t.Setenv("ANBAN_MANAGER_BASE_URL", "http://localhost:8080")
	t.Setenv("ANBAN_MANAGER_API_TOKEN", "tok_123")
	t.Setenv("ANBAN_ACCESS_CODE", "demo")
	t.Setenv("ANBAN_ALLOWED_ORIGINS", "http://child.local:5173, https://demo.example ")

	c, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"http://child.local:5173", "https://demo.example"}
	if !reflect.DeepEqual(c.AllowedOrigins, want) {
		t.Fatalf("AllowedOrigins = %#v, want %#v", c.AllowedOrigins, want)
	}
}

func TestLoadTrimsOptionalEnvValues(t *testing.T) {
	t.Setenv("ANBAN_MANAGER_BASE_URL", "http://localhost:8080")
	t.Setenv("ANBAN_MANAGER_API_TOKEN", "tok_123")
	t.Setenv("ANBAN_ACCESS_CODE", "demo")
	t.Setenv("ANBAN_DB_DSN", " ./data/anban.db ")
	t.Setenv("ANBAN_LISTEN_ADDR", " :8091 ")
	t.Setenv("ANBAN_ALLOWED_ORIGINS", " http://127.0.0.1:5173 , http://localhost:5173 ")

	c, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.DBDSN != "./data/anban.db" {
		t.Fatalf("DBDSN = %q, want trimmed value", c.DBDSN)
	}
	if c.ListenAddr != ":8091" {
		t.Fatalf("ListenAddr = %q, want trimmed value", c.ListenAddr)
	}
	wantOrigins := []string{"http://127.0.0.1:5173", "http://localhost:5173"}
	if !reflect.DeepEqual(c.AllowedOrigins, wantOrigins) {
		t.Fatalf("AllowedOrigins = %#v, want %#v", c.AllowedOrigins, wantOrigins)
	}
}

func TestLoadTrimsRequiredEnvValues(t *testing.T) {
	t.Setenv("ANBAN_MANAGER_BASE_URL", " http://localhost:8080/ ")
	t.Setenv("ANBAN_MANAGER_API_TOKEN", " tok_123 ")
	t.Setenv("ANBAN_ACCESS_CODE", " demo ")

	c, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.ManagerBaseURL != "http://localhost:8080/" {
		t.Fatalf("ManagerBaseURL = %q, want trimmed URL", c.ManagerBaseURL)
	}
	if c.ManagerAPIToken != "tok_123" {
		t.Fatalf("ManagerAPIToken = %q, want trimmed token", c.ManagerAPIToken)
	}
	if c.AccessCode != "demo" {
		t.Fatalf("AccessCode = %q, want trimmed access code", c.AccessCode)
	}
}

func TestLoadRejectsWhitespaceRequiredEnvValues(t *testing.T) {
	t.Setenv("ANBAN_MANAGER_BASE_URL", "   ")
	t.Setenv("ANBAN_MANAGER_API_TOKEN", "tok_123")
	t.Setenv("ANBAN_ACCESS_CODE", "demo")
	if _, err := Load(); err == nil {
		t.Fatal("expected whitespace manager URL to be rejected")
	}
}
