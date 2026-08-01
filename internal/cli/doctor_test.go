package cli

import (
	"bytes"
	stderrors "errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thedavidweng/monarchmoney-cli/internal/auth"
	"github.com/thedavidweng/monarchmoney-cli/internal/config"
	"github.com/thedavidweng/monarchmoney-cli/internal/testutil"
)

func newTestDoctorApp(t *testing.T, configPath, sessionPath string, configErr error, transport http.RoundTripper) (*App, *bytes.Buffer) {
	t.Helper()

	var out bytes.Buffer
	app := New(Deps{
		LoadConfig: func(path string) (*config.Config, error) {
			if path != configPath {
				t.Fatalf("LoadConfig(%q), want %q", path, configPath)
			}
			cfg := config.Default()
			cfg.APIEndpoint = "https://example.invalid/graphql"
			cfg.SessionPath = sessionPath
			cfg.Timeout = time.Second
			return cfg, configErr
		},
		Getenv:        func(string) string { return "" },
		NewRequestID:  func() string { return "request-id" },
		HTTPTransport: transport,
		Stdout:        &out,
		Stderr:        io.Discard,
		Exit:          func(int) {},
	})
	return app, &out
}

func TestAppRootRegistersDoctor(t *testing.T) {
	app, _ := newTestApp(t)
	command, _, err := app.Root.Find([]string{"doctor"})
	if err != nil || command == nil || command.GroupID != "utility" {
		t.Fatalf("Find(doctor) = %#v, %v", command, err)
	}
	if command.Flags().Lookup("connect") == nil {
		t.Fatal("doctor missing --connect flag")
	}
}

func TestAppDoctorReportsSelectedInvalidConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	sessionPath := filepath.Join(dir, "session.json")
	app, out := newTestDoctorApp(t, configPath, sessionPath, stderrors.New("malformed config"), nil)
	app.Root.SetArgs([]string{"--config", configPath, "doctor"})
	if err := app.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	for _, want := range []string{"Monarch Money CLI Doctor", configPath, "Valid: false", sessionPath} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output = %q, want %q", out.String(), want)
		}
	}
}

func TestAppDoctorUsesDefaultSessionPathWhenConfigIsEmpty(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)
	wantSessionPath := config.DefaultSessionPath()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	app, out := newTestDoctorApp(t, configPath, "", nil, nil)
	app.Root.SetArgs([]string{"--config", configPath, "doctor"})
	if err := app.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out.String(), wantSessionPath) {
		t.Fatalf("output = %q, want default session path %q", out.String(), wantSessionPath)
	}
}

func TestAppDoctorConnectUsesConfiguredService(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	sessionPath := filepath.Join(dir, "session.json")
	if err := auth.NewStore(sessionPath).Save(&auth.Session{Token: "token-123"}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	transport := testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "https://example.invalid/graphql" || req.Header.Get("Authorization") != "Token token-123" {
			return nil, fmt.Errorf("request URL=%q Authorization=%q", req.URL, req.Header.Get("Authorization"))
		}
		return testutil.JSONResponse(`{"data":{"me":{"email":"a@example.com"}}}`), nil
	})
	app, out := newTestDoctorApp(t, configPath, sessionPath, nil, transport)
	app.Root.SetArgs([]string{"--config", configPath, "--json", "doctor", "--connect"})
	if err := app.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got := out.String(); !strings.Contains(got, `"api_reachable":true`) || !strings.Contains(got, `"valid":true`) || !strings.Contains(got, `"request_id":"request-id"`) {
		t.Fatalf("output = %q", got)
	}
}
