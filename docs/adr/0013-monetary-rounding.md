# 0013 - Monetary rounding

## Status

Accepted.

## Context

Every monetary value is a `float64`, decoded from JSON and re-encoded without transformation. Binary floating point cannot represent most cent values exactly, so sums accumulate representation error and `encoding/json` prints the shortest round-trip form of the result. `overview.net_worth` emitted `107207.94000000002` from a local sum. Monarch's own responses carry the same artifacts: `investments portfolio` reported `total_value` as `159.92000000000002`, a value its web interface renders as `$159.92`.

Consumers cannot tell a computed artifact from a passthrough artifact, and neither is a figure anyone intends. `internal/analyze` already rounded every value it produced, through a private helper that the rest of the repository could not reach.

## Decision

Every monetary amount the CLI emits is rounded to two decimal places at the point it is assigned into an emitted struct. Ratios, percentages, share quantities, and unit prices are not monetary amounts and are emitted unrounded.

Rounding applies to values Monarch reports as well as values the CLI computes. An artifact on the wire is a representation error, not a figure the provider intends, so rounding it is more faithful to the reported amount than preserving it.

Locally computed sums accumulate their available input values and are rounded once where the total is emitted. Service-layer monetary fields are already rounded when another command consumes them, which keeps aggregates equal to the values exposed by their source command.

`internal/money` owns the helper. It is reachable from the service layer, the command layer, and the analysis layer, and it names the output policy rather than any one of them.

Rounding is applied per assignment rather than by a transform over the encoded payload. Whether a number is money is a property of the field, not of its type or name, so the judgment belongs in the source where a reader and a reviewer can see it.

## Consequences

- No monetary field in JSON output carries more than two decimal places, whichever layer produced it.
- A regression test walks emitted payloads and fails on any numeric leaf with more than two decimals, except for an explicit set of non-monetary floating-point keys. Integers and counts need no exception because they carry no fractional precision. The set is the reviewable statement of what is not money, and new monetary fields are covered without being enumerated.
- Values that Monarch reports with representation artifacts no longer match Monarch's bytes exactly.
- Holding quantities stay unrounded, so a cash holding, whose quantity is denominated in currency rather than shares, can still carry an artifact. Rounding quantities would truncate fractional share counts, which is the larger loss.
- Adding a monetary field to a response requires rounding it at the assignment. Omitting that is caught by the regression test rather than by a reader.
- `internal/analyze` keeps its existing behavior; its private helper is replaced by the shared one.
