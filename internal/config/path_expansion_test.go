package config

import (
	"path/filepath"
	"testing"
)

func TestPathExpansion(t *testing.T) {
	expected := filepath.Join(defaultDir(), "config.yaml")
	if got := DefaultConfigPath(); got != expected {
		t.Fatalf("DefaultConfigPath() = %q, want %q", got, expected)
	}
}
