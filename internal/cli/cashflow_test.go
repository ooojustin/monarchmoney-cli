package cli

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thedavidweng/monarchmoney-cli/internal/testutil"
)

func TestCashflow(t *testing.T) {
	t.Run("summary", testCashflowSummaryWithDates)
	t.Run("categories", testCashflowCategoriesJSON)
	t.Run("merchants", testCashflowMerchantsJSON)
	t.Run("spending", testCashflowSpendingJSON)
	t.Run("list", testCashflowListJSON)
	t.Run("trends", testCashflowTrendsJSON)
	t.Run("trends_invalid_from", testCashflowTrendsInvalidFrom)
	t.Run("trends_invalid_to", testCashflowTrendsInvalidTo)
	t.Run("trends_invalid_group_by", testCashflowTrendsInvalidGroupBy)
	t.Run("trends_invalid_period", testCashflowTrendsInvalidPeriod)
}

func newCashflowTestHarness(t *testing.T, transport http.RoundTripper) *appTestHarness {
	t.Helper()

	sessionPath := filepath.Join(t.TempDir(), "session.json")
	saveTestSession(t, sessionPath)
	return newAppTestHarness(t, func(deps *Deps) {
		deps.LoadConfig = testConfigLoader(sessionPath, "")
		deps.HTTPTransport = transport
	})
}

func testCashflowSummaryWithDates(t *testing.T) {
	h := newCashflowTestHarness(t, testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		gqlReq := decodeGraphQLRequest(t, req)
		if gqlReq.OperationName != "GetCashflowSummary" {
			t.Fatalf("operation = %q, want GetCashflowSummary", gqlReq.OperationName)
		}
		return testutil.JSONResponse(`{"data":{"aggregates":[{"summary":{"sumIncome":8500,"sumExpense":6200,"savings":2300,"savingsRate":0.2706}}]}}`), nil
	}))

	if err := h.execute("--json", "cashflow", "summary", "--from", "2026-01-01", "--to", "2026-03-31"); err != nil {
		t.Fatalf("execute() error = %v", err)
	}
	if h.ExitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", h.ExitCode, h.Stdout.String())
	}
	if got := h.Stdout.String(); !strings.Contains(got, `"command":"cashflow.summary"`) ||
		!strings.Contains(got, `"income":8500`) ||
		!strings.Contains(got, `"expense":6200`) ||
		!strings.Contains(got, `"savings_rate":0.2706`) {
		t.Fatalf("output = %q, want cashflow summary JSON", got)
	}
}

func testCashflowCategoriesJSON(t *testing.T) {
	h := newCashflowTestHarness(t, testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		gqlReq := decodeGraphQLRequest(t, req)
		if gqlReq.OperationName != "GetCashflowCategories" {
			t.Fatalf("operation = %q, want GetCashflowCategories", gqlReq.OperationName)
		}
		return testutil.JSONResponse(`{"data":{"aggregates":[{"groupBy":{"category":{"id":"cat-1","name":"Dining"}},"summary":{"sum":-450.50}},{"groupBy":{"category":{"id":"cat-2","name":"Groceries"}},"summary":{"sum":-320}}]}}`), nil
	}))

	if err := h.execute("--json", "cashflow", "categories", "--from", "2026-01-01", "--to", "2026-03-31"); err != nil {
		t.Fatalf("execute() error = %v", err)
	}
	if h.ExitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", h.ExitCode, h.Stdout.String())
	}
	if got := h.Stdout.String(); !strings.Contains(got, `"command":"cashflow.categories"`) || !strings.Contains(got, `"name":"Dining"`) {
		t.Fatalf("output = %q, want categories JSON", got)
	}
}

func testCashflowMerchantsJSON(t *testing.T) {
	h := newCashflowTestHarness(t, testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		gqlReq := decodeGraphQLRequest(t, req)
		if gqlReq.OperationName != "GetCashflowMerchants" {
			t.Fatalf("operation = %q, want GetCashflowMerchants", gqlReq.OperationName)
		}
		return testutil.JSONResponse(`{"data":{"aggregates":[{"groupBy":{"merchant":{"id":"m-1","name":"Amazon"}},"summary":{"sumIncome":0,"sumExpense":-120.50}}]}}`), nil
	}))

	if err := h.execute("--json", "cashflow", "merchants", "--from", "2026-01-01", "--to", "2026-03-31"); err != nil {
		t.Fatalf("execute() error = %v", err)
	}
	if h.ExitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", h.ExitCode, h.Stdout.String())
	}
	if got := h.Stdout.String(); !strings.Contains(got, `"command":"cashflow.merchants"`) || !strings.Contains(got, `"name":"Amazon"`) {
		t.Fatalf("output = %q, want merchants JSON", got)
	}
}

