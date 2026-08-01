package config

import (
	"path/filepath"
	"testing"
)

func TestDefaultPathsUseDefaultDir(t *testing.T) {
	dir := DefaultDir()
	tests := map[string]struct {
		got  string
		want string
	}{
		"config":  {got: DefaultConfigPath(), want: filepath.Join(dir, "config.yaml")},
		"session": {got: DefaultSessionPath(), want: filepath.Join(dir, "session.json")},
		"cache":   {got: DefaultCachePath(), want: filepath.Join(dir, "cache", "monarch.sqlite")},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("path = %q, want %q", tt.got, tt.want)
			}
		})
	}
}
