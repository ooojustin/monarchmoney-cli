# 0002 - Tiered mutation safety

## Status

Accepted.

## Context

The CLI operates on real financial data and is intended to be driven by scripts and AI agents as well as humans. An accidental or unattended write — refreshing, editing, or deleting remote data — can have real consequences. The interface therefore needs guardrails that are predictable enough for an agent to reason about and strict enough that writes never happen by accident.

## Decision

Every command is classified into one operation tier, and writes must clear a set of safety layers before they execute (`internal/safety`).

Operation tiers:

| Tier | Examples | Requirement |
|---|---|---|
| Read | `list`, `show`, `search` | Always allowed |
| Remote action | `accounts refresh` | Blocked by read-only |
| Mutation | `update`, `set`, `create` | Blocked by read-only; requires `--confirm` |
| Destructive | `delete`, `reset` | Blocked by read-only; requires `--confirm` |

Safety layers, applied in order to any non-read tier:

1. Read-only mode (`--read-only` or `MONARCH_READONLY`) blocks the operation with `READ_ONLY_VIOLATION`.
2. `--dry-run` returns a mutation plan and makes no remote call.
3. Absent `--confirm`, the operation fails with `CONFIRMATION_REQUIRED`.

Every mutation attempted with `--confirm` is written to a local audit log (`~/.monarchmoney-cli/audit/YYYY-MM-DD.jsonl`) recording command, resource, dry-run/confirm flags, profile, result, and error code — never credentials or financial detail.

## Consequences

- Agents can run in a fully read-only posture (`MONARCH_READONLY=1`) with a guarantee that no tier above Read will execute.
- Every mutation requires an explicit, per-invocation `--confirm`; there is no persistent "yes to all" mode.
- `--dry-run` gives a preview path that shares the tier classification, so a plan and its execution cannot diverge in which guard applies.
- The audit log grows over time; `audit` and `cache` commands exist to prune local state.
