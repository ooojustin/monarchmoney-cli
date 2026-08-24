# 0017 - Date range resolution

## Status

Accepted.

## Context

Commands that accept `--from` and `--to` filled in missing values in two different places under two different rules. Five cashflow reporting subcommands resolved each end independently in the command layer, then validated the resolved pair; `cashflow trends` required both bounds. `overview` resolved in the service layer, below validation, and reset both ends whenever either was absent.

The joint reset made a one-sided range unanswerable. `overview --from 2026-01-01` reported the current month: sixty transactions and no income, where the same range stated explicitly reported one thousand one hundred and ninety-nine transactions and eighty-seven thousand dollars of income. The command exited zero. A future start date behaved the same way.

The defect was structural rather than arithmetic. The command layer validated the flags a user typed, the service substituted different values, and no layer saw both. Validation could not catch what it never observed, and the response carried no record of the range actually queried, so the substitution was invisible downstream.

The two rules also differed at the end of the window: cashflow ended at today, `overview` at the last day of the current month. Measured against a live account the difference is empty, because no transaction is dated in the future and net worth is computed from accounts, which take no date at all.

## Decision

`--from` and `--to` resolve before they are validated, in the command layer, through one helper. Each end resolves independently: a missing `--from` becomes the first day of the current month, a missing `--to` becomes today. A lone `--from` therefore reads as "from that date through today".

Commands validate the resolved pair rather than the typed flags. A lone `--to`, or a `--from` after today, resolves to an inverted range and is refused instead of being silently replaced.

The window follows the user's calendar day, not UTC. Deriving it from a UTC clock would end the default window on tomorrow's date for anyone west of Greenwich during their evening, which is the same class of silent substitution this decision removes. The helper takes the current time as a parameter so a test can pin the behaviour at an hour where the two dates disagree.

Services perform no date defaulting. They query the range the command layer resolved. The account-history command defaults to one year ago through today because its upstream series is a bounded lookback rather than a current-month report.

A response scoped to a date range reports the range it used, for humans and in JSON.

Commands whose empty date already means "no filter" keep passing it through: the transaction queries omit an absent bound rather than defaulting it. Commands that require both dates keep requiring them, and single-date lookbacks keep their own windows. Sharing the resolver with those would change what an omitted flag means.

## Consequences

- `overview --from X` answers for the range requested instead of the current month.
- `overview --to X` alone, and any start date after today, exit 2 rather than returning current-month figures.
- `overview` reports `start_date` and `end_date` in JSON and prints the period for humans, so the range a caller receives is checkable against the range it asked for.
- One resolution rule covers six commands, so changing the default window is one edit rather than six.
- `overview` with no flags ends at today rather than at the end of the current month. No emitted figure changes.
- The default end date is a local calendar date. A test fixture set in the evening of a western time zone pins it, because the failure it prevents is invisible for most of the day.
- Callers of the service package resolve their own dates. Account history requires both bounds; transaction filters preserve an empty bound as no filter.
- Monarch's account `recentBalances` response contains values without dates. The series ends on the caller's current local date and may omit days before an account's first balance, so account history assigns dates backward from today before applying the requested inclusive range.
- An ADR whose central decision still holds is corrected in place rather than superseded. ADR 0007's statement of the overview default now points here.
