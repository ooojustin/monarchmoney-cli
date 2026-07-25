# monarchmoney-cli Domain Glossary

## Core Creed

**monarchmoney-cli is a replacement for the Monarch Money web app**, additionally providing automation convenience and agent-friendliness.

## Core Concepts

**monarchmoney-cli** — A local CLI tool for Monarch Money, used to query, manage, and automate personal finance data. The installed binary is `monarch`.

## Users

**Personal finance user** — Someone who needs to manage their Monarch Money account from the terminal.

**Agent** — An automation agent. Requires deterministic behavior.

## Command Design Decisions

**Safety gates** — A three-tier safety mechanism: `--read-only`, `--dry-run`, `--confirm`.

**Audit log** — Every remote write operation is recorded to a JSONL file.

**Cache** — A local SQLite cache used to speed up queries.
