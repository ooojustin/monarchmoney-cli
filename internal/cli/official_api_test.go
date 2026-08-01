package cli

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/thedavidweng/monarchmoney-cli/internal/testutil"
)

func TestAccountsBalanceAtJSON(t *testing.T) {
	h := newJSONCommandHarness(t, testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		var gqlReq struct {
			OperationName string         `json:"operationName"`
			Variables     map[string]any `json:"variables"`
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
		return testutil.JSONResponse(`{"data":{"accounts":[{"id":"acc-1","displayName":"Checking","displayBalance":42.25,"type":{"name":"cash","group":"asset"}}]}}`), nil
	}))

	if err := h.execute("--json", "accounts", "balance-at", "--date", "2026-05-10", "--account-id", "acc-1"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	out := h.Stdout.String()

	if h.ExitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", h.ExitCode, out)
	}
	if !strings.Contains(out, `"command":"accounts.balance-at"`) || !strings.Contains(out, `"display_name":"Checking"`) {
		t.Fatalf("output = %q", out)
	}
}

func TestInvestmentsPerformanceRequiresSecurityID(t *testing.T) {
	h := newJSONCommandHarness(t, nil)
	if err := h.execute("--json", "investments", "performance"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	out := h.Stdout.String()

	if h.ExitCode == 0 {
		t.Fatalf("exitCode = 0, want validation failure; output=%q", out)
	}
	if !strings.Contains(out, "--security-id is required") {
		t.Fatalf("output = %q, want security guidance", out)
	}
}

func TestTransactionsListWithEdgeCases(t *testing.T) {
	h := newJSONCommandHarness(t, testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		var gqlReq struct {
			OperationName string         `json:"operationName"`
			Variables     map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(req.Body).Decode(&gqlReq); err != nil {
			t.Fatalf("Decode request error = %v", err)
		}
		if gqlReq.OperationName != "GetTransactionsList" {
			t.Fatalf("operation = %q, want GetTransactionsList", gqlReq.OperationName)
		}
		return testutil.JSONResponse(`{"data":{"allTransactions":{"results":[
			{"id":"tx-1","date":"2026-05-01","amount":0,"merchant":{"name":"Free Trial"},"category":{"name":"Entertainment"},"account":{"id":"acc-1"},"notes":""},
			{"id":"tx-2","date":"2026-05-02","amount":-12.34,"merchant":{"name":"Café & Bakery"},"category":{"name":"Dining"},"account":{"id":"acc-1"},"notes":"latte"},
			{"id":"tx-3","date":"2026-05-03","amount":5000,"merchant":{"name":"Payroll"},"category":{"name":"Income"},"account":{"id":"acc-1"},"notes":null},
			{"id":"tx-4","date":"2026-05-04","amount":-0.01,"merchant":{"name":"Test Merchant"},"category":{"name":"Misc"},"account":{"id":"acc-1"},"notes":"micro transaction"}
		],"totalCount":4}}}`), nil
	}))

	if err := h.execute("--json", "transactions", "list"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	out := h.Stdout.String()

	if h.ExitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", h.ExitCode, out)
	}
	if !strings.Contains(out, `"command":"transactions.list"`) {
		t.Fatalf("output = %q", out)
	}
	if !strings.Contains(out, `"amount":0`) {
		t.Fatalf("output missing zero amount = %q", out)
	}
	if !strings.Contains(out, "Café") {
		t.Fatalf("output missing special characters = %q", out)
	}
}

