package cli

import (
	"time"

	"github.com/spf13/cobra"
	"github.com/thedavidweng/monarchmoney-cli/internal/errors"
	"github.com/thedavidweng/monarchmoney-cli/internal/monarch"
	"github.com/thedavidweng/monarchmoney-cli/internal/output"
)

func (a *App) buildInvestmentsCommands(parent *cobra.Command) {
	investmentsCmd := &cobra.Command{
		Use:   "investments",
		Short: "Inspect Monarch Money investments",
	}
	investmentsCmd.AddCommand(a.buildInvestmentsPortfolio())
	investmentsCmd.AddCommand(a.buildInvestmentsPerformance())
	parent.AddCommand(investmentsCmd)
}

func (a *App) buildInvestmentsPortfolio() *cobra.Command {
	var (
		investmentFrom       string
		investmentTo         string
		investmentAccountIDs []string
	)
	cmd := &cobra.Command{
		Use:   "portfolio",
		Short: "Get investment portfolio holdings and performance",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, a.Flags.JSONMode, a.Flags.Pretty)

			if err := validateOptionalDate("from", investmentFrom); err != nil {
				a.handleError(renderer, "investments.portfolio", err, start)
				return
			}
			if err := validateOptionalDate("to", investmentTo); err != nil {
				a.handleError(renderer, "investments.portfolio", err, start)
				return
			}

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "investments.portfolio", err.(*errors.Error), start)
				return
			}

			portfolio, err := svc.GetInvestmentPortfolio(cmd.Context(), monarch.InvestmentPortfolioOptions{
				StartDate:  investmentFrom,
				EndDate:    investmentTo,
				AccountIDs: investmentAccountIDs,
			})
			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to get investment portfolio", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "investments.portfolio", cliErr, start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("investments.portfolio", a.Flags.Profile, output.SchemaVersion, "", portfolio, time.Since(start))
				a.renderSuccess(renderer, env, start)
			} else {
				writeText(a.Deps.Stdout, "Total Value: %.2f\n", portfolio.Performance.TotalValue)
				writeText(a.Deps.Stdout, "%-20s %-10s %12s\n", "SECURITY", "TICKER", "VALUE")
				for _, holding := range portfolio.Holdings {
					writeText(a.Deps.Stdout, "%-20s %-10s %12.2f\n", holding.Security.Name, holding.Security.Ticker, holding.TotalValue)
				}
			}
		},
	}
	cmd.Flags().StringVar(&investmentFrom, "from", "", "start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&investmentTo, "to", "", "end date (YYYY-MM-DD)")
	cmd.Flags().StringSliceVar(&investmentAccountIDs, "account-id", nil, "account id filter (repeatable)")
	return cmd
}

func (a *App) buildInvestmentsPerformance() *cobra.Command {
	var (
		investmentFrom          string
		investmentTo            string
		investmentSecurityIDs   []string
		investmentIncludeValues bool
	)
	cmd := &cobra.Command{
		Use:   "performance",
		Short: "Get historical security performance",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, a.Flags.JSONMode, a.Flags.Pretty)

			if len(investmentSecurityIDs) == 0 {
				a.handleError(renderer, "investments.performance", errors.New(errors.InvalidArguments, "--security-id is required", errors.CatValidation, false, nil), start)
				return
			}
			if err := validateRequiredDate("from", investmentFrom); err != nil {
				a.handleError(renderer, "investments.performance", err, start)
				return
			}
			if err := validateRequiredDate("to", investmentTo); err != nil {
				a.handleError(renderer, "investments.performance", err, start)
				return
			}

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "investments.performance", err.(*errors.Error), start)
				return
			}

			performance, err := svc.GetSecurityPerformance(cmd.Context(), monarch.SecurityPerformanceOptions{
				SecurityIDs:   investmentSecurityIDs,
				StartDate:     investmentFrom,
				EndDate:       investmentTo,
				IncludeValues: investmentIncludeValues,
			})
			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to get security performance", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "investments.performance", cliErr, start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("investments.performance", a.Flags.Profile, output.SchemaVersion, "", performance, time.Since(start))
				a.renderSuccess(renderer, env, start)
			} else {
				writeText(a.Deps.Stdout, "%-20s %-10s %6s\n", "SECURITY", "TICKER", "POINTS")
				for _, item := range performance {
					writeText(a.Deps.Stdout, "%-20s %-10s %6d\n", item.Security.Name, item.Security.Ticker, len(item.HistoricalChart))
				}
			}
		},
	}
	cmd.Flags().StringSliceVar(&investmentSecurityIDs, "security-id", nil, "security id to include (repeatable)")
	cmd.Flags().StringVar(&investmentFrom, "from", "", "start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&investmentTo, "to", "", "end date (YYYY-MM-DD)")
	cmd.Flags().BoolVar(&investmentIncludeValues, "values", false, "include chart value fields")
	return cmd
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
