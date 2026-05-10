package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/thedavidweng/monarchmoney-cli/internal/audit"
	"github.com/thedavidweng/monarchmoney-cli/internal/errors"
	"github.com/thedavidweng/monarchmoney-cli/internal/monarch"
	"github.com/thedavidweng/monarchmoney-cli/internal/output"
	"github.com/thedavidweng/monarchmoney-cli/internal/safety"
)

// parseMonthString parses YYYY-MM into year/month integers. Returns an error
// if the format is invalid.
func parseMonthString(monthStr string) (int, int, error) {
	parts := strings.Split(monthStr, "-")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid month format, use YYYY-MM")
	}
	y, err1 := strconv.Atoi(parts[0])
	m, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return 0, 0, fmt.Errorf("invalid month format, use YYYY-MM")
	}
	return y, m, nil
}

// monthDates returns startDate (first of month) and endDate (last of month) as
// YYYY-MM-DD strings.
func monthDates(y, m int) (string, string) {
	startDate := fmt.Sprintf("%04d-%02d-01", y, m)
	lastDay := time.Date(y, time.Month(m+1), 0, 0, 0, 0, 0, time.UTC).Day()
	endDate := fmt.Sprintf("%04d-%02d-%02d", y, m, lastDay)
	return startDate, endDate
}

// resolveMonthOrCurrent returns y/m from monthStr or falls back to current month.
func resolveMonthOrCurrent(monthStr string) (int, int, error) {
	if monthStr == "" {
		now := time.Now()
		return now.Year(), int(now.Month()), nil
	}
	return parseMonthString(monthStr)
}

func (a *App) buildBudgetsCommands(parent *cobra.Command) {
	budgetsCmd := &cobra.Command{
		Use:   "budgets",
		Short: "Manage Monarch Money budgets",
	}
	budgetsCmd.AddCommand(a.buildBudgetsList())
	budgetsCmd.AddCommand(a.buildBudgetsShow())
	budgetsCmd.AddCommand(a.buildBudgetsSet())
	budgetsCmd.AddCommand(a.buildBudgetsReset())
	budgetsCmd.AddCommand(a.buildBudgetsExport())

	flexibleCmd := &cobra.Command{Use: "flexible", Short: "Manage flexible budget settings"}
	flexibleCmd.AddCommand(a.buildBudgetsFlexibleSet())
	budgetsCmd.AddCommand(flexibleCmd)

	flexRolloverCmd := &cobra.Command{Use: "flex-rollover", Short: "Manage flexible budget rollover settings"}
	flexRolloverCmd.AddCommand(a.buildBudgetsFlexRolloverSet())
	budgetsCmd.AddCommand(flexRolloverCmd)

	parent.AddCommand(budgetsCmd)
}

func (a *App) buildBudgetsList() *cobra.Command {
	var monthStr string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List budgets",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, jsonMode, pretty)

			y, m, err := resolveMonthOrCurrent(monthStr)
			if err != nil {
				a.handleError(renderer, "budgets.list", errors.New(errors.InvalidArguments, err.Error(), errors.CatValidation, false, nil), start)
				return
			}

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "budgets.list", err.(*errors.Error), start)
				return
			}

			startDate, endDate := monthDates(y, m)
			budgets, err := svc.ListBudgets(cmd.Context(), monarch.ListBudgetsOptions{StartDate: startDate, EndDate: endDate})
			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to list budgets", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "budgets.list", cliErr, start)
				return
			}

			if jsonMode {
				env := output.NewEnvelope("budgets.list", profile, output.SchemaVersion, "", budgets, time.Since(start))
				renderer.RenderSuccess(env)
			} else {
				fmt.Printf("%-30s %10s %10s %10s\n", "CATEGORY", "PLANNED", "ACTUAL", "REMAINING")
				for _, b := range budgets {
					fmt.Printf("%-30s %10.2f %10.2f %10.2f\n", b.CategoryName, b.Planned, b.Actual, b.Planned-b.Actual)
				}
			}
		},
	}
	cmd.Flags().StringVar(&monthStr, "month", "", "month in YYYY-MM format")
	return cmd
}

func (a *App) buildBudgetsShow() *cobra.Command {
	var monthStr string
	cmd := &cobra.Command{
		Use:   "show <category-id>",
		Short: "Show budget for a specific category",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, jsonMode, pretty)
			categoryID := args[0]

			y, m, err := resolveMonthOrCurrent(monthStr)
			if err != nil {
				a.handleError(renderer, "budgets.show", errors.New(errors.InvalidArguments, err.Error(), errors.CatValidation, false, nil), start)
				return
			}

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "budgets.show", err.(*errors.Error), start)
				return
			}

			startDate, endDate := monthDates(y, m)
			budget, err := svc.GetBudget(cmd.Context(), categoryID, startDate, endDate)
			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to get budget", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "budgets.show", cliErr, start)
				return
			}

			if jsonMode {
				env := output.NewEnvelope("budgets.show", profile, output.SchemaVersion, "", budget, time.Since(start))
				renderer.RenderSuccess(env)
			} else {
				fmt.Printf("Category:  %s\n", budget.CategoryName)
				fmt.Printf("Planned:   %.2f\n", budget.Planned)
				fmt.Printf("Actual:    %.2f\n", budget.Actual)
				fmt.Printf("Remaining: %.2f\n", budget.Planned-budget.Actual)
			}
		},
	}
	cmd.Flags().StringVar(&monthStr, "month", "", "month in YYYY-MM format")
	return cmd
}