func TestBudgetsShowJSON(t *testing.T) {
	h := newJSONCommandHarness(t, testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		var gqlReq struct {
			OperationName string         `json:"operationName"`
			Variables     map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(req.Body).Decode(&gqlReq); err != nil {
			t.Fatalf("Decode request error = %v", err)
		}
		if gqlReq.OperationName != "Common_GetJointPlanningData" {
			t.Fatalf("operation = %q, want GetJointPlanningData", gqlReq.OperationName)
		}
		return testutil.JSONResponse(`{"data":{"budgetData":{"monthlyAmountsByCategory":[{"category":{"id":"cat-dining","name":"Dining"},"monthlyAmounts":[{"month":"2026-05","plannedCashFlowAmount":300,"actualAmount":245.50}]}]}}}`), nil
	}))

	if err := h.execute("--json", "budgets", "show", "cat-dining", "--month", "2026-05"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	out := h.Stdout.String()

	if h.ExitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", h.ExitCode, out)
	}
	if !strings.Contains(out, `"command":"budgets.show"`) || !strings.Contains(out, `"category_name":"Dining"`) {
		t.Fatalf("output = %q", out)
	}
}

func TestCashflowSummaryJSON(t *testing.T) {
	h := newJSONCommandHarness(t, testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		var gqlReq struct {
			OperationName string `json:"operationName"`
		}
		if err := json.NewDecoder(req.Body).Decode(&gqlReq); err != nil {
			t.Fatalf("Decode request error = %v", err)
		}
		if gqlReq.OperationName != "GetCashflowSummary" {
			t.Fatalf("operation = %q, want GetCashflowSummary", gqlReq.OperationName)
		}
		return testutil.JSONResponse(`{"data":{"aggregates":[{"summary":{"sumIncome":8500,"sumExpense":6200,"savings":2300,"savingsRate":0.2706}}]}}`), nil
	}))

	if err := h.execute("--json", "cashflow", "summary", "--from", "2026-05-01", "--to", "2026-05-31"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	out := h.Stdout.String()

	if h.ExitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", h.ExitCode, out)
	}
	if !strings.Contains(out, `"command":"cashflow.summary"`) || !strings.Contains(out, `"income":8500`) {
		t.Fatalf("output = %q", out)
	}
}

func TestTransactionsShowJSON(t *testing.T) {
	h := newJSONCommandHarness(t, testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		var gqlReq struct {
			OperationName string         `json:"operationName"`
			Variables     map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(req.Body).Decode(&gqlReq); err != nil {
			t.Fatalf("Decode request error = %v", err)
		}
		if gqlReq.OperationName != "GetTransactionDrawer" {
			t.Fatalf("operation = %q, want GetTransaction", gqlReq.OperationName)
		}
		if gqlReq.Variables["id"] != "tx-123" {
			t.Fatalf("variables = %#v, want id tx-123", gqlReq.Variables)
		}
		return testutil.JSONResponse(`{"data":{"getTransaction":{"id":"tx-123","date":"2026-05-15","amount":-42.50,"merchant":{"name":"Café & Co"},"category":{"name":"Dining"},"notes":"lunch with émojis 🍕","pending":false,"hideFromReports":false,"plaidName":"CAFE AND CO","isRecurring":false,"reviewStatus":"reviewed","needsReview":false,"isSplitTransaction":false,"createdAt":"2026-05-15T10:00:00Z","updatedAt":"2026-05-15T10:00:00Z","account":{"id":"acc-1","displayName":"Checking"},"tags":[{"id":"tag-1","name":"food","color":"#ff0000","order":1}]}}}`), nil
	}))

	if err := h.execute("--json", "transactions", "show", "tx-123"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	out := h.Stdout.String()

	if h.ExitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", h.ExitCode, out)
	}
	if !strings.Contains(out, `"command":"transactions.show"`) || !strings.Contains(out, "Café") {
		t.Fatalf("output = %q", out)
	}
	if !strings.Contains(out, "émojis") {
		t.Fatalf("output missing special chars in notes = %q", out)
	}
}

