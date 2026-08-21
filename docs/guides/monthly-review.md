# Guide: Monthly financial review

**Scenario:** It's the first of the month. In ten minutes you want to know what last month actually cost, which budgets drifted, which subscriptions grew, and whether anything looks anomalous — then decide what to fix.

Every command below is read-only; nothing here can mutate data. The `analyze` commands do their grouping and arithmetic deterministically in code — no AI, no subjective advice.

All output blocks in the guides were captured from real runs of `monarch` and then anonymized: IDs, names, and amounts are replaced, but the output shape is exactly what the tool emits.

## Step 1: the one-screen answer

```console
$ monarch overview --from 2026-07-01 --to 2026-07-31 --json
{"ok":true,"data":{"as_of":"2026-08-01T09:00:00Z","net_worth":30773.64,"account_count":53,"cashflow":{"income":5401.28,"expense":-2955.65,"savings":2445.63,"savings_rate":0.45},"transactions":[],"transaction_total":55},"meta":{"command":"overview","profile":"default","duration_ms":511,"schema_version":"2026-05-08","request_id":"a4d1f002-..."}}
```

Net worth, cashflow, and the 10 most recent transactions in one call (`transactions` array omitted above; `transaction_total: 55`). Without `--from`/`--to` it covers the current month. If something looks off, drill into it with the steps below.

## Step 2: cashflow detail

```console
$ monarch cashflow summary --from 2026-07-01 --to 2026-07-31 --json
{"ok":true,"data":{"income":5401.28,"expense":-2955.65,"savings":2445.63,"savings_rate":0.4527871171277919},"meta":{"command":"cashflow.summary","profile":"default","duration_ms":288,"schema_version":"2026-05-08","request_id":"3c9b81de-..."}}
```

`savings_rate` is a fraction (0.45 = 45%). For where the money went:

```console
$ monarch cashflow spending --from 2026-07-01 --to 2026-07-31 --json
{"ok":true,"data":{"by_category":[{"name":"Paychecks","amount":5343.40},{"name":"Interest","amount":14.09},{"name":"Rent","amount":-2500.00},{"name":"Groceries","amount":-199.42},{"name":"Restaurants & Bars","amount":-93.65},{"name":"Gas & Electric","amount":-105.74}],"net":2445.63,"period":{"end_date":"2026-07-31","start_date":"2026-07-01"},"total_expenses":-2955.65,"total_income":5401.28},"meta":{"command":"cashflow.spending","profile":"default","duration_ms":301,"schema_version":"2026-05-08","request_id":"e77d20b8-..."}}
```

Income categories are positive, expenses negative. Variants: `cashflow categories`, `cashflow merchants`, `cashflow list`.

## Step 3: direction of travel

One month is a snapshot; three tells you the trend:

```console
$ monarch cashflow trends --from 2026-05-01 --to 2026-07-31 --group-by category-group --period month --json
{"ok":true,"data":[{"group_id":"221000112233001122","group_name":"Food & Dining","group_type":"expense","period":"2026-05-01","sum":-3101.77,"sum_income":0,"sum_expense":-3101.77},{"group_id":"221000112233001122","group_name":"Food & Dining","group_type":"expense","period":"2026-06-01","sum":-2955.65,"sum_income":0,"sum_expense":-2955.65},{"group_id":"221000112233001122","group_name":"Food & Dining","group_type":"expense","period":"2026-07-01","sum":-3010.18,"sum_income":0,"sum_expense":-3010.18}],"meta":{"command":"cashflow.trends","profile":"default","duration_ms":274,"schema_version":"2026-05-08","request_id":"b2e0447c-..."}}
```

(Remaining category groups omitted.) `--group-by category` gives per-category rows instead of groups.

`--group-by category` gives per-category rows instead of groups.

## Step 4: budget drift

```console
$ monarch budgets list --month 2026-07 --json
{"ok":true,"data":[{"category_id":"221000556677889900","category_name":"Groceries","planned":400,"actual":-437.12},{"category_id":"221000556677889901","category_name":"Coffee Shops","planned":40,"actual":-52.30},{"category_id":"221000556677889902","category_name":"Advertising & Promotion","planned":0,"actual":0}],"meta":{"command":"budgets.list","profile":"default","duration_ms":265,"schema_version":"2026-05-08","request_id":"9d33c1aa-..."}}
```

