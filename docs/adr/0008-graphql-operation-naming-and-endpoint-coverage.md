# 0008 - GraphQL operation naming and endpoint coverage

## Status

Accepted.

## Context

The CLI initially used short, descriptive names for GraphQL operations (e.g., `GetAccount`, `CreateTag`, `SetBudget`). These names were invented locally and did not match the operation names Monarch's own web app sends. Other open-source clients (`hammem/monarchmoney`, `312-dev/monarchmoney`) reverse-engineered and use the official Monarch operation names (e.g., `AccountDetails_getAccount`, `Common_CreateTransactionTag`, `Common_UpdateBudgetItem`).

This created a gap: when debugging network traffic or cross-referencing with upstream clients, the operation names did not match, making it harder to identify which Monarch API operation was being called.

Additionally, the CLI was missing five endpoints that `312-dev/monarchmoney` had reverse-engineered:
- `Common_SavingsGoals` — rich goals data (progress, balance, target, contribution)
- `GetSavingsGoals` — savings goal monthly budget amounts
- `Web_UpdateCategory` — update category properties
- `GetCategoryRollover` — category rollover settings
- `Common_UpdateCategoryGroup` — update category group settings

The CLI's `Web_GoalsV2` query only fetched `id` and `name`, making the goals command nearly useless. The `SetBudget` mutation used `setBudget` (a possibly deprecated field) instead of `updateOrCreateBudgetItem` (the field the web app and upstream clients use).

## Decision

1. **Adopt official Monarch operation names.** All GraphQL operations now use the names Monarch's web app sends, matching `hammem/monarchmoney` and `312-dev/monarchmoney`. Fifteen operations were renamed (e.g., `GetAccount` → `AccountDetails_getAccount`, `CreateTag` → `Common_CreateTransactionTag`, `GetJointPlanningData` → `Common_GetJointPlanningData`).

2. **Migrate `SetBudget` to `Common_UpdateBudgetItem`.** The mutation now calls `updateOrCreateBudgetItem` with `timeframe`, `startDate`, and `applyToFuture` parameters, matching the upstream Python client. The response shape changed from `setBudget.budget.planned` to `updateOrCreateBudgetItem.budgetItem.budgetAmount`.

3. **Replace `Web_GoalsV2` with `Common_SavingsGoals`.** The goals list query now fetches the full savings goal shape (progress, currentBalance, targetAmount, plannedMonthlyContribution, etc.) using the `savingsGoals` field instead of the minimal `goalsV2` field.

4. **Add five new endpoints.** `Common_SavingsGoals`, `GetSavingsGoals`, `Web_UpdateCategory`, `GetCategoryRollover`, and `Common_UpdateCategoryGroup` are now implemented with service methods, CLI commands, and tests.

## Consequences

- Operation names in network traffic now match Monarch's web app and upstream clients, improving debuggability.
- The `SetBudget` response no longer includes `CategoryName` (the `updateOrCreateBudgetItem` response only returns `budgetItem.id` and `budgetItem.budgetAmount`). The CLI's human output now shows the category ID instead.
- The `Goal` struct expanded from 2 fields (`ID`, `Name`) to 16 fields, enriching the goals list output.
- New CLI commands: `goals budgets`, `categories update`, `categories rollover`, `categories groups update`.
