# Guide: Clean up transactions end to end

**Scenario:** Month-end is here, the credit card feed dumped a pile of `needs review` transactions, and you want to search, inspect, fix, tag, and then stop this category of noise from coming back with a rule.

This guide exercises the full safety path every mutation follows: **dry-run first, then `--confirm`**. Nothing is sent to Monarch until you explicitly confirm.

All output blocks in the guides were captured from real runs of `monarch` and then anonymized: IDs, names, and amounts are replaced, but the output shape is exactly what the tool emits.

## Step 1: find the work

Search by text across merchant, notes, and category:

```console
$ monarch transactions search "Maple Roast" --from 2026-01-01 --limit 5 --json
{"ok":true,"data":{"total":2,"transactions":[{"id":"331288001122334455","date":"2026-07-12","amount":-6.75,"merchant":"Maple Roast Coffee","category":"Shopping","category_group":{"id":"221000445566778899","name":"Food & Dining","type":"expense"},"notes":"","tags":[{"id":"221000990011223344","name":"Receipt Import","color":"#8e4ec6"}],"goal":{"id":"","name":""},"pending":false,"hide_from_reports":false,"plaid_name":"MAPLE ROAST #212","data_provider_description":"MAPLE ROAST COFFEE TORONTO","is_recurring":false,"review_status":"needs_review","needs_review":true,"is_split_transaction":false,"created_at":"2026-07-14T21:46:18.576612+00:00","updated_at":"2026-07-29T00:35:11.006715+00:00","account_id":"221000112233445566","account_order":23,"account_type_group":"liability","owner_display_name":""}]},"meta":{"command":"transactions.search","profile":"default","duration_ms":287,"schema_version":"2026-08-23","request_id":"c1a4e9d0-...","warnings":["uses legacy Monarch GraphQL root field: allTransactions"]}}
```

(Second match omitted.) Note `plaid_name` / `data_provider_description`: the raw bank-feed names. Rules can match those even when the display merchant differs.

For a systematic pass, list everything Monarch flagged for review (add `--from`/`--to` or filters like `--category-id`, `--account-id`, `--pending`, `--is-split`):

```console
$ monarch transactions list --needs-review --from 2026-07-01 --to 2026-07-31 --json
```

## Step 2: inspect before touching

```console
$ monarch transactions show 331288001122334455 --json
{"ok":true,"data":{"id":"331288001122334455","date":"2025-12-26","amount":-18.62,"merchant":"Maple Roast Coffee","category":"Coffee Shops","category_group":{"id":"","name":"","type":""},"notes":"","tags":[{"id":"221000990011223300","name":"Reimburse","color":"#00a2c7"}],"goal":{"id":"","name":""},"pending":false,"hide_from_reports":false,"plaid_name":"MAPLE ROAST #212","data_provider_description":"","is_recurring":false,"review_status":"","needs_review":false,"is_split_transaction":true,"created_at":"2026-01-05T11:07:59.695264+00:00","updated_at":"2026-01-05T11:07:59.695282+00:00","account_id":"221000112233445566","account_order":0,"account_type_group":"","owner_display_name":""},"meta":{"command":"transactions.show","profile":"default","duration_ms":314,"schema_version":"2026-08-23","request_id":"c0b81015-9a84-4421-afda-0416f1190aad"}}
```

If `is_split_transaction` is true, see the parts:

```console
$ monarch transactions splits 331288001122334455 --json
```

Category IDs come from `monarch categories list --json`; look up what a category belongs to with `monarch categories groups`.

## Step 3: preview the edit

Try to update without any safety flags and the CLI refuses:

```console
$ monarch transactions update 331288001122334455 --category 22100044550011223344
{"ok":false,"error":{"code":"CONFIRMATION_REQUIRED","message":"this mutation operation requires --confirm to execute","category":"safety","retryable":false},"meta":{"command":"transactions.update","profile":"default","duration_ms":0,"schema_version":"2026-08-23","request_id":"9fee843f-dcf2-4484-89ac-df7a2c8c01e1"}}
$ echo $?
10
```

