package cli

import (
	"bytes"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/thedavidweng/monarchmoney-cli/internal/audit"
	"github.com/thedavidweng/monarchmoney-cli/internal/config"
)

type appTestHarness struct {
	App      *App
	Stdout   *bytes.Buffer
	Stderr   *bytes.Buffer
	ExitCode int
}

func newAppTestHarness(t *testing.T, configure func(*Deps)) *appTestHarness {
	t.Helper()

	h := &appTestHarness{
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
	}
	deps := Deps{
		LoadConfig:   testConfigLoader(config.DefaultSessionPath(), ""),
		Getenv:       func(string) string { return "" },
		NewRequestID: func() string { return "request-id" },
		Stdout:       h.Stdout,
		Stderr:       h.Stderr,
		Exit: func(code int) {
			h.ExitCode = code
		},
		WriteAudit: func(*audit.Record) error { return nil },
	}
	if configure != nil {
		configure(&deps)
	}
	h.App = New(deps)
	return h
}

func TestAppHarnessIgnoresAmbientEnvironment(t *testing.T) {
	t.Setenv("MONARCH_JSON", "true")
	t.Setenv("MONARCH_PRETTY", "true")
	t.Setenv("MONARCH_READONLY", "true")
	t.Setenv("MONARCH_DRY_RUN", "true")
	t.Setenv("MONARCH_PROFILE", "ambient")

	h := newAppTestHarness(t, nil)
	if err := h.execute("version"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if h.App.Flags.JSONMode || h.App.Flags.Pretty || h.App.Flags.ReadOnly || h.App.Flags.DryRun {
		t.Fatalf("flags inherited ambient environment: %#v", h.App.Flags)
	}
	if h.App.Flags.Profile != "default" {
		t.Fatalf("profile = %q, want default", h.App.Flags.Profile)
	}
}

func (h *appTestHarness) execute(args ...string) error {
	h.App.Deps.Args = args
	h.App.Root.SetArgs(args)
	return h.App.Execute()
}

func newJSONCommandHarness(t *testing.T, transport http.RoundTripper) *appTestHarness {
	t.Helper()

	sessionPath := filepath.Join(t.TempDir(), "session.json")
	saveTestSession(t, sessionPath)
	return newAppTestHarness(t, func(deps *Deps) {
		deps.LoadConfig = testConfigLoader(sessionPath, "")
		deps.HTTPTransport = transport
	})
}

func testConfigLoader(sessionPath, cachePath string) func(string) (*config.Config, error) {
	return func(string) (*config.Config, error) {
		cfg := config.Default()
		cfg.APIEndpoint = "https://example.invalid/graphql"
		cfg.Timeout = time.Second
		cfg.SessionPath = sessionPath
		if cachePath != "" {
			cfg.CachePath = cachePath
		}
		return cfg, nil
	}
}
