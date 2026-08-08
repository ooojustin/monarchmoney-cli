package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/thedavidweng/monarchmoney-cli/internal/output"
)

func (a *App) buildGoalsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "goals",
		Short:   "Manage Monarch Money goals",
		GroupID: "core",
		Example: "  monarch goals list --json\n  monarch goals budgets --month 2026-05",
	}
	cmd.AddCommand(a.buildGoalsListCommand())
	cmd.AddCommand(a.buildGoalsBudgetsCommand())
	return cmd
}

func (a *App) buildGoalsListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List goals",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)

			svc, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "goals.list", wrapError(err, "failed to load service"), start)
				return
			}

			goals, err := svc.ListGoals(cmd.Context())
			if err != nil {
				a.handleError(renderer, "goals.list", wrapError(err, "failed to list goals"), start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("goals.list", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, goals, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-20s %-10s %12s %12s %12s\n", "ID", "NAME", "STATUS", "BALANCE", "TARGET", "PROGRESS")
			for i := range goals {
				goal := &goals[i]
				fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-20s %-10s %12.2f %12.2f %11.1f%%\n", goal.ID, goal.Name, goal.Status, goal.CurrentBalance, goal.TargetAmount, goal.Progress*100)
			}
		},
	}
}

func (a *App) buildGoalsBudgetsCommand() *cobra.Command {
	var month string
	cmd := &cobra.Command{
		Use:   "budgets",
		Short: "List savings goal monthly budget amounts",
		Run: func(cmd *cobra.Command, _ []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)
			y, m, monthErr := appBudgetMonth(month, time.Now())
			if monthErr != nil {
				a.handleError(renderer, "goals.budgets", monthErr, start)
				return
			}

			svc, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "goals.budgets", wrapError(err, "failed to load service"), start)
				return
			}

			startDate := fmt.Sprintf("%04d-%02d-01", y, m)
			endDate := time.Date(y, time.Month(m+1), 0, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
			budgets, err := svc.ListSavingsGoalBudgets(cmd.Context(), startDate, endDate)
			if err != nil {
				a.handleError(renderer, "goals.budgets", wrapError(err, "failed to list goal budgets"), start)
				return
			}
			if a.Flags.JSONMode {
				renderer.RenderSuccess(output.NewEnvelope("goals.budgets", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, budgets, time.Since(start)))
				return
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-20s %-10s %10s %10s %10s\n", "GOAL", "MONTH", "STATUS", "PLANNED", "ACTUAL", "REMAINING")
			for _, budget := range budgets {
				fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-20s %-10s %10.2f %10.2f %10.2f\n", budget.GoalName, budget.Month, budget.GoalStatus, budget.Planned, budget.Actual, budget.Remaining)
			}
		},
	}
	cmd.Flags().StringVar(&month, "month", "", "month in YYYY-MM format")
	return cmd
}
