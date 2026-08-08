package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/thedavidweng/monarchmoney-cli/internal/errors"
	"github.com/thedavidweng/monarchmoney-cli/internal/monarch"
	"github.com/thedavidweng/monarchmoney-cli/internal/output"
)

func (a *App) buildInvestmentsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "investments",
		Short:   "Inspect Monarch Money investments",
		GroupID: "core",
		Example: "  monarch investments portfolio --json\n  monarch investments performance --json",
	}
	cmd.AddCommand(a.buildInvestmentsPortfolioCommand())
	cmd.AddCommand(a.buildInvestmentsPerformanceCommand())
	return cmd
}

func (a *App) buildInvestmentsPortfolioCommand() *cobra.Command {
	var (
		from       string
		to         string
		accountIDs []string
	)

	cmd := &cobra.Command{
		Use:   "portfolio",
		Short: "Get investment portfolio holdings and performance",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)

			if err := validateDateRange(from, to); err != nil {
				a.handleError(renderer, "investments.portfolio", err, start)
				return
			}

			svc, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "investments.portfolio", wrapError(err, "failed to load service"), start)
				return
			}

			portfolio, err := svc.GetInvestmentPortfolio(cmd.Context(), monarch.InvestmentPortfolioOptions{
				StartDate:  from,
				EndDate:    to,
				AccountIDs: accountIDs,
			})
			if err != nil {
				a.handleError(renderer, "investments.portfolio", wrapError(err, "failed to get investment portfolio"), start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("investments.portfolio", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, portfolio, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Total Value: %.2f\n", portfolio.Performance.TotalValue)
			fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-10s %12s\n", "SECURITY", "TICKER", "VALUE")
			for _, holding := range portfolio.Holdings {
				fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-10s %12.2f\n", holding.Security.Name, holding.Security.Ticker, holding.TotalValue)
			}
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&to, "to", "", "end date (YYYY-MM-DD)")
	cmd.Flags().StringSliceVar(&accountIDs, "account-id", nil, "account id filter (repeatable)")
	return cmd
}

func (a *App) buildInvestmentsPerformanceCommand() *cobra.Command {
	var (
		securityIDs   []string
		from          string
		to            string
		includeValues bool
	)

	cmd := &cobra.Command{
		Use:   "performance",
		Short: "Get historical security performance",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)

			if len(securityIDs) == 0 {
				a.handleError(renderer, "investments.performance", errors.New(errors.InvalidArguments, "--security-id is required", errors.CatValidation, false, nil), start)
				return
			}
			if err := validateRequiredDateFlag("from", from); err != nil {
				a.handleError(renderer, "investments.performance", err, start)
				return
			}
			if err := validateRequiredDateFlag("to", to); err != nil {
				a.handleError(renderer, "investments.performance", err, start)
				return
			}
			if err := validateDateRange(from, to); err != nil {
				a.handleError(renderer, "investments.performance", err, start)
				return
			}

			svc, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "investments.performance", wrapError(err, "failed to load service"), start)
				return
			}

			performance, err := svc.GetSecurityPerformance(cmd.Context(), monarch.SecurityPerformanceOptions{
				SecurityIDs:   securityIDs,
				StartDate:     from,
				EndDate:       to,
				IncludeValues: includeValues,
			})
			if err != nil {
				a.handleError(renderer, "investments.performance", wrapError(err, "failed to get security performance"), start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("investments.performance", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, performance, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-10s %6s\n", "SECURITY", "TICKER", "POINTS")
			for _, item := range performance {
				fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-10s %6d\n", item.Security.Name, item.Security.Ticker, len(item.HistoricalChart))
			}
		},
	}
	cmd.Flags().StringSliceVar(&securityIDs, "security-id", nil, "security id to include (required; repeatable)")
	cmd.Flags().StringVar(&from, "from", "", "start date (required; YYYY-MM-DD)")
	cmd.Flags().StringVar(&to, "to", "", "end date (required; YYYY-MM-DD)")
	cmd.Flags().BoolVar(&includeValues, "values", false, "include chart value fields")
	return cmd
}
