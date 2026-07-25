# 0001 - GraphQL web-session authentication and retry

## Status

Accepted.

## Context

Monarch Money exposes no public, documented API. The only available interface is the GraphQL endpoint used by the Monarch web application. Requests that do not look like they originate from that web app are rejected, and the endpoint applies transient rate limiting and network hiccups that a single request cannot survive.

Two facts about the endpoint drive this decision:

- It authenticates with an `Authorization: Token <token>` header (not the `Bearer` scheme). The token is the session token captured at login.
- It gates responses behind headers the browser sends, including `Client-Platform: web` and a desktop-Chrome `User-Agent`. A default Go `User-Agent` is rejected.

## Decision

The GraphQL client (`internal/graphql`) speaks the web protocol:

- Send `Authorization: Token <token>` for authenticated requests.
- Send `Client-Platform: web` and a desktop-Chrome `User-Agent` on every request. The `User-Agent` is overridable via `MONARCH_USER_AGENT` for callers that need to identify themselves.
- Retry transient (retryable) errors up to three times with exponential backoff of 500ms, 1s, then 2s, aborting early if the context is cancelled. Non-retryable errors return immediately.
- A `401` response maps to `AUTH_SESSION_EXPIRED` so the CLI can tell the user to log in again.

## Consequences

- The client depends on undocumented web behavior; a Monarch web-app change (auth scheme, required headers, response envelope) can break it, surfaced as `API_SCHEMA_CHANGED`.
- Spoofing a browser `User-Agent` is required for the tool to function at all; it is a compatibility measure, not an attempt to hide the client's identity, and can be overridden.
- Backoff bounds retry latency to roughly 3.5s of waiting across three attempts, keeping the CLI responsive while absorbing brief failures.
