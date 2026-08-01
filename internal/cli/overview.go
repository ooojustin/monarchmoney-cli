package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/thedavidweng/monarchmoney-cli/internal/errors"
	"github.com/thedavidweng/monarchmoney-cli/internal/monarch"
	"github.com/thedavidweng/monarchmoney-cli/internal/output"
)

var (
	overviewFrom string
	overviewTo   string
)

var overviewCmd = &cobra.Command{
	Use:     "overview",
	Short:   "Get a compact financial overview",
	GroupID: "core",
	Example: "  monarch overview\n  monarch overview --from 2026-01-01 --to 2026-01-31 --json",
	Run: func(cmd *cobra.Command, args []string) {
		run(cmd.Context(), "overview", "failed to get financial overview",
			func(ctx context.Context, svc *monarch.Service) (*monarch.FinancialOverview, error) {
				return svc.GetFinancialOverview(ctx, overviewFrom, overviewTo)
			},
			func(ov *monarch.FinancialOverview) {
				renderFinancialOverview(os.Stdout, ov)
			})
	},
}

func init() {
	overviewCmd.Flags().StringVar(&overviewFrom, "from", "", "start date (YYYY-MM-DD, defaults to current month)")
	overviewCmd.Flags().StringVar(&overviewTo, "to", "", "end date (YYYY-MM-DD, defaults to current month)")
	RootCmd.AddCommand(overviewCmd)
}

func (a *App) buildOverviewCommand() *cobra.Command {
	var from string
	var to string

	cmd := &cobra.Command{
		Use:     "overview",
		Short:   "Get a compact financial overview",
		GroupID: "core",
		Example: "  monarch overview\n  monarch overview --from 2026-01-01 --to 2026-01-31 --json",
		Run: func(cmd *cobra.Command, _ []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "overview", wrapError(err, "failed to load service"), start)
				return
			}

			overview, err := svc.GetFinancialOverview(cmd.Context(), from, to)
			if err != nil {
				a.handleError(renderer, "overview", errors.New(errors.APIError, fmt.Sprintf("failed to get financial overview: %v", err), errors.CatAPI, false, err), start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("overview", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, overview, time.Since(start))
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}

			renderFinancialOverview(cmd.OutOrStdout(), overview)
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "start date (YYYY-MM-DD, defaults to current month)")
	cmd.Flags().StringVar(&to, "to", "", "end date (YYYY-MM-DD, defaults to current month)")
	return cmd
}

func renderFinancialOverview(out io.Writer, overview *monarch.FinancialOverview) {
	fmt.Fprintf(out, "Financial Overview (as of %s)\n\n", overview.AsOf) //nolint:errcheck // best-effort output
	fmt.Fprintf(out, "Net Worth:       %.2f\n", overview.NetWorth)       //nolint:errcheck // best-effort output
	fmt.Fprintf(out, "Accounts:        %d\n", overview.AccountCount)     //nolint:errcheck // best-effort output
	if overview.Cashflow != nil {
		fmt.Fprintf(out, "Income:          %.2f\n", overview.Cashflow.Income)            //nolint:errcheck // best-effort output
		fmt.Fprintf(out, "Expense:         %.2f\n", overview.Cashflow.Expense)           //nolint:errcheck // best-effort output
		fmt.Fprintf(out, "Savings:         %.2f\n", overview.Cashflow.Savings)           //nolint:errcheck // best-effort output
		fmt.Fprintf(out, "Savings Rate:    %.2f%%\n", overview.Cashflow.SavingsRate*100) //nolint:errcheck // best-effort output
	}
	fmt.Fprintf(out, "Transactions:    %d total (showing %d)\n\n", overview.TransactionTotal, len(overview.Transactions)) //nolint:errcheck // best-effort output
	if len(overview.Transactions) == 0 {
		return
	}

	fmt.Fprintf(out, "%-12s %-20s %-15s %10s\n", "DATE", "MERCHANT", "CATEGORY", "AMOUNT") //nolint:errcheck // best-effort output
	for _, tx := range overview.Transactions {
		fmt.Fprintf(out, "%-12s %-20s %-15s %10.2f\n", tx.Date, tx.Merchant, tx.Category, tx.Amount) //nolint:errcheck // best-effort output
	}
}
