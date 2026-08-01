package cli

import (
	"bytes"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thedavidweng/monarchmoney-cli/internal/testutil"
)

func TestCacheSyncFailsWhenAccountsAPIFails(t *testing.T) {
	dir := t.TempDir()
	sessionPath := filepath.Join(dir, "session.json")
	cachePath := filepath.Join(dir, "cache.sqlite")
	saveTestSession(t, sessionPath)

	h := newAppTestHarness(t, func(deps *Deps) {
		deps.LoadConfig = testConfigLoader(sessionPath, cachePath)
		deps.HTTPTransport = testutil.RoundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(bytes.NewReader(nil)),
			}, nil
		})
	})

	if err := h.execute("--json", "cache", "sync"); err != nil {
		t.Fatalf("execute() error = %v", err)
	}
	if h.ExitCode == 0 {
		t.Fatalf("exitCode = 0, want API failure; output=%q", h.Stdout.String())
	}
	if got := h.Stdout.String(); !strings.Contains(got, "failed to sync accounts") {
		t.Fatalf("output = %q, want account sync failure", got)
	}
}
