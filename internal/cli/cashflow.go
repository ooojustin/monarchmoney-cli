package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/thedavidweng/monarchmoney-cli/internal/errors"
	"github.com/thedavidweng/monarchmoney-cli/internal/monarch"
	"github.com/thedavidweng/monarchmoney-cli/internal/output"
)

func (a *App) buildCashflowCommands(parent *cobra.Command) {
	var (
		cfStartDate string
		cfEndDate   string
	)

	resolveDates := func() (string, string) {
		s, e := cfStartDate, cfEndDate
		if s == "" {
			now := time.Now()
			s = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
		}
		if e == "" {
			e = time.Now().Format("2006-01-02")
		}
		return s, e
	}
	rawDates := func() (string, string) {
		return cfStartDate, cfEndDate
	}

	cashflowCmd := &cobra.Command{
		Use:   "cashflow",
		Short: "Manage Monarch Money cashflow",
	}
	cashflowCmd.PersistentFlags().StringVar(&cfStartDate, "from", "", "start date (YYYY-MM-DD)")
	cashflowCmd.PersistentFlags().StringVar(&cfEndDate, "to", "", "end date (YYYY-MM-DD)")

	cashflowCmd.AddCommand(a.buildCashflowList(resolveDates))
	cashflowCmd.AddCommand(a.buildCashflowSummary(resolveDates))
	cashflowCmd.AddCommand(a.buildCashflowCategories(resolveDates))
	cashflowCmd.AddCommand(a.buildCashflowMerchants(resolveDates))
	cashflowCmd.AddCommand(a.buildCashflowTrends(rawDates))
	cashflowCmd.AddCommand(a.buildCashflowSpending(resolveDates))

	parent.AddCommand(cashflowCmd)
}

func (a *App) buildCashflowList(resolveDates func() (string, string)) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Get cashflow records by period",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, a.Flags.JSONMode, a.Flags.Pretty)

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "cashflow.list", err.(*errors.Error), start)
				return
			}

			cfStart, cfEnd := resolveDates()
			records, err := svc.ListCashflow(cmd.Context(), cfStart, cfEnd)
			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to list cashflow", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "cashflow.list", cliErr, start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("cashflow.list", a.Flags.Profile, output.SchemaVersion, "", records, time.Since(start))
				a.renderSuccess(renderer, env, start)
			} else {
				fmt.Printf("%-12s %10s %10s %10s\n", "PERIOD", "INCOME", "EXPENSE", "SAVINGS")
				for _, r := range records {
					fmt.Printf("%-12s %10.2f %10.2f %10.2f\n", r.Period, r.Income, r.Expense, r.Savings)
				}
			}
		},
	}
}

func (a *App) buildCashflowSummary(resolveDates func() (string, string)) *cobra.Command {
	return &cobra.Command{
		Use:   "summary",
		Short: "Get cashflow summary",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, a.Flags.JSONMode, a.Flags.Pretty)

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "cashflow.summary", err.(*errors.Error), start)
				return
			}

			cfStart, cfEnd := resolveDates()
			summary, err := svc.GetCashflowSummary(cmd.Context(), cfStart, cfEnd)
			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to get cashflow summary", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "cashflow.summary", cliErr, start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("cashflow.summary", a.Flags.Profile, output.SchemaVersion, "", summary, time.Since(start))
				a.renderSuccess(renderer, env, start)
			} else {
				fmt.Printf("Cashflow Summary (%s to %s):\n", cfStart, cfEnd)
				fmt.Printf("Income:       %.2f\n", summary.Income)
				fmt.Printf("Expense:      %.2f\n", summary.Expense)
				fmt.Printf("Savings:      %.2f\n", summary.Savings)
				fmt.Printf("Savings Rate: %.2f%%\n", summary.SavingsRate*100)
			}
		},
	}
}

func (a *App) buildCashflowCategories(resolveDates func() (string, string)) *cobra.Command {
	return &cobra.Command{
		Use:   "categories",
		Short: "Get cashflow by category",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, a.Flags.JSONMode, a.Flags.Pretty)

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "cashflow.categories", err.(*errors.Error), start)
				return
			}

			cfStart, cfEnd := resolveDates()
			records, err := svc.GetCashflowCategories(cmd.Context(), cfStart, cfEnd)
			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to get cashflow categories", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "cashflow.categories", cliErr, start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("cashflow.categories", a.Flags.Profile, output.SchemaVersion, "", records, time.Since(start))
				a.renderSuccess(renderer, env, start)
			} else {
				fmt.Printf("%-30s %10s\n", "CATEGORY", "AMOUNT")
				for _, r := range records {
					fmt.Printf("%-30s %10.2f\n", r.Name, r.Amount)
				}
			}
		},
	}
}

func (a *App) buildCashflowMerchants(resolveDates func() (string, string)) *cobra.Command {
	return &cobra.Command{
		Use:   "merchants",
		Short: "Get cashflow by merchant",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, a.Flags.JSONMode, a.Flags.Pretty)

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "cashflow.merchants", err.(*errors.Error), start)
				return
			}

			cfStart, cfEnd := resolveDates()
			records, err := svc.GetCashflowMerchants(cmd.Context(), cfStart, cfEnd)
			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to get cashflow merchants", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "cashflow.merchants", cliErr, start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("cashflow.merchants", a.Flags.Profile, output.SchemaVersion, "", records, time.Since(start))
				a.renderSuccess(renderer, env, start)
			} else {
				fmt.Printf("%-30s %10s\n", "MERCHANT", "AMOUNT")
				for _, r := range records {
					fmt.Printf("%-30s %10.2f\n", r.Name, r.Amount)
				}
			}
		},
	}
}

