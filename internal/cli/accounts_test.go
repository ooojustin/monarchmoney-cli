package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thedavidweng/monarchmoney-cli/internal/audit"
	"github.com/thedavidweng/monarchmoney-cli/internal/auth"
	"github.com/thedavidweng/monarchmoney-cli/internal/testutil"
)

func TestAccountsListAPIError(t *testing.T) {
	h := newJSONCommandHarness(t, testutil.RoundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       io.NopCloser(bytes.NewReader(nil)),
		}, nil
	}))

	if err := h.execute("--json", "accounts", "list"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	out := h.Stdout.String()
	if h.ExitCode == 0 || !strings.Contains(out, `"API_ERROR"`) {
		t.Fatalf("exitCode = %d; output=%q, want API_ERROR", h.ExitCode, out)
	}
}

func TestAccountsDeleteJSON(t *testing.T) {
	h := newJSONCommandHarness(t, testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		var gqlReq struct {
			OperationName string         `json:"operationName"`
			Variables     map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(req.Body).Decode(&gqlReq); err != nil {
			t.Fatalf("Decode request error = %v", err)
		}
		if gqlReq.OperationName != "Common_DeleteAccount" {
			t.Fatalf("operation = %q, want DeleteAccount", gqlReq.OperationName)
		}
		if gqlReq.Variables["id"] != "acc-1" {
			t.Fatalf("variables = %#v, want id=acc-1", gqlReq.Variables)
		}
		return testutil.JSONResponse(`{"data":{"deleteAccount":{"ok":true}}}`), nil
	}))

	var records []*audit.Record
	h.App.Deps.WriteAudit = func(record *audit.Record) error {
		records = append(records, record)
		return nil
	}
	if err := h.execute("--json", "--confirm", "accounts", "delete", "acc-1"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if h.ExitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", h.ExitCode, h.Stdout.String())
	}
	if got := h.Stdout.String(); !strings.Contains(got, `"command":"accounts.delete"`) || !strings.Contains(got, `"status":"deleted"`) {
		t.Fatalf("output = %q, want accounts delete status", got)
	}
	if len(records) != 1 || records[0].Command != "accounts.delete" || records[0].ResourceID != "acc-1" || !records[0].Confirmed || records[0].Result != "success" {
		t.Fatalf("audit records = %#v", records)
	}
}

func TestAccountsTypesJSON(t *testing.T) {
	h := newJSONCommandHarness(t, testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		var gqlReq struct {
			OperationName string `json:"operationName"`
		}
		if err := json.NewDecoder(req.Body).Decode(&gqlReq); err != nil {
			t.Fatalf("Decode request error = %v", err)
		}
		if gqlReq.OperationName != "GetAccountTypeOptions" {
			t.Fatalf("operation = %q, want GetAccountTypeOptions", gqlReq.OperationName)
		}
		return testutil.JSONResponse(`{"data":{"accountTypes":[{"name":"bank","display":"Bank"},{"name":"credit","display":"Credit"}]}}`), nil
	}))

	if err := h.execute("--json", "accounts", "types"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	out := h.Stdout.String()
	if h.ExitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", h.ExitCode, out)
	}
	for _, want := range []string{`"command":"accounts.types"`, "bank", "credit"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %s = %q", want, out)
		}
	}
}

func TestAccountsHoldingsJSON(t *testing.T) {
	h := newJSONCommandHarness(t, testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		var gqlReq struct {
			OperationName string `json:"operationName"`
		}
		if err := json.NewDecoder(req.Body).Decode(&gqlReq); err != nil {
			t.Fatalf("Decode request error = %v", err)
		}
		if gqlReq.OperationName != "Web_GetHoldings" {
			t.Fatalf("operation = %q, want Web_GetHoldings", gqlReq.OperationName)
		}
		return testutil.JSONResponse(`{"data":{"portfolio":{"aggregateHoldings":{"edges":[{"node":{"id":"h1","quantity":2,"basis":3,"totalValue":6,"holdings":[{"id":"sub-1","quantity":2,"name":"VTI","ticker":"VTI","account":{"id":"acc-1"}}]}}]}}}}`), nil
	}))

	if err := h.execute("--json", "accounts", "holdings", "acc-1"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	out := h.Stdout.String()
	if h.ExitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", h.ExitCode, out)
	}
	for _, want := range []string{`"command":"accounts.holdings"`, `"id":"h1"`, `"total_value":6`} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %s = %q", want, out)
		}
	}
}

func TestAccountsHoldingsRequiresAccountID(t *testing.T) {
	h := newJSONCommandHarness(t, nil)
	if err := h.execute("--json", "accounts", "holdings"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if h.ExitCode != 2 || !strings.Contains(h.Stdout.String(), `"INVALID_ARGUMENTS"`) || !strings.Contains(h.Stdout.String(), "accepts 1 arg") {
		t.Fatalf("exitCode = %d; output=%q, want JSON argument error", h.ExitCode, h.Stdout.String())
	}
}

func TestAccountsHistoryResolvesDefaultRange(t *testing.T) {
	now := time.Now()
	wantFrom := now.AddDate(-1, 0, 0).Format(dateFlagLayout)
	wantTo := now.Format(dateFlagLayout)
	h := newJSONCommandHarness(t, testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		var gqlReq struct {
			OperationName string         `json:"operationName"`
			Variables     map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(req.Body).Decode(&gqlReq); err != nil {
			t.Fatalf("Decode request error = %v", err)
		}
		if gqlReq.OperationName != "GetAccountHistory" || gqlReq.Variables["startDate"] != wantFrom {
			t.Fatalf("request = %#v, want startDate %s", gqlReq, wantFrom)
		}
		return testutil.JSONResponse(`{"data":{"account":{"id":"acc-1","recentBalances":[12.345]}}}`), nil
	}))

	if err := h.execute("--json", "accounts", "history", "acc-1"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if h.ExitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", h.ExitCode, h.Stdout.String())
	}
	for _, want := range []string{`"command":"accounts.history"`, `"date":"` + wantTo + `"`, `"amount":12.35`} {
		if !strings.Contains(h.Stdout.String(), want) {
			t.Fatalf("output = %q, want %s", h.Stdout.String(), want)
		}
	}
}

