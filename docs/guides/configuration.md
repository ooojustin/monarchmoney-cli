# Guide: Configure the CLI

**Scenario:** You want to point the CLI at specific file locations, raise the API timeout on a slow connection, and set defaults once instead of repeating flags - without losing the ability to override anything per command.

All output blocks in the guides were captured from real runs of `monarch` and then anonymized: IDs, names, and amounts are replaced, but the output shape is exactly what the tool emits.

## Where configuration lives

The CLI reads one YAML file. Nothing else is required - every key has a default, and the CLI works with no config file at all.

| | Path |
|---|---|
| Config file (macOS/Windows) | `~/.monarchmoney-cli/config.yaml` |
| Config file (Linux) | `$XDG_STATE_HOME/monarchmoney-cli/config.yaml`, falling back to `~/.local/state/monarchmoney-cli/config.yaml` (or the legacy `~/.monarchmoney-cli` if it already exists) |
| Override location | `--config /path/to/config.yaml` flag or `MONARCH_CONFIG=/path/to/config.yaml` |

Check which paths apply to your machine:

```console
$ monarch auth session path
/Users/david/.monarchmoney-cli/session.json
```

A missing config file is not an error. An unreadable or malformed one fails the command loudly rather than silently falling back to defaults.

## A working config file

```yaml
profile: personal
timeout: 60s
cache_path: /Users/david/finances/cache/monarch.sqlite
backup_path: /Users/david/finances/monarch.journal
```

That is the entire recommended surface for most users:

- `profile` names the session profile to use when no `--profile` flag is given.
- `timeout` raises the per-command HTTP timeout from the 30s default (`60s`, `90s` - Go duration syntax).
- `cache_path` moves the SQLite archive used by `cache` and `hledger backup` commands.
- `backup_path` makes every successful `monarch cache sync` regenerate your hledger journal automatically (see [Ledger backup](ledger-backup.md)). It must differ from `cache_path`.

Paths are used literally; `~` and environment variables are not expanded. Use absolute paths.

Additional keys cover endpoint overrides, read-only mode, session placement, audit logging, and output defaults. See the [key reference](#key-reference).

## Precedence: flags > env > file > defaults

The same setting can come from four places. Higher sources win; nothing merges.

```console
$ monarch version --json --profile work
{"ok":true,"data":{"version":"v0.8.1-...","commit":"b0bdf52","date":"2026-08-21T11:09:16Z","built_by":"unknown"},"meta":{"command":"version","profile":"work","duration_ms":0,"schema_version":"2026-08-23","request_id":"1b320613-4a34-4d8b-9fac-02a2dfd0e91d"}}
```

The envelope's `meta.profile` always tells you which profile actually ran. To verify what a config file resolves to before relying on it, run any command with `--config` pointed at it and read `meta.profile`.

## Key reference

| Key | Type | Default | Honored today |
|---|---|---|---|
| `profile` | string | `default` | yes |
| `api_endpoint` | string | `https://api.monarch.com/graphql` | yes |
| `timeout` | duration | `30s` | yes |
| `cache_path` | string | `<state dir>/cache/monarch.sqlite` | yes |
| `backup_path` | string | *(empty - auto-regen off)* | yes |
| `read_only` | bool | `false` | yes |
| `session_path` | string | `<state dir>/session.json` | yes |
| `audit_log` | bool | `true` | yes |
| `output` | string | *(empty)* | parsed, **not enforced** - use `--json`/`--pretty` |

`output` remains a compatibility key only. Use the output flags or environment variables listed below.

## Environment variables

Every file key has an environment override of the form `MONARCH_<KEY>`:

| Variable | Sets |
|---|---|
| `MONARCH_PROFILE` | `profile` |
| `MONARCH_API_ENDPOINT` | `api_endpoint` |
| `MONARCH_TIMEOUT` | `timeout` |
| `MONARCH_CACHE_PATH` | `cache_path` |
| `MONARCH_BACKUP_PATH` | `backup_path` |
| `MONARCH_SESSION_PATH` | `session_path` |
| `MONARCH_READ_ONLY` / `MONARCH_READONLY` | read-only mode (enforced by the safety gate) |
| `MONARCH_JSON`, `MONARCH_PRETTY`, `MONARCH_EVENTS` | same as the `--json`, `--pretty`, `--events` flags |
| `MONARCH_DRY_RUN`, `MONARCH_CONFIRM` | same as the `--dry-run`, `--confirm` flags |
| `MONARCH_CONFIG` | path to the config file itself |

Login credentials can also come from the environment (`MONARCH_EMAIL`, `MONARCH_PASSWORD`, `MONARCH_MFA_SECRET`) - see [Authentication](../auth.md).

Read-only mode is worth demonstrating because it is the strongest guarantee you can hand to a script or agent:

```console
$ MONARCH_READONLY=1 monarch transactions delete 331288001122334455 --confirm
{"ok":false,"error":{"code":"READ_ONLY_VIOLATION","message":"remote writes are blocked in read-only mode","category":"safety","retryable":false},"meta":{"command":"transactions.delete","profile":"default","duration_ms":0,"schema_version":"2026-08-23","request_id":"d9b7d82f-10aa-4b5e-bed7-ad48d1fcc98b"}}
$ echo $?
4
```

The safety gate fires before authentication and before any network call, so a misconfigured environment cannot mutate data.

## Next steps

- [Ledger backup](ledger-backup.md) - put `backup_path` to work so syncs maintain a plain-text journal.
- [Agent automation](agent-automation.md) - combine these variables into a safe unattended setup.
- [Commands](../../COMMANDS.md) - full command reference.
