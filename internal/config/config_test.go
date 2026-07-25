package config

import (
	"testing"

	"github.com/spf13/viper"
)

func TestLoadDefaults(t *testing.T) {
	viper.Reset()
	cfg, err := Load()
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
