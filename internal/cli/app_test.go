package cli

import (
	"bytes"
	"encoding/json"
	stderrors "errors"
	"io"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/thedavidweng/monarchmoney-cli/internal/config"
	"github.com/thedavidweng/monarchmoney-cli/internal/testutil"
)

func newTestApp(t *testing.T) (*App, *bytes.Buffer) {
	t.Helper()
	var out bytes.Buffer
	app := New(Deps{
		LoadConfig: func(string) (*config.Config, error) {
			return &config.Config{Profile: "default", Timeout: 30 * time.Second, SessionPath: config.DefaultSessionPath(), AuditLog: true}, nil
		},
		Getenv:       func(string) string { return "" },
		NewRequestID: func() string { return "request-id" },
		Stdout:       &out,
		Stderr:       io.Discard,
		Stdin:        bytes.NewReader(nil),
		Exit:         func(int) {},
	})
	return app, &out
}

func TestNewBuildsIndependentRoots(t *testing.T) {
	first, _ := newTestApp(t)
	second, _ := newTestApp(t)
	if first.Root == second.Root {
		t.Fatal("New() returned a shared root command")
	}
	first.Flags.Profile = "changed"
	if second.Flags.Profile == "changed" {
		t.Fatal("New() returned shared flag state")
	}
}

func TestAppRejectsRepeatedExecution(t *testing.T) {
	app, _ := newTestApp(t)
	app.Root.SetArgs([]string{"version"})
	if err := app.Execute(); err != nil {
		t.Fatalf("first Execute() error = %v", err)
	}

	app.Root.SetArgs([]string{"--confirm", "version"})
	err := app.Execute()
	if err == nil || !strings.Contains(err.Error(), "cannot be executed more than once") {
		t.Fatalf("second Execute() error = %v, want single-use failure", err)
	}
}

func TestBooleanArgument(t *testing.T) {
	tests := []struct {
		args []string
		name string
		want bool
		ok   bool
	}{
		{args: []string{"--json"}, name: "--json", want: true, ok: true},
		{args: []string{"--json=true"}, name: "--json", want: true, ok: true},
		{args: []string{"--json=false"}, name: "--json", want: false, ok: true},
		{args: []string{"--json", "--json=false"}, name: "--json", want: false, ok: true},
		{args: []string{"--pretty"}, name: "--json", want: false, ok: false},
		{args: []string{"--", "--json"}, name: "--json", want: false, ok: false},
		{args: []string{"--", "--pretty"}, name: "--pretty", want: false, ok: false},
		{args: []string{"--", "--events"}, name: "--events", want: false, ok: false},
	}
	for _, test := range tests {
		got, ok := booleanArgument(test.args, test.name)
		if got != test.want || ok != test.ok {
			t.Errorf("booleanArgument(%v) = %t, %t, want %t, %t", test.args, got, ok, test.want, test.ok)
		}
	}
}

func TestAppHumanDryRunRendersPlan(t *testing.T) {
	app, out := newTestApp(t)
	app.Root.SetArgs([]string{"transactions", "update", "transaction-test", "--amount", "12.5", "--notes", "hello", "--dry-run"})
	if err := app.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	for _, want := range []string{"Mutation Plan", "transactions.update", "transaction-test", `"amount":12.5`, `"notes":"hello"`} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output = %q, want %q", out.String(), want)
		}
	}
	if strings.Contains(out.String(), "0x") || strings.Contains(out.String(), "<nil>") {
		t.Fatalf("output contains Go implementation values: %q", out.String())
	}
}