func (a *App) buildBudgetsSet() *cobra.Command {
	var (
		monthStr     string
		budgetAmount float64
	)
	cmd := &cobra.Command{
		Use:   "set <category-id>",
		Short: "Set budget for a category",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, jsonMode, pretty)
			logger := audit.NewLogger()
			categoryID := args[0]

			if err := safety.Check(safety.TierMutation, readOnly, dryRun, confirm); err != nil {
				a.handleError(renderer, "budgets.set", err.(*errors.Error), start)
				return
			}

			y, m, err := resolveMonthOrCurrent(monthStr)
			if err != nil {
				a.handleError(renderer, "budgets.set", errors.New(errors.InvalidArguments, err.Error(), errors.CatValidation, false, nil), start)
				return
			}

			if dryRun {
				plan := safety.NewPlan()
				plan.Add("budgets.set", categoryID, nil, map[string]interface{}{"amount": budgetAmount, "month": m, "year": y})
				env := output.NewEnvelope("budgets.set", profile, output.SchemaVersion, "", plan, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "budgets.set", err.(*errors.Error), start)
				return
			}

			budget, err := svc.SetBudget(cmd.Context(), categoryID, budgetAmount, fmt.Sprintf("%04d-%02d-01", y, m))
			result := "success"
			var errCode string
			if err != nil {
				result = "failure"
				if e, ok := err.(*errors.Error); ok {
					errCode = string(e.Code)
				}
			}

			logger.Log(&audit.Record{Command: "budgets.set", ResourceID: categoryID, DryRun: dryRun, Confirmed: confirm, Profile: profile, Result: result, ErrorCode: errCode})

			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to set budget", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "budgets.set", cliErr, start)
				return
			}

			if jsonMode {
				env := output.NewEnvelope("budgets.set", profile, output.SchemaVersion, "", budget, time.Since(start))
				renderer.RenderSuccess(env)
			} else {
				fmt.Printf("Successfully set budget for %s to %.2f.\n", budget.CategoryName, budget.Planned)
			}
		},
	}
	cmd.Flags().StringVar(&monthStr, "month", "", "month in YYYY-MM format")
	cmd.Flags().Float64Var(&budgetAmount, "amount", 0, "budget amount")
	cmd.MarkFlagRequired("amount")
	return cmd
}

func (a *App) buildBudgetsReset() *cobra.Command {
	var monthStr string
	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Reset budget for a month",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, jsonMode, pretty)
			logger := audit.NewLogger()

			if err := safety.Check(safety.TierDestructive, readOnly, dryRun, confirm); err != nil {
				a.handleError(renderer, "budgets.reset", err.(*errors.Error), start)
				return
			}

			if monthStr == "" {
				a.handleError(renderer, "budgets.reset", errors.New(errors.InvalidArguments, "--month is required", errors.CatValidation, false, nil), start)
				return
			}
			y, m, err := parseMonthString(monthStr)
			if err != nil {
				a.handleError(renderer, "budgets.reset", errors.New(errors.InvalidArguments, err.Error(), errors.CatValidation, false, nil), start)
				return
			}

			if dryRun {
				plan := safety.NewPlan()
				plan.Add("budgets.reset", "", nil, map[string]int{"month": m, "year": y})
				env := output.NewEnvelope("budgets.reset", profile, output.SchemaVersion, "", plan, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "budgets.reset", err.(*errors.Error), start)
				return
			}

			err = svc.ResetBudget(cmd.Context(), m, y)
			result := "success"
			var errCode string
			if err != nil {
				result = "failure"
				if e, ok := err.(*errors.Error); ok {
					errCode = string(e.Code)
				}
			}

			logger.Log(&audit.Record{Command: "budgets.reset", DryRun: dryRun, Confirmed: confirm, Profile: profile, Result: result, ErrorCode: errCode})

			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to reset budget", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "budgets.reset", cliErr, start)
				return
			}

			if jsonMode {
				env := output.NewEnvelope("budgets.reset", profile, output.SchemaVersion, "", map[string]string{"status": "budget reset"}, time.Since(start))
				renderer.RenderSuccess(env)
			} else {
				fmt.Printf("Successfully reset budget for %d-%02d.\n", y, m)
			}
		},
	}
	cmd.Flags().StringVar(&monthStr, "month", "", "month in YYYY-MM format")
	cmd.MarkFlagRequired("month")
	return cmd
}

