package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/thedavidweng/monarchmoney-cli/internal/errors"
	"github.com/thedavidweng/monarchmoney-cli/internal/monarch"
	"github.com/thedavidweng/monarchmoney-cli/internal/output"
	"github.com/thedavidweng/monarchmoney-cli/internal/safety"
)

func appBudgetMonth(month string, now time.Time) (int, int, *errors.Error) {
	if month == "" {
		return now.Year(), int(now.Month()), nil
	}

	parts := strings.Split(month, "-")
	if len(parts) != 2 {
		return 0, 0, errors.New(errors.InvalidArguments, "invalid month format, use YYYY-MM", errors.CatValidation, false, nil)
	}
	y, _ := strconv.Atoi(parts[0])
	m, _ := strconv.Atoi(parts[1])
	return y, m, nil
}

func appBudgetListOptions(month string, now time.Time) (monarch.ListBudgetsOptions, *errors.Error) {
	y, m, err := appBudgetMonth(month, now)
	if err != nil {
		return monarch.ListBudgetsOptions{}, err
	}

	lastDay := time.Date(y, time.Month(m+1), 0, 0, 0, 0, 0, time.UTC).Day()
	return monarch.ListBudgetsOptions{
		StartDate: fmt.Sprintf("%04d-%02d-01", y, m),
		EndDate:   fmt.Sprintf("%04d-%02d-%02d", y, m, lastDay),
	}, nil
}

func (a *App) buildBudgetsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "budgets",
		Short:   "Manage Monarch Money budgets",
		GroupID: "core",
		Example: "  monarch budgets list --month 2026-05 --json\n  monarch budgets set --category <id> --amount 500 --confirm",
	}
	cmd.AddCommand(a.buildBudgetsListCommand())
	cmd.AddCommand(a.buildBudgetsShowCommand())
	cmd.AddCommand(a.buildBudgetsSetCommand())
	cmd.AddCommand(a.buildBudgetsResetCommand())
	cmd.AddCommand(a.buildBudgetsExportCommand())
	cmd.AddCommand(a.buildBudgetsFlexibleCommand())
	cmd.AddCommand(a.buildBudgetsFlexRolloverCommand())
	return cmd
}

func (a *App) buildBudgetsListCommand() *cobra.Command {
	var month string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List budgets",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)

			svc, _, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "budgets.list", wrapError(err, "failed to load service"), start)
				return
			}

			opts, cliErr := appBudgetListOptions(month, time.Now())
			if cliErr != nil {
				a.handleError(renderer, "budgets.list", cliErr, start)
				return
			}

			budgets, err := svc.ListBudgets(cmd.Context(), opts)
			if err != nil {
				a.handleError(renderer, "budgets.list", wrapError(err, "failed to list budgets"), start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("budgets.list", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, budgets, time.Since(start))
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%-30s %10s %10s %10s\n", "CATEGORY", "PLANNED", "ACTUAL", "REMAINING") //nolint:errcheck // best-effort output
			for _, b := range budgets {
				fmt.Fprintf(cmd.OutOrStdout(), "%-30s %10.2f %10.2f %10.2f\n", b.CategoryName, b.Planned, b.Actual, b.Planned-b.Actual) //nolint:errcheck // best-effort output
			}
		},
	}
	cmd.Flags().StringVar(&month, "month", "", "month in YYYY-MM format")
	return cmd
}

func (a *App) buildBudgetsSetCommand() *cobra.Command {
	var (
		month  string
		amount float64
	)

	cmd := &cobra.Command{
		Use:   "set <category-id>",
		Short: "Set budget for a category",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)
			categoryID := args[0]

			if !a.checkSafety(renderer, "budgets.set", safety.TierMutation, start) {
				return
			}

			y, m, cliErr := appBudgetMonth(month, time.Now())
			if cliErr != nil {
				a.handleError(renderer, "budgets.set", cliErr, start)
				return
			}

			if a.Flags.DryRun {
				plan := safety.NewPlan()
				plan.Add("budgets.set", categoryID, nil, map[string]any{"amount": amount, "month": m, "year": y})
				a.renderPlan(renderer, "budgets.set", plan, start)
				return
			}

			svc, _, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "budgets.set", wrapError(err, "failed to load service"), start)
				return
			}

			result, err := a.mutate(renderer, "budgets.set", categoryID, start, func() (any, error) {
				return svc.SetBudget(cmd.Context(), categoryID, amount, fmt.Sprintf("%04d-%02d-01", y, m))
			}, "failed to set budget")
			if err != nil {
				return
			}
			budget := result.(*monarch.Budget)

			if a.Flags.JSONMode {
				env := output.NewEnvelope("budgets.set", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, budget, time.Since(start))
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully set budget for %s to %.2f.\n", budget.CategoryName, budget.Planned) //nolint:errcheck // best-effort output
		},
	}
	cmd.Flags().StringVar(&month, "month", "", "month in YYYY-MM format")
	cmd.Flags().Float64Var(&amount, "amount", 0, "budget amount")
	cmd.MarkFlagRequired("amount") //nolint:errcheck // flag registered above
	return cmd
}