func TestAccountsRefreshStatusJSON(t *testing.T) {
	h := newJSONCommandHarness(t, testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		var gqlReq struct {
			OperationName string `json:"operationName"`
		}
		if err := json.NewDecoder(req.Body).Decode(&gqlReq); err != nil {
			t.Fatalf("Decode request error = %v", err)
		}
		if gqlReq.OperationName != "ForceRefreshAccountsQuery" {
			t.Fatalf("operation = %q, want GetAccountsRefreshStatus", gqlReq.OperationName)
		}
		return testutil.JSONResponse(`{"data":{"accounts":[{"id":"a1","hasSyncInProgress":false}]}}`), nil
	}))

	if err := h.execute("--json", "accounts", "refresh-status"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	out := h.Stdout.String()
	if h.ExitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", h.ExitCode, out)
	}
	if !strings.Contains(out, `"command":"accounts.refresh-status"`) || !strings.Contains(out, `"is_complete":true`) {
		t.Fatalf("output = %q, want completed refresh status", out)
	}
}

func TestAccountsUploadHistorySendsDeviceIdentity(t *testing.T) {
	sessionPath := filepath.Join(t.TempDir(), "session.json")
	saveTestSession(t, sessionPath)
	deviceUUID, err := auth.LoadOrCreateDeviceUUID(sessionPath)
	if err != nil {
		t.Fatalf("LoadOrCreateDeviceUUID() error = %v", err)
	}
	historyPath := filepath.Join(t.TempDir(), "history.csv")
	if err := os.WriteFile(historyPath, []byte("date,amount\n2026-08-01,100\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	h := newAppTestHarness(t, func(deps *Deps) {
		deps.LoadConfig = testConfigLoader(sessionPath, "")
		deps.HTTPTransport = testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
			if got := req.Header.Get("device-uuid"); got != deviceUUID {
				t.Fatalf("device-uuid = %q, want %q", got, deviceUUID)
			}
			if req.URL.Path == "/account-balance-history/upload/" {
				if req.Method != http.MethodPost {
					t.Fatalf("method = %q, want POST", req.Method)
				}
				body, err := io.ReadAll(req.Body)
				if err != nil {
					t.Fatalf("ReadAll() error = %v", err)
				}
				if !strings.Contains(string(body), `name="files"`) || !strings.Contains(string(body), `"upload.csv":"acc-1"`) {
					t.Fatalf("upload body missing fields: %s", body)
				}
				return testutil.JSONResponse(`{"session_key":"sk-1"}`), nil
			}
			var gqlReq struct {
				OperationName string `json:"operationName"`
			}
			if err := json.NewDecoder(req.Body).Decode(&gqlReq); err != nil {
				t.Fatalf("Decode request error = %v", err)
			}
			if gqlReq.OperationName != "Web_ParseUploadBalanceHistorySession" {
				t.Fatalf("operation = %q, want Web_ParseUploadBalanceHistorySession", gqlReq.OperationName)
			}
			return testutil.JSONResponse(`{"data":{"parseBalanceHistory":{"uploadBalanceHistorySession":{"sessionKey":"sk-1","status":"completed"}}}}`), nil
		})
	})
	if err := h.execute("--json", "--confirm", "accounts", "upload-history", "acc-1", historyPath); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if h.ExitCode != 0 || !strings.Contains(h.Stdout.String(), `"status":"uploaded"`) {
		t.Fatalf("exitCode = %d; output=%q", h.ExitCode, h.Stdout.String())
	}
}

func TestAccountsRefreshEventsImplyJSON(t *testing.T) {
	h := newJSONCommandHarness(t, testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		var gqlReq struct {
			OperationName string `json:"operationName"`
		}
		if err := json.NewDecoder(req.Body).Decode(&gqlReq); err != nil {
			t.Fatalf("Decode request error = %v", err)
		}
		if gqlReq.OperationName == "Common_ForceRefreshAccountsMutation" {
			return testutil.JSONResponse(`{"data":{"requestAccountsRefresh":{"ok":true}}}`), nil
		}
		if gqlReq.OperationName == "ForceRefreshAccountsQuery" {
			return testutil.JSONResponse(`{"data":{"accounts":[{"id":"acc-1","hasSyncInProgress":false}]}}`), nil
		}
		t.Fatalf("unexpected operation %q", gqlReq.OperationName)
		return nil, nil
	}))

	if err := h.execute("--events", "--pretty", "--confirm", "accounts", "refresh", "--wait"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if h.ExitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", h.ExitCode, h.Stdout.String())
	}
	lines := strings.Split(strings.TrimSpace(h.Stdout.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("event lines = %d; output=%q", len(lines), h.Stdout.String())
	}
	for _, line := range lines {
		var envelope map[string]any
		if err := json.Unmarshal([]byte(line), &envelope); err != nil {
			t.Fatalf("event JSON error = %v; line=%q", err, line)
		}
	}
}
