package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thedavidweng/monarchmoney-cli/internal/auth"
)

// TestAuthLoginWithFlags exercises the login command with email/password
// supplied via flags. Authenticate is overridden via Deps to avoid the
// network. Verifies the new App+Deps architecture: session is saved to the
// test sessionPath via Deps.NewStore.
func TestAuthLoginWithFlags(t *testing.T) {
	t.Parallel()
	sessionPath := filepath.Join(t.TempDir(), "session.json")
	app, buf, exitCode := newTestApp(t, sessionPath)

	app.Deps.Authenticate = func(email, password, mfaCode, mfaSecret string) (*auth.Session, error) {
		return &auth.Session{
			Email:     email,
			Token:     "token-123",
			CreatedAt: time.Date(2026, 5, 9, 10, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2026, 5, 9, 10, 0, 0, 0, time.UTC),
		}, nil
	}

	cmd, _, err := app.Root.Find([]string{"auth", "login"})
	if err != nil {
		t.Fatalf("Find login = %v", err)
	}
	_ = cmd.Flags().Set("email", "a@example.com")
	_ = cmd.Flags().Set("password", "secret")
	cmd.SetContext(context.Background())
	cmd.Run(cmd, nil)

	if *exitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", *exitCode, buf.String())
	}

	var env struct {
		OK   bool `json:"ok"`
		Data struct {
			Email       string `json:"email"`
			SessionPath string `json:"session_path"`
		} `json:"data"`
	}
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal output = %v; out=%q", err, buf.String())
	}
	if !env.OK || env.Data.Email != "a@example.com" || env.Data.SessionPath != sessionPath {
		t.Fatalf("login env = %#v", env)
	}

	loaded, err := auth.NewStore(sessionPath).Load()
	if err != nil {
		t.Fatalf("Load saved session = %v", err)
	}
	if loaded.Email != "a@example.com" || loaded.Token != "token-123" {
		t.Fatalf("loaded session = %#v", loaded)
	}
}

// TestAuthStatusWithValidSession exercises the auth status command when the
// session file exists and the GraphQL backend returns a successful identity.
// Uses http.DefaultTransport injection because graphql.NewClient consumes
// the default transport.
func TestAuthStatusWithValidSession(t *testing.T) {
	sessionPath := filepath.Join(t.TempDir(), "session.json")
	app, buf, exitCode := newTestApp(t, sessionPath)
	saveTestSession(t, sessionPath)

	oldTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = oldTransport })
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return jsonHTTPResponse(`{"data":{"me":{"email":"a@example.com"}}}`), nil
	})

	cmd, _, err := app.Root.Find([]string{"auth", "status"})
	if err != nil {
		t.Fatalf("Find status = %v", err)
	}
	cmd.SetContext(context.Background())
	cmd.Run(cmd, nil)

	if *exitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", *exitCode, buf.String())
	}
	if !strings.Contains(buf.String(), `"authenticated":true`) {
		t.Fatalf("output = %q", buf.String())
	}
}

// TestAuthStatusMissingSession verifies AUTH_REQUIRED is returned when no
// session file exists at the configured path.
func TestAuthStatusMissingSession(t *testing.T) {
	t.Parallel()
	sessionPath := filepath.Join(t.TempDir(), "missing.json")
	app, buf, exitCode := newTestApp(t, sessionPath)

	cmd, _, _ := app.Root.Find([]string{"auth", "status"})
	cmd.SetContext(context.Background())
	cmd.Run(cmd, nil)

	if *exitCode != 3 {
		t.Fatalf("exitCode = %d, want 3 (AUTH_REQUIRED); output=%q", *exitCode, buf.String())
	}
	if !strings.Contains(buf.String(), "AUTH_REQUIRED") {
		t.Fatalf("output = %q", buf.String())
	}
}
