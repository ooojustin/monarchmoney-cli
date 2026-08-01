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

	"github.com/thedavidweng/monarchmoney-cli/internal/cache"
	"github.com/thedavidweng/monarchmoney-cli/internal/config"
	"github.com/thedavidweng/monarchmoney-cli/internal/testutil"
)

func newTestCacheApp(t *testing.T, cachePath string, transport http.RoundTripper, exitCode *int) (*App, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()

	var out bytes.Buffer
	var errOut bytes.Buffer
	app := New(Deps{
		LoadConfig: func(string) (*config.Config, error) {
			return &config.Config{Profile: "default", APIEndpoint: "https://example.invalid/graphql", Timeout: time.Second, SessionPath: config.DefaultSessionPath(), CachePath: cachePath, AuditLog: true}, nil
		},
		Getenv:        func(string) string { return "" },
		NewRequestID:  func() string { return "request-id" },
		Stdout:        &out,
		Stderr:        &errOut,
		Exit:          func(code int) { *exitCode = code },
		HTTPTransport: transport,
	})
	return app, &out, &errOut
}

type appGraphQLRequest struct {
	OperationName string         `json:"operationName"`
	Variables     map[string]any `json:"variables"`
}

func decodeGraphQLRequest(t *testing.T, req *http.Request) appGraphQLRequest {
	t.Helper()

	var gqlReq appGraphQLRequest
	if err := json.NewDecoder(req.Body).Decode(&gqlReq); err != nil {
		t.Fatalf("decode GraphQL request: %v", err)
	}
	return gqlReq
}

func TestAppRootRegistersCache(t *testing.T) {
	app, _ := newTestApp(t)

	cacheCmd, _, err := app.Root.Find([]string{"cache"})
	if err != nil {
		t.Fatalf("Find(cache) error = %v", err)
	}
	if cacheCmd == nil || cacheCmd.Name() != "cache" {
		t.Fatalf("Find(cache) = %#v", cacheCmd)
	}
	if cacheCmd.GroupID != "utility" {
		t.Fatalf("cache GroupID = %q, want utility", cacheCmd.GroupID)
	}
	for _, example := range []string{"monarch cache sync", `monarch cache search "grocery"`, "monarch cache stats"} {
		if !strings.Contains(cacheCmd.Example, example) {
			t.Fatalf("cache example = %q, want %q", cacheCmd.Example, example)
		}
	}

	tests := []struct {
		args  []string
		name  string
		flags []string
	}{
		{args: []string{"cache", "sync"}, name: "sync", flags: []string{"from", "limit", "all"}},
		{args: []string{"cache", "search"}, name: "search"},
		{args: []string{"cache", "stats"}, name: "stats"},
		{args: []string{"cache", "cleanup"}, name: "cleanup", flags: []string{"before"}},
	}
	for _, tt := range tests {
		cmd, _, err := app.Root.Find(tt.args)
		if err != nil {
			t.Fatalf("Find(%v) error = %v", tt.args, err)
		}
		if cmd == nil || cmd.Name() != tt.name {
			t.Fatalf("Find(%v) = %#v", tt.args, cmd)
		}
		for _, flag := range tt.flags {
			if cmd.Flags().Lookup(flag) == nil {
				t.Fatalf("%s missing --%s flag", strings.Join(tt.args, " "), flag)
			}
		}
	}

	syncCmd, _, err := app.Root.Find([]string{"cache", "sync"})
	if err != nil {
		t.Fatalf("Find(cache sync) error = %v", err)
	}
	if got := syncCmd.Flags().Lookup("limit").DefValue; got != "1000" {
		t.Fatalf("cache sync --limit default = %q, want 1000", got)
	}
	if got := syncCmd.Flags().Lookup("all").DefValue; got != "false" {
		t.Fatalf("cache sync --all default = %q, want false", got)
	}
}

