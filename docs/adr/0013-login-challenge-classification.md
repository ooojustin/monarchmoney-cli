# 0013 - Login challenge classification

## Status

Accepted.

## Context

Monarch's undocumented login endpoint can challenge a new device with an email one-time code even when account MFA is disabled. It returns structured `error_code` values that distinguish email verification from TOTP MFA. Treating every `401` or `403` as MFA discards that distinction, prompts for a code the user may not have, and misreports invalid credentials or other authentication denials.

Monarch also associates login challenges and authenticated requests with a client-generated device UUID. The initial request and challenge response must use the same value, and successful sessions must retain it.

## Decision

- Advertise email OTP and MFA support in login requests. Do not advertise challenge types the CLI cannot complete.
- Persist one device UUID independently of session credentials, reuse it for challenge retries and later invocations, retain it across logout, and send it on authenticated GraphQL and REST requests.
- Decode bounded login responses before classifying failures.
- Map explicit email OTP and TOTP challenges to distinct auth error codes. Map ambiguous authentication denials to `AUTH_LOGIN_FAILED` instead of inferring MFA from HTTP status.
- Keep all login failures on exit code 3.
- Consume interactive email and challenge input by line. Blank input and end-of-file fail without another request.
- Report device identity validity through `doctor` and skip connectivity probes when that state is corrupt.

## Consequences

- New-device login works for accounts that require email verification but do not use TOTP MFA.
- JSON consumers can distinguish rejected credentials, email verification, and TOTP MFA without parsing messages.
- Existing session files remain valid. The first login attempt creates the separate device identifier before making a request.
- The implementation remains coupled to an undocumented protocol. Unknown challenges fail as `AUTH_LOGIN_FAILED` with bounded server detail rather than being guessed.
