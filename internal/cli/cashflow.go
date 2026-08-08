package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/thedavidweng/monarchmoney-cli/internal/errors"
	"github.com/thedavidweng/monarchmoney-cli/internal/monarch"
	"github.com/thedavidweng/monarchmoney-cli/internal/money"
	"github.com/thedavidweng/monarchmoney-cli/internal/output"
)

func resolveCashflowDates(startDate, endDate string, now time.Time) (resolvedStartDate, resolvedEndDate string) {
	if startDate == "" {
		startDate = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
	}
	if endDate == "" {
		endDate = now.Format("2006-01-02")
	}
	return startDate, endDate
}

func (a *App) buildCashflowCommand() *cobra.Command {
	var (
		startDate   string
		endDate     string
		groupBy     string
		period      string
		accountIDs  []string
		categoryIDs []string
	)

	cmd := &cobra.Command{
		Use:     "cashflow",
		Short:   "Manage Monarch Money cashflow",
		GroupID: "core",
		Example: "  monarch cashflow summary --from 2026-01-01 --to 2026-03-31\n  monarch cashflow spending --from 2026-01-01 --json",
	}
	cmd.PersistentFlags().StringVar(&startDate, "from", "", "start date (YYYY-MM-DD)")
	cmd.PersistentFlags().StringVar(&endDate, "to", "", "end date (YYYY-MM-DD)")

	trendsCmd := a.buildCashflowTrendsCommand(&startDate, &endDate, &groupBy, &period, &accountIDs, &categoryIDs)
	trendsCmd.Flags().StringVar(&startDate, "from", "", "start date (YYYY-MM-DD)")
	trendsCmd.Flags().StringVar(&endDate, "to", "", "end date (YYYY-MM-DD)")
	trendsCmd.Flags().StringVar(&groupBy, "group-by", "category", "group dimension: category or category-group")
	must(trendsCmd.RegisterFlagCompletionFunc("group-by", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"category", "category-group"}, cobra.ShellCompDirectiveNoFileComp
	}))
	trendsCmd.Flags().StringVar(&period, "period", "month", "period bucket: month, quarter, or year")
	must(trendsCmd.RegisterFlagCompletionFunc("period", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"month", "quarter", "year"}, cobra.ShellCompDirectiveNoFileComp
	}))
	trendsCmd.Flags().StringSliceVar(&accountIDs, "account-id", nil, "account id filter (repeatable)")
	trendsCmd.Flags().StringSliceVar(&categoryIDs, "category-id", nil, "category id filter (repeatable)")

	cmd.AddCommand(a.buildCashflowListCommand(&startDate, &endDate))
	cmd.AddCommand(a.buildCashflowSummaryCommand(&startDate, &endDate))
	cmd.AddCommand(a.buildCashflowCategoriesCommand(&startDate, &endDate))
	cmd.AddCommand(a.buildCashflowMerchantsCommand(&startDate, &endDate))
	cmd.AddCommand(trendsCmd)
	cmd.AddCommand(a.buildCashflowSpendingCommand(&startDate, &endDate))
	return cmd
}

func (a *App) buildCashflowSummaryCommand(startDate, endDate *string) *cobra.Command {
	return &cobra.Command{
		Use:   "summary",
		Short: "Get cashflow summary",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)

			resolvedStart, resolvedEnd := resolveCashflowDates(*startDate, *endDate, time.Now())
			if err := validateDateRange(resolvedStart, resolvedEnd); err != nil {
				a.handleError(renderer, "cashflow.summary", err, start)
				return
			}

			svc, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "cashflow.summary", wrapError(err, "failed to load service"), start)
				return
			}
			summary, err := svc.GetCashflowSummary(cmd.Context(), resolvedStart, resolvedEnd)
			if err != nil {
				a.handleError(renderer, "cashflow.summary", wrapError(err, "failed to get cashflow summary"), start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("cashflow.summary", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, summary, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Cashflow Summary (%s to %s):\n", resolvedStart, resolvedEnd)
			fmt.Fprintf(cmd.OutOrStdout(), "Income:       %.2f\n", summary.Income)
			fmt.Fprintf(cmd.OutOrStdout(), "Expense:      %.2f\n", summary.Expense)
			fmt.Fprintf(cmd.OutOrStdout(), "Savings:      %.2f\n", summary.Savings)
			fmt.Fprintf(cmd.OutOrStdout(), "Savings Rate: %.2f%%\n", summary.SavingsRate*100)
		},
	}
}

