package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/thedavidweng/monarchmoney-cli/internal/analyze"
	"github.com/thedavidweng/monarchmoney-cli/internal/errors"
	"github.com/thedavidweng/monarchmoney-cli/internal/monarch"
	"github.com/thedavidweng/monarchmoney-cli/internal/output"
)

type appAnalyzeAnomaliesFlags struct {
	month         string
	historyMonths int
	minRatio      float64
	minAmount     float64
}

func (f *appAnalyzeAnomaliesFlags) bind(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.month, "month", "", "month to analyze (YYYY-MM)")
	cmd.Flags().IntVar(&f.historyMonths, "history-months", 6, "number of prior full months to compare")
	cmd.Flags().Float64Var(&f.minRatio, "min-ratio", 1.5, "minimum current/history ratio")
	cmd.Flags().Float64Var(&f.minAmount, "min-amount", 100, "minimum current month expense")
}

type appAnalyzeSubscriptionsFlags struct {
	pastDays   int
	futureDays int
}

func (f *appAnalyzeSubscriptionsFlags) bind(cmd *cobra.Command) {
	cmd.Flags().IntVar(&f.pastDays, "past-days", 370, "days in the past to inspect")
	cmd.Flags().IntVar(&f.futureDays, "future-days", 370, "days in the future to inspect")
}

type appAnalyzeMerchantsFlags struct {
	month   string
	compare string
	limit   int
}

func (f *appAnalyzeMerchantsFlags) bind(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.compare, "compare", "previous-month", "comparison period (previous-month)")
	must(cmd.RegisterFlagCompletionFunc("compare", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"previous-month", "previous-quarter", "previous-year"}, cobra.ShellCompDirectiveNoFileComp
	}))
	cmd.Flags().StringVar(&f.month, "month", "", "month to analyze (YYYY-MM)")
	cmd.Flags().IntVar(&f.limit, "limit", 20, "maximum merchants to return")
}

type appAnalyzeBurnRateFlags struct {
	month string
}

func (f *appAnalyzeBurnRateFlags) bind(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.month, "month", "", "month to analyze (YYYY-MM)")
}

func (a *App) buildAnalyzeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "analyze",
		Short:   "Run deterministic financial analyses",
		GroupID: "analysis",
		Long: `Run deterministic financial analyses for agent workflows.

These commands do programmatic aggregation, ratios, comparisons, and stable JSON
shaping. They do not use AI, make subjective recommendations, or mutate data.`,
		Example: `  monarch analyze anomalies --month 2026-05 --json
  monarch analyze subscriptions --json
  monarch analyze merchants --compare previous-month --limit 10 --json
  monarch analyze burn-rate --month 2026-05 --json`,
	}
	cmd.AddCommand(a.buildAnalyzeAnomaliesCommand())
	cmd.AddCommand(a.buildAnalyzeSubscriptionsCommand())
	cmd.AddCommand(a.buildAnalyzeMerchantsCommand())
	cmd.AddCommand(a.buildAnalyzeBurnRateCommand())
	return cmd
}

func (a *App) buildAnalyzeAnomaliesCommand() *cobra.Command {
	var flags appAnalyzeAnomaliesFlags

	cmd := &cobra.Command{
		Use:   "anomalies",
		Short: "Find category spending anomalies",
		Long: `Find category spending anomalies by comparing one month of expenses
against prior full-month category averages.

This command pages through transactions and performs deterministic aggregation
locally so agents do not need to group transactions themselves.`,
		Example: `  monarch analyze anomalies --json
  monarch analyze anomalies --month 2026-05 --history-months 6 --min-ratio 1.5 --min-amount 100 --json`,
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)
			month := normalizeAnalyzeMonth(flags.month, start)
			if _, err := time.Parse("2006-01", month); err != nil {
				a.handleError(renderer, "analyze.anomalies", errors.New(errors.InvalidArguments, "--month must use YYYY-MM", errors.CatValidation, false, err), start)
				return
			}
			if flags.historyMonths <= 0 {
				a.handleError(renderer, "analyze.anomalies", errors.New(errors.InvalidArguments, "--history-months must be greater than zero", errors.CatValidation, false, nil), start)
				return
			}

			svc, _, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "analyze.anomalies", wrapError(err, "failed to load service"), start)
				return
			}

			currentStart, currentEnd, historyStart, err := analyze.AnomalyWindow(month, flags.historyMonths)
			if err != nil {
				a.handleError(renderer, "analyze.anomalies", errors.New(errors.InvalidArguments, "--month must use YYYY-MM", errors.CatValidation, false, err), start)
				return
			}
			txs, err := svc.ListAllTransactions(cmd.Context(), &monarch.ListTransactionsOptions{Limit: 1000, StartDate: historyStart, EndDate: currentEnd})
			if err != nil {
				a.handleError(renderer, "analyze.anomalies", wrapError(err, "failed to list transactions"), start)
				return
			}
			result, err := analyze.BuildAnomalies(txs, analyze.AnomalyOptions{
				Month:         strings.TrimSuffix(currentStart, "-01"),
				HistoryMonths: flags.historyMonths,
				MinRatio:      flags.minRatio,
				MinAmount:     flags.minAmount,
			})
			if err != nil {
				a.handleError(renderer, "analyze.anomalies", errors.New(errors.InvalidArguments, "failed to analyze anomalies", errors.CatValidation, false, err), start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("analyze.anomalies", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, map[string]any{"period": map[string]string{"start_date": currentStart, "end_date": currentEnd}, "anomalies": result}, time.Since(start))
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-30s %12s %12s %8s %-8s %-20s %12s\n", "CATEGORY", "CURRENT", "AVG", "RATIO", "SEVERITY", "LARGEST MERCHANT", "AMOUNT") //nolint:errcheck // best-effort output
			for _, anomaly := range result {
				fmt.Fprintf(cmd.OutOrStdout(), "%-30s %12.2f %12.2f %8.2f %-8s %-20s %12.2f\n", anomaly.Category, anomaly.CurrentMonth, anomaly.AvgHistory, anomaly.Ratio, anomaly.Severity, anomaly.LargestMerchant, anomaly.LargestAmount) //nolint:errcheck // best-effort output
			}
		},
	}
	flags.bind(cmd)
	return cmd
}

