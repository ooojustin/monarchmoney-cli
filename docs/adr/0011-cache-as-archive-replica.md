# 0011 - Cache as archive replica

## Status

Accepted.

## Context

ADR-0003 positioned the SQLite cache as a query accelerator holding a lossy projection: seven fields per transaction (no tags, splits, pending flag, category group, or raw merchant names) and five per account (no type group, manual, or closed flags). The hledger backup (ADR-0010) needs archive-grade fidelity and cheap offline regeneration. Generating backups against the live API would make frequent automatic backups slow, network-dependent, and rate-limited.

## Decision

Reposition the cache from query accelerator to local archive replica. `cache sync` pulls full-fidelity data — transaction tags, splits, pending and review state, category groups, raw merchant names, account type groups and lifecycle flags, investment holdings, and closing balances — so the backup is a pure function of local data.

- The schema change is breaking; existing caches are invalidated once and users re-run `cache sync` after upgrading.
- The backup command reads only from the cache, never from the API.
- Data at rest keeps the ADR-0003 permission model (directory `0700`, files `0600`) and now covers strictly more sensitive fields.

## Consequences

- Two clean one-way pipelines emerge: Monarch API to SQLite (`cache sync`) and SQLite to plain text (`hledger backup`). Each is independently testable.
- `cache sync` becomes slower and heavier; it remains the only operation that touches the API for this feature.
- User financial history now lives in two independent local carriers (SQLite and journal text), reducing dependence on the Monarch account itself.
