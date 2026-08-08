package auth

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/google/uuid"
)

func TestLoadOrCreateDeviceUUID(t *testing.T) {
	sessionPath := filepath.Join(t.TempDir(), "session.json")
	if got, err := LoadDeviceUUID(sessionPath); err != nil || got != "" {
		t.Fatalf("LoadDeviceUUID() = %q, %v", got, err)
	}

	first, err := LoadOrCreateDeviceUUID(sessionPath)
	if err != nil {
		t.Fatalf("LoadOrCreateDeviceUUID() error = %v", err)
	}
	if _, err := uuid.Parse(first); err != nil {
		t.Fatalf("device UUID = %q: %v", first, err)
	}
	second, err := LoadOrCreateDeviceUUID(sessionPath)
	if err != nil || second != first {
		t.Fatalf("second LoadOrCreateDeviceUUID() = %q, %v; want %q", second, err, first)
	}

	if runtime.GOOS != "windows" {
		info, err := os.Stat(DevicePath(sessionPath))
		if err != nil {
			t.Fatalf("Stat() error = %v", err)
		}
		if got := info.Mode().Perm(); got != 0o600 {
			t.Fatalf("permissions = %v, want 0600", got)
		}
	}
}

func TestLoadOrCreateDeviceUUIDConcurrent(t *testing.T) {
	sessionPath := filepath.Join(t.TempDir(), "session.json")
	results := make(chan string, 2)
	errors := make(chan error, 2)
	var group sync.WaitGroup
	for range 2 {
		group.Add(1)
		go func() {
			defer group.Done()
			deviceUUID, err := LoadOrCreateDeviceUUID(sessionPath)
			results <- deviceUUID
			errors <- err
		}()
	}
	group.Wait()
	close(results)
	close(errors)

	for err := range errors {
		if err != nil {
			t.Fatalf("LoadOrCreateDeviceUUID() error = %v", err)
		}
	}
	want := ""
	for result := range results {
		if want == "" {
			want = result
		} else if result != want {
			t.Fatalf("device UUID = %q, want %q", result, want)
		}
	}
}

func TestLoadDeviceUUIDRejectsInvalidValue(t *testing.T) {
	sessionPath := filepath.Join(t.TempDir(), "session.json")
	if err := os.WriteFile(DevicePath(sessionPath), []byte("invalid\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, err := LoadDeviceUUID(sessionPath); err == nil {
		t.Fatal("LoadDeviceUUID() error = nil")
	}
}
