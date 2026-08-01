package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/thedavidweng/monarchmoney-cli/internal/audit"
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
	err := h.execute("--json", "accounts", "holdings")
	if err == nil || !strings.Contains(err.Error(), "accepts 1 arg") {
		t.Fatalf("Execute() error = %v, want exact-args failure", err)
	}
	if h.ExitCode != 0 {
		t.Fatalf("exitCode = %d, want Cobra validation without handler exit", h.ExitCode)
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