func (a *App) buildAnalyzeSubscriptionsCommand() *cobra.Command {
	var flags appAnalyzeSubscriptionsFlags

	cmd := &cobra.Command{
		Use:   "subscriptions",
		Short: "Summarize recurring subscriptions",
		Long: `Summarize recurring subscriptions from Monarch recurring items.

The output includes monthly and annualized amounts, last and next charges, and
potential overlap facts. Overlaps are facts for agent review, not judgments that
the services are wasteful.`,
		Example: `  monarch analyze subscriptions --json
  monarch analyze subscriptions --past-days 370 --future-days 370 --json`,
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)
			if flags.pastDays < 0 || flags.futureDays < 0 {
				a.handleError(renderer, "analyze.subscriptions", errors.New(errors.InvalidArguments, "day windows must be non-negative", errors.CatValidation, false, nil), start)
				return
			}

			svc, _, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "analyze.subscriptions", wrapError(err, "failed to load service"), start)
				return
			}
			startDate := start.AddDate(0, 0, -flags.pastDays).Format("2006-01-02")
			endDate := start.AddDate(0, 0, flags.futureDays).Format("2006-01-02")
			items, err := svc.ListRecurringItems(cmd.Context(), startDate, endDate)
			if err != nil {
				a.handleError(renderer, "analyze.subscriptions", wrapError(err, "failed to list recurring items"), start)
				return
			}
			result := analyze.BuildSubscriptions(items)
			if a.Flags.JSONMode {
				env := output.NewEnvelope("analyze.subscriptions", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, result, time.Since(start))
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-24s %10s %10s %-12s %-12s %-12s %s\n", "MERCHANT", "MONTHLY", "ANNUAL", "FREQUENCY", "LAST", "NEXT", "CATEGORY") //nolint:errcheck // best-effort output
			for _, sub := range result.Subscriptions {
				fmt.Fprintf(cmd.OutOrStdout(), "%-24s %10.2f %10.2f %-12s %-12s %-12s %s\n", sub.Merchant, sub.Monthly, sub.Annual, sub.Frequency, sub.LastCharge, sub.NextCharge, sub.Category) //nolint:errcheck // best-effort output
			}
		},
	}
	flags.bind(cmd)
	return cmd
}