func (a *App) buildBudgetsExport() *cobra.Command {
	var monthStr string
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export budgets",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, jsonMode, pretty)

			y, m, err := resolveMonthOrCurrent(monthStr)
			if err != nil {
				a.handleError(renderer, "budgets.export", errors.New(errors.InvalidArguments, err.Error(), errors.CatValidation, false, nil), start)
				return
			}

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "budgets.export", err.(*errors.Error), start)
				return
			}

			startDate, endDate := monthDates(y, m)
			budgets, err := svc.ListBudgets(cmd.Context(), monarch.ListBudgetsOptions{StartDate: startDate, EndDate: endDate})
			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to list budgets", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "budgets.export", cliErr, start)
				return
			}

			env := output.NewEnvelope("budgets.export", profile, output.SchemaVersion, "", budgets, time.Since(start))
			renderer.RenderSuccess(env)
		},
	}
	cmd.Flags().StringVar(&monthStr, "month", "", "month in YYYY-MM format")
	return cmd
}

func (a *App) buildBudgetsFlexibleSet() *cobra.Command {
	var (
		monthStr     string
		budgetAmount float64
	)
	cmd := &cobra.Command{
		Use:   "set",
		Short: "Set flexible budget amount for a month",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, jsonMode, pretty)
			logger := audit.NewLogger()

			if err := safety.Check(safety.TierMutation, readOnly, dryRun, confirm); err != nil {
				a.handleError(renderer, "budgets.flexible.set", err.(*errors.Error), start)
				return
			}

			y, m, err := resolveMonthOrCurrent(monthStr)
			if err != nil {
				a.handleError(renderer, "budgets.flexible.set", errors.New(errors.InvalidArguments, err.Error(), errors.CatValidation, false, nil), start)
				return
			}

			if dryRun {
				plan := safety.NewPlan()
				plan.Add("budgets.flexible.set", fmt.Sprintf("%d-%02d", y, m), nil, map[string]interface{}{"amount": budgetAmount})
				env := output.NewEnvelope("budgets.flexible.set", profile, output.SchemaVersion, "", plan, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "budgets.flexible.set", err.(*errors.Error), start)
				return
			}

			err = svc.UpdateFlexibleBudget(cmd.Context(), m, y, budgetAmount)
			result := "success"
			var errCode string
			if err != nil {
				result = "failure"
				if e, ok := err.(*errors.Error); ok {
					errCode = string(e.Code)
				}
			}

			logger.Log(&audit.Record{Command: "budgets.flexible.set", DryRun: dryRun, Confirmed: confirm, Profile: profile, Result: result, ErrorCode: errCode})

			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to set flexible budget", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "budgets.flexible.set", cliErr, start)
				return
			}

			if jsonMode {
				env := output.NewEnvelope("budgets.flexible.set", profile, output.SchemaVersion, "", map[string]string{"status": "budget set"}, time.Since(start))
				renderer.RenderSuccess(env)
			} else {
				fmt.Printf("Successfully set flexible budget for %d-%02d to %.2f.\n", y, m, budgetAmount)
			}
		},
	}
	cmd.Flags().StringVar(&monthStr, "month", "", "month in YYYY-MM format")
	cmd.Flags().Float64Var(&budgetAmount, "amount", 0, "budget amount")
	cmd.MarkFlagRequired("amount")
	return cmd
}

func (a *App) buildBudgetsFlexRolloverSet() *cobra.Command {
	var (
		monthStr     string
		budgetAmount float64
	)
	cmd := &cobra.Command{
		Use:   "set",
		Short: "Set flexible budget rollover settings",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, jsonMode, pretty)
			logger := audit.NewLogger()

			if err := safety.Check(safety.TierMutation, readOnly, dryRun, confirm); err != nil {
				a.handleError(renderer, "budgets.flex-rollover.set", err.(*errors.Error), start)
				return
			}

			if dryRun {
				plan := safety.NewPlan()
				plan.Add("budgets.flex-rollover.set", monthStr, nil, map[string]interface{}{"balance": budgetAmount})
				env := output.NewEnvelope("budgets.flex-rollover.set", profile, output.SchemaVersion, "", plan, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "budgets.flex-rollover.set", err.(*errors.Error), start)
				return
			}

			err = svc.UpdateFlexRolloverSettings(cmd.Context(), monthStr, budgetAmount, true)
			result := "success"
			var errCode string
			if err != nil {
				result = "failure"
				if e, ok := err.(*errors.Error); ok {
					errCode = string(e.Code)
				}
			}

			logger.Log(&audit.Record{Command: "budgets.flex-rollover.set", DryRun: dryRun, Confirmed: confirm, Profile: profile, Result: result, ErrorCode: errCode})

			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to set flex rollover", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "budgets.flex-rollover.set", cliErr, start)
				return
			}

			if jsonMode {
				env := output.NewEnvelope("budgets.flex-rollover.set", profile, output.SchemaVersion, "", map[string]string{"status": "rollover set"}, time.Since(start))
				renderer.RenderSuccess(env)
			} else {
				fmt.Printf("Successfully set flex rollover starting %s with balance %.2f.\n", monthStr, budgetAmount)
			}
		},
	}
	cmd.Flags().StringVar(&monthStr, "month", "", "start month in YYYY-MM-DD format")
	cmd.Flags().Float64Var(&budgetAmount, "amount", 0, "starting balance")
	cmd.MarkFlagRequired("month")
	return cmd
}
