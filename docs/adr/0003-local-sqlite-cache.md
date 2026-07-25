# 0003 - Local SQLite cache

## Status

Accepted.

## Context

Some commands (`cache sync`, `cache search`, `cache stats`, `cache cleanup`) let users query accounts and transactions locally rather than calling the Monarch GraphQL endpoint every time. That needs a local store that supports indexed text search, survives across invocations, and holds financial data at rest. The CLI ships as a single static binary built with `CGO_ENABLED=0`, so the store cannot depend on a C library.

## Decision

Persist cached data in a local SQLite database (`internal/cache`) at `~/.monarchmoney-cli/cache/monarch.sqlite`.

- Use the pure-Go `glebarez/sqlite` driver with GORM so the binary stays CGO-free and cross-compiles cleanly.
- Store three models: `Account`, `Transaction` (indexed on date, merchant, category, account), and `SyncMeta` (last sync timestamp and counts).
- Create the cache directory `0700` and the database file `0600`; financial data must not be world-readable.
- Enable WAL journal mode so a read (e.g. `cache search`) is safe while a `cache sync` writes.

## Consequences

- No CGO toolchain is needed to build or run the CLI, at the cost of the pure-Go driver's performance ceiling — acceptable for a single-user local cache.
- The cache is a convenience copy, not a source of truth; it can be deleted at any time and rebuilt with `cache sync`.
- Cached financial data lives on disk under the user's home directory with restrictive permissions; callers who do not want data at rest simply never run `cache sync`.