func (a *App) buildAnalyzeMerchantsCommand() *cobra.Command {
	var flags appAnalyzeMerchantsFlags

	cmd := &cobra.Command{
		Use:   "merchants",
		Short: "Compare merchant spending",
		Long: `Compare merchant expenses between the selected month and a previous period.

The v1 comparison mode is previous-month. The command returns expense_current,
expense_previous, change_pct, and direction with stable semantics for agents.`,
		Example: `  monarch analyze merchants --compare previous-month --json
  monarch analyze merchants --month 2026-05 --compare previous-month --limit 20 --json`,
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)
			if flags.compare != "previous-month" {
				a.handleError(renderer, "analyze.merchants", errors.New(errors.InvalidArguments, "--compare currently supports previous-month", errors.CatValidation, false, nil), start)
				return
			}
			month := normalizeAnalyzeMonth(flags.month, start)
			current, previous, err := analyze.PreviousMonthComparisonWindow(month)
			if err != nil {
				a.handleError(renderer, "analyze.merchants", errors.New(errors.InvalidArguments, "--month must use YYYY-MM", errors.CatValidation, false, err), start)
				return
			}

			svc, _, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "analyze.merchants", wrapError(err, "failed to load service"), start)
				return
			}
			currentRecords, err := svc.GetCashflowMerchants(cmd.Context(), current.StartDate, current.EndDate)
			if err != nil {
				a.handleError(renderer, "analyze.merchants", wrapError(err, "failed to get current merchant spending"), start)
				return
			}
			previousRecords, err := svc.GetCashflowMerchants(cmd.Context(), previous.StartDate, previous.EndDate)
			if err != nil {
				a.handleError(renderer, "analyze.merchants", wrapError(err, "failed to get previous merchant spending"), start)
				return
			}
			result := analyze.BuildMerchantComparison(currentRecords, previousRecords, flags.limit)
			if a.Flags.JSONMode {
				env := output.NewEnvelope("analyze.merchants", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, map[string]any{"period": current, "previous_period": previous, "comparison": result}, time.Since(start))
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-24s %12s %12s %12s %s\n", "MERCHANT", "CURRENT", "PREVIOUS", "CHANGE %", "DIRECTION") //nolint:errcheck // best-effort output
			for _, row := range result {
				change := "null"
				if row.ChangePct != nil {
					change = fmt.Sprintf("%.2f", *row.ChangePct)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-24s %12.2f %12.2f %12s %s\n", row.Merchant, row.ExpenseCurrent, row.ExpensePrevious, change, row.Direction) //nolint:errcheck // best-effort output
			}
		},
	}
	flags.bind(cmd)
	return cmd
}

func (a *App) buildAnalyzeBurnRateCommand() *cobra.Command {
	var flags appAnalyzeBurnRateFlags

	cmd := &cobra.Command{
		Use:   "burn-rate",
		Short: "Compare budget usage with elapsed month time",
		Long: `Compare current budget usage with elapsed month time.

This command uses Monarch budget actual/planned values and deterministic date
math. It does not re-sum transactions or make subjective budget advice.`,
		Example: `  monarch analyze burn-rate --json
  monarch analyze burn-rate --month 2026-05 --json`,
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)
			month := normalizeAnalyzeMonth(flags.month, start)
			monthStart, monthEnd, err := monthRange(month)
			if err != nil {
				a.handleError(renderer, "analyze.burn-rate", errors.New(errors.InvalidArguments, "--month must use YYYY-MM", errors.CatValidation, false, err), start)
				return
			}

			svc, _, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "analyze.burn-rate", wrapError(err, "failed to load service"), start)
				return
			}
			budgets, err := svc.ListBudgets(cmd.Context(), monarch.ListBudgetsOptions{StartDate: monthStart, EndDate: monthEnd})
			if err != nil {
				a.handleError(renderer, "analyze.burn-rate", wrapError(err, "failed to list budgets"), start)
				return
			}
			now := start
			if month != start.Format("2006-01") {
				parsed, _ := time.Parse("2006-01", month)
				now = time.Date(parsed.Year(), parsed.Month(), time.Date(parsed.Year(), parsed.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day(), 12, 0, 0, 0, time.UTC)
			}
			result, err := analyze.BuildBurnRate(budgets, now)
			if err != nil {
				a.handleError(renderer, "analyze.burn-rate", errors.New(errors.InternalError, "failed to analyze burn rate", errors.CatInternal, false, err), start)
				return
			}
			if a.Flags.JSONMode {
				env := output.NewEnvelope("analyze.burn-rate", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, map[string]any{"period": map[string]string{"start_date": monthStart, "end_date": monthEnd}, "budgets": result}, time.Since(start))
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-30s %10s %10s %10s %8s %8s %s\n", "CATEGORY", "BUDGETED", "SPENT", "REMAINING", "BURN %", "TIME %", "STATUS") //nolint:errcheck // best-effort output
			for _, budget := range result {
				fmt.Fprintf(cmd.OutOrStdout(), "%-30s %10.2f %10.2f %10.2f %8.2f %8.2f %s\n", budget.Category, budget.Budgeted, budget.Spent, budget.Remaining, budget.BurnPct, budget.TimePct, budget.Status) //nolint:errcheck // best-effort output
			}
		},
	}
	flags.bind(cmd)
	return cmd
}

func normalizeAnalyzeMonth(value string, now time.Time) string {
	if value != "" {
		return value
	}
	return now.Format("2006-01")
}

func monthRange(month string) (startDate, endDate string, err error) {
	start, err := time.Parse("2006-01", month)
	if err != nil {
		return "", "", err
	}
	end := time.Date(start.Year(), start.Month()+1, 0, 0, 0, 0, 0, time.UTC)
	return start.Format("2006-01-02"), end.Format("2006-01-02"), nil
}