(Remaining budget rows omitted.) Compare `actual` against `planned` per category. Fix drift with `monarch budgets set --category <id> --amount N --confirm` (dry-run first), or reset the whole month with `monarch budgets reset`.

## Step 5: anomalies worth your attention

```console
$ monarch analyze anomalies --month 2026-07 --json
{"ok":true,"data":{"anomalies":[{"category":"Financial & Legal Services","current_month":110.29,"avg_history":3.68,"ratio":29.98,"severity":"high","largest_merchant":"Monarch","largest_amount":110.29},{"category":"Rent","current_month":2500.00,"avg_history":452.79,"ratio":5.52,"severity":"high","largest_merchant":"Yunfei Long","largest_amount":2500.00},{"category":"Groceries","current_month":199.42,"avg_history":107.97,"ratio":1.85,"severity":"low","largest_merchant":"T&T Supermarket","largest_amount":166.09}],"period":{"end_date":"2026-07-31","start_date":"2026-07-01"}},"meta":{"command":"analyze.anomalies","profile":"default","duration_ms":298,"schema_version":"2026-05-08","request_id":"5fa09b31-..."}}
```

Each row compares this month's category spend to its own history (`ratio`, `severity`). The first two rows above are explainable (an annual subscription; a real rent move) — the value of this step is that unexplained ones can't hide. Investigate any surprise with `monarch transactions search <merchant> --from ... --to ...`.

## Step 6: subscriptions

```console
$ monarch analyze subscriptions --json
{"ok":true,"data":{"subscriptions":[{"merchant":"Payment to Apple Services","monthly":-5.27,"annual":-63.24,"frequency":"monthly","last_charge":"2026-08-09","next_charge":"2026-09-09","category":"Electronics","is_approximate":false},{"merchant":"Revenue Services Bc Victoria","monthly":-75.00,"annual":-900.00,"frequency":"monthly","last_charge":"2026-08-17","next_charge":"2026-09-17","category":"Insurance","is_approximate":false}],"total_monthly":-118.89,"total_annual":-1426.64,"potential_overlaps":[{"group":"Streaming","services":["Netflix","Max"],"combined_annual":-287.40}]},"meta":{"command":"analyze.subscriptions","profile":"default","duration_ms":305,"schema_version":"2026-05-08","request_id":"70cc41d9-..."}}
```

Scan for services you forgot about; `potential_overlaps` flags merchants that look like duplicates of the same service. Cancel by acting on the merchant's website, or at least tag and watch them.

## Step 7: mid-month burn control

For the *current* month, compare pace-of-spending against pace-of-calendar before overspending becomes history:

```console
$ monarch analyze burn-rate --month 2026-08 --json
{"ok":true,"data":{"budgets":[{"category":"Education","budgeted":410.00,"spent":3032.00,"remaining":-2622.00,"days_elapsed":21,"days_total":31,"burn_pct":739.51,"time_pct":67.74,"status":"overspending"},{"category":"Groceries","budgeted":400.00,"spent":322.10,"remaining":77.90,"days_elapsed":21,"days_total":31,"burn_pct":80.53,"time_pct":67.74,"status":"on_track"},{"category":"Interest","budgeted":13.31,"spent":1.07,"remaining":12.24,"days_elapsed":21,"days_total":31,"burn_pct":8.04,"time_pct":67.74,"status":"underused"}],"period":{"end_date":"2026-08-31","start_date":"2026-08-01"}},"meta":{"command":"analyze.burn-rate","profile":"default","duration_ms":322,"schema_version":"2026-05-08","request_id":"88ac71e0-..."}}
```

`status` compares `burn_pct` against `time_pct`: `overspending` (at 100% of budget, or ≥10 points ahead of the calendar), `ahead` (≥5 points ahead), `underused` (≥25 points behind), otherwise `on_track`. This is the step to run *during* the month, not after.

## Next steps

- [Transactions workflow](transactions-workflow.md) — fix whatever steps 5–7 surfaced.
- [Ledger backup](ledger-backup.md) — close the month by refreshing your plain-text archive.
- [JSON Schema](../../JSON_SCHEMA.md) — field-level contract for everything shown above.
