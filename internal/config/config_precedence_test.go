package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigPrecedence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("profile: file-profile\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// File over default.
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Profile != "file-profile" {
		t.Fatalf("Profile = %q, want %q (file over default)", cfg.Profile, "file-profile")
	}

	// Env over file.
	t.Setenv("MONARCH_PROFILE", "env-profile")
	cfg, err = Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Profile != "env-profile" {
		t.Fatalf("Profile = %q, want %q (env over file)", cfg.Profile, "env-profile")
	}
}