func (a *App) buildCashflowCategoriesCommand(startDate, endDate *string) *cobra.Command {
	return &cobra.Command{
		Use:   "categories",
		Short: "Get cashflow by category",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)

			resolvedStart, resolvedEnd := resolveCashflowDates(*startDate, *endDate, time.Now())
			if err := validateDateRange(resolvedStart, resolvedEnd); err != nil {
				a.handleError(renderer, "cashflow.categories", err, start)
				return
			}

			svc, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "cashflow.categories", wrapError(err, "failed to load service"), start)
				return
			}
			records, err := svc.GetCashflowCategories(cmd.Context(), resolvedStart, resolvedEnd)
			if err != nil {
				a.handleError(renderer, "cashflow.categories", wrapError(err, "failed to get cashflow categories"), start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("cashflow.categories", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, records, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%-30s %10s\n", "CATEGORY", "AMOUNT")
			for _, r := range records {
				fmt.Fprintf(cmd.OutOrStdout(), "%-30s %10.2f\n", r.Name, r.Amount)
			}
		},
	}
}

func (a *App) buildCashflowMerchantsCommand(startDate, endDate *string) *cobra.Command {
	return &cobra.Command{
		Use:   "merchants",
		Short: "Get cashflow by merchant",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)

			resolvedStart, resolvedEnd := resolveCashflowDates(*startDate, *endDate, time.Now())
			if err := validateDateRange(resolvedStart, resolvedEnd); err != nil {
				a.handleError(renderer, "cashflow.merchants", err, start)
				return
			}

			svc, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "cashflow.merchants", wrapError(err, "failed to load service"), start)
				return
			}
			records, err := svc.GetCashflowMerchants(cmd.Context(), resolvedStart, resolvedEnd)
			if err != nil {
				a.handleError(renderer, "cashflow.merchants", wrapError(err, "failed to get cashflow merchants"), start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("cashflow.merchants", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, records, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%-30s %10s\n", "MERCHANT", "AMOUNT")
			for _, r := range records {
				fmt.Fprintf(cmd.OutOrStdout(), "%-30s %10.2f\n", r.Name, r.Amount)
			}
		},
	}
}