func TestAppCacheSyncUsesInjectedDepsAndConfiguredCache(t *testing.T) {
	dir := t.TempDir()
	sessionPath := filepath.Join(dir, "session.json")
	cachePath := filepath.Join(dir, "cache.sqlite")
	saveTestSession(t, sessionPath)

	exitCode := 0
	var gotAuth string
	var gotStartDate string
	var gotLimit float64
	app, out, _ := newTestCacheApp(t, cachePath, testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		gotAuth = req.Header.Get("Authorization")
		gqlReq := decodeGraphQLRequest(t, req)
		switch gqlReq.OperationName {
		case "GetAccounts":
			return testutil.JSONResponse(`{"data":{"accounts":[{"id":"acc_1","displayName":"Checking","type":{"name":"cash"},"subtype":{"name":"checking"},"displayBalance":1250.5,"updatedAt":"2026-05-09T10:00:00Z"}]}}`), nil
		case "GetTransactionsList":
			gotLimit, _ = gqlReq.Variables["limit"].(float64)
			filters := gqlReq.Variables["filters"].(map[string]any)
			gotStartDate, _ = filters["startDate"].(string)
			return testutil.JSONResponse(`{"data":{"allTransactions":{"results":[{"id":"tx_1","date":"2026-05-09","amount":-12.34,"merchant":{"name":"Cafe"},"category":{"name":"Dining"},"account":{"id":"acc_1"},"notes":"latte"}],"totalCount":1}}}`), nil
		default:
			t.Fatalf("unexpected operation %q", gqlReq.OperationName)
		}
		return nil, nil
	}), &exitCode)
	app.Deps.SessionPath = func() string { return sessionPath }

	app.Root.SetArgs([]string{"--json", "cache", "sync", "--from", "2026-01-01", "--limit", "2"})
	if err := app.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", exitCode, out.String())
	}
	if gotAuth != "Token test-token" {
		t.Fatalf("Authorization header = %q, want Token test-token", gotAuth)
	}
	if gotStartDate != "2026-01-01" || gotLimit != 2 {
		t.Fatalf("transaction request startDate=%q limit=%v, want 2026-01-01 and 2", gotStartDate, gotLimit)
	}
	if got := out.String(); !strings.Contains(got, `"command":"cache.sync"`) || !strings.Contains(got, `"accounts":1`) || !strings.Contains(got, `"transactions":1`) {
		t.Fatalf("output = %q, want cache sync count envelope", got)
	}

	store, err := cache.NewStore(cachePath)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	defer store.Close() //nolint:errcheck // test cleanup
	txs, err := store.SearchTransactions("Cafe")
	if err != nil {
		t.Fatalf("SearchTransactions() error = %v", err)
	}
	if len(txs) != 1 || txs[0].AccountID != "acc_1" {
		t.Fatalf("cached transaction = %#v, want account id acc_1", txs)
	}
	stats, err := store.GetStats()
	if err != nil {
		t.Fatalf("GetStats() error = %v", err)
	}
	if _, ok := stats["last_synced_at"]; !ok {
		t.Fatalf("stats = %#v, want last_synced_at after sync", stats)
	}
}

