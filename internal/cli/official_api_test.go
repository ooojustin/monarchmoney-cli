package cli

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestAccountsBalanceAtJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	sessionPath := filepath.Join(dir, "session.json")
	app, out, exitCode := newTestApp(t, sessionPath)
	saveTestSession(t, sessionPath)

	app.Deps.HTTPTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		var gqlReq struct {
			OperationName string                 `json:"operationName"`
			Variables     map[string]interface{} `json:"variables"`
		}
		if err := json.NewDecoder(req.Body).Decode(&gqlReq); err != nil {
			t.Fatalf("Decode request error = %v", err)
		}
		if gqlReq.OperationName != "Common_GetDisplayBalanceAtDate" {
			t.Fatalf("operation = %q, want balance at date", gqlReq.OperationName)
		}
		if gqlReq.Variables["date"] != "2026-05-10" {
			t.Fatalf("variables = %#v, want date", gqlReq.Variables)
		}
		return jsonHTTPResponse(`{"data":{"accounts":[{"id":"acc-1","displayName":"Checking","displayBalance":42.25,"type":{"name":"cash","group":"asset"}}]}}`), nil
	})

	app.Root.SetArgs([]string{"--json", "accounts", "balance-at", "--date", "2026-05-10", "--account-id", "acc-1"})
	if err := app.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if *exitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", *exitCode, out.String())
	}
	if !strings.Contains(out.String(), `"command":"accounts.balance-at"`) || !strings.Contains(out.String(), `"display_name":"Checking"`) {
		t.Fatalf("output = %q", out.String())
	}
}

func TestCashflowTrendsRejectsInvalidPeriod(t *testing.T) {
	t.Parallel()

	app, out, exitCode := newTestApp(t, filepath.Join(t.TempDir(), "session.json"))

	app.Root.SetArgs([]string{"--json", "cashflow", "trends", "--from", "2026-01-01", "--to", "2026-03-31", "--period", "week"})
	if err := app.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if *exitCode == 0 {
		t.Fatalf("exitCode = 0, want validation failure; output=%q", out.String())
	}
	if !strings.Contains(out.String(), "month, quarter, or year") {
		t.Fatalf("output = %q, want period guidance", out.String())
	}
}

func TestGoalsListJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	sessionPath := filepath.Join(dir, "session.json")
	app, out, exitCode := newTestApp(t, sessionPath)
	saveTestSession(t, sessionPath)

	app.Deps.HTTPTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		var gqlReq struct {
			OperationName string `json:"operationName"`
		}
		if err := json.NewDecoder(req.Body).Decode(&gqlReq); err != nil {
			t.Fatalf("Decode request error = %v", err)
		}
		if gqlReq.OperationName != "Web_GoalsV2" {
			t.Fatalf("operation = %q, want goals", gqlReq.OperationName)
		}
		return jsonHTTPResponse(`{"data":{"goalsV2":[{"id":"goal-1","name":"Vacation"}]}}`), nil
	})

	app.Root.SetArgs([]string{"--json", "goals", "list"})
	if err := app.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if *exitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", *exitCode, out.String())
	}
	if !strings.Contains(out.String(), `"command":"goals.list"`) || !strings.Contains(out.String(), `"Vacation"`) {
		t.Fatalf("output = %q", out.String())
	}
}

func TestInvestmentsPerformanceRequiresSecurityID(t *testing.T) {
	t.Parallel()

	app, out, exitCode := newTestApp(t, filepath.Join(t.TempDir(), "session.json"))

	app.Root.SetArgs([]string{"--json", "investments", "performance"})
	if err := app.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if *exitCode == 0 {
		t.Fatalf("exitCode = 0, want validation failure; output=%q", out.String())
	}
	if !strings.Contains(out.String(), "--security-id is required") {
		t.Fatalf("output = %q, want security guidance", out.String())
	}
}

func TestTransactionsListPassesExtendedFilters(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	sessionPath := filepath.Join(dir, "session.json")
	app, out, exitCode := newTestApp(t, sessionPath)
	saveTestSession(t, sessionPath)

	app.Deps.HTTPTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		var gqlReq struct {
			OperationName string                 `json:"operationName"`
			Variables     map[string]interface{} `json:"variables"`
		}
		if err := json.NewDecoder(req.Body).Decode(&gqlReq); err != nil {
			t.Fatalf("Decode request error = %v", err)
		}
		if gqlReq.OperationName != "GetTransactionsList" {
			t.Fatalf("operation = %q, want transactions", gqlReq.OperationName)
		}
		filters := gqlReq.Variables["filters"].(map[string]interface{})
		if filters["isPending"] != true || filters["hideFromReports"] != false {
			t.Fatalf("filters = %#v, want pending/hide-from-reports", filters)
		}
		goals, ok := filters["goals"].([]interface{})
		if !ok || len(goals) != 2 || goals[0] != "goal-1" || goals[1] != "goal-2" {
			t.Fatalf("filters goals = %#v, want goal ids", filters["goals"])
		}
		return jsonHTTPResponse(`{"data":{"allTransactions":{"results":[],"totalCount":0}}}`), nil
	})

	app.Root.SetArgs([]string{"--json", "transactions", "list", "--pending=true", "--hide-from-reports=false", "--goal-id", "goal-1,goal-2"})
	if err := app.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if *exitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", *exitCode, out.String())
	}
	if !strings.Contains(out.String(), `"command":"transactions.list"`) {
		t.Fatalf("output = %q", out.String())
	}
}
