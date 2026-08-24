# Guide: Run an AI agent safely

**Scenario:** You want Claude, Gemini CLI, or a cron script to query your finances unattended - with a hard guarantee it cannot move money or mutate data, without your Monarch password sitting on disk, and with outputs an automation can actually parse.

This guide is the operator's checklist. For the command-by-command view see the [Agent Integration Guide](../agent-guide.md); for the underlying model see the [Safety Model](../safety.md).

All output blocks in the guides were captured from real runs of `monarch` and then anonymized: IDs, names, and amounts are replaced, but the output shape is exactly what the tool emits.

## Step 1: take writes off the table

Set read-only mode in the environment you hand to the agent:

```bash
export MONARCH_READONLY=1
```

Now every mutation - even with `--confirm` - is rejected before any network call:

```console
$ MONARCH_READONLY=1 monarch transactions delete 331288001122334455 --confirm
{"ok":false,"error":{"code":"READ_ONLY_VIOLATION","message":"remote writes are blocked in read-only mode","category":"safety","retryable":false},"meta":{"command":"transactions.delete","profile":"default","duration_ms":0,"schema_version":"2026-08-23","request_id":"d9b7d82f-10aa-4b5e-bed7-ad48d1fcc98b"}}
$ echo $?
4
```

`duration_ms: 0` is the tell: the gate fires before authentication and before any API request. The equivalent flag (`--read-only`) exists for one-off use; prefer the env var for agents since it survives arbitrary subcommands.

## Step 2: keep the secret out of the session file

Normal `auth login` stores a long-lived token in `~/.monarchmoney-cli/session.json`. For unattended machines, store an indirection instead: write a `0600` session file whose token field names an environment variable.

```json
{
  "profile": "default",
  "email": "you@example.com",
  "created_at": "2026-01-15T08:00:00Z",
  "updated_at": "2026-01-15T08:00:00Z",
  "token": "env:MONARCH_TOKEN"
}
```

Each run resolves `MONARCH_TOKEN` from the environment - supplied by your shell profile, a secret manager, or CI secrets. The real secret never sits in a file; if the variable is unset the CLI fails with an explicit auth error rather than proceeding unauthenticated. Rationale in [ADR-0006](../adr/0006-secrets-storage.md).

For interactive logins from scripts, the same idea applies to credentials: pass `MONARCH_EMAIL`, `MONARCH_PASSWORD`, and `MONARCH_MFA_SECRET` as environment variables, never flags (flags leak into the process table).

## Step 3: give the agent a parseable contract

Always run commands with `--json`; then stdout carries exactly one envelope and stderr carries diagnostics only:

```console
$ monarch transactions list --limit 1 --json
{"ok":true,"data":{"total":1,"transactions":[{"id":"331288001122334455","date":"2026-07-12","amount":-6.75,"merchant":"Maple Roast Coffee","category":"Shopping","category_group":{"id":"221000445566778899","name":"Food & Dining","type":"expense"},"notes":"","tags":[],"goal":{"id":"","name":""},"pending":false,"hide_from_reports":false,"plaid_name":"MAPLE ROAST #212","data_provider_description":"MAPLE ROAST COFFEE TORONTO","is_recurring":false,"review_status":"needs_review","needs_review":true,"is_split_transaction":false,"created_at":"2026-07-14T21:46:18.576612+00:00","updated_at":"2026-07-29T00:35:11.006715+00:00","account_id":"221000112233445566","account_order":23,"account_type_group":"liability","owner_display_name":""}]},"meta":{"command":"transactions.list","profile":"default","duration_ms":301,"schema_version":"2026-08-23","request_id":"c1a4e9d0-...","warnings":["uses legacy Monarch GraphQL root field: allTransactions"]}}
```

The agent's decision loop should branch on two things: the process exit code, and `ok`.

| Exit | Meaning | Agent action |
|---|---|---|
| 0 | Success | Parse `data` from stdout |
| 1 | Internal | Report; do not retry |
| 2 | Invalid arguments | Fix arguments and retry |
| 3 | Auth | Ask the human to re-authenticate |
| 4 | Read-only | Explain the operation is blocked by policy |
| 5 | Network / rate limit | Retry with backoff |
| 6 | API error | Surface the message; do not retry blindly |
| 7 | Validation | Correct the input values |
| 8 | Not found | Report that the resource does not exist; do not retry |
| 10 | Confirmation required | Present the plan, ask the human about `--confirm` |

Exit 10 is the designed human-in-the-loop hook: when an agent proposes a mutation, it runs with `--dry-run`, shows the returned mutation plan, and only escalates to `--confirm --json` after explicit approval (see [Transactions workflow](transactions-workflow.md#step-3-preview-the-edit)). Full envelope details live in [JSON_SCHEMA.md](../../JSON_SCHEMA.md#exit-codes).

## Step 4: know what works offline

Reads against the local cache need no session and no network - ideal for flaky schedulers or air-gapped review:

```console
$ monarch cache stats
Cache Statistics
holdings: 3
last_synced_at: 2026-08-21T05:43:15Z
accounts: 69
transactions: 3394

$ monarch cache search "Maple Roast" --json
```

And the full plain-text export ([Ledger backup](ledger-backup.md)) is a pure cache-to-file function:

```console
$ monarch hledger backup /tmp/snapshot.journal --json
{"ok":true,"data":{"accounts":69,"file":"/tmp/snapshot.journal","holdings":3,"status":"backup complete","transactions":3394},"meta":{"command":"hledger.backup","profile":"default","duration_ms":194,"schema_version":"2026-08-23","request_id":"d58b98f7-51ff-4203-8398-819209cc9482"}}
```

Schedule `monarch cache sync` separately (it is the networked step) and keep agents pointed at the offline commands.

## Step 5: watch long-running refreshes

`accounts refresh --wait --events` emits NDJSON progress envelopes so an orchestrator can show or log progress:

```json
{"ok":true,"data":{"is_complete":false,"status":"syncing","accounts":[{"id":"221000112233445566","has_sync_in_progress":true}]},"meta":{"command":"accounts.refresh.progress","profile":"default","duration_ms":2010,"schema_version":"2026-08-23","request_id":"2d3f07a0-8b1e-4cc0-a995-623985ed0c52"}}
{"ok":true,"data":{"is_complete":true,"status":"complete","accounts":[]},"meta":{"command":"accounts.refresh.progress","profile":"default","duration_ms":4012,"schema_version":"2026-08-23","request_id":"2d3f07a0-8b1e-4cc0-a995-623985ed0c52"}}
{"ok":true,"data":{"status":"refresh complete"},"meta":{"command":"accounts.refresh","profile":"default","duration_ms":6020,"schema_version":"2026-08-23","request_id":"2d3f07a0-8b1e-4cc0-a995-623985ed0c52"}}
```

## Step 6: leave an audit trail

Every executed mutation appends to `~/.monarchmoney-cli/audit/YYYY-MM-DD.jsonl`:

```json
{
  "timestamp": "2026-08-01T22:12:00Z",
  "command": "transactions.update",
  "resource_id": "331288001122334455",
  "dry_run": false,
  "confirmed": true,
  "profile": "default",
  "result": "success",
  "error_code": ""
}
```

Records never contain credentials or financial details. Prune old files periodically: `monarch audit cleanup` (default 30 days, customize with `--older-than N`). In read-only mode there is nothing to audit - which is the point of step 1.

## Next steps

- [Configuration](configuration.md) - env var reference and precedence rules.
- [Monthly review](monthly-review.md) - the analysis commands agents are best at summarizing.
- [Agent Integration Guide](../agent-guide.md) - quick-reference version of this page.
