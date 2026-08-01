package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thedavidweng/monarchmoney-cli/internal/testutil"
)

func TestAnalyzeAnomaliesJSON(t *testing.T) {
	var sawHistoryWindow bool
	h := newJSONCommandHarness(t, testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		var gqlReq struct {
			OperationName string         `json:"operationName"`
			Variables     map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(req.Body).Decode(&gqlReq); err != nil {
			return nil, err
		}
		if gqlReq.OperationName != "GetTransactionsList" {
			return nil, fmt.Errorf("operation = %q", gqlReq.OperationName)
		}
		filters, _ := gqlReq.Variables["filters"].(map[string]any)
		sawHistoryWindow = filters["startDate"] == "2025-11-01" && filters["endDate"] == "2026-05-31"
		return testutil.JSONResponse(`{"data":{"allTransactions":{"results":[
			{"id":"h1","date":"2025-11-15","amount":-100,"merchant":{"name":"Cafe"},"category":{"name":"Dining"},"account":{"id":"acc"}},
			{"id":"h2","date":"2025-12-15","amount":-100,"merchant":{"name":"Cafe"},"category":{"name":"Dining"},"account":{"id":"acc"}},
			{"id":"h3","date":"2026-01-15","amount":-100,"merchant":{"name":"Cafe"},"category":{"name":"Dining"},"account":{"id":"acc"}},
			{"id":"h4","date":"2026-02-15","amount":-100,"merchant":{"name":"Cafe"},"category":{"name":"Dining"},"account":{"id":"acc"}},
			{"id":"h5","date":"2026-03-15","amount":-100,"merchant":{"name":"Cafe"},"category":{"name":"Dining"},"account":{"id":"acc"}},
			{"id":"h6","date":"2026-04-15","amount":-100,"merchant":{"name":"Cafe"},"category":{"name":"Dining"},"account":{"id":"acc"}},
			{"id":"c1","date":"2026-05-03","amount":-300,"merchant":{"name":"Restaurant"},"category":{"name":"Dining"},"account":{"id":"acc"}}
		],"totalCount":7}}}`), nil
	}))

	if err := h.execute("--json", "analyze", "anomalies", "--month", "2026-05", "--history-months", "6", "--min-ratio", "1.5", "--min-amount", "100"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if h.ExitCode != 0 || !sawHistoryWindow {
		t.Fatalf("exitCode=%d historyWindow=%v output=%q", h.ExitCode, sawHistoryWindow, h.Stdout.String())
	}
	if got := h.Stdout.String(); !strings.Contains(got, `"command":"analyze.anomalies"`) || !strings.Contains(got, `"largest_merchant":"Restaurant"`) {
		t.Fatalf("output = %q", got)
	}
}

func TestAnalyzeMerchantsRejectsUnsupportedCompare(t *testing.T) {
	h := newAppTestHarness(t, nil)
	if err := h.execute("--json", "analyze", "merchants", "--compare", "quarter"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if h.ExitCode == 0 || !strings.Contains(h.Stdout.String(), "previous-month") {
		t.Fatalf("exitCode=%d output=%q", h.ExitCode, h.Stdout.String())
	}
}

func TestAnalyzeBurnRateJSON(t *testing.T) {
	h := newJSONCommandHarness(t, testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		var gqlReq struct {
			OperationName string `json:"operationName"`
		}
		if err := json.NewDecoder(req.Body).Decode(&gqlReq); err != nil {
			return nil, err
		}
		if gqlReq.OperationName != "Common_GetJointPlanningData" {
			return nil, fmt.Errorf("operation = %q", gqlReq.OperationName)
		}
		return testutil.JSONResponse(`{"data":{"budgetData":{"monthlyAmountsByCategory":[{"category":{"id":"cat","name":"Dining"},"monthlyAmounts":[{"month":"2026-05","plannedCashFlowAmount":600,"actualAmount":670}]}]}}}`), nil
	}))
	if err := h.execute("--json", "analyze", "burn-rate", "--month", "2026-05"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if h.ExitCode != 0 || !strings.Contains(h.Stdout.String(), `"status":"overspending"`) {
		t.Fatalf("exitCode=%d output=%q", h.ExitCode, h.Stdout.String())
	}
}

func TestAnalyzeSubscriptionsJSON(t *testing.T) {
	h := newJSONCommandHarness(t, testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		var gqlReq struct {
			OperationName string         `json:"operationName"`
			Variables     map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(req.Body).Decode(&gqlReq); err != nil {
			return nil, err
		}
		if gqlReq.OperationName != "Web_GetUpcomingRecurringTransactionItems" || gqlReq.Variables["startDate"] == "" || gqlReq.Variables["endDate"] == "" {
			return nil, fmt.Errorf("recurring request = %#v", gqlReq)
		}
		return testutil.JSONResponse(`{"data":{"recurringTransactionItems":[{"stream":{"id":"netflix","frequency":"monthly","amount":15.49,"isApproximate":false,"merchant":{"name":"Netflix"}},"date":"2026-05-01","isPast":false,"transactionId":"","amount":15.49,"amountDiff":0,"category":{"id":"cat","name":"Entertainment"},"account":{"id":"acc","displayName":"Checking"}}]}}`), nil
	}))
	if err := h.execute("--json", "analyze", "subscriptions"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if h.ExitCode != 0 || !strings.Contains(h.Stdout.String(), `"annual":185.88`) {
		t.Fatalf("exitCode=%d output=%q", h.ExitCode, h.Stdout.String())
	}
}

func TestAnalyzeAnomaliesRequiresAuth(t *testing.T) {
	sessionPath := filepath.Join(t.TempDir(), "missing.json")
	h := newAppTestHarness(t, func(deps *Deps) {
		deps.LoadConfig = testConfigLoader(sessionPath, "")
	})
	if err := h.execute("--json", "analyze", "anomalies"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if h.ExitCode != 3 || !strings.Contains(h.Stdout.String(), "AUTH_REQUIRED") {
		t.Fatalf("exitCode=%d output=%q", h.ExitCode, h.Stdout.String())
	}
}
