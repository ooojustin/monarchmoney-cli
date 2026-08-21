package cache

import (
	"testing"
	"time"
)

func TestSearchTransactionsReturnsFullFidelityRows(t *testing.T) {
	store := openTestStore(t)
	mustNoError(t, store.SaveTransactions([]Transaction{{
		ID: "tx_1", Date: time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC), Amount: -12.34,
		Merchant: "Coffee Bar", PlaidName: "COFFEE BAR LLC", ProviderDescription: "Coffee Bar",
		Category: "Dining", CategoryGroup: "Food & Drink", CategoryGroupType: "expense",
		Notes: "latte", Pending: true, ReviewStatus: "flagged", NeedsReview: true,
		GoalID: "goal_1", GoalName: "House", AccountID: "acc_1",
		Tags:   []Tag{{ID: "tag_1", Name: "work"}},
		Splits: []Split{{ID: "sp_1", Amount: -10, Category: "Dining", Merchant: "Coffee Bar", Notes: "latte"}},
	}}), "SaveTransactions()")

	matches, err := store.SearchTransactions("latte")
	if err != nil {
		t.Fatalf("SearchTransactions() error = %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("SearchTransactions() len = %d, want 1", len(matches))
	}
	got := matches[0]
	if got.PlaidName != "COFFEE BAR LLC" || got.ProviderDescription != "Coffee Bar" ||
		got.CategoryGroup != "Food & Drink" || got.CategoryGroupType != "expense" ||
		!got.Pending || got.ReviewStatus != "flagged" || !got.NeedsReview ||
		got.GoalID != "goal_1" || got.GoalName != "House" {
		t.Fatalf("search row incomplete: %+v", got)
	}
	if len(got.Tags) != 1 || got.Tags[0].Name != "work" {
		t.Fatalf("search tags = %+v, want [work]", got.Tags)
	}
	if len(got.Splits) != 1 || got.Splits[0].ID != "sp_1" || got.Splits[0].Amount != -10 {
		t.Fatalf("search splits = %+v, want [sp_1]", got.Splits)
	}
}

func TestRebuildDropsLegacyChildTables(t *testing.T) {
	path := t.TempDir() + "/monarch.sqlite"
	db, err := openDB(path)
	if err != nil {
		t.Fatalf("open stale db: %v", err)
	}
	const stale = `
CREATE TABLE accounts (id TEXT PRIMARY KEY);
CREATE TABLE transactions (id TEXT PRIMARY KEY);
CREATE TABLE transaction_tags (transaction_id TEXT, tag_id TEXT);
CREATE TABLE transaction_splits (id TEXT PRIMARY KEY);
INSERT INTO transaction_tags VALUES ('tx_old', 'tag_old');
INSERT INTO transaction_splits VALUES ('sp_old');
`
	if _, err := db.Exec(stale); err != nil {
		t.Fatalf("create stale schema: %v", err)
	}
	mustNoError(t, db.Close(), "Close()")

	store, err := RebuildStore(path)
	if err != nil {
		t.Fatalf("RebuildStore() error = %v", err)
	}
	defer store.Close()

	var tags, splits int64
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM transaction_tags`).Scan(&tags); err != nil {
		t.Fatalf("query rebuilt tags: %v", err)
	}
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM transaction_splits`).Scan(&splits); err != nil {
		t.Fatalf("query rebuilt splits: %v", err)
	}
	if tags != 0 || splits != 0 {
		t.Fatalf("rebuilt cache kept child rows: %d tags, %d splits", tags, splits)
	}
}
