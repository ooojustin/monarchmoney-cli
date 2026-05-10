package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestDefaultAuditDir(t *testing.T) {
	t.Parallel()
	home, _ := os.UserHomeDir()
	if got, want := DefaultAuditDir(), filepath.Join(home, ".monarchmoney-cli", "audit"); got != want {
		t.Fatalf("DefaultAuditDir() = %q, want %q", got, want)
	}
}

func TestLoadIncludesAllDefaults(t *testing.T) {
	t.Parallel()
	v := viper.New()
	SetDefaults(v)
	cfg, err := Load(v)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Profile != "default" ||
		cfg.APIBaseURL != DefaultAPIBaseURL ||
		cfg.Timeout != 30*time.Second ||
		cfg.ReadOnly ||
		cfg.SessionPath == "" ||
		!cfg.AuditLog ||
		cfg.CachePath == "" {
		t.Fatalf("Load() returned unexpected config: %#v", cfg)
	}
}
