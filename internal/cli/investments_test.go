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

func TestInvestments(t *testing.T) {
	t.Run("portfolio", testInvestmentsPortfolio)
	t.Run("portfolio_api_error", testInvestmentsPortfolioAPIError)
}

func testInvestmentsPortfolio(t *testing.T) {
	dir := t.TempDir()
	sessionPath := filepath.Join(dir, "session.json")
	saveTestSession(t, sessionPath)

	var gotInput map[string]any
	h := newAppTestHarness(t, func(deps *Deps) {
		deps.LoadConfig = testConfigLoader(sessionPath, "")
		deps.HTTPTransport = testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
			var gqlReq struct {
				OperationName string         `json:"operationName"`
				Variables     map[string]any `json:"variables"`
			}
			if err := json.NewDecoder(req.Body).Decode(&gqlReq); err != nil {
				t.Fatalf("Decode request error = %v", err)
			}
			if gqlReq.OperationName != "Web_GetPortfolio" {
				t.Fatalf("operation = %q, want Web_GetPortfolio", gqlReq.OperationName)
			}
			gotInput, _ = gqlReq.Variables["portfolioInput"].(map[string]any)
			return testutil.JSONResponse(`{"data":{"portfolio":{"performance":{"totalValue":1000,"totalChangePercent":0.12,"totalChangeDollars":120},"aggregateHoldings":{"edges":[{"node":{"id":"node-1","quantity":2,"basis":400,"totalValue":1000,"security":{"id":"sec-1","ticker":"ABC","name":"ABC Fund","currentPrice":500},"holdings":[]}}]}}}}`), nil
		})
	})

	if err := h.execute("--json", "investments", "portfolio", "--from", "2026-01-01", "--to", "2026-05-01", "--account-id", "acc-1"); err != nil {
		t.Fatalf("execute() error = %v", err)
	}
	if h.ExitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", h.ExitCode, h.Stdout.String())
	}
	accountIDs, _ := gotInput["accounts"].([]any)
	if gotInput["startDate"] != "2026-01-01" || gotInput["endDate"] != "2026-05-01" || len(accountIDs) != 1 || accountIDs[0] != "acc-1" {
		t.Fatalf("portfolio input = %#v, want dates and account acc-1", gotInput)
	}
	if got := h.Stdout.String(); !strings.Contains(got, `"command":"investments.portfolio"`) || !strings.Contains(got, `"total_value":1000`) || !strings.Contains(got, `"ticker":"ABC"`) {
		t.Fatalf("output = %q, want portfolio JSON", got)
	}
}

func testInvestmentsPortfolioAPIError(t *testing.T) {
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

	if err := h.execute("--json", "investments", "portfolio"); err != nil {
		t.Fatalf("execute() error = %v", err)
	}
	if h.ExitCode == 0 {
		t.Fatalf("exitCode = 0, want API failure; output=%q", h.Stdout.String())
	}
	if got := h.Stdout.String(); !strings.Contains(got, `"API_ERROR"`) {
		t.Fatalf("output = %q, want API_ERROR", got)
	}
}