func TestAppCommandTreeTopology(t *testing.T) {
	app, _ := newTestApp(t)

	commands := app.Root.Commands()
	topLevel := make([]string, 0, len(commands))
	for _, cmd := range commands {
		topLevel = append(topLevel, cmd.Name())
	}
	slices.Sort(topLevel)
	wantTopLevel := []string{
		"accounts", "analyze", "audit", "auth", "budgets", "cache", "cashflow",
		"categories", "completion", "credit", "doctor", "goals", "hledger", "institutions",
		"investments", "networth", "overview", "receipts", "recurring", "rules", "subscription",
		"tags", "transactions", "version",
	}
	if !slices.Equal(topLevel, wantTopLevel) {
		t.Fatalf("top-level commands = %v, want %v", topLevel, wantTopLevel)
	}

	var checkUniqueSiblings func(*cobra.Command)
	checkUniqueSiblings = func(parent *cobra.Command) {
		seen := make(map[string]struct{})
		for _, child := range parent.Commands() {
			if _, exists := seen[child.Name()]; exists {
				t.Errorf("duplicate command %q under %q", child.Name(), parent.CommandPath())
			}
			seen[child.Name()] = struct{}{}
			checkUniqueSiblings(child)
		}
	}
	checkUniqueSiblings(app.Root)
}

func TestAppConfigPrecedence(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	app, _ := newTestApp(t)
	loadCalls := 0
	app.Deps.Getenv = func(key string) string {
		if key == "MONARCH_CONFIG" {
			return configPath
		}
		return ""
	}
	app.Deps.LoadConfig = func(path string) (*config.Config, error) {
		loadCalls++
		if path != configPath {
			t.Fatalf("LoadConfig(%q), want %q", path, configPath)
		}
		return &config.Config{Profile: "config-profile", Timeout: time.Minute, ReadOnly: true, SessionPath: "/tmp/session.json", AuditLog: true}, nil
	}
	app.Root.SetArgs([]string{"version"})
	if err := app.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if app.Flags.Profile != "config-profile" || app.Flags.Timeout != time.Minute || !app.Flags.ReadOnly {
		t.Fatalf("Flags = %#v", app.Flags)
	}
	if loadCalls != 1 {
		t.Fatalf("LoadConfig calls = %d, want 1", loadCalls)
	}
	if app.sessionPath() != "/tmp/session.json" {
		t.Fatalf("sessionPath() = %q", app.sessionPath())
	}
}

func TestAppConfigFlagOverridesEnvironment(t *testing.T) {
	flagPath := filepath.Join(t.TempDir(), "flag.yaml")
	envPath := filepath.Join(t.TempDir(), "env.yaml")
	app, _ := newTestApp(t)
	app.Deps.Getenv = func(key string) string {
		if key == "MONARCH_CONFIG" {
			return envPath
		}
		return ""
	}
	loadCalls := 0
	app.Deps.LoadConfig = func(path string) (*config.Config, error) {
		loadCalls++
		if path != flagPath {
			t.Fatalf("LoadConfig(%q), want %q", path, flagPath)
		}
		return config.Default(), nil
	}

	app.Root.SetArgs([]string{"--config", flagPath, "version"})
	if err := app.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if loadCalls != 1 {
		t.Fatalf("LoadConfig calls = %d, want 1", loadCalls)
	}
}

func TestAppFlagsOverrideConfig(t *testing.T) {
	app, _ := newTestApp(t)
	app.Deps.LoadConfig = func(string) (*config.Config, error) {
		return &config.Config{Profile: "config-profile", Timeout: time.Minute, ReadOnly: true, AuditLog: true}, nil
	}
	app.Root.SetArgs([]string{"--profile", "flag-profile", "--timeout", "2m", "--read-only=false", "version"})
	if err := app.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if app.Flags.Profile != "flag-profile" || app.Flags.Timeout != 2*time.Minute || app.Flags.ReadOnly {
		t.Fatalf("Flags = %#v", app.Flags)
	}
}

