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

func TestInstitutions(t *testing.T) {
	t.Run("list", testInstitutionsListJSON)
	t.Run("list_dedup", testInstitutionsListDedup)
	t.Run("list_api_error", testInstitutionsListAPIError)
}

func testInstitutionsListJSON(t *testing.T) {
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
			if gqlReq.OperationName != "Web_GetInstitutionSettings" {
				t.Fatalf("operation = %q, want GetInstitutionSettings", gqlReq.OperationName)
			}
			return testutil.JSONResponse(`{"data":{"credentials":[
				{"id":"cred-1","updateRequired":false,"disconnectedFromDataProviderAt":null,"dataProvider":"plaid","institution":{"id":"inst-1","plaidInstitutionId":"ins_1","name":"Chase","status":"active"}},
				{"id":"cred-2","updateRequired":false,"disconnectedFromDataProviderAt":null,"dataProvider":"plaid","institution":{"id":"inst-2","plaidInstitutionId":"ins_2","name":"Wells Fargo","status":"active"}}
			]}}`), nil
		})
	})

	if err := h.execute("--json", "institutions", "list"); err != nil {
		t.Fatalf("execute() error = %v", err)
	}
	if h.ExitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", h.ExitCode, h.Stdout.String())
	}
	if got := h.Stdout.String(); !strings.Contains(got, `"command":"institutions.list"`) || !strings.Contains(got, `"name":"Chase"`) || !strings.Contains(got, `"name":"Wells Fargo"`) {
		t.Fatalf("output = %q, want institutions JSON", got)
	}
}

func testInstitutionsListDedup(t *testing.T) {
	dir := t.TempDir()
	sessionPath := filepath.Join(dir, "session.json")
	saveTestSession(t, sessionPath)

	h := newAppTestHarness(t, func(deps *Deps) {
		deps.LoadConfig = testConfigLoader(sessionPath, "")
		deps.HTTPTransport = testutil.RoundTripFunc(func(*http.Request) (*http.Response, error) {
			return testutil.JSONResponse(`{"data":{"credentials":[
				{"id":"cred-1","updateRequired":false,"disconnectedFromDataProviderAt":null,"dataProvider":"plaid","institution":{"id":"inst-1","plaidInstitutionId":"ins_1","name":"Chase","status":"active"}},
				{"id":"cred-2","updateRequired":false,"disconnectedFromDataProviderAt":null,"dataProvider":"plaid","institution":{"id":"inst-1","plaidInstitutionId":"ins_1","name":"Chase","status":"active"}}
			]}}`), nil
		})
	})

	if err := h.execute("--json", "institutions", "list"); err != nil {
		t.Fatalf("execute() error = %v", err)
	}
	if h.ExitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", h.ExitCode, h.Stdout.String())
	}
	if count := strings.Count(h.Stdout.String(), `"name":"Chase"`); count != 1 {
		t.Fatalf("Chase appeared %d times, want 1; output=%q", count, h.Stdout.String())
	}
}

func testInstitutionsListAPIError(t *testing.T) {
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

	if err := h.execute("--json", "institutions", "list"); err != nil {
		t.Fatalf("execute() error = %v", err)
	}
	if h.ExitCode == 0 {
		t.Fatalf("exitCode = 0, want API failure; output=%q", h.Stdout.String())
	}
	if got := h.Stdout.String(); !strings.Contains(got, `"API_ERROR"`) {
		t.Fatalf("output = %q, want API_ERROR", got)
	}
}
