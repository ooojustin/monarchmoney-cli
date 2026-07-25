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