func TestAppRemoteCommandRejectsConfigError(t *testing.T) {
	var out bytes.Buffer
	exitCode := 0
	app := New(Deps{
		LoadConfig: func(string) (*config.Config, error) {
			return config.Default(), stderrors.New("malformed config")
		},
		Getenv:       func(string) string { return "" },
		NewRequestID: func() string { return "request-id" },
		Stdout:       &out,
		Stderr:       io.Discard,
		Exit:         func(code int) { exitCode = code },
		HTTPTransport: testutil.RoundTripFunc(func(*http.Request) (*http.Response, error) {
			t.Fatal("remote command should fail before making a request")
			return nil, nil
		}),
	})

	app.Root.SetArgs([]string{"--json", "credit", "history"})
	if err := app.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if exitCode == 0 {
		t.Fatalf("exitCode = 0; output=%q", out.String())
	}
	if got := out.String(); !strings.Contains(got, `"command":"credit.history"`) || !strings.Contains(got, "failed to load config") {
		t.Fatalf("output = %q, want config error envelope", got)
	}
}

func TestAppRootRegistersMixedCommandFamilies(t *testing.T) {
	app, _ := newTestApp(t)
	for _, path := range [][]string{
		{"categories", "list"},
		{"categories", "update"},
		{"categories", "rollover"},
		{"categories", "groups", "update"},
		{"recurring", "list"},
		{"recurring", "update"},
		{"tags", "list"},
		{"tags", "create"},
	} {
		cmd, _, err := app.Root.Find(path)
		if err != nil || cmd == nil || cmd.Name() != path[len(path)-1] {
			t.Fatalf("Find(%v) = %#v, %v", path, cmd, err)
		}
	}
}

func TestAppRootRegistersRulesAndBudgets(t *testing.T) {
	app, _ := newTestApp(t)
	for _, path := range [][]string{
		{"rules", "list"},
		{"rules", "create"},
		{"rules", "update"},
		{"rules", "delete"},
		{"budgets", "list"},
		{"budgets", "flexible", "set"},
		{"budgets", "flex-rollover", "set"},
	} {
		cmd, _, err := app.Root.Find(path)
		if err != nil || cmd == nil || cmd.Name() != path[len(path)-1] {
			t.Fatalf("Find(%v) = %#v, %v", path, cmd, err)
		}
	}
}

func TestAppRootRegistersAccounts(t *testing.T) {
	app, _ := newTestApp(t)
	for _, path := range [][]string{
		{"accounts", "list"},
		{"accounts", "balance-at"},
		{"accounts", "refresh"},
		{"accounts", "upload-history"},
		{"accounts", "recent-balances"},
		{"accounts", "snapshots"},
		{"accounts", "aggregate-snapshots"},
		{"networth"},
	} {
		cmd, _, err := app.Root.Find(path)
		if err != nil || cmd == nil || cmd.Name() != path[len(path)-1] {
			t.Fatalf("Find(%v) = %#v, %v", path, cmd, err)
		}
	}
}

func TestAppRootRegistersTransactions(t *testing.T) {
	app, _ := newTestApp(t)
	for _, path := range [][]string{
		{"transactions", "list"},
		{"transactions", "summary"},
		{"transactions", "bulk-categorize"},
		{"transactions", "tags", "add"},
		{"transactions", "tags", "clear"},
		{"transactions", "attachments", "list"},
		{"transactions", "attachments", "download"},
		{"transactions", "attachments", "upload"},
	} {
		cmd, _, err := app.Root.Find(path)
		if err != nil || cmd == nil || cmd.Name() != path[len(path)-1] {
			t.Fatalf("Find(%v) = %#v, %v", path, cmd, err)
		}
	}
}

func TestAppRootRegistersAnalyze(t *testing.T) {
	app, _ := newTestApp(t)
	for _, name := range []string{"anomalies", "subscriptions", "merchants", "burn-rate"} {
		cmd, _, err := app.Root.Find([]string{"analyze", name})
		if err != nil || cmd == nil || cmd.Name() != name {
			t.Fatalf("Find(analyze %s) = %#v, %v", name, cmd, err)
		}
	}
}