func testCashflowSpendingJSON(t *testing.T) {
	h := newCashflowTestHarness(t, testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		gqlReq := decodeGraphQLRequest(t, req)
		if gqlReq.OperationName != "GetCashflowCategories" {
			t.Fatalf("operation = %q, want GetCashflowCategories", gqlReq.OperationName)
		}
		return testutil.JSONResponse(`{"data":{"aggregates":[{"groupBy":{"category":{"id":"cat-1","name":"Income"}},"summary":{"sum":5000}},{"groupBy":{"category":{"id":"cat-2","name":"Dining"}},"summary":{"sum":-200}}]}}`), nil
	}))

	if err := h.execute("--json", "cashflow", "spending", "--from", "2026-01-01", "--to", "2026-03-31"); err != nil {
		t.Fatalf("execute() error = %v", err)
	}
	if h.ExitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", h.ExitCode, h.Stdout.String())
	}
	if got := h.Stdout.String(); !strings.Contains(got, `"command":"cashflow.spending"`) ||
		!strings.Contains(got, `"total_income":5000`) ||
		!strings.Contains(got, `"total_expenses":200`) ||
		!strings.Contains(got, `"net":4800`) {
		t.Fatalf("output = %q, want spending JSON", got)
	}
}

func testCashflowListJSON(t *testing.T) {
	h := newCashflowTestHarness(t, testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		gqlReq := decodeGraphQLRequest(t, req)
		if gqlReq.OperationName != "GetTransactionsList" {
			t.Fatalf("operation = %q, want GetTransactionsList", gqlReq.OperationName)
		}
		return testutil.JSONResponse(`{"data":{"allTransactions":{"results":[{"id":"tx-1","date":"2026-05-01","amount":5000,"merchant":{"name":"Payroll"},"category":{"name":"Income"},"account":{"id":"acc-1"},"notes":""},{"id":"tx-2","date":"2026-05-01","amount":-12.34,"merchant":{"name":"Cafe"},"category":{"name":"Dining"},"account":{"id":"acc-1"},"notes":""},{"id":"tx-3","date":"2026-05-02","amount":-50,"merchant":{"name":"Store"},"category":{"name":"Shopping"},"account":{"id":"acc-1"},"notes":""}],"totalCount":3}}}`), nil
	}))

	if err := h.execute("--json", "cashflow", "list", "--from", "2026-05-01", "--to", "2026-05-02"); err != nil {
		t.Fatalf("execute() error = %v", err)
	}
	if h.ExitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", h.ExitCode, h.Stdout.String())
	}
	if got := h.Stdout.String(); !strings.Contains(got, `"command":"cashflow.list"`) {
		t.Fatalf("output = %q, want cashflow list JSON", got)
	}
}

func testCashflowTrendsJSON(t *testing.T) {
	h := newCashflowTestHarness(t, testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		return testutil.JSONResponse(`{"data":{"aggregates":[{"groupBy":{"category":{"id":"cat-1"},"month":"2026-01"},"summary":{"sum":-500,"sumIncome":0,"sumExpense":500}},{"groupBy":{"category":{"id":"cat-2"},"month":"2026-02"},"summary":{"sum":-300,"sumIncome":0,"sumExpense":300}}]}}`), nil
	}))

	if err := h.execute("--json", "cashflow", "trends", "--from", "2026-01-01", "--to", "2026-03-31", "--group-by", "category", "--period", "month"); err != nil {
		t.Fatalf("execute() error = %v", err)
	}
	if h.ExitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", h.ExitCode, h.Stdout.String())
	}
	if got := h.Stdout.String(); !strings.Contains(got, `"command":"cashflow.trends"`) ||
		!strings.Contains(got, `"group_id":"cat-1"`) ||
		!strings.Contains(got, `"sum_income":0`) {
		t.Fatalf("output = %q, want trends JSON", got)
	}
}

func testCashflowTrendsInvalidFrom(t *testing.T) {
	testCashflowTrendsValidation(t,
		[]string{"--from", "01-01-2026", "--to", "2026-03-31"},
		"YYYY-MM-DD",
	)
}

func testCashflowTrendsInvalidTo(t *testing.T) {
	testCashflowTrendsValidation(t,
		[]string{"--from", "2026-01-01", "--to", "bad-date"},
		"YYYY-MM-DD",
	)
}

func testCashflowTrendsInvalidGroupBy(t *testing.T) {
	testCashflowTrendsValidation(t,
		[]string{"--from", "2026-01-01", "--to", "2026-03-31", "--group-by", "week"},
		"group-by must be category or category-group",
	)
}

func testCashflowTrendsInvalidPeriod(t *testing.T) {
	testCashflowTrendsValidation(t,
		[]string{"--from", "2026-01-01", "--to", "2026-03-31", "--group-by", "category", "--period", "week"},
		"month, quarter, or year",
	)
}

func testCashflowTrendsValidation(t *testing.T, args []string, want string) {
	t.Helper()

	h := newCashflowTestHarness(t, testutil.RoundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("cashflow trends should validate flags before making API requests")
		return nil, nil
	}))
	commandArgs := append([]string{"--json", "cashflow", "trends"}, args...)
	if err := h.execute(commandArgs...); err != nil {
		t.Fatalf("execute() error = %v", err)
	}
	if h.ExitCode == 0 {
		t.Fatalf("exitCode = 0, want validation failure; output=%q", h.Stdout.String())
	}
	if got := h.Stdout.String(); !strings.Contains(got, want) {
		t.Fatalf("output = %q, want %q", got, want)
	}
}
