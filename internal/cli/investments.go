package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/thedavidweng/monarchmoney-cli/internal/errors"
	"github.com/thedavidweng/monarchmoney-cli/internal/monarch"
	"github.com/thedavidweng/monarchmoney-cli/internal/output"
)

var (
	investmentFrom          string
	investmentTo            string
	investmentAccountIDs    []string
	investmentSecurityIDs   []string
	investmentIncludeValues bool
)

var investmentsCmd = &cobra.Command{
	Use:     "investments",
	Short:   "Inspect Monarch Money investments",
	GroupID: "core",
	Example: "  monarch investments portfolio --json\n  monarch investments performance --json",
}

var investmentsPortfolioCmd = &cobra.Command{
	Use:   "portfolio",
	Short: "Get investment portfolio holdings and performance",
	Run: func(cmd *cobra.Command, args []string) {
		run(cmd.Context(), "investments.portfolio", "failed to get investment portfolio",
			func(ctx context.Context, svc *monarch.Service) (*monarch.InvestmentPortfolio, error) {
				if err := validateOptionalDate("from", investmentFrom); err != nil {
					return nil, err
				}
				if err := validateOptionalDate("to", investmentTo); err != nil {
					return nil, err
				}
				return svc.GetInvestmentPortfolio(ctx, monarch.InvestmentPortfolioOptions{
					StartDate:  investmentFrom,
					EndDate:    investmentTo,
					AccountIDs: investmentAccountIDs,
				})
			},
			func(portfolio *monarch.InvestmentPortfolio) {
				fmt.Printf("Total Value: %.2f\n", portfolio.Performance.TotalValue)
				fmt.Printf("%-20s %-10s %12s\n", "SECURITY", "TICKER", "VALUE")
				for _, holding := range portfolio.Holdings {
					fmt.Printf("%-20s %-10s %12.2f\n", holding.Security.Name, holding.Security.Ticker, holding.TotalValue)
				}
			})
	},
}

var investmentsPerformanceCmd = &cobra.Command{
	Use:   "performance",
	Short: "Get historical security performance",
	Run: func(cmd *cobra.Command, args []string) {
		run(cmd.Context(), "investments.performance", "failed to get security performance",
			func(ctx context.Context, svc *monarch.Service) ([]monarch.SecurityPerformance, error) {
				if len(investmentSecurityIDs) == 0 {
					return nil, errors.New(errors.InvalidArguments, "--security-id is required", errors.CatValidation, false, nil)
				}
				if err := validateRequiredDate("from", investmentFrom); err != nil {
					return nil, err
				}
				if err := validateRequiredDate("to", investmentTo); err != nil {
					return nil, err
				}
				return svc.GetSecurityPerformance(ctx, monarch.SecurityPerformanceOptions{
					SecurityIDs:   investmentSecurityIDs,
					StartDate:     investmentFrom,
					EndDate:       investmentTo,
					IncludeValues: investmentIncludeValues,
				})
			},
			func(performance []monarch.SecurityPerformance) {
				fmt.Printf("%-20s %-10s %6s\n", "SECURITY", "TICKER", "POINTS")
				for _, item := range performance {
					fmt.Printf("%-20s %-10s %6d\n", item.Security.Name, item.Security.Ticker, len(item.HistoricalChart))
				}
			})
	},
}

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

			if err := validateOptionalDate("from", from); err != nil {
				a.handleError(renderer, "investments.portfolio", err, start)
				return
			}
			if err := validateOptionalDate("to", to); err != nil {
				a.handleError(renderer, "investments.portfolio", err, start)
				return
			}

			svc, _, err := a.loadService()
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
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Total Value: %.2f\n", portfolio.Performance.TotalValue) //nolint:errcheck // best-effort output
			fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-10s %12s\n", "SECURITY", "TICKER", "VALUE")     //nolint:errcheck // best-effort output
			for _, holding := range portfolio.Holdings {
				fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-10s %12.2f\n", holding.Security.Name, holding.Security.Ticker, holding.TotalValue) //nolint:errcheck // best-effort output
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
			if err := validateRequiredDate("from", from); err != nil {
				a.handleError(renderer, "investments.performance", err, start)
				return
			}
			if err := validateRequiredDate("to", to); err != nil {
				a.handleError(renderer, "investments.performance", err, start)
				return
			}

			svc, _, err := a.loadService()
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
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-10s %6s\n", "SECURITY", "TICKER", "POINTS") //nolint:errcheck // best-effort output
			for _, item := range performance {
				fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-10s %6d\n", item.Security.Name, item.Security.Ticker, len(item.HistoricalChart)) //nolint:errcheck // best-effort output
			}
		},
	}
	cmd.Flags().StringSliceVar(&securityIDs, "security-id", nil, "security id to include (repeatable)")
	cmd.Flags().StringVar(&from, "from", "", "start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&to, "to", "", "end date (YYYY-MM-DD)")
	cmd.Flags().BoolVar(&includeValues, "values", false, "include chart value fields")
	return cmd
}

func init() {
	investmentsPortfolioCmd.Flags().StringVar(&investmentFrom, "from", "", "start date (YYYY-MM-DD)")
	investmentsPortfolioCmd.Flags().StringVar(&investmentTo, "to", "", "end date (YYYY-MM-DD)")
	investmentsPortfolioCmd.Flags().StringSliceVar(&investmentAccountIDs, "account-id", nil, "account id filter (repeatable)")

	investmentsPerformanceCmd.Flags().StringSliceVar(&investmentSecurityIDs, "security-id", nil, "security id to include (repeatable)")
	investmentsPerformanceCmd.Flags().StringVar(&investmentFrom, "from", "", "start date (YYYY-MM-DD)")
	investmentsPerformanceCmd.Flags().StringVar(&investmentTo, "to", "", "end date (YYYY-MM-DD)")
	investmentsPerformanceCmd.Flags().BoolVar(&investmentIncludeValues, "values", false, "include chart value fields")

	investmentsCmd.AddCommand(investmentsPortfolioCmd)
	investmentsCmd.AddCommand(investmentsPerformanceCmd)
	RootCmd.AddCommand(investmentsCmd)
}

func validateOptionalDate(name, value string) *errors.Error {
	if value == "" {
		return nil
	}
	if _, err := time.Parse("2006-01-02", value); err != nil {
		return errors.New(errors.InvalidArguments, name+" date must use YYYY-MM-DD", errors.CatValidation, false, err)
	}
	return nil
}

func validateRequiredDate(name, value string) *errors.Error {
	if value == "" {
		return errors.New(errors.InvalidArguments, "--"+name+" is required", errors.CatValidation, false, nil)
	}
	return validateOptionalDate(name, value)
}