func TestAppRootRegistersSimpleReadCommands(t *testing.T) {
	app, _ := newTestApp(t)
	tests := []struct {
		args []string
		name string
	}{
		{args: []string{"credit", "history"}, name: "history"},
		{args: []string{"subscription", "show"}, name: "show"},
		{args: []string{"institutions", "list"}, name: "list"},
		{args: []string{"goals", "list"}, name: "list"},
		{args: []string{"goals", "budgets"}, name: "budgets"},
	}

	for _, tt := range tests {
		cmd, _, err := app.Root.Find(tt.args)
		if err != nil {
			t.Fatalf("Find(%v) error = %v", tt.args, err)
		}
		if cmd == nil || cmd.Name() != tt.name {
			t.Fatalf("Find(%v) = %#v", tt.args, cmd)
		}
	}

	for _, name := range []string{"credit", "subscription", "institutions", "goals"} {
		cmd, _, err := app.Root.Find([]string{name})
		if err != nil {
			t.Fatalf("Find(%s) error = %v", name, err)
		}
		if cmd.GroupID != "core" {
			t.Fatalf("%s GroupID = %q, want core", name, cmd.GroupID)
		}
	}
}

func TestAppCreditHistoryUsesInjectedServiceDeps(t *testing.T) {
	dir := t.TempDir()
	sessionPath := filepath.Join(dir, "session.json")
	saveTestSession(t, sessionPath)

	var out bytes.Buffer
	var gotAuth string
	app := New(Deps{
		LoadConfig: func(string) (*config.Config, error) {
			return &config.Config{
				Profile:     "default",
				APIEndpoint: "https://example.invalid/graphql",
				Timeout:     time.Second,
				SessionPath: sessionPath,
				AuditLog:    true,
			}, nil
		},
		Getenv:       func(string) string { return "" },
		NewRequestID: func() string { return "request-id" },
		Stdout:       &out,
		Stderr:       io.Discard,
		Exit:         func(int) {},
		HTTPTransport: testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
			gotAuth = req.Header.Get("Authorization")
			return testutil.JSONResponse(`{"data":{"creditScoreSnapshots":[{"reportedDate":"2026-05-01","score":790,"user":{"id":"u-1"}}]}}`), nil
		}),
	})

	app.Root.SetArgs([]string{"--json", "credit", "history"})
	if err := app.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if gotAuth != "Token test-token" {
		t.Fatalf("Authorization header = %q, want Token test-token", gotAuth)
	}
	if got := out.String(); !strings.Contains(got, `"command":"credit.history"`) || !strings.Contains(got, `"score":790`) {
		t.Fatalf("output = %q, want credit history JSON", got)
	}
}

