package output

import (
	"reflect"
	"testing"
	"time"

	"github.com/thedavidweng/monarchmoney-cli/internal/errors"
)

func TestSchemaVersion(t *testing.T) {
	if SchemaVersion != "2026-08-07" {
		t.Fatalf("SchemaVersion = %q, want 2026-08-07", SchemaVersion)
	}
}

func TestNewEnvelope(t *testing.T) {
	t.Run("sets ok to true and populates all fields", func(t *testing.T) {
		data := map[string]string{"key": "value"}
		env := NewEnvelope("get-accounts", "default", "1.0", "req-123", data, 150*time.Millisecond)

		if env == nil {
			t.Fatal("NewEnvelope() = nil")
		}
		if !env.OK {
			t.Error("OK = false, want true")
		}
		if !reflect.DeepEqual(env.Data, data) {
			t.Errorf("Data = %v, want %v", env.Data, data)
		}
		if env.Meta.Command != "get-accounts" {
			t.Errorf("Command = %q, want %q", env.Meta.Command, "get-accounts")
		}
		if env.Meta.Profile != "default" {
			t.Errorf("Profile = %q, want %q", env.Meta.Profile, "default")
		}
		if env.Meta.SchemaVersion != "1.0" {
			t.Errorf("SchemaVersion = %q, want %q", env.Meta.SchemaVersion, "1.0")
		}
		if env.Meta.RequestID != "req-123" {
			t.Errorf("RequestID = %q, want %q", env.Meta.RequestID, "req-123")
		}
		if env.Meta.DurationMS != 150 {
			t.Errorf("DurationMS = %d, want 150", env.Meta.DurationMS)
		}
	})

	t.Run("with nil data", func(t *testing.T) {
		env := NewEnvelope("cmd", "p", "1", "r", nil, 0)
		if env == nil {
			t.Fatal("NewEnvelope() = nil")
		}
		if !env.OK {
			t.Error("OK = false, want true")
		}
		if env.Data != nil {
			t.Errorf("Data = %v, want nil", env.Data)
		}
	})

	t.Run("with empty strings", func(t *testing.T) {
		env := NewEnvelope("", "", "", "", "something", 0)
		if env == nil {
			t.Fatal("NewEnvelope() = nil")
		}
		if !env.OK {
			t.Error("OK = false, want true")
		}
		if env.Meta.Command != "" || env.Meta.Profile != "" || env.Meta.SchemaVersion != "" || env.Meta.RequestID != "" {
			t.Errorf("meta = %+v, want empty strings", env.Meta)
		}
	})

	t.Run("converts duration to milliseconds correctly", func(t *testing.T) {
		tests := []struct {
			name     string
			dur      time.Duration
			expected int64
		}{
			{"zero", 0, 0},
			{"1ms", 1 * time.Millisecond, 1},
			{"1s", 1 * time.Second, 1000},
			{"250ms", 250 * time.Millisecond, 250},
			{"1m", 1 * time.Minute, 60000},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				env := NewEnvelope("cmd", "p", "1", "r", nil, tt.dur)
				if env.Meta.DurationMS != tt.expected {
					t.Errorf("DurationMS = %d, want %d", env.Meta.DurationMS, tt.expected)
				}
			})
		}
	})
}

func TestNewErrorEnvelope(t *testing.T) {
	t.Run("sets ok to false and populates all fields", func(t *testing.T) {
		err := errors.New(errors.AuthRequired, "login needed", errors.CatAuth, false, nil)
		env := NewErrorEnvelope("sync", "work", "1.0", err, 50*time.Millisecond)

		if env == nil {
			t.Fatal("NewErrorEnvelope() = nil")
		}
		if env.OK {
			t.Error("OK = true, want false")
		}
		if env.Error != err {
			t.Errorf("Error = %v, want %v", env.Error, err)
		}
		if env.Meta.Command != "sync" {
			t.Errorf("Command = %q, want %q", env.Meta.Command, "sync")
		}
		if env.Meta.Profile != "work" {
			t.Errorf("Profile = %q, want %q", env.Meta.Profile, "work")
		}
		if env.Meta.SchemaVersion != "1.0" {
			t.Errorf("SchemaVersion = %q, want %q", env.Meta.SchemaVersion, "1.0")
		}
		if env.Meta.DurationMS != 50 {
			t.Errorf("DurationMS = %d, want 50", env.Meta.DurationMS)
		}
	})

	t.Run("with nil error", func(t *testing.T) {
		env := NewErrorEnvelope("cmd", "p", "1", nil, 0)
		if env == nil {
			t.Fatal("NewErrorEnvelope() = nil")
		}
		if env.OK {
			t.Error("OK = true, want false")
		}
		if env.Error != nil {
			t.Errorf("Error = %v, want nil", env.Error)
		}
	})

	t.Run("converts duration to milliseconds correctly", func(t *testing.T) {
		err := errors.New(errors.NetworkTimeout, "timeout", errors.CatNetwork, true, nil)
		env := NewErrorEnvelope("cmd", "p", "1", err, 3*time.Second+250*time.Millisecond)
		if env.Meta.DurationMS != 3250 {
			t.Errorf("DurationMS = %d, want 3250", env.Meta.DurationMS)
		}
	})
}