func TestCategoriesListJSON(t *testing.T) {
	h := newJSONCommandHarness(t, testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		var gqlReq struct {
			OperationName string `json:"operationName"`
		}
		if err := json.NewDecoder(req.Body).Decode(&gqlReq); err != nil {
			t.Fatalf("Decode request error = %v", err)
		}
		if gqlReq.OperationName != "GetCategories" {
			t.Fatalf("operation = %q, want GetCategories", gqlReq.OperationName)
		}
		return testutil.JSONResponse(`{"data":{"categories":[
			{"id":"cat-1","name":"Dining","order":1,"icon":"utensils","group":{"id":"grp-1","name":"Food & Drink","type":"expense"}},
			{"id":"cat-2","name":"Income","order":2,"icon":"dollar","group":{"id":"grp-2","name":"Income","type":"income"}}
		]}}`), nil
	}))

	if err := h.execute("--json", "categories", "list"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	out := h.Stdout.String()

	if h.ExitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", h.ExitCode, out)
	}
	if !strings.Contains(out, `"command":"categories.list"`) || !strings.Contains(out, `"name":"Dining"`) {
		t.Fatalf("output = %q", out)
	}
	if !strings.Contains(out, "Food") {
		t.Fatalf("output missing group name = %q", out)
	}
}

func TestTransactionsCreateJSON(t *testing.T) {
	h := newJSONCommandHarness(t, testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		var gqlReq struct {
			OperationName string         `json:"operationName"`
			Variables     map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(req.Body).Decode(&gqlReq); err != nil {
			t.Fatalf("Decode request error = %v", err)
		}
		if gqlReq.OperationName != "Common_CreateTransactionMutation" {
			t.Fatalf("operation = %q, want Common_CreateTransactionMutation", gqlReq.OperationName)
		}
		input, ok := gqlReq.Variables["input"].(map[string]any)
		if !ok {
			t.Fatalf("input = %#v, want object", gqlReq.Variables["input"])
		}
		if input["amount"] != float64(-25.50) {
			t.Fatalf("input amount = %v, want -25.50", input["amount"])
		}
		if input["merchantName"] != "Coffee Shop" {
			t.Fatalf("input merchantName = %v, want Coffee Shop", input["merchantName"])
		}
		if input["categoryId"] != "cat-1" {
			t.Fatalf("input categoryId = %v, want cat-1", input["categoryId"])
		}
		if input["accountId"] != "acc-1" {
			t.Fatalf("input accountId = %v, want acc-1", input["accountId"])
		}
		return testutil.JSONResponse(`{"data":{"createTransaction":{"transaction":{"id":"tx-new-1","amount":-25.50,"date":"2026-06-01","merchant":{"name":"Coffee Shop"}}}}}`), nil
	}))

	if err := h.execute("--json", "--confirm", "transactions", "create", "--amount", "-25.50", "--merchant", "Coffee Shop", "--date", "2026-06-01", "--category", "cat-1", "--account", "acc-1"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	out := h.Stdout.String()

	if h.ExitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", h.ExitCode, out)
	}
	if !strings.Contains(out, `"command":"transactions.create"`) {
		t.Fatalf("output missing command = %q", out)
	}
	if !strings.Contains(out, "tx-new-1") {
		t.Fatalf("output missing transaction ID = %q", out)
	}
	if !strings.Contains(out, `"amount":-25.5`) {
		t.Fatalf("output missing amount = %q", out)
	}
}

