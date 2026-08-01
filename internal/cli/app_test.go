package cli

import (
	"bytes"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thedavidweng/monarchmoney-cli/internal/config"
	"github.com/thedavidweng/monarchmoney-cli/internal/testutil"
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

func TestAppRootRegistersSimpleReadCommands(t *testing.T) {
	app, _ := newTestApp(t)
	tests := []struct {
		args []string
		name string
	}{
		{args: []string{"credit", "history"}, name: "history"},
		{args: []string{"subscription", "show"}, name: "show"},
		{args: []string{"institutions", "list"}, name: "list"},
		{args: []string{"goals", "list"}, name: "list"},
		{args: []string{"goals", "budgets"}, name: "budgets"},
	}

	for _, tt := range tests {
		cmd, _, err := app.Root.Find(tt.args)
		if err != nil {
			t.Fatalf("Find(%v) error = %v", tt.args, err)
		}
		if cmd == nil || cmd.Name() != tt.name {
			t.Fatalf("Find(%v) = %#v", tt.args, cmd)
		}
	}

	for _, name := range []string{"credit", "subscription", "institutions", "goals"} {
		cmd, _, err := app.Root.Find([]string{name})
		if err != nil {
			t.Fatalf("Find(%s) error = %v", name, err)
		}
		if cmd.GroupID != "core" {
			t.Fatalf("%s GroupID = %q, want core", name, cmd.GroupID)
		}
	}
}

func TestAppCreditHistoryUsesInjectedServiceDeps(t *testing.T) {
	dir := t.TempDir()
	sessionPath := filepath.Join(dir, "session.json")
	saveTestSession(t, sessionPath)

	var out bytes.Buffer
	var gotAuth string
	app := New(Deps{
		LoadConfig: func(string) (*config.Config, error) {
			return &config.Config{
				Profile:     "default",
				APIEndpoint: "https://example.invalid/graphql",
				Timeout:     time.Second,
				SessionPath: sessionPath,
				AuditLog:    true,
			}, nil
		},
		Getenv:       func(string) string { return "" },
		NewRequestID: func() string { return "request-id" },
		Stdout:       &out,
		Stderr:       io.Discard,
		Exit:         func(int) {},
		HTTPTransport: testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
			gotAuth = req.Header.Get("Authorization")
			return testutil.JSONResponse(`{"data":{"creditScoreSnapshots":[{"reportedDate":"2026-05-01","score":790,"user":{"id":"u-1"}}]}}`), nil
		}),
	})

	app.Root.SetArgs([]string{"--json", "credit", "history"})
	if err := app.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if gotAuth != "Token test-token" {
		t.Fatalf("Authorization header = %q, want Token test-token", gotAuth)
	}
	if got := out.String(); !strings.Contains(got, `"command":"credit.history"`) || !strings.Contains(got, `"score":790`) {
		t.Fatalf("output = %q, want credit history JSON", got)
	}
}
