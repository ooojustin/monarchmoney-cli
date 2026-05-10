package cli

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/spf13/cobra"
	"github.com/thedavidweng/monarchmoney-cli/internal/auth"
)

// roundTripFunc adapts a function to the http.RoundTripper interface so
// tests can inject HTTP behavior via app.Deps.HTTPTransport.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

// newTestApp constructs an App wired for testing. It returns the App, a
// stdout capture buffer, and a pointer to the captured exit code.
//
// Each test gets its own App with isolated Flags state and an injected
// HTTPTransport, so tests are free to call t.Parallel(). The remaining
// exceptions are tests that touch the global viper instance (viper.Set /
// viper.Reset). Those still race with each other and should not call
// t.Parallel() until viper is per-App as well.
func newTestApp(t *testing.T, sessionPath string) (*App, *bytes.Buffer, *int) {
	t.Helper()

	var buf bytes.Buffer
	exitCode := 0

	deps := DefaultDeps()
	deps.SessionPath = func() string { return sessionPath }
	deps.Stdout = &buf
	deps.Stderr = &buf
	deps.Exit = func(code int) { exitCode = code }

	app := New(deps)

	// New(deps) calls buildRoot which registers PersistentFlags via cobra.BoolVar,
	// resetting App.Flags fields to flag defaults. Apply test-mode values
	// AFTER construction.
	app.Flags.JSONMode = true
	app.Flags.Profile = "default"

	return app, &buf, &exitCode
}

// saveTestSession persists a default session at sessionPath for tests that
// need an authenticated state.
func saveTestSession(t *testing.T, sessionPath string) {
	t.Helper()
	store := auth.NewStore(sessionPath)
	if err := store.Save(&auth.Session{
		Profile: "default",
		Email:   "a@example.com",
		Token:   "token-123",
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
}

// jsonHTTPResponse builds a 200 OK *http.Response with the given JSON body.
func jsonHTTPResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: 200,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytesReader(body)),
	}
}

func bytesReader(s string) *bytes.Reader { return bytes.NewReader([]byte(s)) }

// runCommand finds the command at args[0..], applies optional flag setters,
// gives it a context.Background, and invokes its Run handler. It returns the
// found command for tests that want to inspect its post-run state.
func runCommand(t *testing.T, app *App, path []string, setFlags func(*cobra.Command)) *cobra.Command {
	t.Helper()
	cmd, _, err := app.Root.Find(path)
	if err != nil {
		t.Fatalf("Find %v = %v", path, err)
	}
	cmd.SetContext(context.Background())
	if setFlags != nil {
		setFlags(cmd)
	}
	cmd.Run(cmd, nil)
	return cmd
}
