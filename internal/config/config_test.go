package config

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestLoadDefaults(t *testing.T) {
	t.Parallel()
	v := viper.New()
	SetDefaults(v)
	cfg, err := Load(v)
	assert.NoError(t, err)
	assert.Equal(t, "default", cfg.Profile)
	assert.Equal(t, "https://api.monarch.com/graphql", cfg.APIEndpoint)
}
