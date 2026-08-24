package cache

import (
	"database/sql"
	"errors"
	"time"
)

type Account struct {
	ID             string    `json:"id"`
	DisplayName    string    `json:"display_name"`
	AccountType    string    `json:"account_type"`
	TypeGroup      string    `json:"account_type_group"`
	DisplayBalance float64   `json:"display_balance"`
	CurrentBalance float64   `json:"current_balance"`
	IsManual       bool      `json:"is_manual"`
	IsHidden       bool      `json:"is_hidden"`
	IsClosed       bool      `json:"is_closed"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type Tag struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Split struct {
	ID       string  `json:"id"`
	Amount   float64 `json:"amount"`
	Category string  `json:"category"`
	Merchant string  `json:"merchant"`
	Notes    string  `json:"notes"`
}

type Transaction struct {
	ID                  string    `json:"id"`
	Date                time.Time `json:"date"`
	Amount              float64   `json:"amount"`
	Merchant            string    `json:"merchant"`
	PlaidName           string    `json:"plaid_name"`
	ProviderDescription string    `json:"data_provider_description"`
	Category            string    `json:"category"`
	CategoryGroup       string    `json:"category_group"`
	CategoryGroupType   string    `json:"category_group_type"`
	Notes               string    `json:"notes"`
	Pending             bool      `json:"pending"`
	ReviewStatus        string    `json:"review_status"`
	NeedsReview         bool      `json:"needs_review"`
	GoalID              string    `json:"goal_id"`
	GoalName            string    `json:"goal_name"`
	AccountID           string    `json:"account_id"`
	Tags                []Tag     `json:"tags"`
	Splits              []Split   `json:"splits"`
}

type Holding struct {
	ID        string  `json:"id"`
	Ticker    string  `json:"ticker"`
	Name      string  `json:"name"`
	Quantity  float64 `json:"quantity"`
	Basis     float64 `json:"basis"`
	Value     float64 `json:"value"`
	AccountID string  `json:"account_id"`
}

type SyncMeta struct {
	ID       uint      `json:"-"`
	SyncedAt time.Time `json:"synced_at"`
	Accounts int       `json:"accounts"`
	TxCount  int       `json:"transactions"`
}

var ErrSchemaOutdated = errors.New("cache predates the archive-replica schema; run 'monarch cache sync' to rebuild it")

const schema = `
CREATE TABLE IF NOT EXISTS accounts (
	id              TEXT PRIMARY KEY,
	display_name    TEXT,
	account_type    TEXT,
	type_group      TEXT,
	display_balance REAL,
	current_balance REAL,
	is_manual       INTEGER,
	is_hidden       INTEGER,
	is_closed       INTEGER,
	updated_at      TEXT
);
CREATE TABLE IF NOT EXISTS transactions (
	id                   TEXT PRIMARY KEY,
	date                 TEXT,
	amount               REAL,
	merchant             TEXT,
	plaid_name           TEXT,
	provider_description TEXT,
	category             TEXT,
	category_group       TEXT,
	category_group_type  TEXT,
	notes                TEXT,
	pending              INTEGER,
	review_status        TEXT,
	needs_review         INTEGER,
	goal_id              TEXT,
	goal_name            TEXT,
	account_id           TEXT
);
CREATE INDEX IF NOT EXISTS idx_transactions_date ON transactions(date);
CREATE INDEX IF NOT EXISTS idx_transactions_merchant ON transactions(merchant);
CREATE INDEX IF NOT EXISTS idx_transactions_category ON transactions(category);
CREATE INDEX IF NOT EXISTS idx_transactions_account_id ON transactions(account_id);
CREATE TABLE IF NOT EXISTS transaction_tags (
	transaction_id TEXT,
	tag_id         TEXT,
	name           TEXT,
	PRIMARY KEY (transaction_id, tag_id)
);
CREATE TABLE IF NOT EXISTS transaction_splits (
	id             TEXT PRIMARY KEY,
	transaction_id TEXT,
	amount         REAL,
	category       TEXT,
	merchant       TEXT,
	notes          TEXT
);
CREATE INDEX IF NOT EXISTS idx_transaction_splits_transaction_id ON transaction_splits(transaction_id);
CREATE TABLE IF NOT EXISTS holdings (
	id         TEXT PRIMARY KEY,
	ticker     TEXT,
	name       TEXT,
	quantity   REAL,
	basis      REAL,
	value      REAL,
	account_id TEXT
);
CREATE INDEX IF NOT EXISTS idx_holdings_account_id ON holdings(account_id);
CREATE TABLE IF NOT EXISTS sync_meta (
	id        INTEGER PRIMARY KEY AUTOINCREMENT,
	synced_at TEXT,
	accounts  INTEGER,
	tx_count  INTEGER
);
`

func Migrate(db *sql.DB) error {
	legacy, err := isLegacySchema(db)
	if err != nil {
		return err
	}
	if legacy {
		return ErrSchemaOutdated
	}
	_, err = db.Exec(schema)
	return err
}

func Rebuild(db *sql.DB) error {
	legacy, err := isLegacySchema(db)
	if err != nil {
		return err
	}
	if legacy {
		if _, err := db.Exec(`DROP TABLE IF EXISTS accounts; DROP TABLE IF EXISTS transactions; DROP TABLE IF EXISTS transaction_tags; DROP TABLE IF EXISTS transaction_splits; DROP TABLE IF EXISTS sync_meta;`); err != nil {
			return err
		}
	}
	_, err = db.Exec(schema)
	return err
}

func isLegacySchema(db *sql.DB) (bool, error) {
	var dataTables int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name IN ('accounts', 'transactions')`).Scan(&dataTables); err != nil {
		return false, err
	}
	if dataTables == 0 {
		return false, nil
	}
	var archiveTables int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'holdings'`).Scan(&archiveTables); err != nil {
		return false, err
	}
	return archiveTables == 0, nil
}