Preview instead with `--dry-run` - it prints a mutation plan without contacting the write API:

```console
$ monarch transactions update 331288001122334455 --notes "team coffee, reimbursable" --dry-run --json
{"ok":true,"data":{"planned_mutations":[{"operation":"transactions.update","resource_id":"331288001122334455","after":{"amount":null,"categoryId":null,"date":null,"hideFromReports":null,"merchant":null,"needsReview":null,"notes":"team coffee, reimbursable"}}]},"meta":{"command":"transactions.update","profile":"default","duration_ms":0,"schema_version":"2026-08-23","request_id":"778c0632-eab9-478d-9aa0-49dcdfadc0d1"}}
```

The plan shows exactly which fields will change (`null` = untouched).

## Step 4: execute

Happy with the plan? Re-run with `--confirm`:

```console
$ monarch transactions update 331288001122334455 --notes "team coffee, reimbursable" --confirm --json
```

With `--confirm --json` the envelope's `data` carries the updated transaction exactly as Monarch returns it; without `--json` the CLI prints `Successfully updated transaction <id>.` The executed mutation is recorded in the audit log at `~/.monarchmoney-cli/audit/YYYY-MM-DD.jsonl` ([Safety Model](../safety.md)).

`update` also takes `--date`, `--amount`, `--merchant`, `--category`, `--hide-from-reports`, `--needs-review`, and `--mark-reviewed`.

## Step 5: bulk-categorize the rest

For the remaining coffee-shop charges that all need the same category:

```console
$ monarch transactions bulk-categorize --category-id 221000556677889900 --id 331288001122334455 --id 331288001122334456 --dry-run --json
```

Review the plan, then re-run with `--confirm`. (`--mark-reviewed` defaults to true; pass `--mark-reviewed=false` to keep review state.)

## Step 6: tags

Tag the reimbursable ones so expense reports are one filter away:

```console
$ monarch transactions tags add 331288001122334455 --tag 221000990011223300 --confirm
```

`tags set <tx-id> --tag ... --confirm` replaces the whole tag list; `tags clear <tx-id> --confirm` removes all tags. Create new tags with `monarch tags create --name "reimbursable" --confirm` and find IDs with `monarch tags list --json`.

## Step 7: make it stick with a rule

So next month's charges categorize themselves - match on the raw feed name and assign the category automatically:

```console
$ monarch rules create --merchant-operator contains --merchant-value "MAPLE ROAST" --set-category-id 221000556677889900 --dry-run --json
```

Then re-run with `--confirm`. Useful extras: `--apply-to-existing` retro-applies the rule, `--account-id` scopes it to one account, `--amount-operator/--amount-value` gate by amount, `--add-tag-id` adds a tag on match. List and manage existing rules with `monarch rules list`, `rules update <id>`, `rules delete <id>`.

## Finding duplicates

Double-charges and feed replays are a common month-end find. The CLI pulls your transactions, groups them by date + amount + raw feed name (`plaid_name`) within the same account, and returns every member of a duplicated group:

```console
$ monarch transactions duplicates --from 2026-07-01 --to 2026-07-31 --json
{"ok":true,"data":[],"meta":{"command":"transactions.duplicates","profile":"default","duration_ms":412,"schema_version":"2026-08-23","request_id":"6b0e2f8a-..."}}
```

An empty array means nothing matched. When groups appear, each element is the same transaction object you get from `transactions show` - inspect both members with `transactions show <id>`, keep one, and remove the other with `transactions delete <id> --dry-run` -> `--confirm`.

## Next steps

- [Monthly review](monthly-review.md) - verify the effect of your cleanup against budgets and trends.
- [Agent automation](agent-automation.md) - let an agent propose these dry-run plans for you to approve.
- [Commands](../../COMMANDS.md) - every flag for the commands above.
