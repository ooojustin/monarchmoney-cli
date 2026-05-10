package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thedavidweng/monarchmoney-cli/internal/cache"
)

// TestCacheSyncPassesFromDateAndPersistsAccountID is the regression that
// originally failed in upstream/main: the cache sync command must use the
// injected session path (via Deps), not config.DefaultSessionPath() directly.
// Under the App+Deps architecture this works by construction.
func TestCacheSyncPassesFromDateAndPersistsAccountID(t *testing.T) {
	dir := t.TempDir()
	sessionPath := filepath.Join(dir, "session.json")
	cachePath := filepath.Join(dir, "cache.sqlite")
	app, buf, exitCode := newTestApp(t, sessionPath)
	saveTestSession(t, sessionPath)
	app.Deps.Viper.Set("cache_path", cachePath)

	var sawStartDate bool
	app.Deps.HTTPTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		var gqlReq struct {
			OperationName string                 `json:"operationName"`
			Variables     map[string]interface{} `json:"variables"`
		}
		if err := json.NewDecoder(req.Body).Decode(&gqlReq); err != nil {
			t.Fatalf("Decode request error = %v", err)
		}
		switch gqlReq.OperationName {
		case "GetAccounts":
			return jsonHTTPResponse(`{"data":{"accounts":[{"id":"acc_1","displayName":"Checking","type":{"name":"cash","display":"Cash"},"subtype":{"name":"checking","display":"Checking"},"displayBalance":1250.5,"currentBalance":1250.5,"updatedAt":"2026-05-09T10:00:00Z","displayLastUpdatedAt":"2026-05-09","createdAt":"2026-01-01T00:00:00Z"}]}}`), nil
		case "GetTransactionsList":
			filters, _ := gqlReq.Variables["filters"].(map[string]interface{})
			if filters["startDate"] == "2026-01-01" {
				sawStartDate = true
			}
			return jsonHTTPResponse(`{"data":{"allTransactions":{"results":[{"id":"tx_1","date":"2026-05-09","amount":-12.34,"merchant":{"name":"Cafe"},"category":{"name":"Dining"},"account":{"id":"acc_1"},"notes":"latte"}],"totalCount":1}}}`), nil
		default:
			t.Fatalf("unexpected operation %q", gqlReq.OperationName)
			return nil, nil
		}
	})

	cmd, _, err := app.Root.Find([]string{"cache", "sync"})
	if err != nil {
		t.Fatalf("Find cache sync = %v", err)
	}
	_ = cmd.Flags().Set("from", "2026-01-01")
	cmd.SetContext(context.Background())
	cmd.Run(cmd, nil)

	if *exitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", *exitCode, buf.String())
	}
	if !sawStartDate {
		t.Fatal("cache sync did not pass --from as transaction startDate")
	}

	store, err := cache.NewStore(cachePath)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	txs, err := store.SearchTransactions("Cafe")
	if err != nil {
		t.Fatalf("SearchTransactions() error = %v", err)
	}
	if len(txs) != 1 || txs[0].AccountID != "acc_1" {
		t.Fatalf("cached transaction = %#v, want account id acc_1", txs)
	}
}

// TestCacheSyncRejectsInvalidFromDate verifies that --from is validated
// before any API call is made.
func TestCacheSyncRejectsInvalidFromDate(t *testing.T) {
	dir := t.TempDir()
	sessionPath := filepath.Join(dir, "session.json")
	app, buf, exitCode := newTestApp(t, sessionPath)
	saveTestSession(t, sessionPath)

	app.Deps.HTTPTransport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("cache sync should validate --from before making API requests")
		return nil, nil
	})

	cmd, _, _ := app.Root.Find([]string{"cache", "sync"})
	_ = cmd.Flags().Set("from", "01-01-2026")
	cmd.SetContext(context.Background())
	cmd.Run(cmd, nil)

	if *exitCode == 0 {
		t.Fatalf("exitCode = 0, want validation failure; output=%q", buf.String())
	}
	if !strings.Contains(buf.String(), "YYYY-MM-DD") {
		t.Fatalf("output = %q, want date format guidance", buf.String())
	}
}

// TestCacheCleanupValidatesDate covers the cleanup command's date validation
// and configured cache path resolution.
func TestCacheCleanupValidatesDate(t *testing.T) {
	dir := t.TempDir()
	sessionPath := filepath.Join(dir, "session.json")
	cachePath := filepath.Join(dir, "configured.sqlite")
	app, buf, exitCode := newTestApp(t, sessionPath)
	app.Deps.Viper.Set("cache_path", cachePath)

	t.Setenv("HOME", filepath.Join(dir, "home"))
	_ = os.MkdirAll(filepath.Join(dir, "home"), 0700)

	store, err := cache.NewStore(cachePath)
	if err != nil {
		t.Fatalf("NewStore(configured) error = %v", err)
	}
	if err := store.SaveTransactions([]cache.Transaction{{
		ID:       "tx_old",
		Date:     time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC),
		Merchant: "Old",
	}}); err != nil {
		t.Fatalf("SaveTransactions() error = %v", err)
	}

	cmd, _, _ := app.Root.Find([]string{"cache", "cleanup"})
	_ = cmd.Flags().Set("before", "2026-01-01")
	cmd.SetContext(context.Background())
	cmd.Run(cmd, nil)

	if *exitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", *exitCode, buf.String())
	}
	stats, err := store.GetStats()
	if err != nil {
		t.Fatalf("GetStats() error = %v", err)
	}
	if got := stats["transactions"]; got != 0 {
		t.Fatalf("transactions = %d, want 0", got)
	}

	app2, buf2, exitCode2 := newTestApp(t, sessionPath)
	cmd2, _, _ := app2.Root.Find([]string{"cache", "cleanup"})
	_ = cmd2.Flags().Set("before", "not-a-date")
	cmd2.SetContext(context.Background())
	cmd2.Run(cmd2, nil)
	if *exitCode2 == 0 {
		t.Fatalf("exitCode = 0, want validation failure; output=%q", buf2.String())
	}
	if !strings.Contains(buf2.String(), "YYYY-MM-DD") {
		t.Fatalf("output = %q, want date format guidance", buf2.String())
	}
}