func TestTransactionsUpdateJSON(t *testing.T) {
	h := newJSONCommandHarness(t, testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		var gqlReq struct {
			OperationName string         `json:"operationName"`
			Variables     map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(req.Body).Decode(&gqlReq); err != nil {
			t.Fatalf("Decode request error = %v", err)
		}
		if gqlReq.OperationName != "Web_TransactionDrawerUpdateTransaction" {
			t.Fatalf("operation = %q, want Web_TransactionDrawerUpdateTransaction", gqlReq.OperationName)
		}
		input, ok := gqlReq.Variables["input"].(map[string]any)
		if !ok {
			t.Fatalf("input = %#v, want object", gqlReq.Variables["input"])
		}
		if input["id"] != "tx-100" {
			t.Fatalf("input id = %v, want tx-100", input["id"])
		}
		if input["notes"] != "updated notes" {
			t.Fatalf("input notes = %v, want updated notes", input["notes"])
		}
		if input["category"] != "cat-new" {
			t.Fatalf("input category = %v, want cat-new", input["category"])
		}
		return testutil.JSONResponse(`{"data":{"updateTransaction":{"transaction":{"id":"tx-100","amount":-50,"date":"2026-05-15","notes":"updated notes","hideFromReports":false,"needsReview":false,"category":{"name":"Dining"},"merchant":{"name":"Restaurant"}}}}}`), nil
	}))

	if err := h.execute("--json", "--confirm", "transactions", "update", "tx-100", "--notes", "updated notes", "--category", "cat-new"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	out := h.Stdout.String()

	if h.ExitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", h.ExitCode, out)
	}
	if !strings.Contains(out, `"command":"transactions.update"`) {
		t.Fatalf("output missing command = %q", out)
	}
	if !strings.Contains(out, "tx-100") {
		t.Fatalf("output missing transaction ID = %q", out)
	}
	if !strings.Contains(out, `"notes":"updated notes"`) {
		t.Fatalf("output missing notes = %q", out)
	}
}

func TestCategoriesCreateJSON(t *testing.T) {
	h := newJSONCommandHarness(t, testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		var gqlReq struct {
			OperationName string         `json:"operationName"`
			Variables     map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(req.Body).Decode(&gqlReq); err != nil {
			t.Fatalf("Decode request error = %v", err)
		}
		if gqlReq.OperationName != "Web_CreateCategory" {
			t.Fatalf("operation = %q, want CreateCategory", gqlReq.OperationName)
		}
		if gqlReq.Variables["name"] != "Streaming Services" {
			t.Fatalf("variables name = %v, want Streaming Services", gqlReq.Variables["name"])
		}
		if gqlReq.Variables["groupId"] != "grp-entertainment" {
			t.Fatalf("variables groupId = %v, want grp-entertainment", gqlReq.Variables["groupId"])
		}
		return testutil.JSONResponse(`{"data":{"createCategory":{"category":{"id":"cat-new-1","name":"Streaming Services"}}}}`), nil
	}))

	if err := h.execute("--json", "--confirm", "categories", "create", "--name", "Streaming Services", "--group", "grp-entertainment"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	out := h.Stdout.String()

	if h.ExitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", h.ExitCode, out)
	}
	if !strings.Contains(out, `"command":"categories.create"`) {
		t.Fatalf("output missing command = %q", out)
	}
	if !strings.Contains(out, "cat-new-1") {
		t.Fatalf("output missing category ID = %q", out)
	}
	if !strings.Contains(out, "Streaming Services") {
		t.Fatalf("output missing category name = %q", out)
	}
}

func TestBudgetsSetJSON(t *testing.T) {
	h := newJSONCommandHarness(t, testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		var gqlReq struct {
			OperationName string         `json:"operationName"`
			Variables     map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(req.Body).Decode(&gqlReq); err != nil {
			t.Fatalf("Decode request error = %v", err)
		}
		if gqlReq.OperationName != "Common_UpdateBudgetItem" {
			t.Fatalf("operation = %q, want SetBudget", gqlReq.OperationName)
		}
		input, ok := gqlReq.Variables["input"].(map[string]any)
		if !ok {
			t.Fatalf("input = %#v, want object", gqlReq.Variables["input"])
		}
		if input["categoryId"] != "cat-dining" {
			t.Fatalf("input categoryId = %v, want cat-dining", input["categoryId"])
		}
		if input["amount"] != float64(500) {
			t.Fatalf("input amount = %v, want 500", input["amount"])
		}
		return testutil.JSONResponse(`{"data":{"updateOrCreateBudgetItem":{"budgetItem":{"id":"budget-1","budgetAmount":500}}}}`), nil
	}))

	if err := h.execute("--json", "--confirm", "budgets", "set", "cat-dining", "--amount", "500", "--month", "2026-06"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	out := h.Stdout.String()

	if h.ExitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", h.ExitCode, out)
	}
	if !strings.Contains(out, `"command":"budgets.set"`) {
		t.Fatalf("output missing command = %q", out)
	}
	if !strings.Contains(out, `"category_id":"cat-dining"`) {
		t.Fatalf("output missing category ID = %q", out)
	}
	if !strings.Contains(out, `"planned":500`) {
		t.Fatalf("output missing planned amount = %q", out)
	}
}