func (a *App) buildBudgetsResetCommand() *cobra.Command {
	var month string

	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Reset budget for a month",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)

			if !a.checkSafety(renderer, "budgets.reset", safety.TierDestructive, start) {
				return
			}

			if month == "" {
				a.handleError(renderer, "budgets.reset", errors.New(errors.InvalidArguments, "--month is required", errors.CatValidation, false, nil), start)
				return
			}
			y, m, cliErr := appBudgetMonth(month, time.Now())
			if cliErr != nil {
				a.handleError(renderer, "budgets.reset", cliErr, start)
				return
			}

			if a.Flags.DryRun {
				plan := safety.NewPlan()
				plan.Add("budgets.reset", "", nil, map[string]int{"month": m, "year": y})
				a.renderPlan(renderer, "budgets.reset", plan, start)
				return
			}

			svc, _, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "budgets.reset", wrapError(err, "failed to load service"), start)
				return
			}

			if _, err := a.mutate(renderer, "budgets.reset", "", start, func() (any, error) {
				return nil, svc.ResetBudget(cmd.Context(), m, y)
			}, "failed to reset budget"); err != nil {
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("budgets.reset", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, map[string]string{"status": "budget reset"}, time.Since(start))
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully reset budget for %d-%02d.\n", y, m) //nolint:errcheck // best-effort output
		},
	}
	cmd.Flags().StringVar(&month, "month", "", "month in YYYY-MM format")
	cmd.MarkFlagRequired("month") //nolint:errcheck // flag registered above
	return cmd
}

func (a *App) buildBudgetsShowCommand() *cobra.Command {
	var month string

	cmd := &cobra.Command{
		Use:   "show <category-id>",
		Short: "Show budget for a specific category",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)
			categoryID := args[0]

			y, m, cliErr := appBudgetMonth(month, time.Now())
			if cliErr != nil {
				a.handleError(renderer, "budgets.show", cliErr, start)
				return
			}

			svc, _, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "budgets.show", wrapError(err, "failed to load service"), start)
				return
			}

			startDate := fmt.Sprintf("%04d-%02d-01", y, m)
			endDate := time.Date(y, time.Month(m+1), 0, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
			budget, err := svc.GetBudget(cmd.Context(), categoryID, startDate, endDate)
			if err != nil {
				a.handleError(renderer, "budgets.show", wrapError(err, "failed to get budget"), start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("budgets.show", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, budget, time.Since(start))
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Category:  %s\n", budget.CategoryName)            //nolint:errcheck // best-effort output
			fmt.Fprintf(cmd.OutOrStdout(), "Planned:   %.2f\n", budget.Planned)               //nolint:errcheck // best-effort output
			fmt.Fprintf(cmd.OutOrStdout(), "Actual:    %.2f\n", budget.Actual)                //nolint:errcheck // best-effort output
			fmt.Fprintf(cmd.OutOrStdout(), "Remaining: %.2f\n", budget.Planned-budget.Actual) //nolint:errcheck // best-effort output
		},
	}
	cmd.Flags().StringVar(&month, "month", "", "month in YYYY-MM format")
	return cmd
}

