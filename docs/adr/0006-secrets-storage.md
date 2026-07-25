# 0006 - Secrets storage

## Status

Accepted.

## Context

monarchmoney-cli holds one bearer secret: the Monarch Money session token, persisted in `~/.monarchmoney-cli/session.json`. Anyone holding that token can act as the user against the Monarch API. Two questions follow: where does the secret live, and how does it stay out of plaintext when a user or agent prefers not to write it to disk.

An obvious option is the operating-system keychain (macOS Keychain, the Freedesktop Secret Service, Windows Credential Manager). But this CLI is agent-first: its primary callers are scripts, CI jobs, and unattended local agents. Keychain access on those platforms triggers an interactive unlock prompt on first use and after re-lock, which hangs a headless process with no one to answer it. A tool that blocks waiting for a GUI prompt is unusable in the environment it is built for.

## Decision

The secrets contract is a `0600` file plus environment-variable indirection, consistent with the rest of the CLI fleet. There is no keychain integration.

- The session file is written atomically-in-effect with `0600` permissions (owner read/write only) in a `0700` directory; the local SQLite cache is likewise `0600` in a `0700` directory.
- The stored `token` may be a literal token, or the indirection form `env:NAME`. When it is `env:NAME`, the value is resolved from environment variable `NAME` when the session is loaded. If `NAME` is unset, loading fails with an explicit error — the CLI never silently proceeds with an empty token.
- Indirection is resolved once, in `auth.Store.Load`, the single point where a stored session becomes usable credentials, so every caller (`accounts`, `transactions`, `auth status`, `doctor`, the cache sync) sees the same resolved token and the same failure.

## Consequences

### Positive

- Fully headless: no GUI prompt can ever block an unattended run. An agent can write `{"token":"env:MONARCH_TOKEN", ...}` to a `0600` session file and supply the real token from the environment at run time, keeping the secret out of the file entirely.
- The behaviour is identical across macOS, Linux, and Windows; there is no per-platform keychain code path to maintain or debug.
- A literal token continues to work unchanged, so existing sessions are unaffected.

### Negative

- A literal token left in the session file is protected only by filesystem permissions, not by an OS-level encrypted store.
- The indirection resolves eagerly at load, so a session referencing an unset variable is an error rather than a fallback.

### Mitigations

- `env:NAME` is the documented path for callers who do not want a plaintext token on disk; combined with full-disk encryption (FileVault, LUKS) it covers the at-rest threat model appropriate for a single-user local tool.
- The unset-variable error names the exact variable, so the fix is obvious.
