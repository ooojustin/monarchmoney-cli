package cli

import (
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/thedavidweng/monarchmoney-cli/internal/errors"
	"github.com/thedavidweng/monarchmoney-cli/internal/monarch"
	"github.com/thedavidweng/monarchmoney-cli/internal/output"
)

func (a *App) buildOverviewCommand() *cobra.Command {
	var from string
	var to string

	cmd := &cobra.Command{
		Use:     "overview",
		Short:   "Get a compact financial overview",
		GroupID: "core",
		Example: "  monarch overview\n  monarch overview --from 2026-01-01\n  monarch overview --from 2026-01-01 --to 2026-01-31 --json",
		Run: func(cmd *cobra.Command, _ []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)

			resolvedFrom, resolvedTo := resolveDateRange(from, to, time.Now())
			if err := validateDateRange(resolvedFrom, resolvedTo); err != nil {
				a.handleError(renderer, "overview", err, start)
				return
			}

			svc, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "overview", wrapError(err, "failed to load service"), start)
				return
			}

			overview, err := svc.GetFinancialOverview(cmd.Context(), resolvedFrom, resolvedTo)
			if err != nil {
				a.handleError(renderer, "overview", errors.New(errors.APIError, fmt.Sprintf("failed to get financial overview: %v", err), errors.CatAPI, false, err), start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("overview", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, overview, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}

			renderFinancialOverview(cmd.OutOrStdout(), overview)
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "start date (YYYY-MM-DD, defaults to the first of the current month)")
	cmd.Flags().StringVar(&to, "to", "", "end date (YYYY-MM-DD, defaults to today)")
	return cmd
}

func renderFinancialOverview(out io.Writer, overview *monarch.FinancialOverview) {
	fmt.Fprintf(out, "Financial Overview (as of %s)\n\n", overview.AsOf)
	fmt.Fprintf(out, "Period:          %s to %s\n", overview.StartDate, overview.EndDate)
	fmt.Fprintf(out, "Net Worth:       %.2f\n", overview.NetWorth)
	fmt.Fprintf(out, "Accounts:        %d\n", overview.AccountCount)
	if overview.Cashflow != nil {
		fmt.Fprintf(out, "Income:          %.2f\n", overview.Cashflow.Income)
		fmt.Fprintf(out, "Expense:         %.2f\n", overview.Cashflow.Expense)
		fmt.Fprintf(out, "Savings:         %.2f\n", overview.Cashflow.Savings)
		fmt.Fprintf(out, "Savings Rate:    %.2f%%\n", overview.Cashflow.SavingsRate*100)
	}
	fmt.Fprintf(out, "Transactions:    %d total (showing %d)\n\n", overview.TransactionTotal, len(overview.Transactions))
	if len(overview.Transactions) == 0 {
		return
	}

	fmt.Fprintf(out, "%-12s %-20s %-15s %10s\n", "DATE", "MERCHANT", "CATEGORY", "AMOUNT")
	for i := range overview.Transactions {
		tx := &overview.Transactions[i]
		fmt.Fprintf(out, "%-12s %-20s %-15s %10.2f\n", tx.Date, tx.Merchant, tx.Category, tx.Amount)
	}
}