func (a *App) buildBudgetsExportCommand() *cobra.Command {
	var month string

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export budgets",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), true, a.Flags.Pretty)

			svc, _, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "budgets.export", wrapError(err, "failed to load service"), start)
				return
			}

			opts, cliErr := appBudgetListOptions(month, time.Now())
			if cliErr != nil {
				a.handleError(renderer, "budgets.export", cliErr, start)
				return
			}

			budgets, err := svc.ListBudgets(cmd.Context(), opts)
			if err != nil {
				a.handleError(renderer, "budgets.export", wrapError(err, "failed to list budgets"), start)
				return
			}

			env := output.NewEnvelope("budgets.export", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, budgets, time.Since(start))
			renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
		},
	}
	cmd.Flags().StringVar(&month, "month", "", "month in YYYY-MM format")
	return cmd
}

func (a *App) buildBudgetsFlexibleCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "flexible",
		Short: "Manage flexible budget settings",
	}
	cmd.AddCommand(a.buildBudgetsFlexibleSetCommand())
	return cmd
}

func (a *App) buildBudgetsFlexibleSetCommand() *cobra.Command {
	var (
		month  string
		amount float64
	)

	cmd := &cobra.Command{
		Use:   "set",
		Short: "Set flexible budget amount for a month",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)

			if !a.checkSafety(renderer, "budgets.flexible.set", safety.TierMutation, start) {
				return
			}

			y, m, cliErr := appBudgetMonth(month, time.Now())
			if cliErr != nil {
				a.handleError(renderer, "budgets.flexible.set", cliErr, start)
				return
			}

			resourceID := fmt.Sprintf("%d-%02d", y, m)
			if a.Flags.DryRun {
				plan := safety.NewPlan()
				plan.Add("budgets.flexible.set", resourceID, nil, map[string]any{"amount": amount})
				a.renderPlan(renderer, "budgets.flexible.set", plan, start)
				return
			}

			svc, _, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "budgets.flexible.set", wrapError(err, "failed to load service"), start)
				return
			}

			if _, err := a.mutate(renderer, "budgets.flexible.set", resourceID, start, func() (any, error) {
				return nil, svc.UpdateFlexibleBudget(cmd.Context(), m, y, amount)
			}, "failed to set flexible budget"); err != nil {
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("budgets.flexible.set", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, map[string]string{"status": "budget set"}, time.Since(start))
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully set flexible budget for %d-%02d to %.2f.\n", y, m, amount) //nolint:errcheck // best-effort output
		},
	}
	cmd.Flags().StringVar(&month, "month", "", "month in YYYY-MM format")
	cmd.Flags().Float64Var(&amount, "amount", 0, "budget amount")
	cmd.MarkFlagRequired("amount") //nolint:errcheck // flag registered above
	return cmd
}

func (a *App) buildBudgetsFlexRolloverCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "flex-rollover",
		Short: "Manage flexible budget rollover settings",
	}
	cmd.AddCommand(a.buildBudgetsFlexRolloverSetCommand())
	return cmd
}

func (a *App) buildBudgetsFlexRolloverSetCommand() *cobra.Command {
	var (
		month  string
		amount float64
	)

	cmd := &cobra.Command{
		Use:   "set",
		Short: "Set flexible budget rollover settings",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)

			if !a.checkSafety(renderer, "budgets.flex-rollover.set", safety.TierMutation, start) {
				return
			}

			if a.Flags.DryRun {
				plan := safety.NewPlan()
				plan.Add("budgets.flex-rollover.set", month, nil, map[string]any{"balance": amount})
				a.renderPlan(renderer, "budgets.flex-rollover.set", plan, start)
				return
			}

			svc, _, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "budgets.flex-rollover.set", wrapError(err, "failed to load service"), start)
				return
			}

			if _, err := a.mutate(renderer, "budgets.flex-rollover.set", month, start, func() (any, error) {
				return nil, svc.UpdateFlexRolloverSettings(cmd.Context(), month, amount, true)
			}, "failed to set flex rollover"); err != nil {
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("budgets.flex-rollover.set", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, map[string]string{"status": "rollover set"}, time.Since(start))
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully set flex rollover starting %s with balance %.2f.\n", month, amount) //nolint:errcheck // best-effort output
		},
	}
	cmd.Flags().StringVar(&month, "month", "", "start month in YYYY-MM-DD format")
	cmd.Flags().Float64Var(&amount, "amount", 0, "starting balance")
	cmd.MarkFlagRequired("month") //nolint:errcheck // flag registered above
	return cmd
}
