# 0009 - App-owned command tree and process boundaries

## Status

Accepted.

## Context

The CLI command tree, persistent flags, process streams, session access, HTTP clients, audit writer, and exit behavior were package-global. Tests replaced global functions and `http.DefaultTransport`, so command executions could leak state into one another and could not run safely in parallel.

The configuration and HTTP implementations are intentionally concrete. The missing boundary is command orchestration, not a generic framework around domain packages.

## Decision

`cli.App` owns one Cobra tree, its persistent flag state, loaded configuration, request ID, and explicit process dependencies. `cli.New` constructs an independent tree. The process entrypoint constructs a fresh App, while tests construct Apps with local streams, transports, stores, and effect functions.

Command families expose private builders that attach fresh command and flag state to each App. Domain packages remain independent of Cobra. HTTP and storage options stay concrete and private unless a real process boundary requires injection.

## Consequences

- Independent CLI executions no longer share parsed flags or command-local state.
- An App is single-use; repeated execution is rejected instead of retaining Cobra flag state.
- Tests can inject transports and effects without mutating process globals.
- Configuration remains implemented by the stdlib loader; the App owns precedence between config, environment, and Cobra flags.
- Adding a command requires registering its builder on `App.buildRoot` and testing the resulting command topology.
