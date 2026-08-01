package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/thedavidweng/monarchmoney-cli/internal/config"
	"github.com/thedavidweng/monarchmoney-cli/internal/testutil"
)

type overviewGraphQLRequest struct {
	OperationName string         `json:"operationName"`
	Variables     map[string]any `json:"variables"`
}

func newTestOverviewApp(t *testing.T, transport http.RoundTripper) (*App, *bytes.Buffer, *int) {
	t.Helper()

	dir := t.TempDir()
	sessionPath := filepath.Join(dir, "session.json")
	saveTestSession(t, sessionPath)

	var out bytes.Buffer
	exitCode := 0
	app := New(Deps{
		LoadConfig: func(string) (*config.Config, error) {
			return &config.Config{
				Profile:     "default",
				APIEndpoint: "https://example.invalid/graphql",
				Timeout:     time.Second,
				SessionPath: sessionPath,
			}, nil
		},
		Getenv:        func(string) string { return "" },
		NewRequestID:  func() string { return "request-id" },
		HTTPTransport: transport,
		Stdout:        &out,
		Stderr:        &out,
		Exit:          func(code int) { exitCode = code },
	})
	return app, &out, &exitCode
}

func overviewGraphQLResponse(operation string) (*http.Response, error) {
	switch operation {
	case "GetAccounts":
		return testutil.JSONResponse(`{"data":{"accounts":[{"id":"a1","displayName":"Checking","type":{"name":"bank","display":"Bank"},"subtype":{"name":"checking","display":"Checking"},"displayBalance":1000,"currentBalance":1000,"updatedAt":"2026-05-01","isHidden":false,"isAsset":true,"mask":"1234","isManual":false,"includeInNetWorth":true,"includeBalanceInNetWorth":false}]}}`), nil
	case "GetCashflowSummary":
		return testutil.JSONResponse(`{"data":{"aggregates":[{"summary":{"sumIncome":5000,"sumExpense":3000,"savings":2000,"savingsRate":0.4}}]}}`), nil
	case "GetTransactionsList":
		return testutil.JSONResponse(`{"data":{"allTransactions":{"results":[{"id":"t1","date":"2026-07-01","amount":-50,"pending":false,"category":{"id":"c1","name":"Food","group":{"id":"g1","name":"Expenses","type":"expense"}},"merchant":{"name":"Grocery Store","id":"m1"},"account":{"id":"a1","displayName":"Checking","order":0,"type":{"group":"bank"}}}],"totalCount":42}}}`), nil
	default:
		return nil, fmt.Errorf("unexpected operation %q", operation)
	}
}

func TestAppRootRegistersOverview(t *testing.T) {
	app, _ := newTestApp(t)
	cmd, _, err := app.Root.Find([]string{"overview"})
	if err != nil {
		t.Fatalf("Find(overview) error = %v", err)
	}
	if cmd == nil || cmd.Name() != "overview" || cmd.GroupID != "core" {
		t.Fatalf("Find(overview) = %#v", cmd)
	}
	for _, flag := range []string{"from", "to"} {
		if cmd.Flags().Lookup(flag) == nil {
			t.Fatalf("overview missing --%s flag", flag)
		}
	}
}

func TestAppOverviewJSONUsesFlagsAndRequestID(t *testing.T) {
	var mu sync.Mutex
	requests := make(map[string]map[string]any)
	transport := testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		var gqlReq overviewGraphQLRequest
		if err := json.NewDecoder(req.Body).Decode(&gqlReq); err != nil {
			return nil, fmt.Errorf("decode GraphQL request: %w", err)
		}
		mu.Lock()
		requests[gqlReq.OperationName] = gqlReq.Variables
		mu.Unlock()
		return overviewGraphQLResponse(gqlReq.OperationName)
	})

	app, out, exitCode := newTestOverviewApp(t, transport)
	app.Root.SetArgs([]string{"--json", "overview", "--from", "2026-01-01", "--to", "2026-01-31"})
	if err := app.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if *exitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", *exitCode, out.String())
	}

	var env struct {
		Data struct {
			NetWorth         float64 `json:"net_worth"`
			AccountCount     int     `json:"account_count"`
			TransactionTotal int     `json:"transaction_total"`
		} `json:"data"`
		Meta struct {
			Command   string `json:"command"`
			RequestID string `json:"request_id"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &env); err != nil {
		t.Fatalf("json.Unmarshal() error = %v; output=%q", err, out.String())
	}
	if env.Meta.Command != "overview" || env.Meta.RequestID != "request-id" {
		t.Fatalf("metadata = %#v", env.Meta)
	}
	if env.Data.NetWorth != 1000 || env.Data.AccountCount != 1 || env.Data.TransactionTotal != 42 {
		t.Fatalf("overview data = %#v", env.Data)
	}

	mu.Lock()
	defer mu.Unlock()
	cashflowFilters, _ := requests["GetCashflowSummary"]["filters"].(map[string]any)
	transactionFilters, _ := requests["GetTransactionsList"]["filters"].(map[string]any)
	for operation, filters := range map[string]map[string]any{
		"GetCashflowSummary":  cashflowFilters,
		"GetTransactionsList": transactionFilters,
	} {
		if filters["startDate"] != "2026-01-01" || filters["endDate"] != "2026-01-31" {
			t.Fatalf("%s filters = %#v", operation, filters)
		}
	}
}

func TestAppOverviewHuman(t *testing.T) {
	transport := testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		var gqlReq overviewGraphQLRequest
		if err := json.NewDecoder(req.Body).Decode(&gqlReq); err != nil {
			return nil, fmt.Errorf("decode GraphQL request: %w", err)
		}
		return overviewGraphQLResponse(gqlReq.OperationName)
	})

	app, out, exitCode := newTestOverviewApp(t, transport)
	app.Root.SetArgs([]string{"overview"})
	if err := app.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if *exitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", *exitCode, out.String())
	}
	for _, want := range []string{"Net Worth:", "Savings Rate:", "DATE", "Grocery Store"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output = %q, want %q", out.String(), want)
		}
	}
}
