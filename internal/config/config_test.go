package config

import (
	"path/filepath"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "config.yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Profile != "default" {
		t.Errorf("Profile = %q, want %q", cfg.Profile, "default")
	}
	if want := "https://api.monarch.com/graphql"; cfg.APIEndpoint != want {
		t.Errorf("APIEndpoint = %q, want %q", cfg.APIEndpoint, want)
	}
}