func TestAppRootRegistersCashflowAndInvestments(t *testing.T) {
	app, _ := newTestApp(t)
	tests := []struct {
		args []string
		name string
	}{
		{args: []string{"cashflow", "list"}, name: "list"},
		{args: []string{"cashflow", "summary"}, name: "summary"},
		{args: []string{"cashflow", "categories"}, name: "categories"},
		{args: []string{"cashflow", "merchants"}, name: "merchants"},
		{args: []string{"cashflow", "trends"}, name: "trends"},
		{args: []string{"cashflow", "spending"}, name: "spending"},
		{args: []string{"investments", "portfolio"}, name: "portfolio"},
		{args: []string{"investments", "performance"}, name: "performance"},
	}

	for _, tt := range tests {
		cmd, _, err := app.Root.Find(tt.args)
		if err != nil {
			t.Fatalf("Find(%v) error = %v", tt.args, err)
		}
		if cmd == nil || cmd.Name() != tt.name {
			t.Fatalf("Find(%v) = %#v", tt.args, cmd)
		}
	}

	cashflowCmd, _, err := app.Root.Find([]string{"cashflow"})
	if err != nil {
		t.Fatalf("Find(cashflow) error = %v", err)
	}
	if cashflowCmd.GroupID != "core" {
		t.Fatalf("cashflow GroupID = %q, want core", cashflowCmd.GroupID)
	}
	if cashflowCmd.PersistentFlags().Lookup("from") == nil || cashflowCmd.PersistentFlags().Lookup("to") == nil {
		t.Fatal("cashflow missing parent date flags")
	}

	trendsCmd, _, err := app.Root.Find([]string{"cashflow", "trends"})
	if err != nil {
		t.Fatalf("Find(cashflow trends) error = %v", err)
	}
	for _, flag := range []string{"from", "to", "group-by", "period", "account-id", "category-id"} {
		if trendsCmd.Flags().Lookup(flag) == nil {
			t.Fatalf("cashflow trends missing --%s flag", flag)
		}
	}

	investmentsCmd, _, err := app.Root.Find([]string{"investments"})
	if err != nil {
		t.Fatalf("Find(investments) error = %v", err)
	}
	if investmentsCmd.GroupID != "core" {
		t.Fatalf("investments GroupID = %q, want core", investmentsCmd.GroupID)
	}
	performanceCmd, _, err := app.Root.Find([]string{"investments", "performance"})
	if err != nil {
		t.Fatalf("Find(investments performance) error = %v", err)
	}
	for _, flag := range []string{"security-id", "from", "to", "values"} {
		if performanceCmd.Flags().Lookup(flag) == nil {
			t.Fatalf("investments performance missing --%s flag", flag)
		}
	}
}

func TestAppCashflowSummaryUsesInjectedServiceDeps(t *testing.T) {
	dir := t.TempDir()
	sessionPath := filepath.Join(dir, "session.json")
	saveTestSession(t, sessionPath)

	var out bytes.Buffer
	var gotAuth string
	var gotStartDate string
	var gotEndDate string
	app := New(Deps{
		LoadConfig: func(string) (*config.Config, error) {
			return &config.Config{Profile: "default", APIEndpoint: "https://example.invalid/graphql", Timeout: time.Second, SessionPath: sessionPath, AuditLog: true}, nil
		},
		Getenv:       func(string) string { return "" },
		NewRequestID: func() string { return "request-id" },
		Stdout:       &out,
		Stderr:       io.Discard,
		Exit:         func(int) {},
		HTTPTransport: testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
			gotAuth = req.Header.Get("Authorization")
			var gqlReq struct {
				OperationName string         `json:"operationName"`
				Variables     map[string]any `json:"variables"`
			}
			if err := json.NewDecoder(req.Body).Decode(&gqlReq); err != nil {
				t.Fatalf("Decode request error = %v", err)
			}
			if gqlReq.OperationName != "GetCashflowSummary" {
				t.Fatalf("operation = %q, want GetCashflowSummary", gqlReq.OperationName)
			}
			filters, ok := gqlReq.Variables["filters"].(map[string]any)
			if !ok {
				t.Fatalf("filters = %#v, want object", gqlReq.Variables["filters"])
			}
			gotStartDate, _ = filters["startDate"].(string)
			gotEndDate, _ = filters["endDate"].(string)
			return testutil.JSONResponse(`{"data":{"aggregates":[{"summary":{"sumIncome":8500,"sumExpense":6200,"savings":2300,"savingsRate":0.2706}}]}}`), nil
		}),
	})

	app.Root.SetArgs([]string{"--json", "cashflow", "--from", "2026-01-01", "--to", "2026-03-31", "summary"})
	if err := app.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if gotAuth != "Token test-token" {
		t.Fatalf("Authorization header = %q, want Token test-token", gotAuth)
	}
	if gotStartDate != "2026-01-01" || gotEndDate != "2026-03-31" {
		t.Fatalf("cashflow dates = %q to %q, want 2026-01-01 to 2026-03-31", gotStartDate, gotEndDate)
	}
	if got := out.String(); !strings.Contains(got, `"command":"cashflow.summary"`) || !strings.Contains(got, `"income":8500`) {
		t.Fatalf("output = %q, want cashflow summary JSON", got)
	}
}

