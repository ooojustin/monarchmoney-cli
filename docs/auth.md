# Authentication & Session Management

`monarchmoney-cli` uses the unofficial Monarch Money API. It handles authentication, MFA, and session persistence locally and securely.

## Authentication Flow

### Standard Login
Run the following command to start the interactive login process:
```bash
monarch auth login
```
You will be prompted for your email and password.
New devices may also require a one-time code sent to the account email. The CLI identifies this separately from authenticator-based MFA and prompts for the correct code.
After login completes, the CLI prints the account email, the login timestamp, and the local session file path so you can confirm exactly which account was stored.

### Email Verification

Monarch may require a one-time email code when the CLI logs in from a new device, even when account MFA is disabled. The interactive login waits for the code and retries with the same device identifier. Blank input and end-of-file fail immediately without another login request. Noninteractive callers can resume with `--email-otp` or `MONARCH_EMAIL_OTP`.

### MFA Support
If your account has authenticator-based Multi-Factor Authentication enabled:
1. The CLI will detect the requirement and prompt you for the 6-digit code interactively.
2. Alternatively, you can provide the code via the `--mfa-code` flag.

**Automatic MFA:**
If you have your TOTP secret key, you can automate the process:
```bash
monarch auth login --email user@example.com --password "..." --mfa-secret "YOUR_SECRET"
```

## Session Persistence

Once authenticated, a session token is stored locally. This token is used for all subsequent commands.

- **Storage Path**: platform-specific; run `monarch auth session path` to inspect it.
- **Configuration**: set `session_path` in the selected config file or `MONARCH_SESSION_PATH` in the environment.
- **Security**: The file is saved with `0600` permissions (read/write by owner only).
- **Contents**: The session file stores the token, account email, timestamps, and profile. A non-secret device identifier is stored beside it as `device-id`, survives logout, and binds email verification to subsequent login attempts.

### Secret Indirection (`env:NAME`)

The stored `token` value may be a literal token or the indirection form `env:NAME`. When the token is `env:NAME`, it is resolved from the environment variable `NAME` each time the session is loaded, so the real secret never has to sit in the file. If `NAME` is unset, the CLI fails with an explicit error rather than proceeding unauthenticated.

This is the agent-first alternative to an OS keychain (which cannot be used unattended). For example, write a `0600` session file containing `"token": "env:MONARCH_TOKEN"` and supply the secret from the environment or a secret manager at run time:

```bash
export MONARCH_TOKEN="..."
monarch accounts list --json
```

See `docs/adr/0006-secrets-storage.md` for the full rationale.

### Checking Status
To check if you have a valid local session:
```bash
monarch auth status
```
This command now performs a live Monarch identity check by default, so it can tell you whether the stored session is still valid.
If the token has expired or been revoked, the command reports `AUTH_SESSION_EXPIRED` and keeps the stored email visible so you know which account needs to be re-authenticated.

### Logging Out
To remove the local session token:
```bash
monarch auth logout
```

## Session Status and Local Commands

- `auth login`, `auth status`, `auth logout`, and remote commands use the same configured session path.
- `auth status` reports the stored email, the last login time, and whether the session is still valid.
- Cache-backed commands such as `cache stats`, `cache search`, and `hledger backup` work entirely from local data without a session or network access. Everything else talks to the Monarch API and requires a valid session.

## Local Cache Database

The local cache lives at `~/.monarchmoney-cli/cache/monarch.sqlite`.

- It is a standard SQLite database, not AES-256 encrypted.
- The cache file is created with `0600` permissions and the directory is created with `0700` permissions.
- It may contain cached account and transaction data, so treat it as sensitive local data.
- `cache sync` is manual. It upserts all accounts, up to the latest 1000 transactions (optionally filtered with `--from YYYY-MM-DD`), and replaces the investment-holdings snapshot on every run.
- The transaction cache is cumulative: rows returned by later syncs replace matching IDs and add new IDs, but old cached rows are not removed automatically.
- The cache stores archive-grade detail (tags, splits, review state, category groups, raw merchant names, account lifecycle flags, holdings, closing balances) so offline commands and ledger backups never need the API. A cache created by an older version of the CLI is rejected with a prompt to re-run `cache sync`, which rebuilds it.
- The cache is not a complete mirror of Monarch. Remote deletions are not reconciled locally; use `cache cleanup --before YYYY-MM-DD` to explicitly prune old cached transactions.
- If you want stronger at-rest protection, rely on full-disk encryption such as FileVault or store the profile on an encrypted volume.

## Security Best Practices

1. **Permissions**: Ensure your `~/.monarchmoney-cli` directory has `0700` permissions.
2. **Environment Variables**: For scripts, you can use environment variables instead of interactive prompts:
   - `MONARCH_EMAIL`: Your account email address.
   - `MONARCH_PASSWORD`: Your account password.
   - `MONARCH_MFA_CODE`: A 6-digit MFA code (for single-use scripts).
   - `MONARCH_MFA_SECRET`: Your TOTP secret key for automatic code generation.
   - `MONARCH_EMAIL_OTP`: A one-time code sent to the account email.
   - `MONARCH_SESSION_PATH`: Override the local session file path.
   - `MONARCH_USER_AGENT`: Override the default HTTP User-Agent string.
3. **Session Safety**: Never share your `session.json` file. It contains a long-lived token that grants access to your Monarch account.

### Credential Security

**Prefer environment variables over CLI flags.** The `--password` and `--mfa-secret` flags expose credentials in the process table (`/proc/PID/cmdline`), which is visible to other processes on the system. Environment variables are safer for scripts and automation:

```bash
# Safer: credentials via environment variables
export MONARCH_PASSWORD="..."
export MONARCH_MFA_SECRET="..."
monarch auth login --email user@example.com

# Less safe: credentials visible in process table
monarch auth login --email user@example.com --password "..." --mfa-secret "..."
```

The interactive prompt (default behavior, no flags) is the most secure option for manual use, as it reads the password via `term.ReadPassword()` without echoing.