func TestAppCacheSyncAllPaginates(t *testing.T) {
	dir := t.TempDir()
	sessionPath := filepath.Join(dir, "session.json")
	cachePath := filepath.Join(dir, "cache.sqlite")
	saveTestSession(t, sessionPath)

	exitCode := 0
	var offsets []float64
	app, _, _ := newTestCacheApp(t, cachePath, testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		gqlReq := decodeGraphQLRequest(t, req)
		switch gqlReq.OperationName {
		case "GetAccounts":
			return testutil.JSONResponse(`{"data":{"accounts":[{"id":"acc_1","displayName":"Checking","type":{"name":"cash"},"subtype":{"name":"checking"},"displayBalance":1250.5,"updatedAt":"2026-05-09"}]}}`), nil
		case "GetTransactionsList":
			offset, _ := gqlReq.Variables["offset"].(float64)
			limit, _ := gqlReq.Variables["limit"].(float64)
			if limit != 1 {
				t.Fatalf("limit = %v, want 1", limit)
			}
			offsets = append(offsets, offset)
			if offset == 0 {
				return testutil.JSONResponse(`{"data":{"allTransactions":{"results":[{"id":"tx_1","date":"2026-05-09","amount":-12.34,"merchant":{"name":"Cafe"},"category":{"name":"Dining"},"account":{"id":"acc_1"}}],"totalCount":2}}}`), nil
			}
			return testutil.JSONResponse(`{"data":{"allTransactions":{"results":[{"id":"tx_2","date":"2026-05-10","amount":-9.99,"merchant":{"name":"Deli"},"category":{"name":"Dining"},"account":{"id":"acc_1"}}],"totalCount":2}}}`), nil
		default:
			t.Fatalf("unexpected operation %q", gqlReq.OperationName)
		}
		return nil, nil
	}), &exitCode)
	app.Deps.SessionPath = func() string { return sessionPath }

	app.Root.SetArgs([]string{"--json", "cache", "sync", "--all", "--limit", "1"})
	if err := app.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("exitCode = %d", exitCode)
	}
	if len(offsets) != 2 || offsets[0] != 0 || offsets[1] != 1 {
		t.Fatalf("offsets = %v, want [0 1]", offsets)
	}
}

func TestAppCacheLocalCommandsUseConfiguredCacheWithoutSession(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "configured.sqlite")
	store, err := cache.NewStore(cachePath)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	if err := store.SaveTransactions([]cache.Transaction{
		{ID: "tx_old", Date: time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC), Amount: -10, Merchant: "Cafe", Category: "Dining", Notes: "old"},
		{ID: "tx_new", Date: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), Amount: -20, Merchant: "Store", Category: "Shopping", Notes: "new"},
	}); err != nil {
		t.Fatalf("SaveTransactions() error = %v", err)
	}
	if err := store.RecordSync(0, 2); err != nil {
		t.Fatalf("RecordSync() error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	exitCode := 0
	app, out, _ := newTestCacheApp(t, cachePath, testutil.RoundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("local cache command should not make API requests")
		return nil, nil
	}), &exitCode)
	app.Root.SetArgs([]string{"cache", "search", "Cafe"})
	if err := app.Execute(); err != nil {
		t.Fatalf("Execute(search) error = %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("search exitCode = %d", exitCode)
	}
	if got := out.String(); !strings.Contains(got, "Cafe") || !strings.Contains(got, "Total matches: 1") {
		t.Fatalf("search output = %q, want Cafe result", got)
	}

	exitCode = 0
	app, out, _ = newTestCacheApp(t, cachePath, nil, &exitCode)
	app.Root.SetArgs([]string{"--json", "cache", "stats"})
	if err := app.Execute(); err != nil {
		t.Fatalf("Execute(stats) error = %v", err)
	}
	if got := out.String(); !strings.Contains(got, `"command":"cache.stats"`) || !strings.Contains(got, `"transactions":2`) || !strings.Contains(got, `"last_synced_at"`) {
		t.Fatalf("stats output = %q, want configured cache stats", got)
	}

	exitCode = 0
	app, out, _ = newTestCacheApp(t, cachePath, nil, &exitCode)
	app.Root.SetArgs([]string{"--json", "cache", "cleanup", "--before", "2026-01-01"})
	if err := app.Execute(); err != nil {
		t.Fatalf("Execute(cleanup) error = %v", err)
	}
	if got := out.String(); !strings.Contains(got, `"command":"cache.cleanup"`) || !strings.Contains(got, `"deleted":1`) {
		t.Fatalf("cleanup output = %q, want one deleted transaction", got)
	}
}

