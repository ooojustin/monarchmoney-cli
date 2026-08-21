package cache

import (
	"reflect"
	"testing"
	"time"
)

func TestStoreReadsAccountsOrderedByID(t *testing.T) {
	store := openTestStore(t)
	mustNoError(t, store.SaveAccounts([]Account{
		{ID: "acc_b", DisplayName: "Savings", AccountType: "savings", TypeGroup: "asset", DisplayBalance: 200, CurrentBalance: 199.5, UpdatedAt: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)},
		{ID: "acc_a", DisplayName: "Checking", AccountType: "checking", TypeGroup: "asset", IsHidden: true, IsClosed: true, IsManual: true, DisplayBalance: 100, CurrentBalance: 99.5, UpdatedAt: time.Date(2026, 5, 2, 3, 4, 5, 0, time.UTC)},
	}), "SaveAccounts()")

	got, err := store.Accounts()
	if err != nil {
		t.Fatalf("Accounts() error = %v", err)
	}
	want := []Account{
		{ID: "acc_a", DisplayName: "Checking", AccountType: "checking", TypeGroup: "asset", IsManual: true, IsHidden: true, IsClosed: true, DisplayBalance: 100, CurrentBalance: 99.5, UpdatedAt: time.Date(2026, 5, 2, 3, 4, 5, 0, time.UTC)},
		{ID: "acc_b", DisplayName: "Savings", AccountType: "savings", TypeGroup: "asset", DisplayBalance: 200, CurrentBalance: 199.5, UpdatedAt: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Accounts() = %+v, want %+v", got, want)
	}
}

func TestStoreReadsTransactionsWithTagsAndSplits(t *testing.T) {
	store := openTestStore(t)
	mustNoError(t, store.SaveTransactions([]Transaction{
		{
			ID: "tx_2", Date: time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC), Amount: -12.34,
			Merchant: "Coffee", PlaidName: "BLUE BOTTLE LLC", ProviderDescription: "Blue Bottle",
			Category: "Dining", CategoryGroup: "Food & Drink", CategoryGroupType: "expense",
			Notes: "latte", Pending: true, ReviewStatus: "flagged", NeedsReview: true,
			GoalID: "goal_1", GoalName: "House", AccountID: "acc_1",
			Tags:   []Tag{{ID: "tag_1", Name: "work"}},
			Splits: []Split{{ID: "sp_1", Amount: -10, Category: "Dining", Merchant: "Coffee", Notes: "latte"}},
		},
		{
			ID: "tx_1", Date: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), Amount: 100,
			Merchant: "Deposit", Category: "Income", CategoryGroup: "Income", CategoryGroupType: "income",
			AccountID: "acc_1",
			Tags:      []Tag{{ID: "tag_b", Name: "bonus"}, {ID: "tag_a", Name: "salary"}},
		},
	}), "SaveTransactions()")

	got, err := store.Transactions()
	if err != nil {
		t.Fatalf("Transactions() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("Transactions() len = %d, want 2", len(got))
	}
	if got[0].ID != "tx_1" || got[1].ID != "tx_2" {
		t.Fatalf("Transactions() order = %q, %q; want date-ascending tx_1, tx_2", got[0].ID, got[1].ID)
	}

	full := got[1]
	if full.Amount != -12.34 || full.Merchant != "Coffee" || full.PlaidName != "BLUE BOTTLE LLC" ||
		full.ProviderDescription != "Blue Bottle" || full.Category != "Dining" ||
		full.CategoryGroup != "Food & Drink" || full.CategoryGroupType != "expense" ||
		full.Notes != "latte" || !full.Pending || full.ReviewStatus != "flagged" || !full.NeedsReview ||
		full.GoalID != "goal_1" || full.GoalName != "House" || full.AccountID != "acc_1" {
		t.Fatalf("Transactions() row incomplete: %+v", full)
	}
	wantTags := []Tag{{ID: "tag_1", Name: "work"}}
	if !reflect.DeepEqual(full.Tags, wantTags) {
		t.Fatalf("tags = %+v, want %+v", full.Tags, wantTags)
	}
	wantSplits := []Split{{ID: "sp_1", Amount: -10, Category: "Dining", Merchant: "Coffee", Notes: "latte"}}
	if !reflect.DeepEqual(full.Splits, wantSplits) {
		t.Fatalf("splits = %+v, want %+v", full.Splits, wantSplits)
	}
	if !reflect.DeepEqual(got[0].Tags, []Tag{{ID: "tag_b", Name: "bonus"}, {ID: "tag_a", Name: "salary"}}) {
		t.Fatalf("tags not name-ordered: %+v", got[0].Tags)
	}
}

func TestStoreReadsHoldingsOrderedByID(t *testing.T) {
	store := openTestStore(t)
	mustNoError(t, store.SaveHoldings([]Holding{
		{ID: "h_b", Ticker: "VTI", Name: "Vanguard Total Stock Market ETF", Quantity: 10.5, Basis: 2500, Value: 3000.5, AccountID: "acc_1"},
		{ID: "h_a", Ticker: "CUR:USD", Name: "USD", Quantity: 500, Basis: 500, Value: 500, AccountID: "acc_1"},
	}), "SaveHoldings()")

	got, err := store.Holdings()
	if err != nil {
		t.Fatalf("Holdings() error = %v", err)
	}
	want := []Holding{
		{ID: "h_a", Ticker: "CUR:USD", Name: "USD", Quantity: 500, Basis: 500, Value: 500, AccountID: "acc_1"},
		{ID: "h_b", Ticker: "VTI", Name: "Vanguard Total Stock Market ETF", Quantity: 10.5, Basis: 2500, Value: 3000.5, AccountID: "acc_1"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Holdings() = %+v, want %+v", got, want)
	}
}

func TestStoreReadsReturnEmptySlicesOnEmptyCache(t *testing.T) {
	store := openTestStore(t)

	for name, fn := range map[string]func() (int, error){
		"Accounts":     func() (int, error) { v, err := store.Accounts(); return len(v), err },
		"Transactions": func() (int, error) { v, err := store.Transactions(); return len(v), err },
		"Holdings":     func() (int, error) { v, err := store.Holdings(); return len(v), err },
	} {
		n, err := fn()
		if err != nil {
			t.Fatalf("%s() error = %v", name, err)
		}
		if n != 0 {
			t.Fatalf("%s() len = %d, want 0", name, n)
		}
	}
}
