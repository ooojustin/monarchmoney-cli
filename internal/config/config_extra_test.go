package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultAuditDir(t *testing.T) {
	if got, want := DefaultAuditDir(), filepath.Join(defaultDir(), "audit"); got != want {
		t.Fatalf("DefaultAuditDir() = %q, want %q", got, want)
	}
}

func TestLoadIncludesAllDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "config.yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Profile != "default" || cfg.APIEndpoint != "https://api.monarch.com/graphql" || cfg.Timeout != 30*time.Second || cfg.ReadOnly || cfg.SessionPath == "" || !cfg.AuditLog || cfg.CachePath == "" {
		t.Fatalf("Load() returned unexpected config: %#v", cfg)
	}
}

func TestLoadReturnsFileErrorButUsableConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("- not\n- a\n- mapping\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := Load(path)
	if err == nil {
		t.Fatal("Load() error = nil, want parse failure")
	}
	if cfg == nil {
		t.Fatal("Load() cfg = nil, want defaults even on parse error")
	}
	if cfg.APIEndpoint != "https://api.monarch.com/graphql" {
		t.Fatalf("cfg.APIEndpoint = %q, want default (env/defaults still applied)", cfg.APIEndpoint)
	}
}

func TestLoadAppliesFileAndEnvValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	body := "api_endpoint: https://file.example/graphql\ntimeout: 45s\ncache_path: /tmp/file-cache.sqlite\naudit_log: false\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.APIEndpoint != "https://file.example/graphql" {
		t.Errorf("APIEndpoint = %q, want file value", cfg.APIEndpoint)
	}
	if cfg.Timeout != 45*time.Second {
		t.Errorf("Timeout = %v, want 45s from file", cfg.Timeout)
	}
	if cfg.CachePath != "/tmp/file-cache.sqlite" {
		t.Errorf("CachePath = %q, want file value", cfg.CachePath)
	}
	if cfg.AuditLog {
		t.Errorf("AuditLog = true, want false from file")
	}

	t.Setenv("MONARCH_TIMEOUT", "90s")
	t.Setenv("MONARCH_CACHE_PATH", "/tmp/env-cache.sqlite")
	cfg, err = Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Timeout != 90*time.Second {
		t.Errorf("Timeout = %v, want 90s from env override", cfg.Timeout)
	}
	if cfg.CachePath != "/tmp/env-cache.sqlite" {
		t.Errorf("CachePath = %q, want env override", cfg.CachePath)
	}
}
