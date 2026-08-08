# 0011 - Structured output routing

## Status

Accepted.

## Context

Structured output is a public automation contract. `transactions export --format json --output <path>` created the file but rendered through the original stdout writer, and `auth session path --json` ignored JSON mode. Invalid transaction export formats also fell through to JSON output.

## Decision

JSON exports always emit standard success and error envelopes and route successful output exclusively to `--output` when provided. File encoding and close failures are command failures. Transaction export accepts only `json` and `csv`. Commands that return CLI data honor global `--json`, including Cobra validation failures and the root version shortcut. Event mode emits only structured envelopes. Artifact generators such as shell completion and CSV export retain their native formats.

The App process boundary retains raw argv so output flags remain available when Cobra fails before parsing them, such as unknown-command errors.

## Consequences

- JSON export files contain the same envelope that stdout would receive.
- Successful file exports do not duplicate content on stdout.
- JSON export failures emit an error envelope and exit nonzero even without global `--json`.
- Unsupported export formats fail with `INVALID_ARGUMENTS` before an API request.
- `auth session path --json` is machine-readable without changing its default plain-text output.
- Cobra argument and flag errors use `INVALID_ARGUMENTS`, exit code 2, and the standard error envelope in JSON mode.
- Human dry runs render mutation plans; JSON dry runs retain the standard success envelope.
- Event mode emits compact success and error envelopes even when `--pretty` is also present.
