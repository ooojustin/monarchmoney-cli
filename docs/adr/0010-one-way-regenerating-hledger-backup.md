# 0010 - One-way regenerating hledger backup

## Status

Accepted.

## Context

Users want plain-text ownership of their complete financial history so they can eventually migrate away from Monarch Money. Monarch holds the transaction source of truth (bank feeds via Plaid/Open Banking); hledger cannot pull data itself. Bidirectional sync was considered and rejected: Monarch syncs transactions from institutions automatically, so writing journal transactions back into Monarch would create duplicates, and two-way convergence requires a conflict-resolution state machine at odds with this repo's shortest-path principle.

Two export architectures existed: incremental append into a user-maintained journal (requires dedup state, assertion-ordering rules, partial-failure handling), or wholesale regeneration of a disposable derived artifact.

## Decision

Provide `monarch hledger backup [FILE]` (default `./monarch.journal`), which regenerates the complete journal from the local cache on every run. The journal is a disposable derived artifact with zero sync state; personal annotations belong in a user-owned journal that `include`s it.

- Scope: transactions, closing balance assertions, investment holdings. Budgets stay in Monarch.
- Account names are derived deterministically from Monarch data (type-group prefix plus slugified name): `assets:monarch:<name>`, `liabilities:monarch:<name>`, `investments:monarch:<name>`, `expenses:<category>`, `income:<category>`. No mapping configuration.
- Every transaction carries a `monarch-id:` comment tag; `account` declarations carry the same tag. Regeneration is deterministic, so `git diff` between runs shows exactly what changed.
- Transfers are single transactions with two postings and never touch P&L categories; splits become multiple expense/income postings; pending transactions are excluded.
- Amounts carry commodity symbols with `commodity` declarations; one closing balance assertion per account; hidden and closed accounts are included.
- Investment holdings are expressed as opening positions (`N TICKER @ cost = value` balanced by `equity:monarch:opening`). Live verification (2026-08) confirmed this is the final form, not a fallback: investment activity does flow through `allTransactions` (buy, sell, dividends, fees), but those transactions carry only a net cash amount with generic merchant names ("Managed Buy") — no quantity, price, or ticker fields exist on the type, so individual trades cannot be reconstructed as share movements. Brokerage accounts therefore use snapshot mode: one opening-position transaction covering all non-cash holdings, no transaction history for the account. Inter-account transfers survive through the counterparty account's feed; dividend and fee history inside brokerages is not representable.
- When a backup path is configured, every successful `cache sync` regenerates the journal automatically.

## Consequences

- Hand-editing the generated file is pointless; edits are lost on the next regeneration. The `include` pattern is the supported way to extend the ledger.
- Regeneration cost is independent of API rate limits because the journal is produced entirely from the local cache (see ADR-0011).
- No dedup, cursor, or merge logic exists anywhere in the feature.
- If Monarch renames an account, the derived hledger account name changes; the `monarch-id:` tag preserves traceability across renames.
- Brokerage P&L inside hledger starts from the snapshot date; realized-gain history from before the first backup is not reconstructible from Monarch's API.
