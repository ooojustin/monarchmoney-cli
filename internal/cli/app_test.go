package cli

import (
	"bytes"
	"io"
	"path/filepath"
	"testing"
	"time"

	"github.com/thedavidweng/monarchmoney-cli/internal/config"
)

func newTestApp(t *testing.T) (*App, *bytes.Buffer) {
	t.Helper()
	var out bytes.Buffer
	app := New(Deps{
		LoadConfig: func(string) (*config.Config, error) {
			return &config.Config{Profile: "default", Timeout: 30 * time.Second, SessionPath: config.DefaultSessionPath(), AuditLog: true}, nil
		},
		Getenv:       func(string) string { return "" },
		NewRequestID: func() string { return "request-id" },
		Stdout:       &out,
		Stderr:       io.Discard,
		Stdin:        bytes.NewReader(nil),
		Exit:         func(int) {},
	})
	return app, &out
}

func TestNewBuildsIndependentRoots(t *testing.T) {
	first, _ := newTestApp(t)
	second, _ := newTestApp(t)
	if first.Root == second.Root {
		t.Fatal("New() returned a shared root command")
	}
	first.Flags.Profile = "changed"
	if second.Flags.Profile == "changed" {
		t.Fatal("New() returned shared flag state")
	}
}

func TestAppConfigPrecedence(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	app, _ := newTestApp(t)
	app.Deps.Getenv = func(key string) string {
		if key == "MONARCH_CONFIG" {
			return configPath
		}
		return ""
	}
	app.Deps.LoadConfig = func(path string) (*config.Config, error) {
		if path != configPath {
			t.Fatalf("LoadConfig(%q), want %q", path, configPath)
		}
		return &config.Config{Profile: "config-profile", Timeout: time.Minute, ReadOnly: true, SessionPath: "/tmp/session.json", AuditLog: true}, nil
	}
	app.Root.SetArgs([]string{"version"})
	if err := app.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if app.Flags.Profile != "config-profile" || app.Flags.Timeout != time.Minute || !app.Flags.ReadOnly {
		t.Fatalf("Flags = %#v", app.Flags)
	}
	if app.Deps.SessionPath() != "/tmp/session.json" {
		t.Fatalf("SessionPath() = %q", app.Deps.SessionPath())
	}
}

func TestAppFlagsOverrideConfig(t *testing.T) {
	app, _ := newTestApp(t)
	app.Deps.LoadConfig = func(string) (*config.Config, error) {
		return &config.Config{Profile: "config-profile", Timeout: time.Minute, ReadOnly: true, AuditLog: true}, nil
	}
	app.Root.SetArgs([]string{"--profile", "flag-profile", "--timeout", "2m", "--read-only=false", "version"})
	if err := app.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if app.Flags.Profile != "flag-profile" || app.Flags.Timeout != 2*time.Minute || app.Flags.ReadOnly {
		t.Fatalf("Flags = %#v", app.Flags)
	}
}
