package cache

import (
	"database/sql"
	"time"
)

type Account struct {
	ID             string
	DisplayName    string
	AccountType    string
	DisplayBalance float64
	UpdatedAt      time.Time
}

type Transaction struct {
	ID        string
	Date      time.Time
	Amount    float64
	Merchant  string
	Category  string
	Notes     string
	AccountID string
}

// SyncMeta tracks the last successful sync timestamp.
type SyncMeta struct {
	ID       uint      `json:"-"`
	SyncedAt time.Time `json:"synced_at"`
	Accounts int       `json:"accounts"`
	TxCount  int       `json:"transactions"`
}

// schema is the DDL for the cache store: the same tables, columns, and indexes
// the ORM previously generated (accounts, transactions with its four indexes,
// and sync_meta with an autoincrement id).
const schema = `
CREATE TABLE IF NOT EXISTS accounts (
	id              TEXT PRIMARY KEY,
	display_name    TEXT,
	account_type    TEXT,
	display_balance REAL,
	updated_at      TEXT
);
CREATE TABLE IF NOT EXISTS transactions (
	id         TEXT PRIMARY KEY,
	date       TEXT,
	amount     REAL,
	merchant   TEXT,
	category   TEXT,
	notes      TEXT,
	account_id TEXT
);
CREATE INDEX IF NOT EXISTS idx_transactions_date ON transactions(date);
CREATE INDEX IF NOT EXISTS idx_transactions_merchant ON transactions(merchant);
CREATE INDEX IF NOT EXISTS idx_transactions_category ON transactions(category);
CREATE INDEX IF NOT EXISTS idx_transactions_account_id ON transactions(account_id);
CREATE TABLE IF NOT EXISTS sync_meta (
	id        INTEGER PRIMARY KEY AUTOINCREMENT,
	synced_at TEXT,
	accounts  INTEGER,
	tx_count  INTEGER
);
`

// Migrate creates the cache schema. It is idempotent.
func Migrate(db *sql.DB) error {
	_, err := db.Exec(schema)
	return err
}
