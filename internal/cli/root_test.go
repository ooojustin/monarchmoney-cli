package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thedavidweng/monarchmoney-cli/internal/auth"
)

// saveTestSession writes a valid session file for test commands that require auth.
func saveTestSession(t *testing.T, sessionPath string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(sessionPath), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	s := auth.Session{
		Profile:   "default",
		Email:     "test@example.com",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Token:     "test-token",
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent() error = %v", err)
	}
	if err := os.WriteFile(sessionPath, data, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func TestWriteVersion(t *testing.T) {
	t.Run("plain text", func(t *testing.T) {
		var buf bytes.Buffer
		if err := writeVersion(&buf, "default", false, false, "request-id", time.Second); err != nil {
			t.Fatalf("writeVersion() error = %v", err)
		}
		got := buf.String()
		if !strings.Contains(got, "monarch version ") {
			t.Fatalf("writeVersion() = %q", got)
		}
	})

	t.Run("compact json", func(t *testing.T) {
		var buf bytes.Buffer
		if err := writeVersion(&buf, "default", true, false, "request-id", time.Second); err != nil {
			t.Fatalf("writeVersion() error = %v", err)
		}
		var got struct {
			OK   bool `json:"ok"`
			Data struct {
				Version string `json:"version"`
				Commit  string `json:"commit"`
				Date    string `json:"date"`
			} `json:"data"`
			Meta struct {
				Command  string   `json:"command"`
				Profile  string   `json:"profile"`
				Warnings []string `json:"warnings"`
			} `json:"meta"`
		}
		if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &got); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		if !got.OK || got.Data.Version == "" || got.Meta.Command != "version" || got.Meta.Profile != "default" {
			t.Fatalf("writeVersion() = %#v", got)
		}
	})

	t.Run("pretty json", func(t *testing.T) {
		var buf bytes.Buffer
		if err := writeVersion(&buf, "default", true, true, "request-id", time.Second); err != nil {
			t.Fatalf("writeVersion() error = %v", err)
		}
		got := buf.String()
		if !strings.Contains(got, "\n  \"ok\"") {
			t.Fatalf("writeVersion() = %q", got)
		}
		var decoded map[string]any
		if err := json.Unmarshal([]byte(got), &decoded); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
	})
}
