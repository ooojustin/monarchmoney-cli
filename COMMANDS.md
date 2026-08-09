# Commands

`monarchmoney-cli` is designed to cover the full capability surface of the Monarch Money API, matching and exceeding the feature set of the [monarch-mcp-server](https://github.com/robcerda/monarch-mcp-server) project.

## Core Domain Coverage

| Domain | Capability | CLI Command |
|---|---|---|
| **Accounts** | List, show, dated balances, holdings, history, refresh, net worth snapshots | `monarch accounts` |
| **Transactions** | List/export with advanced filters, search, summary, duplicates, splits, bulk-categorize | `monarch transactions` |
| **Rules** | List, create, update, delete auto-categorization rules | `monarch rules` |
| **Budgets** | List, show, set, reset, flexible, rollover | `monarch budgets` |
| **Cashflow** | Summary, category/merchant breakdown, grouped trends, spending totals | `monarch cashflow` |
| **Overview** | Net worth, cashflow, and recent transactions in one call | `monarch overview` |
| **Analysis** | Deterministic anomaly, subscription, merchant, and budget burn-rate analysis | `monarch analyze` |
| **Categories** | List, groups, create, update, rollover, delete | `monarch categories` |
| **Goals** | List goals with progress/balance, savings goal budgets | `monarch goals` |
| **Investments** | Portfolio holdings and security performance | `monarch investments` |
| **Tags** | List, create, set, add, clear | `monarch tags` |
| **Institutions** | List linked financial institutions | `monarch institutions` |
| **Recurring** | List and update recurring transactions | `monarch recurring` |
| **Credit** | Get credit score history | `monarch credit` |
| **Subscription** | Show Monarch subscription details | `monarch subscription show` |
| **Attachments** | List, download (upload unavailable — API limitation) | `monarch transactions attachments` |
| **Cache** | Local data cache (sync, search, stats, cleanup) | `monarch cache` |
| **Audit** | Audit log cleanup and management | `monarch audit` |
| **Doctor** | Verify environment and authentication | `monarch doctor` |
| **Version** | Print version information | `monarch version` |

## Read Commands

- `monarch accounts list`: List all accounts.
- `monarch accounts show <id>`: Show detailed account info.
- `monarch accounts types`: List available account types.
- `monarch accounts balance-at --date YYYY-MM-DD`: Get account balances as of a specific date.
- `monarch accounts history <id>`: Get balance history with `--from`/`--to`.
- `monarch accounts holdings <id>`: List investment holdings.
- `monarch accounts aggregate-snapshots`: Get net worth history over time.
- `monarch accounts snapshots`: Get net worth by account type.
- `monarch accounts recent-balances`: Get recent balances for all accounts.
- `monarch accounts refresh-status`: Check the refresh status of linked accounts.
- `monarch networth`: Top-level alias for `accounts aggregate-snapshots`.
- `monarch transactions list`: List latest transactions with advanced filters.
- `monarch transactions export`: Export transactions to stdout or `--output <path>` in JSON or CSV format.
- `monarch transactions search <query>`: Search transactions by text.
- `monarch transactions show <id>`: Get full transaction details.
- `monarch transactions summary`: Get aggregated spending summary.
- `monarch transactions splits <id>`: View split details for a transaction.
- `monarch transactions duplicates`: Find potential duplicate transactions.
- `monarch transactions attachments list <id>`: List attachments for a transaction.
- `monarch transactions attachments download <id>`: Download attachments for a transaction.
- `monarch rules list`: List all auto-categorization rules.
- `monarch budgets list`: View planned vs actual for a month.
- `monarch budgets show <category-id>`: Show budget details for a category.
- `monarch budgets export`: Export budget data.
- `monarch cashflow summary`: View income, expenses, and savings rate.
- `monarch cashflow spending`: View spending breakdown with totals.
- `monarch cashflow list`: List cashflow transactions.
- `monarch cashflow categories`: View spending by category.
- `monarch cashflow merchants`: View spending by merchant.
- `monarch cashflow trends`: View aggregate trends by category or category group and period.
- `monarch overview`: Get a compact financial overview (net worth, cashflow, recent transactions). The response reports the range it covered as `start_date` and `end_date`.
- `monarch goals list`: List goals with progress, balance, and target.
- `monarch goals budgets`: View savings goal monthly budget amounts.
- `monarch investments portfolio`: View portfolio performance and holdings.
- `monarch investments performance --security-id <id> --from YYYY-MM-DD --to YYYY-MM-DD`: View historical security performance.
- `monarch analyze anomalies`: Find category spending anomalies from transaction history.
- `monarch analyze subscriptions`: Summarize recurring subscription costs and potential overlap facts.
- `monarch analyze merchants --compare previous-month`: Compare merchant expenses period-over-period.
- `monarch analyze burn-rate`: Compare budget usage with elapsed month time.
- `monarch recurring list`: View recurring transactions.
- `monarch credit history`: View credit score history.
- `monarch categories groups`: List category groups.
- `monarch categories groups update <group-id>`: Update a category group (name, budget variability, rollover settings).
- `monarch categories rollover <category-id>`: Show rollover settings for a category.
- `monarch institutions list`: List linked financial institutions.
- `monarch subscription show`: Show Monarch subscription details.
- `monarch auth status`: Check current authentication status.
- `monarch auth session path`: Print the session file path; `--json` emits an `auth.session.path` envelope.
- `monarch doctor`: Verify the selected config, session, and device identity state. Add `--connect` to check the configured API endpoint.
- `monarch version`: Print version information.
- `monarch --version` / `monarch -v`: Print the same version output; combine with `--json` for an envelope.

## Mutation and Remote-Action Commands

All mutations are protected by the [Safety Model](./safety.md).

- `monarch auth login`: Authenticate and persist a trusted-device session, prompting for email verification or authenticator MFA when required.
- `monarch auth logout`: Remove the local session token.
- `monarch accounts refresh [account-id...]`: Trigger a remote sync of all accounts (or specific ones).
- `monarch accounts create-manual`: Create a manual account.
- `monarch accounts update <id>`: Update account name or balance.
- `monarch accounts delete <id>`: Delete an account.
- `monarch accounts upload-history <id> <file>`: Upload balance history for an account.
- `monarch transactions create`: Manually add a transaction.
- `monarch transactions update <id>`: Modify transaction fields (notes, category, amount, date, merchant, hide-from-reports, mark-reviewed).
- `monarch transactions delete <id>`: Remove a transaction.
- `monarch transactions split <id>`: Split a transaction into parts.
- `monarch transactions bulk-categorize`: Apply a category to multiple transactions.
- `monarch transactions tags set <id>`: Set tags on a transaction.
- `monarch transactions tags add <id>`: Append tags to a transaction.
- `monarch transactions tags clear <id>`: Remove all tags.
- `monarch rules create`: Create an auto-categorization rule.
- `monarch rules update <id>`: Update an existing rule.
- `monarch rules delete <id>`: Delete a rule.
- `monarch budgets set <category-id>`: Set budget amount for a category.
- `monarch budgets reset`: Reset budget for a month.
- `monarch budgets flexible set`: Set flexible budget amount.
- `monarch budgets flex-rollover set`: Set flex-rollover budget amount.
- `monarch categories create`: Create a new category.
- `monarch categories update <id>`: Update a category (name, icon, budget variability, exclude from budget).
- `monarch categories delete <id>`: Delete a category.
- `monarch categories delete-many --file <path>`: Delete categories listed in a file.
- `monarch recurring update <id>`: Update a recurring transaction.
- `monarch tags create`: Create a new tag.
- `monarch cache sync`: Sync data from Monarch to local cache. Use `--limit N` to set page size (default 1000), `--all` to paginate through all matching transactions.
- `monarch cache search <query>`: Search transactions in local cache.
- `monarch cache stats`: Show cache statistics including last sync time.
- `monarch cache cleanup --before YYYY-MM-DD`: Delete old transactions from cache.
- `monarch audit cleanup`: Remove audit log files older than N days (default 30). Use `--older-than N` to customize.
- `monarch completion [bash|zsh|fish|powershell]`: Generate shell completion scripts.

## Error Surface

Exit codes are listed in [JSON_SCHEMA.md](JSON_SCHEMA.md#exit-codes).

- Date flags must use `YYYY-MM-DD`, and `--from` must not be after `--to`. Both are checked locally, before any request.
- On `overview` and the `cashflow` subcommands an omitted `--from` means the first of the current month and an omitted `--to` means today, resolved against your local calendar. A lone `--from` therefore reads as "through today"; a lone `--to` resolves to an inverted range and exits 2.
- `--limit` must be at least 1 and `--offset` must not be negative.
- A command that owns subcommands requires one. Invoked bare or with an unknown subcommand it exits 2 with `INVALID_ARGUMENTS`.
- Looking up one resource by identifier exits 8 with `RESOURCE_NOT_FOUND` when it does not exist, for example `monarch accounts show <unknown-id>`.
- Listing, searching, and filtering return an empty array and exit 0 when nothing matches, for example `monarch accounts holdings <unknown-id>`.

## Safety & Audit

- **Dry-run**: Every mutation supports `--dry-run` to preview changes.
- **Confirmation**: Remote writes require the `--confirm` flag.
- **Read-only**: Use `MONARCH_READONLY=1` to block all mutations.
- **Audit Logs**: Executed mutations are logged when `audit_log` is enabled (the default). The directory is platform-specific. Use `monarch audit cleanup --older-than N` to remove logs older than N days (default 30).

## Feature Parity with monarch-mcp-server

This CLI covers all features provided by the [monarch-mcp-server](https://github.com/robcerda/monarch-mcp-server) MCP server:

| monarch-mcp-server Tool | monarchmoney-cli Command |
|---|---|
| `get_accounts` | `accounts list` |
| `get_account_holdings` | `accounts holdings <id>` |
| `get_account_balance_history` | `accounts history <id>` |
| `refresh_accounts` | `accounts refresh` |
| `get_net_worth` | `accounts aggregate-snapshots` |
| `get_net_worth_by_account_type` | `accounts snapshots` |
| `get_transactions` | `transactions list` (with advanced filters) |
| `search_transactions` | `transactions search` |
| `get_transaction_details` | `transactions show <id>` |
| `create_transaction` | `transactions create` |
| `update_transaction` | `transactions update <id>` |
| `delete_transaction` | `transactions delete <id>` |
| `get_transactions_needing_review` | `transactions list --needs-review` |
| `mark_transaction_reviewed` | `transactions update <id> --mark-reviewed` |
| `bulk_categorize_transactions` | `transactions bulk-categorize` |
| `get_transaction_splits` | `transactions splits <id>` |
| `split_transaction` | `transactions split <id>` |
| `get_transaction_rules` | `rules list` |
| `create_transaction_rule` | `rules create` |
| `update_transaction_rule` | `rules update <id>` |
| `delete_transaction_rule` | `rules delete <id>` |
| `get_budgets` | `budgets list` |
| `set_budget_amount` | `budgets set` |
| `get_cashflow` | `cashflow summary` |
| `get_spending_summary` | `cashflow spending` |
| `get_categories` | `categories list` |
| `get_category_groups` | `categories groups` |
| `get_tags` | `tags list` |
| `set_transaction_tags` | `transactions tags set <id>` |
| `create_tag` | `tags create` |
| `get_recurring_transactions` | `recurring list` |
| `get_transactions_summary` | `transactions summary` |
