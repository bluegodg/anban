package config

import "testing"

func TestLoadFailsWhenManagerTokenMissing(t *testing.T) {
	t.Setenv("ANBAN_MANAGER_BASE_URL", "http://localhost:8080")
	t.Setenv("ANBAN_MANAGER_API_TOKEN", "")
	t.Setenv("ANBAN_ACCESS_CODE", "demo")
	if _, err := Load(); err == nil {
		t.Fatal("expected error when ANBAN_MANAGER_API_TOKEN missing, got nil")
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
}
