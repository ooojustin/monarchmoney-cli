package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thedavidweng/monarchmoney-cli/internal/testutil"
)

func TestGoals(t *testing.T) {
	t.Run("list", testGoalsListJSON)
	t.Run("list_api_error", testGoalsListAPIError)
}

func testGoalsListJSON(t *testing.T) {
	dir := t.TempDir()
	sessionPath := filepath.Join(dir, "session.json")
	saveTestSession(t, sessionPath)

	h := newAppTestHarness(t, func(deps *Deps) {
		deps.LoadConfig = testConfigLoader(sessionPath, "")
		deps.HTTPTransport = testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
			var gqlReq struct {
				OperationName string `json:"operationName"`
			}
			if err := json.NewDecoder(req.Body).Decode(&gqlReq); err != nil {
				t.Fatalf("Decode request error = %v", err)
			}
			if gqlReq.OperationName != "Common_SavingsGoals" {
				t.Fatalf("operation = %q, want Web_GoalsV2", gqlReq.OperationName)
			}
			return testutil.JSONResponse(`{"data":{"savingsGoals":[{"id":"goal-1","name":"Vacation"},{"id":"goal-2","name":"Emergency Fund"}]}}`), nil
		})
	})

	if err := h.execute("--json", "goals", "list"); err != nil {
		t.Fatalf("execute() error = %v", err)
	}
	if h.ExitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", h.ExitCode, h.Stdout.String())
	}
	if got := h.Stdout.String(); !strings.Contains(got, `"command":"goals.list"`) || !strings.Contains(got, `"Vacation"`) || !strings.Contains(got, `"Emergency Fund"`) {
		t.Fatalf("output = %q, want goals JSON", got)
	}
}

func testGoalsListAPIError(t *testing.T) {
	dir := t.TempDir()
	sessionPath := filepath.Join(dir, "session.json")
	saveTestSession(t, sessionPath)

	h := newAppTestHarness(t, func(deps *Deps) {
		deps.LoadConfig = testConfigLoader(sessionPath, "")
		deps.HTTPTransport = testutil.RoundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(bytes.NewReader(nil)),
			}, nil
		})
	})

	if err := h.execute("--json", "goals", "list"); err != nil {
		t.Fatalf("execute() error = %v", err)
	}
	if h.ExitCode == 0 {
		t.Fatalf("exitCode = 0, want API failure; output=%q", h.Stdout.String())
	}
	if got := h.Stdout.String(); !strings.Contains(got, `"API_ERROR"`) {
		t.Fatalf("output = %q, want API_ERROR", got)
	}
}