func (a *App) buildCashflowTrends(rawDates func() (string, string)) *cobra.Command {
	var (
		cfTrendGroupBy     string
		cfTrendPeriod      string
		cfTrendAccountIDs  []string
		cfTrendCategoryIDs []string
	)
	cmd := &cobra.Command{
		Use:   "trends",
		Short: "Get cashflow trends grouped by category or category group",
		Long:  "Get aggregate cashflow rows grouped by category or category group and bucketed by month, quarter, or year.",
		Example: `  monarch cashflow trends --from 2026-01-01 --to 2026-03-31 --group-by category --period month
  monarch cashflow trends --from 2026-01-01 --to 2026-12-31 --group-by category-group --period quarter --account-id acc_123 --json --pretty`,
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, a.Flags.JSONMode, a.Flags.Pretty)
			cfStartDate, cfEndDate := rawDates()

			if cfStartDate == "" || cfEndDate == "" {
				a.handleError(renderer, "cashflow.trends", errors.New(errors.InvalidArguments, "--from and --to are required", errors.CatValidation, false, nil), start)
				return
			}
			if _, err := time.Parse("2006-01-02", cfStartDate); err != nil {
				a.handleError(renderer, "cashflow.trends", errors.New(errors.InvalidArguments, "from date must use YYYY-MM-DD", errors.CatValidation, false, err), start)
				return
			}
			if _, err := time.Parse("2006-01-02", cfEndDate); err != nil {
				a.handleError(renderer, "cashflow.trends", errors.New(errors.InvalidArguments, "to date must use YYYY-MM-DD", errors.CatValidation, false, err), start)
				return
			}
			if cfTrendGroupBy != "category" && cfTrendGroupBy != "category-group" {
				a.handleError(renderer, "cashflow.trends", errors.New(errors.InvalidArguments, "group-by must be category or category-group", errors.CatValidation, false, nil), start)
				return
			}
			if cfTrendPeriod != "month" && cfTrendPeriod != "quarter" && cfTrendPeriod != "year" {
				a.handleError(renderer, "cashflow.trends", errors.New(errors.InvalidArguments, "period must be month, quarter, or year", errors.CatValidation, false, nil), start)
				return
			}

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "cashflow.trends", err.(*errors.Error), start)
				return
			}

			rows, err := svc.GetCashflowTrends(cmd.Context(), monarch.CashflowTrendOptions{
				StartDate:   cfStartDate,
				EndDate:     cfEndDate,
				GroupBy:     cfTrendGroupBy,
				Period:      cfTrendPeriod,
				AccountIDs:  cfTrendAccountIDs,
				CategoryIDs: cfTrendCategoryIDs,
			})
			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to get cashflow trends", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "cashflow.trends", cliErr, start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("cashflow.trends", a.Flags.Profile, output.SchemaVersion, "", rows, time.Since(start))
				a.renderSuccess(renderer, env, start)
			} else {
				fmt.Printf("%-12s %-30s %12s %12s %12s\n", "PERIOD", "GROUP", "SUM", "INCOME", "EXPENSE")
				for _, row := range rows {
					group := row.GroupName
					if group == "" {
						group = row.GroupID
					}
					fmt.Printf("%-12s %-30s %12.2f %12.2f %12.2f\n", row.Period, group, row.Sum, row.SumIncome, row.SumExpense)
				}
			}
		},
	}
	cmd.Flags().StringVar(&cfTrendGroupBy, "group-by", "category", "group dimension: category or category-group")
	cmd.Flags().StringVar(&cfTrendPeriod, "period", "month", "period bucket: month, quarter, or year")
	cmd.Flags().StringSliceVar(&cfTrendAccountIDs, "account-id", nil, "account id filter (repeatable)")
	cmd.Flags().StringSliceVar(&cfTrendCategoryIDs, "category-id", nil, "category id filter (repeatable)")
	return cmd
}

func (a *App) buildCashflowSpending(resolveDates func() (string, string)) *cobra.Command {
	return &cobra.Command{
		Use:   "spending",
		Short: "Get spending breakdown by category with totals",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, a.Flags.JSONMode, a.Flags.Pretty)

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "cashflow.spending", err.(*errors.Error), start)
				return
			}

			cfStart, cfEnd := resolveDates()
			records, err := svc.GetCashflowCategories(cmd.Context(), cfStart, cfEnd)
			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to get spending data", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "cashflow.spending", cliErr, start)
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
				data := map[string]interface{}{
					"period":         map[string]string{"start_date": cfStart, "end_date": cfEnd},
					"total_income":   totalIncome,
					"total_expenses": totalExpenses,
					"net":            totalIncome - totalExpenses,
					"by_category":    records,
				}
				env := output.NewEnvelope("cashflow.spending", a.Flags.Profile, output.SchemaVersion, "", data, time.Since(start))
				a.renderSuccess(renderer, env, start)
			} else {
				fmt.Printf("Spending Summary (%s to %s):\n\n", cfStart, cfEnd)
				fmt.Printf("%-30s %10s\n", "CATEGORY", "AMOUNT")
				for _, r := range records {
					fmt.Printf("%-30s %10.2f\n", r.Name, r.Amount)
				}
				fmt.Printf("\n%-30s %10.2f\n", "Total Income:", totalIncome)
				fmt.Printf("%-30s %10.2f\n", "Total Expenses:", totalExpenses)
				fmt.Printf("%-30s %10.2f\n", "Net:", totalIncome-totalExpenses)
			}
		},
	}
}
