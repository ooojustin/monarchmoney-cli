package config

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestConfigPrecedence(t *testing.T) {
	t.Setenv("MONARCH_PROFILE", "env-profile")
	t.Setenv("MONARCH_API_BASE_URL", "https://api.example")

	// Precedence: CLI overrides (viper.Set) > Env vars > Config file > Defaults.
	v := viper.New()
	v.SetEnvPrefix("MONARCH")
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	v.AutomaticEnv()
	SetDefaults(v)

	// Env beats default.
	cfg, err := Load(v)
	assert.NoError(t, err)
	assert.Equal(t, "env-profile", cfg.Profile)
	assert.Equal(t, "https://api.example", cfg.APIBaseURL)

	// Explicit override beats env.
	v.Set("profile", "flag-profile")
	cfg, err = Load(v)
	assert.NoError(t, err)
	assert.Equal(t, "flag-profile", cfg.Profile)
}