func TestAppCacheValidationBeforeSideEffects(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "cache.sqlite")

	exitCode := 0
	app, out, _ := newTestCacheApp(t, cachePath, testutil.RoundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("cache sync should validate --from before API requests")
		return nil, nil
	}), &exitCode)
	app.Deps.SessionPath = func() string { return filepath.Join(dir, "missing-session.json") }
	app.Root.SetArgs([]string{"--json", "cache", "sync", "--from", "01-01-2026"})
	if err := app.Execute(); err != nil {
		t.Fatalf("Execute(sync) error = %v", err)
	}
	if exitCode == 0 {
		t.Fatalf("exitCode = 0, want validation failure; output=%q", out.String())
	}
	if got := out.String(); !strings.Contains(got, `"command":"cache.sync"`) || !strings.Contains(got, "YYYY-MM-DD") {
		t.Fatalf("sync output = %q, want date validation envelope", got)
	}

	exitCode = 0
	app, out, _ = newTestCacheApp(t, cachePath, nil, &exitCode)
	app.Root.SetArgs([]string{"--json", "cache", "cleanup", "--before", "not-a-date"})
	if err := app.Execute(); err != nil {
		t.Fatalf("Execute(cleanup) error = %v", err)
	}
	if exitCode == 0 {
		t.Fatalf("exitCode = 0, want validation failure; output=%q", out.String())
	}
	if got := out.String(); !strings.Contains(got, `"command":"cache.cleanup"`) || !strings.Contains(got, "YYYY-MM-DD") {
		t.Fatalf("cleanup output = %q, want date validation envelope", got)
	}

	if _, err := os.Stat(cachePath); !os.IsNotExist(err) {
		t.Fatalf("cache path stat error = %v, want cache file not created during validation failures", err)
	}
}

func TestAppCacheSyncFailsWhenTransactionsAPIFails(t *testing.T) {
	dir := t.TempDir()
	sessionPath := filepath.Join(dir, "session.json")
	cachePath := filepath.Join(dir, "cache.sqlite")
	saveTestSession(t, sessionPath)

	exitCode := 0
	app, out, _ := newTestCacheApp(t, cachePath, testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		gqlReq := decodeGraphQLRequest(t, req)
		switch gqlReq.OperationName {
		case "GetAccounts":
			return testutil.JSONResponse(`{"data":{"accounts":[{"id":"acc_1","displayName":"Checking","type":{"name":"cash"},"subtype":{"name":"checking"},"displayBalance":1250.5,"updatedAt":"2026-05-09"}]}}`), nil
		case "GetTransactionsList":
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(bytes.NewReader(nil)),
			}, nil
		default:
			t.Fatalf("unexpected operation %q", gqlReq.OperationName)
		}
		return nil, nil
	}), &exitCode)
	app.Deps.SessionPath = func() string { return sessionPath }

	app.Root.SetArgs([]string{"--json", "cache", "sync"})
	if err := app.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if exitCode == 0 {
		t.Fatalf("exitCode = 0, want API failure; output=%q", out.String())
	}
	if got := out.String(); !strings.Contains(got, `"command":"cache.sync"`) || !strings.Contains(got, "failed to sync transactions") {
		t.Fatalf("output = %q, want transaction sync failure", got)
	}
}

func TestAppCacheStatsJSONShape(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "cache.sqlite")
	store, err := cache.NewStore(cachePath)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	if err := store.SaveTransactions([]cache.Transaction{{ID: "tx_1", Date: time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC), Merchant: "Cafe"}}); err != nil {
		t.Fatalf("SaveTransactions() error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	exitCode := 0
	app, out, _ := newTestCacheApp(t, cachePath, nil, &exitCode)
	app.Root.SetArgs([]string{"--json", "cache", "stats"})
	if err := app.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", exitCode, out.String())
	}

	var env struct {
		OK   bool           `json:"ok"`
		Data map[string]any `json:"data"`
		Meta struct {
			Command string `json:"command"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &env); err != nil {
		t.Fatalf("json.Unmarshal() error = %v; output=%q", err, out.String())
	}
	if !env.OK || env.Meta.Command != "cache.stats" || env.Data["transactions"] != float64(1) {
		t.Fatalf("stats envelope = %#v", env)
	}
}
