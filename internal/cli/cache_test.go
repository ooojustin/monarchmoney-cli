package cli

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thedavidweng/monarchmoney-cli/internal/testutil"
)

func TestCacheSyncPreservesAccountServiceError(t *testing.T) {
	dir := t.TempDir()
	sessionPath := filepath.Join(dir, "session.json")
	cachePath := filepath.Join(dir, "cache.sqlite")
	saveTestSession(t, sessionPath)

	h := newAppTestHarness(t, func(deps *Deps) {
		deps.LoadConfig = testConfigLoader(sessionPath, cachePath)
		deps.HTTPTransport = testutil.RoundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusUnauthorized, Body: http.NoBody}, nil
		})
	})

	if err := h.execute("--json", "cache", "sync"); err != nil {
		t.Fatalf("execute() error = %v", err)
	}
	if h.ExitCode != 3 || !strings.Contains(h.Stdout.String(), `"code":"AUTH_SESSION_EXPIRED"`) {
		t.Fatalf("exitCode = %d; output=%q, want preserved auth error", h.ExitCode, h.Stdout.String())
	}
}

func TestCacheSyncPreservesTransactionServiceError(t *testing.T) {
	dir := t.TempDir()
	sessionPath := filepath.Join(dir, "session.json")
	cachePath := filepath.Join(dir, "cache.sqlite")
	saveTestSession(t, sessionPath)
	requests := 0

	h := newAppTestHarness(t, func(deps *Deps) {
		deps.LoadConfig = testConfigLoader(sessionPath, cachePath)
		deps.HTTPTransport = testutil.RoundTripFunc(func(*http.Request) (*http.Response, error) {
			requests++
			if requests == 1 {
				return testutil.JSONResponse(`{"data":{"accounts":[{"id":"acc-1","displayName":"Checking","type":{"name":"bank"},"displayBalance":100,"updatedAt":"2026-08-08"}]}}`), nil
			}
			return &http.Response{StatusCode: http.StatusUnauthorized, Body: http.NoBody}, nil
		})
	})

	if err := h.execute("--json", "cache", "sync"); err != nil {
		t.Fatalf("execute() error = %v", err)
	}
	if h.ExitCode != 3 || !strings.Contains(h.Stdout.String(), `"code":"AUTH_SESSION_EXPIRED"`) {
		t.Fatalf("exitCode = %d; output=%q, want preserved auth error", h.ExitCode, h.Stdout.String())
	}
}