func TestRulesCreateJSON(t *testing.T) {
	h := newJSONCommandHarness(t, testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		var gqlReq struct {
			OperationName string         `json:"operationName"`
			Variables     map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(req.Body).Decode(&gqlReq); err != nil {
			t.Fatalf("Decode request error = %v", err)
		}
		if gqlReq.OperationName != "Common_CreateTransactionRuleMutationV2" {
			t.Fatalf("operation = %q, want Common_CreateTransactionRuleMutationV2", gqlReq.OperationName)
		}
		input, ok := gqlReq.Variables["input"].(map[string]any)
		if !ok {
			t.Fatalf("input = %#v, want object", gqlReq.Variables["input"])
		}
		criteria, ok := input["merchantNameCriteria"].([]any)
		if !ok {
			t.Fatalf("merchantNameCriteria = %#v, want array", input["merchantNameCriteria"])
		}
		if len(criteria) == 0 {
			t.Fatalf("input merchantNameCriteria is empty")
		}
		first, ok := criteria[0].(map[string]any)
		if !ok {
			t.Fatalf("first merchant criterion = %#v, want object", criteria[0])
		}
		if first["value"] != "Uber" {
			t.Fatalf("input merchantNameCriteria value = %v, want Uber", first["value"])
		}
		if input["setCategoryAction"] != "cat-transport" {
			t.Fatalf("input setCategoryAction = %v, want cat-transport", input["setCategoryAction"])
		}
		return testutil.JSONResponse(`{"data":{"createTransactionRuleV2":{}}}`), nil
	}))

	if err := h.execute("--json", "--confirm", "rules", "create", "--merchant-operator", "contains", "--merchant-value", "Uber", "--set-category-id", "cat-transport"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	out := h.Stdout.String()

	if h.ExitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", h.ExitCode, out)
	}
	if !strings.Contains(out, `"command":"rules.create"`) {
		t.Fatalf("output missing command = %q", out)
	}
	if !strings.Contains(out, `"status":"created"`) {
		t.Fatalf("output missing status = %q", out)
	}
}

func TestTransactionsListPassesExtendedFilters(t *testing.T) {
	h := newJSONCommandHarness(t, testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		var gqlReq struct {
			OperationName string         `json:"operationName"`
			Variables     map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(req.Body).Decode(&gqlReq); err != nil {
			t.Fatalf("Decode request error = %v", err)
		}
		if gqlReq.OperationName != "GetTransactionsList" {
			t.Fatalf("operation = %q, want transactions", gqlReq.OperationName)
		}
		filters, ok := gqlReq.Variables["filters"].(map[string]any)
		if !ok {
			t.Fatalf("filters = %#v, want object", gqlReq.Variables["filters"])
		}
		if filters["isPending"] != true || filters["hideFromReports"] != false {
			t.Fatalf("filters = %#v, want pending/hide-from-reports", filters)
		}
		goals, ok := filters["goals"].([]any)
		if !ok || len(goals) != 2 || goals[0] != "goal-1" || goals[1] != "goal-2" {
			t.Fatalf("filters goals = %#v, want goal ids", filters["goals"])
		}
		return testutil.JSONResponse(`{"data":{"allTransactions":{"results":[],"totalCount":0}}}`), nil
	}))

	if err := h.execute("--json", "transactions", "list", "--pending=true", "--hide-from-reports=false", "--goal-id", "goal-1,goal-2"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	out := h.Stdout.String()

	if h.ExitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", h.ExitCode, out)
	}
	if !strings.Contains(out, `"command":"transactions.list"`) {
		t.Fatalf("output = %q", out)
	}
}
