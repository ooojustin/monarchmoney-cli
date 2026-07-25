package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/thedavidweng/monarchmoney-cli/internal/errors"
	"github.com/thedavidweng/monarchmoney-cli/internal/monarch"
)

var (
	cfStartDate        string
	cfEndDate          string
	cfTrendGroupBy     string
	cfTrendPeriod      string
	cfTrendAccountIDs  []string
	cfTrendCategoryIDs []string
)

var cashflowCmd = &cobra.Command{
	Use:     "cashflow",
	Short:   "Manage Monarch Money cashflow",
	GroupID: "core",
	Example: "  monarch cashflow summary --from 2026-01-01 --to 2026-03-31\n  monarch cashflow spending --from 2026-01-01 --json",
}

var cashflowSummaryCmd = &cobra.Command{
	Use:   "summary",
	Short: "Get cashflow summary",
	Run: func(cmd *cobra.Command, args []string) {
		run(cmd.Context(), "cashflow.summary", "failed to get cashflow summary",
			func(ctx context.Context, svc *monarch.Service) (*monarch.CashflowSummary, error) {
				setCashflowDates()
				return svc.GetCashflowSummary(ctx, cfStartDate, cfEndDate)
			},
			func(summary *monarch.CashflowSummary) {
				fmt.Printf("Cashflow Summary (%s to %s):\n", cfStartDate, cfEndDate)
				fmt.Printf("Income:       %.2f\n", summary.Income)
				fmt.Printf("Expense:      %.2f\n", summary.Expense)
				fmt.Printf("Savings:      %.2f\n", summary.Savings)
				fmt.Printf("Savings Rate: %.2f%%\n", summary.SavingsRate*100)
			})
	},
}

var cashflowCategoriesCmd = &cobra.Command{
	Use:   "categories",
	Short: "Get cashflow by category",
	Run: func(cmd *cobra.Command, args []string) {
		run(cmd.Context(), "cashflow.categories", "failed to get cashflow categories",
			func(ctx context.Context, svc *monarch.Service) ([]monarch.CashflowRecord, error) {
				setCashflowDates()
				return svc.GetCashflowCategories(ctx, cfStartDate, cfEndDate)
			},
			func(records []monarch.CashflowRecord) {
				fmt.Printf("%-30s %10s\n", "CATEGORY", "AMOUNT")
				for _, r := range records {
					fmt.Printf("%-30s %10.2f\n", r.Name, r.Amount)
				}
			})
	},
}

var cashflowMerchantsCmd = &cobra.Command{
	Use:   "merchants",
	Short: "Get cashflow by merchant",
	Run: func(cmd *cobra.Command, args []string) {
		run(cmd.Context(), "cashflow.merchants", "failed to get cashflow merchants",
			func(ctx context.Context, svc *monarch.Service) ([]monarch.CashflowRecord, error) {
				setCashflowDates()
				return svc.GetCashflowMerchants(ctx, cfStartDate, cfEndDate)
			},
			func(records []monarch.CashflowRecord) {
				fmt.Printf("%-30s %10s\n", "MERCHANT", "AMOUNT")
				for _, r := range records {
					fmt.Printf("%-30s %10.2f\n", r.Name, r.Amount)
				}
			})
	},
}

var cashflowTrendsCmd = &cobra.Command{
	Use:   "trends",
	Short: "Get cashflow trends grouped by category or category group",
	Long:  "Get aggregate cashflow rows grouped by category or category group and bucketed by month, quarter, or year.",
	Example: `  monarch cashflow trends --from 2026-01-01 --to 2026-03-31 --group-by category --period month
  monarch cashflow trends --from 2026-01-01 --to 2026-12-31 --group-by category-group --period quarter --account-id acc_123 --json --pretty`,
	Run: func(cmd *cobra.Command, args []string) {
		run(cmd.Context(), "cashflow.trends", "failed to get cashflow trends",
			func(ctx context.Context, svc *monarch.Service) ([]monarch.CashflowTrendRow, error) {
				if cfStartDate == "" || cfEndDate == "" {
					return nil, errors.New(errors.InvalidArguments, "--from and --to are required", errors.CatValidation, false, nil)
				}
				if _, err := time.Parse("2006-01-02", cfStartDate); err != nil {
					return nil, errors.New(errors.InvalidArguments, "from date must use YYYY-MM-DD", errors.CatValidation, false, err)
				}
				if _, err := time.Parse("2006-01-02", cfEndDate); err != nil {
					return nil, errors.New(errors.InvalidArguments, "to date must use YYYY-MM-DD", errors.CatValidation, false, err)
				}
				if cfTrendGroupBy != "category" && cfTrendGroupBy != "category-group" {
					return nil, errors.New(errors.InvalidArguments, "group-by must be category or category-group", errors.CatValidation, false, nil)
				}
				if cfTrendPeriod != "month" && cfTrendPeriod != "quarter" && cfTrendPeriod != "year" {
					return nil, errors.New(errors.InvalidArguments, "period must be month, quarter, or year", errors.CatValidation, false, nil)
				}
				return svc.GetCashflowTrends(ctx, &monarch.CashflowTrendOptions{
					StartDate:   cfStartDate,
					EndDate:     cfEndDate,
					GroupBy:     cfTrendGroupBy,
					Period:      cfTrendPeriod,
					AccountIDs:  cfTrendAccountIDs,
					CategoryIDs: cfTrendCategoryIDs,
				})
			},
			func(rows []monarch.CashflowTrendRow) {
				fmt.Printf("%-12s %-30s %12s %12s %12s\n", "PERIOD", "GROUP", "SUM", "INCOME", "EXPENSE")
				for _, row := range rows {
					group := row.GroupName
					if group == "" {
						group = row.GroupID
					}
					fmt.Printf("%-12s %-30s %12.2f %12.2f %12.2f\n", row.Period, group, row.Sum, row.SumIncome, row.SumExpense)
				}
			})
	},
}

