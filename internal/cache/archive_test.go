package cache

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/ncruces/go-sqlite3/driver"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := NewStore(filepath.Join(t.TempDir(), "monarch.sqlite"))
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestArchiveRoundTripPersistsFullFidelity(t *testing.T) {
	store := openTestStore(t)

	mustNoError(t, store.SaveAccounts([]Account{{
		ID:             "acc_1",
		DisplayName:    "Brokerage",
		AccountType:    "brokerage",
		TypeGroup:      "investment",
		DisplayBalance: 9500.25,
		CurrentBalance: 9400.75,
		IsManual:       true,
		IsHidden:       true,
		IsClosed:       true,
		UpdatedAt:      time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC),
	}}), "SaveAccounts()")

	mustNoError(t, store.SaveTransactions([]Transaction{{
		ID:                  "tx_1",
		Date:                time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		Amount:              -42.75,
		Merchant:            "Coffee Shop",
		PlaidName:           "COFFEE SHOP INC",
		ProviderDescription: "Blue Bottle Coffee",
		Category:            "Dining",
		CategoryGroup:       "Food & Drink",
		CategoryGroupType:   "expense",
		Notes:               "Morning latte",
		Pending:             true,
		ReviewStatus:        "needs_review",
		NeedsReview:         true,
		GoalID:              "goal_1",
		GoalName:            "House fund",
		AccountID:           "acc_1",
		Tags: []Tag{
			{ID: "tag_1", Name: "work"},
			{ID: "tag_2", Name: "reimbursable"},
		},
		Splits: []Split{
			{ID: "split_1", Amount: -30.00, Category: "Dining", Merchant: "Coffee Shop", Notes: "lattes"},
			{ID: "split_2", Amount: -12.75, Category: "Gifts", Merchant: "Coffee Shop", Notes: "gift card"},
		},
	}}), "SaveTransactions()")

	mustNoError(t, store.SaveHoldings([]Holding{{
		ID:        "h_1",
		Ticker:    "VTI",
		Name:      "Vanguard Total Stock Market ETF",
		Quantity:  10.5,
		Basis:     2500.00,
		Value:     3000.50,
		AccountID: "acc_1",
	}}), "SaveHoldings()")

	var typeGroup string
	var currentBalance float64
	var isManual, isHidden, isClosed int
	if err := store.db.QueryRow(
		`SELECT type_group, current_balance, is_manual, is_hidden, is_closed FROM accounts WHERE id = 'acc_1'`,
	).Scan(&typeGroup, &currentBalance, &isManual, &isHidden, &isClosed); err != nil {
		t.Fatalf("query account row: %v", err)
	}
	if typeGroup != "investment" || currentBalance != 9400.75 {
		t.Fatalf("account archive fields = %q %v, want investment 9400.75", typeGroup, currentBalance)
	}
	if isManual != 1 || isHidden != 1 || isClosed != 1 {
		t.Fatalf("account lifecycle flags = %d/%d/%d, want 1/1/1", isManual, isHidden, isClosed)
	}

	assertTransactionRow(t, store)
	assertTagRows(t, store)
	assertSplitRows(t, store)
	assertHoldingRows(t, store)
}

func assertTransactionRow(t *testing.T, store *Store) {
	t.Helper()
	var (
		plaidName, providerDescription     string
		category, categoryGroup, groupType string
		pending, needsReview               int
		reviewStatus                       string
		goalID, goalName                   string
	)
	err := store.db.QueryRow(
		`SELECT plaid_name, provider_description, category, category_group, category_group_type,
		        pending, review_status, needs_review, goal_id, goal_name
		 FROM transactions WHERE id = 'tx_1'`,
	).Scan(&plaidName, &providerDescription, &category, &categoryGroup, &groupType,
		&pending, &reviewStatus, &needsReview, &goalID, &goalName)
	if err != nil {
		t.Fatalf("query transaction row: %v", err)
	}
	if plaidName != "COFFEE SHOP INC" || providerDescription != "Blue Bottle Coffee" {
		t.Fatalf("raw merchant names = %q / %q", plaidName, providerDescription)
	}
	if categoryGroup != "Food & Drink" || groupType != "expense" {
		t.Fatalf("category group = %q (%q)", categoryGroup, groupType)
	}
	if pending != 1 || needsReview != 1 || reviewStatus != "needs_review" {
		t.Fatalf("review state = pending %d, status %q, needsReview %d", pending, reviewStatus, needsReview)
	}
	if goalID != "goal_1" || goalName != "House fund" {
		t.Fatalf("goal linkage = %q / %q", goalID, goalName)
	}
}

