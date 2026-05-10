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
	assert.Equal(t, DefaultAPIBaseURL, cfg.APIBaseURL)
}

func TestEndpointDerivation(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "https://api.example/graphql", GraphQLEndpoint("https://api.example/"))
	assert.Equal(t, "https://api.example/auth/login/", AuthEndpoint("https://api.example"))
	assert.Equal(t, "https://api.example/account-balance-history/upload/", AccountBalanceHistoryUploadEndpoint("https://api.example/"))
}