func TestAppCashflowTrendsValidation(t *testing.T) {
	var out bytes.Buffer
	exitCode := 0
	app := New(Deps{
		LoadConfig: func(string) (*config.Config, error) {
			return &config.Config{Profile: "default", Timeout: time.Second, SessionPath: config.DefaultSessionPath(), AuditLog: true}, nil
		},
		Getenv:       func(string) string { return "" },
		NewRequestID: func() string { return "request-id" },
		Stdout:       &out,
		Stderr:       io.Discard,
		Exit: func(code int) {
			exitCode = code
		},
	})

	app.Root.SetArgs([]string{"--json", "cashflow", "trends"})
	if err := app.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if exitCode != 2 {
		t.Fatalf("exitCode = %d, want 2", exitCode)
	}
	if got := out.String(); !strings.Contains(got, `"command":"cashflow.trends"`) || !strings.Contains(got, "--from is required") {
		t.Fatalf("output = %q, want trends validation error", got)
	}
}

func TestAppInvestmentsPerformanceUsesInjectedServiceDeps(t *testing.T) {
	dir := t.TempDir()
	sessionPath := filepath.Join(dir, "session.json")
	saveTestSession(t, sessionPath)

	var out bytes.Buffer
	var gotSecurityID string
	var gotStartDate string
	var gotEndDate string
	app := New(Deps{
		LoadConfig: func(string) (*config.Config, error) {
			return &config.Config{Profile: "default", APIEndpoint: "https://example.invalid/graphql", Timeout: time.Second, SessionPath: sessionPath, AuditLog: true}, nil
		},
		Getenv:       func(string) string { return "" },
		NewRequestID: func() string { return "request-id" },
		Stdout:       &out,
		Stderr:       io.Discard,
		Exit:         func(int) {},
		HTTPTransport: testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
			var gqlReq struct {
				OperationName string         `json:"operationName"`
				Variables     map[string]any `json:"variables"`
			}
			if err := json.NewDecoder(req.Body).Decode(&gqlReq); err != nil {
				t.Fatalf("Decode request error = %v", err)
			}
			if gqlReq.OperationName != "Web_GetSecuritiesHistoricalPerformance" {
				t.Fatalf("operation = %q, want Web_GetSecuritiesHistoricalPerformance", gqlReq.OperationName)
			}
			input, ok := gqlReq.Variables["input"].(map[string]any)
			if !ok {
				t.Fatalf("input = %#v, want object", gqlReq.Variables["input"])
			}
			securityIDs, ok := input["securityIds"].([]any)
			if !ok || len(securityIDs) == 0 {
				t.Fatalf("securityIds = %#v, want non-empty array", input["securityIds"])
			}
			gotSecurityID, _ = securityIDs[0].(string)
			gotStartDate, _ = input["startDate"].(string)
			gotEndDate, _ = input["endDate"].(string)
			return testutil.JSONResponse(`{"data":{"securityHistoricalPerformance":[{"security":{"id":"sec-1","ticker":"ABC","name":"ABC Fund"},"historicalChart":[{"date":"2026-01-01","returnPercent":0.1}]}]}}`), nil
		}),
	})

	app.Root.SetArgs([]string{"--json", "investments", "performance", "--security-id", "sec-1", "--from", "2026-01-01", "--to", "2026-05-01"})
	if err := app.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if gotSecurityID != "sec-1" || gotStartDate != "2026-01-01" || gotEndDate != "2026-05-01" {
		t.Fatalf("performance input = %q %q %q, want sec-1 2026-01-01 2026-05-01", gotSecurityID, gotStartDate, gotEndDate)
	}
	if got := out.String(); !strings.Contains(got, `"command":"investments.performance"`) || !strings.Contains(got, `"ticker":"ABC"`) {
		t.Fatalf("output = %q, want investments performance JSON", got)
	}
}
