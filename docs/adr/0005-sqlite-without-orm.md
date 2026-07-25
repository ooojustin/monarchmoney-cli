# 0005 - SQLite without an ORM

## Status

Accepted.

## Context

ADR 0003 introduced the local cache (`internal/cache`) and, at the time, reached for `gorm` on top of the pure-Go `glebarez/sqlite` driver. Two costs became clear as the CLI joined a fleet of sibling tools (zenodo, flickr, canvas, money):

- `gorm` is the only ORM anywhere in the fleet. Every other store is hand-written `database/sql`, so the ORM was a lone idiom that new contributors and agents had to learn for one small package.
- The cache is a fixed, three-table schema (`accounts`, `transactions`, `sync_meta`) that never changes shape at runtime. `gorm`'s reflection-driven query building and auto-migration buy nothing here; the queries are a handful of upserts, one `LIKE` search, two counts, and a date-bounded delete.

Separately, the fleet standardised on a single SQLite driver (see the fleet SQLite-driver decision): `github.com/ncruces/go-sqlite3`, a pure-Go, cgo-free driver that is actively maintained and also provides the encryption VFS a sibling tool requires. Keeping `glebarez` here would have meant two pure-Go SQLite implementations in one fleet.

## Decision

Rewrite `internal/cache` directly on `database/sql` with the `github.com/ncruces/go-sqlite3/driver` driver, and drop `gorm` and `glebarez/sqlite` entirely.

- The schema is a single hand-written DDL string (`CREATE TABLE IF NOT EXISTS ...`) with the same tables, columns, and indexes the ORM previously generated. Migration is one idempotent `Exec`.
- Writes are batched inside a transaction using `INSERT OR REPLACE`, preserving the previous full-row upsert semantics (a later sync replaces a matching id and its columns).
- Timestamps are stored as RFC3339 text — the same format the previous driver wrote — so `cache cleanup --before YYYY-MM-DD` keeps working via lexicographic comparison, and `date DESC` ordering stays chronological.
- The database file is still created `0600` inside a `0700` directory, and WAL journal mode is still enabled so a read is safe while a sync writes.
- Benchmarks (`BenchmarkSearchTransactions`, `BenchmarkGetStats`) are retained.

## Consequences

- The cache package is now the same idiom as every other store in the fleet: plain `database/sql`, no reflection, no ORM to learn.
- The binary stays cgo-free and cross-compiles cleanly; `ncruces/go-sqlite3` is pure Go (WebAssembly runtime), consistent with ADR 0003's `CGO_ENABLED=0` constraint.
- The queries are explicit SQL, which is easier to audit and reason about than generated statements, at the cost of writing the CRUD by hand — acceptable for a fixed five-method surface.
- On-disk format is unchanged in the ways that matter (RFC3339 timestamps, same table and column names), and the cache remains a rebuildable convenience copy, not a source of truth, so any drift can be resolved by deleting and re-running `cache sync`.
