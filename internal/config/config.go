package config

import (
	"strings"
	"time"

	"github.com/spf13/viper"
)

const DefaultAPIBaseURL = "https://api.monarch.com"

// Config represents the application configuration.
type Config struct {
	Profile     string        `mapstructure:"profile"`
	APIBaseURL  string        `mapstructure:"api_base_url"`
	Output      string        `mapstructure:"output"`
	Timeout     time.Duration `mapstructure:"timeout"`
	ReadOnly    bool          `mapstructure:"read_only"`
	SessionPath string        `mapstructure:"session_path"`
	AuditLog    bool          `mapstructure:"audit_log"`
	CachePath   string        `mapstructure:"cache_path"`
}

// GraphQLEndpoint derives the Monarch GraphQL endpoint from the configured API
// base URL.
func GraphQLEndpoint(baseURL string) string {
	return endpoint(baseURL, "/graphql")
}

// AuthEndpoint derives the Monarch login endpoint from the configured API base
// URL.
func AuthEndpoint(baseURL string) string {
	return endpoint(baseURL, "/auth/login/")
}

// AccountBalanceHistoryUploadEndpoint derives the balance-history upload
// endpoint from the configured API base URL.
func AccountBalanceHistoryUploadEndpoint(baseURL string) string {
	return endpoint(baseURL, "/account-balance-history/upload/")
}

func endpoint(baseURL, path string) string {
	return strings.TrimRight(baseURL, "/") + path
}

// SetDefaults applies the application's default configuration values onto v.
// Callers (cli.DefaultDeps, tests) invoke this once on a fresh *viper.Viper
// so that subsequent v.GetX calls see sensible defaults.
func SetDefaults(v *viper.Viper) {
	v.SetDefault("profile", "default")
	v.SetDefault("api_base_url", DefaultAPIBaseURL)
	v.SetDefault("timeout", 30*time.Second)
	v.SetDefault("read_only", false)
	v.SetDefault("session_path", DefaultSessionPath())
	v.SetDefault("audit_log", true)
	v.SetDefault("cache_path", DefaultCachePath())
}

// Load unmarshals v into a Config. Defaults must be set on v beforehand
// (via SetDefaults). v is per-App; callers pass app.Deps.Viper.
func Load(v *viper.Viper) (*Config, error) {
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
