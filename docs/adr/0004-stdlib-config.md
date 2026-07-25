# 0004 - Standard-library configuration

## Status

Accepted.

## Context

Configuration was loaded through `spf13/viper`: a global singleton that merged persistent flags, `MONARCH_*` environment variables, and an optional `~/.monarchmoney-cli/config.yaml` file into a handful of settings (profile, api_endpoint, timeout, read_only, session_path, audit_log, cache_path, output).

`viper` is a heavy dependency — it pulls in `fsnotify`, `afero`, `mapstructure`, `pflag` bindings, and several codec libraries — and monarch used almost none of its feature set: no config watching, no remote providers, no multiple config formats, no sub-trees. It resolves about six scalar keys. Across the sibling CLI fleet, four of five tools already resolve config with the standard library (a plain YAML unmarshal plus explicit environment overrides); `viper` was the lone exception, so contributors and agents faced two different config idioms depending on which tool they were in.

## Decision

Remove `spf13/viper` and resolve configuration with the standard library, matching the fleet pattern (as in zenodo-cli and flickr-cli): read the YAML file with `gopkg.in/yaml.v3`, then apply explicit `MONARCH_*` environment overrides.

- `config.Load(path)` returns defaults overlaid with the file (when present) and then environment variables, giving precedence **env > file > defaults**. The root command applies command-line flags on top, so the overall precedence is **flags > env > file > defaults** — identical to before.
- Every previously supported `MONARCH_*` variable and every config-file key keeps working: the file format is unchanged, so an existing `config.yaml` loads exactly as it did.
- A missing config file is not an error (all keys have defaults). An unreadable or malformed file returns an error while still yielding a usable, defaults-populated `*Config`, so no caller can nil-dereference and read/mutation commands can fail loud on a broken file.
- The root command reads its persistent flags directly from Cobra and applies environment overrides explicitly, rather than binding them through a global `viper` singleton.

## Consequences

- `viper` and its transitive dependencies (`fsnotify`, `afero`, `cast`, `mapstructure`, `pflag` as a direct dep, `gotenv`, and others) are dropped from the module graph via `go mod tidy`, shrinking the dependency surface.
- Config resolution is now the same idiom as the rest of the fleet: read a struct, apply env overrides — easy to read and to audit, with no reflection-driven magic.
- The precedence rules are written out in code instead of being implied by `viper`'s internal ordering, which makes them explicit and testable.
- One deliberate behaviour refinement: a malformed config file, previously ignored silently, now surfaces as an error in commands that check it — a fail-loud improvement over the previous silent fallback to defaults.