func assertTagRows(t *testing.T, store *Store) {
	t.Helper()
	rows, err := store.db.Query(`SELECT tag_id, name FROM transaction_tags WHERE transaction_id = 'tx_1' ORDER BY tag_id`)
	if err != nil {
		t.Fatalf("query tags: %v", err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			t.Fatalf("scan tag: %v", err)
		}
		got = append(got, id+"="+name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate tags: %v", err)
	}
	if len(got) != 2 || got[0] != "tag_1=work" || got[1] != "tag_2=reimbursable" {
		t.Fatalf("tags = %v, want [tag_1=work tag_2=reimbursable]", got)
	}
}

func assertSplitRows(t *testing.T, store *Store) {
	t.Helper()
	rows, err := store.db.Query(`SELECT id, amount, category, merchant, notes FROM transaction_splits WHERE transaction_id = 'tx_1' ORDER BY id`)
	if err != nil {
		t.Fatalf("query splits: %v", err)
	}
	defer rows.Close()
	var ids []string
	var secondAmount float64
	for i := 0; rows.Next(); i++ {
		var id, category, merchant, notes string
		var amount float64
		if err := rows.Scan(&id, &amount, &category, &merchant, &notes); err != nil {
			t.Fatalf("scan split: %v", err)
		}
		ids = append(ids, id)
		if id == "split_2" {
			secondAmount = amount
			if category != "Gifts" || merchant != "Coffee Shop" || notes != "gift card" {
				t.Fatalf("split_2 detail = %q/%q/%q", category, merchant, notes)
			}
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate splits: %v", err)
	}
	if len(ids) != 2 || ids[0] != "split_1" || ids[1] != "split_2" {
		t.Fatalf("splits = %v, want [split_1 split_2]", ids)
	}
	if secondAmount != -12.75 {
		t.Fatalf("split_2 amount = %v, want -12.75", secondAmount)
	}
}

func assertHoldingRows(t *testing.T, store *Store) {
	t.Helper()
	var ticker, name string
	var quantity, basis, value float64
	var accountID string
	if err := store.db.QueryRow(
		`SELECT ticker, name, quantity, basis, value, account_id FROM holdings WHERE id = 'h_1'`,
	).Scan(&ticker, &name, &quantity, &basis, &value, &accountID); err != nil {
		t.Fatalf("query holding: %v", err)
	}
	if ticker != "VTI" || name != "Vanguard Total Stock Market ETF" {
		t.Fatalf("holding identity = %q / %q", ticker, name)
	}
	if quantity != 10.5 || basis != 2500.0 || value != 3000.5 || accountID != "acc_1" {
		t.Fatalf("holding numbers = %v/%v/%v for %q", quantity, basis, value, accountID)
	}
}

func TestSaveTransactionsReplacesTagsAndSplits(t *testing.T) {
	store := openTestStore(t)

	first := Transaction{
		ID:   "tx_1",
		Date: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		Tags: []Tag{{ID: "tag_1", Name: "old"}},
		Splits: []Split{
			{ID: "split_1", Amount: -30},
			{ID: "split_2", Amount: -12},
		},
	}
	mustNoError(t, store.SaveTransactions([]Transaction{first}), "SaveTransactions(first)")

	second := first
	second.Tags = []Tag{{ID: "tag_2", Name: "new"}}
	second.Splits = []Split{{ID: "split_1", Amount: -42}}
	mustNoError(t, store.SaveTransactions([]Transaction{second}), "SaveTransactions(second)")

	var tagCount, splitCount int
	mustNoError(t, store.db.QueryRow(`SELECT COUNT(*) FROM transaction_tags WHERE transaction_id = 'tx_1'`).Scan(&tagCount), "count tags")
	mustNoError(t, store.db.QueryRow(`SELECT COUNT(*) FROM transaction_splits WHERE transaction_id = 'tx_1'`).Scan(&splitCount), "count splits")
	if tagCount != 1 || splitCount != 1 {
		t.Fatalf("child rows after replace = %d tags / %d splits, want 1/1", tagCount, splitCount)
	}
}

func TestSaveHoldingsReplacesSnapshot(t *testing.T) {
	store := openTestStore(t)

	mustNoError(t, store.SaveHoldings([]Holding{
		{ID: "h_1", Ticker: "OLD", Quantity: 1},
		{ID: "h_2", Ticker: "KEEP", Quantity: 2},
	}), "SaveHoldings(first)")

	mustNoError(t, store.SaveHoldings([]Holding{{ID: "h_2", Ticker: "KEEP", Quantity: 3}}), "SaveHoldings(second)")

	var ids []string
	rows, err := store.db.Query(`SELECT id FROM holdings ORDER BY id`)
	mustNoError(t, err, "query holdings")
	for rows.Next() {
		var id string
		mustNoError(t, rows.Scan(&id), "scan holding id")
		ids = append(ids, id)
	}
	mustNoError(t, rows.Close(), "close rows")
	if len(ids) != 1 || ids[0] != "h_2" {
		t.Fatalf("holdings after replace = %v, want [h_2]", ids)
	}

	mustNoError(t, store.SaveHoldings(nil), "SaveHoldings(empty)")
	var count int
	mustNoError(t, store.db.QueryRow(`SELECT COUNT(*) FROM holdings`).Scan(&count), "count holdings")
	if count != 0 {
		t.Fatalf("holdings after empty snapshot = %d, want 0", count)
	}
}

func TestStatsIncludeHoldings(t *testing.T) {
	store := openTestStore(t)

	mustNoError(t, store.SaveHoldings([]Holding{
		{ID: "h_1", Ticker: "VTI"},
		{ID: "h_2", Ticker: "BND"},
	}), "SaveHoldings()")

	stats, err := store.GetStats()
	mustNoError(t, err, "GetStats()")
	assertStat(t, stats, "holdings", 2)
}

func TestSearchMatchesRawMerchantNamesAndTags(t *testing.T) {
	store := openTestStore(t)

	mustNoError(t, store.SaveTransactions([]Transaction{
		{
			ID:                  "tx_plaid",
			Date:                time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
			Merchant:            "Unhelpful LLC",
			PlaidName:           "BLUE BOTTLE COFFEE",
			Category:            "Dining",
			AccountID:           "acc_1",
			ProviderDescription: "Roasters United",
		},
		{
			ID:        "tx_tagged",
			Date:      time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC),
			Merchant:  "Bookstore",
			Category:  "Shopping",
			AccountID: "acc_1",
			Tags:      []Tag{{ID: "tag_1", Name: "vacation-2026"}},
		},
	}), "SaveTransactions()")

	for query, wantID := range map[string]string{
		"BLUE BOTTLE":   "tx_plaid",
		"Roasters":      "tx_plaid",
		"vacation-2026": "tx_tagged",
	} {
		matches, err := store.SearchTransactions(query)
		mustNoError(t, err, "SearchTransactions("+query+")")
		if len(matches) != 1 || matches[0].ID != wantID {
			t.Fatalf("SearchTransactions(%q) = %v, want [%s]", query, matches, wantID)
		}
	}
}

func TestCleanupRemovesOrphanedTagsAndSplits(t *testing.T) {
	store := openTestStore(t)

	kept := Transaction{ID: "tx_keep", Date: time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC), Tags: []Tag{{ID: "t1", Name: "keep"}}, Splits: []Split{{ID: "s1", Amount: -5}}}
	dropped := Transaction{ID: "tx_drop", Date: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), Tags: []Tag{{ID: "t2", Name: "drop"}}, Splits: []Split{{ID: "s2", Amount: -7}}}
	mustNoError(t, store.SaveTransactions([]Transaction{kept, dropped}), "SaveTransactions()")

	deleted, err := store.Cleanup("2026-05-01")
	mustNoError(t, err, "Cleanup()")
	if deleted != 1 {
		t.Fatalf("Cleanup() deleted = %d, want %d", deleted, 1)
	}

	var orphans int
	mustNoError(t, store.db.QueryRow(`SELECT COUNT(*) FROM transaction_tags WHERE transaction_id = 'tx_drop'`).Scan(&orphans), "count orphan tags")
	if orphans != 0 {
		t.Fatalf("orphaned tag rows = %d, want 0", orphans)
	}
	mustNoError(t, store.db.QueryRow(`SELECT COUNT(*) FROM transaction_splits WHERE transaction_id = 'tx_drop'`).Scan(&orphans), "count orphan splits")
	if orphans != 0 {
		t.Fatalf("orphaned split rows = %d, want 0", orphans)
	}
	mustNoError(t, store.db.QueryRow(`SELECT COUNT(*) FROM transaction_tags WHERE transaction_id = 'tx_keep'`).Scan(&orphans), "count kept tags")
	if orphans != 1 {
		t.Fatalf("kept tag rows = %d, want 1", orphans)
	}
}

func TestNewStoreRejectsLegacySchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "monarch.sqlite")
	createLegacySchema(t, path)

	store, err := NewStore(path)
	if !errors.Is(err, ErrSchemaOutdated) {
		t.Fatalf("NewStore() error = %v, want ErrSchemaOutdated", err)
	}
	if store != nil {
		t.Fatal("NewStore() returned a store for a legacy cache")
	}
}

func TestRebuildStoreReplacesLegacySchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "monarch.sqlite")
	createLegacySchema(t, path)
	seedLegacyData(t, path)

	store, err := RebuildStore(path)
	if err != nil {
		t.Fatalf("RebuildStore() error = %v", err)
	}
	defer store.Close()

	var count int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM accounts`).Scan(&count); err != nil {
		t.Fatalf("query rebuilt accounts: %v", err)
	}
	if count != 0 {
		t.Fatalf("rebuilt cache kept %d legacy account rows, want 0", count)
	}
	if _, err := store.db.Exec(`SELECT holdings.account_id FROM holdings`); err != nil {
		t.Fatalf("rebuilt cache missing holdings table: %v", err)
	}
}

func TestNewStoreAcceptsCurrentSchemaOnReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "monarch.sqlite")
	store, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	mustNoError(t, store.Close(), "Close()")

	store, err = NewStore(path)
	if err != nil {
		t.Fatalf("NewStore() on current schema error = %v", err)
	}
	mustNoError(t, store.Close(), "Close() second")
}

func createLegacySchema(t *testing.T, path string) {
	t.Helper()
	db, err := openDB(path)
	if err != nil {
		t.Fatalf("open legacy db: %v", err)
	}
	defer db.Close()
	const legacy = `
CREATE TABLE IF NOT EXISTS accounts (
	id TEXT PRIMARY KEY, display_name TEXT, account_type TEXT, display_balance REAL, updated_at TEXT
);
CREATE TABLE IF NOT EXISTS transactions (
	id TEXT PRIMARY KEY, date TEXT, amount REAL, merchant TEXT, category TEXT, notes TEXT, account_id TEXT
);
CREATE TABLE IF NOT EXISTS sync_meta (
	id INTEGER PRIMARY KEY AUTOINCREMENT, synced_at TEXT, accounts INTEGER, tx_count INTEGER
);
`
	if _, err := db.Exec(legacy); err != nil {
		t.Fatalf("create legacy schema: %v", err)
	}
}

func seedLegacyData(t *testing.T, path string) {
	t.Helper()
	db, err := openDB(path)
	if err != nil {
		t.Fatalf("open legacy db: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(`INSERT INTO accounts (id, display_name) VALUES ('acc_old', 'Legacy')`); err != nil {
		t.Fatalf("seed legacy account: %v", err)
	}
}
