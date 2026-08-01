package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/thedavidweng/monarchmoney-cli/internal/errors"
	"github.com/thedavidweng/monarchmoney-cli/internal/monarch"
	"github.com/thedavidweng/monarchmoney-cli/internal/output"
)

var goalsCmd = &cobra.Command{
	Use:     "goals",
	Short:   "Manage Monarch Money goals",
	GroupID: "core",
	Example: "  monarch goals list --json\n  monarch goals budgets --month 2026-05",
}

var goalsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List goals",
	Run: func(cmd *cobra.Command, args []string) {
		run(cmd.Context(), "goals.list", "failed to list goals",
			func(ctx context.Context, svc *monarch.Service) ([]monarch.Goal, error) {
				return svc.ListGoals(ctx)
			},
			func(goals []monarch.Goal) {
				fmt.Printf("%-20s %-20s %-10s %12s %12s %12s\n", "ID", "NAME", "STATUS", "BALANCE", "TARGET", "PROGRESS")
				for _, g := range goals {
					fmt.Printf("%-20s %-20s %-10s %12.2f %12.2f %11.1f%%\n", g.ID, g.Name, g.Status, g.CurrentBalance, g.TargetAmount, g.Progress*100)
				}
			})
	},
}

var goalsBudgetsCmd = &cobra.Command{
	Use:   "budgets",
	Short: "List savings goal monthly budget amounts",
	Run: func(cmd *cobra.Command, args []string) {
		run(cmd.Context(), "goals.budgets", "failed to list goal budgets",
			func(ctx context.Context, svc *monarch.Service) ([]monarch.SavingsGoalBudget, error) {
				var y, m int
				if monthStr != "" {
					parts := strings.Split(monthStr, "-")
					if len(parts) != 2 {
						return nil, errors.New(errors.InvalidArguments, "invalid month format, use YYYY-MM", errors.CatValidation, false, nil)
					}
					y, _ = strconv.Atoi(parts[0])
					m, _ = strconv.Atoi(parts[1])
				} else {
					now := time.Now()
					y = now.Year()
					m = int(now.Month())
				}
				startDate := fmt.Sprintf("%04d-%02d-01", y, m)
				endDate := time.Date(y, time.Month(m+1), 0, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
				return svc.ListSavingsGoalBudgets(ctx, startDate, endDate)
			},
			func(budgets []monarch.SavingsGoalBudget) {
				fmt.Printf("%-20s %-20s %-10s %10s %10s %10s\n", "GOAL", "MONTH", "STATUS", "PLANNED", "ACTUAL", "REMAINING")
				for _, b := range budgets {
					fmt.Printf("%-20s %-20s %-10s %10.2f %10.2f %10.2f\n", b.GoalName, b.Month, b.GoalStatus, b.Planned, b.Actual, b.Remaining)
				}
			})
	},
}

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

			svc, _, err := a.Deps.LoadService()
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
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-20s %-10s %12s %12s %12s\n", "ID", "NAME", "STATUS", "BALANCE", "TARGET", "PROGRESS") //nolint:errcheck // best-effort output
			for _, goal := range goals {
				fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-20s %-10s %12.2f %12.2f %11.1f%%\n", goal.ID, goal.Name, goal.Status, goal.CurrentBalance, goal.TargetAmount, goal.Progress*100) //nolint:errcheck // best-effort output
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
			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "goals.budgets", wrapError(err, "failed to load service"), start)
				return
			}

			y, m := time.Now().Year(), int(time.Now().Month())
			if month != "" {
				parts := strings.Split(month, "-")
				if len(parts) != 2 {
					a.handleError(renderer, "goals.budgets", errors.New(errors.InvalidArguments, "invalid month format, use YYYY-MM", errors.CatValidation, false, nil), start)
					return
				}
				y, _ = strconv.Atoi(parts[0])
				m, _ = strconv.Atoi(parts[1])
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
			fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-20s %-10s %10s %10s %10s\n", "GOAL", "MONTH", "STATUS", "PLANNED", "ACTUAL", "REMAINING") //nolint:errcheck
			for _, budget := range budgets {
				fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-20s %-10s %10.2f %10.2f %10.2f\n", budget.GoalName, budget.Month, budget.GoalStatus, budget.Planned, budget.Actual, budget.Remaining) //nolint:errcheck
			}
		},
	}
	cmd.Flags().StringVar(&month, "month", "", "month in YYYY-MM format")
	return cmd
}

func init() {
	goalsBudgetsCmd.Flags().StringVar(&monthStr, "month", "", "month in YYYY-MM format")
	goalsCmd.AddCommand(goalsListCmd)
	goalsCmd.AddCommand(goalsBudgetsCmd)
	RootCmd.AddCommand(goalsCmd)
}
