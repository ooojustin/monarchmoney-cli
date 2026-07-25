package config

import (
	"testing"

	"github.com/spf13/viper"
)

func TestConfigPrecedence(t *testing.T) {
	viper.Reset()
	t.Setenv("MONARCH_PROFILE", "env-profile")

	cfg, _ := Load()
	if cfg.Profile != "env-profile" {
		t.Fatalf("Profile = %q, want %q (env over default)", cfg.Profile, "env-profile")
	}

	viper.Set("profile", "flag-profile")
	cfg, _ = Load()
	if cfg.Profile != "flag-profile" {
		t.Fatalf("Profile = %q, want %q (flag over env)", cfg.Profile, "flag-profile")
	}
}