func setCashflowDates() {
	if cfStartDate == "" {
		now := time.Now()
		cfStartDate = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
	}
	if cfEndDate == "" {
		now := time.Now()
		cfEndDate = now.Format("2006-01-02")
	}
}

var cashflowListCmd = &cobra.Command{
	Use:   "list",
	Short: "Get cashflow records by period",
	Run: func(cmd *cobra.Command, args []string) {
		run(cmd.Context(), "cashflow.list", "failed to list cashflow",
			func(ctx context.Context, svc *monarch.Service) ([]monarch.CashflowPeriod, error) {
				setCashflowDates()
				return svc.ListCashflow(ctx, cfStartDate, cfEndDate)
			},
			func(records []monarch.CashflowPeriod) {
				fmt.Printf("%-12s %10s %10s %10s\n", "PERIOD", "INCOME", "EXPENSE", "SAVINGS")
				for _, r := range records {
					fmt.Printf("%-12s %10.2f %10.2f %10.2f\n", r.Period, r.Income, r.Expense, r.Savings)
				}
			})
	},
}

func init() {
	cashflowCmd.PersistentFlags().StringVar(&cfStartDate, "from", "", "start date (YYYY-MM-DD)")
	cashflowCmd.PersistentFlags().StringVar(&cfEndDate, "to", "", "end date (YYYY-MM-DD)")
	cashflowTrendsCmd.Flags().StringVar(&cfStartDate, "from", "", "start date (YYYY-MM-DD)")
	cashflowTrendsCmd.Flags().StringVar(&cfEndDate, "to", "", "end date (YYYY-MM-DD)")
	cashflowTrendsCmd.Flags().StringVar(&cfTrendGroupBy, "group-by", "category", "group dimension: category or category-group")
	must(cashflowTrendsCmd.RegisterFlagCompletionFunc("group-by", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"category", "category-group"}, cobra.ShellCompDirectiveNoFileComp
	}))
	cashflowTrendsCmd.Flags().StringVar(&cfTrendPeriod, "period", "month", "period bucket: month, quarter, or year")
	must(cashflowTrendsCmd.RegisterFlagCompletionFunc("period", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"month", "quarter", "year"}, cobra.ShellCompDirectiveNoFileComp
	}))
	cashflowTrendsCmd.Flags().StringSliceVar(&cfTrendAccountIDs, "account-id", nil, "account id filter (repeatable)")
	cashflowTrendsCmd.Flags().StringSliceVar(&cfTrendCategoryIDs, "category-id", nil, "category id filter (repeatable)")

	cashflowCmd.AddCommand(cashflowListCmd)
	cashflowCmd.AddCommand(cashflowSummaryCmd)
	cashflowCmd.AddCommand(cashflowCategoriesCmd)
	cashflowCmd.AddCommand(cashflowMerchantsCmd)
	cashflowCmd.AddCommand(cashflowTrendsCmd)
	cashflowCmd.AddCommand(cashflowSpendingCmd)
	RootCmd.AddCommand(cashflowCmd)
}

var cashflowSpendingCmd = &cobra.Command{
	Use:   "spending",
	Short: "Get spending breakdown by category with totals",
	Run: func(cmd *cobra.Command, args []string) {
		var records []monarch.CashflowRecord
		var totalIncome, totalExpenses float64
		run(cmd.Context(), "cashflow.spending", "failed to get spending data",
			func(ctx context.Context, svc *monarch.Service) (map[string]any, error) {
				setCashflowDates()
				recs, err := svc.GetCashflowCategories(ctx, cfStartDate, cfEndDate)
				if err != nil {
					return nil, err
				}
				records = recs
				for _, r := range recs {
					if r.Amount > 0 {
						totalIncome += r.Amount
					} else {
						totalExpenses += -r.Amount
					}
				}
				return map[string]any{
					"period":         map[string]string{"start_date": cfStartDate, "end_date": cfEndDate},
					"total_income":   totalIncome,
					"total_expenses": totalExpenses,
					"net":            totalIncome - totalExpenses,
					"by_category":    recs,
				}, nil
			},
			func(_ map[string]any) {
				fmt.Printf("Spending Summary (%s to %s):\n\n", cfStartDate, cfEndDate)
				fmt.Printf("%-30s %10s\n", "CATEGORY", "AMOUNT")
				for _, r := range records {
					fmt.Printf("%-30s %10.2f\n", r.Name, r.Amount)
				}
				fmt.Printf("\n%-30s %10.2f\n", "Total Income:", totalIncome)
				fmt.Printf("%-30s %10.2f\n", "Total Expenses:", totalExpenses)
				fmt.Printf("%-30s %10.2f\n", "Net:", totalIncome-totalExpenses)
			})
	},
}
