# 0018 - Expired session retry classification

## Status

Accepted.

## Context

A `401` from the GraphQL endpoint produced an `AUTH_SESSION_EXPIRED` error carrying `retryable: true`. The retry loop branches on that flag alone, with no allowlist of codes, so an expired token was resent four times with 537ms, 1074ms, and 2111ms of backoff between attempts. A test exercising that path measured 3.73 seconds.

There is no token refresh anywhere in the CLI. The session store holds no expiry and no refresh token, and the client's token is never mutated after construction. Every one of the four attempts sent a byte-identical dead authorization header, so the second, third, and fourth could not produce an outcome different from the first.

The flag was never a decision. The `401` was marked retryable before any retry loop existed, when the field was an envelope annotation with no behavioural effect. A later change introduced retry driven by that flag, for transient network errors, and promoted the pre-existing value into behaviour without revisiting it. ADR 0001 records both the retry policy and the `401` mapping but never claims a `401` is transient; it states the mapping exists so the CLI can tell the user to log in again, which is a terminal action.

The value also contradicted shipped guidance. The agent guide tells callers that exit code 3 means prompt the user to log in, while the envelope told the same callers the request was safe to retry. The REST login path had already made the opposite call, classifying its own `401` as a non-retryable login failure. The GraphQL `401` was the only auth error in the repository marked retryable.

## Decision

A `401` produces `AUTH_SESSION_EXPIRED` with `retryable: false`.

`error.retryable` means the identical request may succeed on a later attempt. A failure that can only be cleared by a credential the process does not hold is not retryable, whatever its transport shape. The test is whether a repeat can differ, not whether the caller was at fault.

Should a refresh path ever exist, retry belongs at that path rather than in the transport loop. Refreshing changes the request, which is precisely what this classification says is missing.

## Consequences

- An expired session fails after one request instead of four, returning roughly 3.7 seconds sooner. `doctor --connect` against an expired session returns immediately.
- The envelope stops advising callers to retry a request that cannot succeed, so `error.retryable` and the exit-code-3 guidance agree.
- The published value of `error.retryable` for `AUTH_SESSION_EXPIRED` changes from `true` to `false`. A caller branching on it stops instead of looping.
- A regression test asserts one attempt and zero scheduled backoffs, so the latency claim is pinned rather than the request count alone.
- ADR 0015's schema-version policy is extended to name the retryable classification of an existing error code, a case it did not cover.
