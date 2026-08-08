# JSON Output Schema

`monarchmoney-cli` uses a standardized JSON envelope for all structured output. This ensures that AI Agents and automated scripts can reliably parse the results.

## Success Envelope

```json
{
  "ok": true,
  "data": { ... },
  "meta": {
    "command": "accounts.list",
    "profile": "default",
    "duration_ms": 125,
    "schema_version": "2026-08-07",
    "request_id": "2d3f07a0-8b1e-4cc0-a995-623985ed0c52",
    "warnings": ["optional deprecation or migration notice"]
  }
}
```

- `ok`: Always `true` for successful operations.
- `data`: The command-specific results (object or array).
- `meta`: Diagnostic information about the request.
- `meta.request_id`: A UUID generated per invocation, identical across every envelope emitted by a single command run.
- `meta.warnings` (optional): Non-fatal notices about deprecated fields or migration advice. Emitted by commands that interact with legacy API fields (e.g., `transactions list`, `accounts history`).
- `transactions export --format json` emits this envelope even without global `--json`; `--output <path>` writes the envelope to that file instead of stdout.
- `auth session path --json` returns `{"path":"..."}` in `data` with command metadata `auth.session.path`.
- `monarch --version --json` and `monarch version --json` emit the same `version` envelope.

### Doctor Data

`monarch doctor --json` reports the selected paths and whether the loaded configuration is valid:

```json
{
  "config": {
    "path": "/path/to/config.yaml",
    "exists": true,
    "valid": true
  },
  "session": {
    "path": "/path/to/session.json",
    "exists": true,
    "authenticated": true,
    "permission_ok": true,
    "device_id_valid": true
  }
}
```

A missing optional config file has `exists: false` and `valid: true`; a config read or parse failure has `valid: false`. `session.device_id_valid` reports whether the persisted trusted-device identifier is absent or valid; a malformed identifier prevents `doctor --connect` from making a misleading probe without it.

## Error Envelope

```json
{
  "ok": false,
  "error": {
    "code": "AUTH_REQUIRED",
    "message": "not logged in",
    "category": "auth",
    "retryable": false
  },
  "meta": {
    "command": "accounts.list",
    "profile": "default",
    "duration_ms": 10,
    "schema_version": "2026-08-07",
    "request_id": "2d3f07a0-8b1e-4cc0-a995-623985ed0c52"
  }
}
```

- `ok`: Always `false` when an error occurs.
- `error.code`: A machine-readable string (e.g., `API_ERROR`, `READ_ONLY_VIOLATION`).
- `error.message`: A human-readable description of the error.
- `error.category`: High-level error grouping (`auth`, `network`, `api`, `validation`, `safety`, `internal`).
- `error.retryable`: Boolean indicating if the operation can be safely retried.
- `error.retry_after_ms`: Present on `RATE_LIMITED` and retryable 5xx errors when the server supplies a `Retry-After` header. Milliseconds to wait before retrying.
- Login challenge codes distinguish rejected credentials (`AUTH_LOGIN_FAILED`), email verification (`AUTH_EMAIL_OTP_REQUIRED`, `AUTH_EMAIL_OTP_INVALID`), and authenticator MFA (`AUTH_MFA_REQUIRED`, `AUTH_MFA_INVALID`).

## Exit Codes

The process exit code is derived from `error.code` (see `internal/errors`). A successful command exits `0`.

| Exit code | Error code | Category |
|---|---|---|
| 0 | (success) | — |
| 1 | `INTERNAL_ERROR` | internal |
| 1 | `RESOURCE_NOT_FOUND` | api |
| 2 | `INVALID_ARGUMENTS` | validation |
| 3 | `AUTH_REQUIRED` | auth |
| 3 | `AUTH_LOGIN_FAILED` | auth |
| 3 | `AUTH_SESSION_EXPIRED` | auth |
| 3 | `AUTH_EMAIL_OTP_REQUIRED` | auth |
| 3 | `AUTH_EMAIL_OTP_INVALID` | auth |
| 3 | `AUTH_MFA_REQUIRED` | auth |
| 3 | `AUTH_MFA_INVALID` | auth |
| 4 | `READ_ONLY_VIOLATION` | safety |
| 5 | `NETWORK_UNREACHABLE` | network |
| 5 | `NETWORK_TIMEOUT` | network |
| 5 | `RATE_LIMITED` | api |
| 6 | `API_ERROR` | api |
| 6 | `API_SCHEMA_CHANGED` | api |
| 6 | `FEATURE_UNAVAILABLE` | api |
| 7 | `VALIDATION_FAILED` | validation |
| 10 | `CONFIRMATION_REQUIRED` | safety |

## Event Stream (NDJSON)

For `accounts refresh --wait`, the CLI emits a stream of progress events when the `--events` flag is set. `--events` implies compact structured output for progress, final, and error envelopes, so each stdout line is valid JSON even when `--pretty` is also present.

```json
{"ok":true,"data":{"is_complete":false,"status":"syncing","accounts":[{"id":"acc_123","has_sync_in_progress":true}]},"meta":{"command":"accounts.refresh.progress","profile":"default","duration_ms":2010,"schema_version":"2026-08-07","request_id":"2d3f07a0-8b1e-4cc0-a995-623985ed0c52"}}
{"ok":true,"data":{"is_complete":true,"status":"complete","accounts":[{"id":"acc_123","has_sync_in_progress":false}]},"meta":{"command":"accounts.refresh.progress","profile":"default","duration_ms":4012,"schema_version":"2026-08-07","request_id":"2d3f07a0-8b1e-4cc0-a995-623985ed0c52"}}
{"ok":true,"data":{"status":"refresh complete"},"meta":{"command":"accounts.refresh","profile":"default","duration_ms":6020,"schema_version":"2026-08-07","request_id":"2d3f07a0-8b1e-4cc0-a995-623985ed0c52"}}
```