func (a *App) buildCashflowTrendsCommand(startDate, endDate, groupBy, period *string, accountIDs, categoryIDs *[]string) *cobra.Command {
	return &cobra.Command{
		Use:   "trends",
		Short: "Get cashflow trends grouped by category or category group",
		Long:  "Get aggregate cashflow rows grouped by category or category group and bucketed by month, quarter, or year.",
		Example: `  monarch cashflow trends --from 2026-01-01 --to 2026-03-31 --group-by category --period month
  monarch cashflow trends --from 2026-01-01 --to 2026-12-31 --group-by category-group --period quarter --account-id acc_123 --json --pretty`,
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)

			if err := validateRequiredDateFlag("from", *startDate); err != nil {
				a.handleError(renderer, "cashflow.trends", err, start)
				return
			}
			if err := validateRequiredDateFlag("to", *endDate); err != nil {
				a.handleError(renderer, "cashflow.trends", err, start)
				return
			}
			if err := validateDateRange(*startDate, *endDate); err != nil {
				a.handleError(renderer, "cashflow.trends", err, start)
				return
			}
			if *groupBy != "category" && *groupBy != "category-group" {
				a.handleError(renderer, "cashflow.trends", errors.New(errors.InvalidArguments, "group-by must be category or category-group", errors.CatValidation, false, nil), start)
				return
			}
			if *period != "month" && *period != "quarter" && *period != "year" {
				a.handleError(renderer, "cashflow.trends", errors.New(errors.InvalidArguments, "period must be month, quarter, or year", errors.CatValidation, false, nil), start)
				return
			}

			svc, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "cashflow.trends", wrapError(err, "failed to load service"), start)
				return
			}

			rows, err := svc.GetCashflowTrends(cmd.Context(), &monarch.CashflowTrendOptions{
				StartDate:   *startDate,
				EndDate:     *endDate,
				GroupBy:     *groupBy,
				Period:      *period,
				AccountIDs:  *accountIDs,
				CategoryIDs: *categoryIDs,
			})
			if err != nil {
				a.handleError(renderer, "cashflow.trends", wrapError(err, "failed to get cashflow trends"), start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("cashflow.trends", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, rows, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%-12s %-30s %12s %12s %12s\n", "PERIOD", "GROUP", "SUM", "INCOME", "EXPENSE")
			for _, row := range rows {
				group := row.GroupName
				if group == "" {
					group = row.GroupID
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-12s %-30s %12.2f %12.2f %12.2f\n", row.Period, group, row.Sum, row.SumIncome, row.SumExpense)
			}
		},
	}
}

func (a *App) buildCashflowListCommand(startDate, endDate *string) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Get cashflow records by period",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)

			resolvedStart, resolvedEnd := resolveCashflowDates(*startDate, *endDate, time.Now())
			if err := validateDateRange(resolvedStart, resolvedEnd); err != nil {
				a.handleError(renderer, "cashflow.list", err, start)
				return
			}

			svc, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "cashflow.list", wrapError(err, "failed to load service"), start)
				return
			}
			records, err := svc.ListCashflow(cmd.Context(), resolvedStart, resolvedEnd)
			if err != nil {
				a.handleError(renderer, "cashflow.list", wrapError(err, "failed to list cashflow"), start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("cashflow.list", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, records, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%-12s %10s %10s %10s\n", "PERIOD", "INCOME", "EXPENSE", "SAVINGS")
			for _, r := range records {
				fmt.Fprintf(cmd.OutOrStdout(), "%-12s %10.2f %10.2f %10.2f\n", r.Period, r.Income, r.Expense, r.Savings)
			}
		},
	}
}

func (a *App) buildCashflowSpendingCommand(startDate, endDate *string) *cobra.Command {
	return &cobra.Command{
		Use:   "spending",
		Short: "Get spending breakdown by category with totals",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)

			resolvedStart, resolvedEnd := resolveCashflowDates(*startDate, *endDate, time.Now())
			if err := validateDateRange(resolvedStart, resolvedEnd); err != nil {
				a.handleError(renderer, "cashflow.spending", err, start)
				return
			}

			svc, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "cashflow.spending", wrapError(err, "failed to load service"), start)
				return
			}
			records, err := svc.GetCashflowCategories(cmd.Context(), resolvedStart, resolvedEnd)
			if err != nil {
				a.handleError(renderer, "cashflow.spending", wrapError(err, "failed to get spending data"), start)
				return
			}

			var totalIncome, totalExpenses float64
			for _, r := range records {
				if r.Amount > 0 {
					totalIncome += r.Amount
				} else {
					totalExpenses += -r.Amount
				}
			}

			if a.Flags.JSONMode {
				data := map[string]any{
					"period":         map[string]string{"start_date": resolvedStart, "end_date": resolvedEnd},
					"total_income":   money.Round2(totalIncome),
					"total_expenses": money.Round2(totalExpenses),
					"net":            money.Round2(totalIncome - totalExpenses),
					"by_category":    records,
				}
				env := output.NewEnvelope("cashflow.spending", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, data, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Spending Summary (%s to %s):\n\n", resolvedStart, resolvedEnd)
			fmt.Fprintf(cmd.OutOrStdout(), "%-30s %10s\n", "CATEGORY", "AMOUNT")
			for _, r := range records {
				fmt.Fprintf(cmd.OutOrStdout(), "%-30s %10.2f\n", r.Name, r.Amount)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n%-30s %10.2f\n", "Total Income:", totalIncome)
			fmt.Fprintf(cmd.OutOrStdout(), "%-30s %10.2f\n", "Total Expenses:", totalExpenses)
			fmt.Fprintf(cmd.OutOrStdout(), "%-30s %10.2f\n", "Net:", totalIncome-totalExpenses)
		},
	}
}
