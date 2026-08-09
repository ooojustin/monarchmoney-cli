# 0012 - Resource absence and subcommand contracts

## Status

Accepted.

## Context

The GraphQL client raises only when a response carries a non-empty `errors` array. Monarch resolves an unknown identifier to `null` with no error, so `encoding/json` leaves the destination struct zero-valued and the service returns success. `monarch accounts show <unknown-id>` therefore exits 0 and reports a balance of 0, which a caller cannot distinguish from a real zero balance. `GetBudget` signalled absence as `(nil, nil)`, which its caller dereferenced.

`RESOURCE_NOT_FOUND` already existed but had no case in the exit-code mapping, so it fell through to exit 1 alongside `INTERNAL_ERROR`. Absence and internal failure were indistinguishable to a script.

Cobra validates unknown subcommands only on the root command. `legacyArgs` returns early for any command with a parent, and `Command.execute` returns `flag.ErrHelp` for a command with no `Run` before it reaches `ValidateArgs`. `ExecuteC` converts that sentinel into help output and a nil error. Every parent command therefore printed help and exited 0, including under `--json`, where stdout is contractually machine-readable.

`meta.schema_version` had no documented bump policy. It had moved once, without a stated rule.

## Decision

`RESOURCE_NOT_FOUND` maps to exit code 8. `INTERNAL_ERROR` maps to exit code 1 explicitly, so the default branch means an unrecognized code.

A read that resolves exactly one resource from a caller-supplied identifier reports `RESOURCE_NOT_FOUND` when the API resolves that identifier to null or returns its canonical `Not found` GraphQL error for that lookup. Lists, searches, filters, and client-side projections are collections: they succeed with an empty array. Mutations continue to surface the API's own error, because the server is authoritative about whether a write target exists. The check belongs in the service layer, which is the only layer that knows the response shape and its identity field. GraphQL error translation stays local to a lookup until another endpoint proves the same server contract.

Absence detection is worthless if the code is discarded in transit, so `wrapError` traverses wrapped errors and `errors.Error` exposes its cause.

A command that owns subcommands and no action of its own requires a subcommand. Invoked bare or with an unknown subcommand it fails with `INVALID_ARGUMENTS` and exit code 2. Human invocations still render help before failing; structured invocations emit only the error envelope. The rule is applied by walking the built command tree rather than by annotating each builder, so nested and future parents are covered and a command that owns both an action and subcommands keeps its action.

`output.SchemaVersion` is a `YYYY-MM-DD` stamp, not a semantic version. It moves, in the commit that causes the change, when the envelope structure changes, when the set of `error.code` or `error.category` values changes, when the code-to-exit mapping changes, when the retryable classification of an existing `error.code` changes, when a previously succeeding invocation starts failing, or when an existing `meta.command` changes. It does not move for new commands, new fields inside one command's payload, message wording, or documentation.

## Consequences

- Callers distinguish an absent resource from an internal failure and from a transport failure by exit code alone.
- `budgets show` on a category with no budget reports absence instead of dereferencing a nil pointer.
- A missing attachment moves from exit 1 to exit 8.
- `accounts holdings` and other collections keep returning an empty array and exit 0 for an unknown identifier, because a client-side filter cannot prove absence.
- Parent commands exit 2 instead of 0. Their help output is unchanged, and `--help` still exits 0.
- Parent commands become runnable, so their help gains the usage line that the root command already renders.
- Unknown subcommands of a parent do not offer spelling suggestions; the root command still does.
- The schema stamp is a change marker, not a compatibility promise. `TestSchemaVersion` pins the literal so the bump is a deliberate edit.
- The retryable classification of an existing code is part of the contract, because callers branch on it exactly as they branch on an exit code. ADR 0015 is the first change to exercise that rule.
