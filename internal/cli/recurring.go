package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/thedavidweng/monarchmoney-cli/internal/errors"
	"github.com/thedavidweng/monarchmoney-cli/internal/monarch"
	"github.com/thedavidweng/monarchmoney-cli/internal/output"
	"github.com/thedavidweng/monarchmoney-cli/internal/safety"
)

var (
	recurringAmount float64
)

var recurringCmd = &cobra.Command{
	Use:     "recurring",
	Short:   "Manage recurring transactions",
	GroupID: "core",
	Example: "  monarch recurring list --json",
}

var recurringListCmd = &cobra.Command{
	Use:   "list",
	Short: "List recurring transactions",
	Run: func(cmd *cobra.Command, args []string) {
		run(cmd.Context(), "recurring.list", "failed to list recurring transactions",
			func(ctx context.Context, svc *monarch.Service) ([]monarch.RecurringTransaction, error) {
				now := time.Now()
				startDate := now.Format("2006-01-02")
				endDate := time.Date(now.Year(), now.Month()+2, 0, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
				return svc.ListRecurring(ctx, startDate, endDate)
			},
			func(recurring []monarch.RecurringTransaction) {
				fmt.Printf("%-20s %10s %-12s %-12s %s\n", "MERCHANT", "AMOUNT", "FREQUENCY", "NEXT DATE", "STATUS")
				for _, r := range recurring {
					fmt.Printf("%-20s %10.2f %-12s %-12s %s\n", r.Merchant, r.Amount, r.Frequency, r.NextDate, r.Status)
				}
			})
	},
}

var recurringUpdateCmd = &cobra.Command{
	Use:   "update <recurring-id>",
	Short: "Update a recurring transaction",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		runMutation(cmd, "recurring.update", "failed to update recurring transaction", safety.TierMutation, func() (mutation, *errors.Error) {
			var r *monarch.RecurringTransaction
			return mutation{
				resourceID: id,
				planAfter:  map[string]any{"amount": recurringAmount},
				do: func(ctx context.Context, svc *monarch.Service) (any, error) {
					updated, err := svc.UpdateRecurring(ctx, id, recurringAmount)
					if err != nil {
						return nil, err
					}
					r = updated
					return updated, nil
				},
				human: func() { fmt.Printf("Successfully updated recurring transaction %s.\n", r.ID) },
			}, nil
		})
	},
}

func (a *App) buildRecurringCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "recurring",
		Short:   "Manage recurring transactions",
		GroupID: "core",
		Example: "  monarch recurring list --json",
	}
	cmd.AddCommand(a.buildRecurringListCommand())
	cmd.AddCommand(a.buildRecurringUpdateCommand())
	return cmd
}

func (a *App) buildRecurringListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List recurring transactions",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "recurring.list", wrapError(err, "failed to load service"), start)
				return
			}

			now := time.Now()
			startDate := now.Format("2006-01-02")
			endDate := time.Date(now.Year(), now.Month()+2, 0, 0, 0, 0, 0, time.UTC).Format("2006-01-02")

			recurring, err := svc.ListRecurring(cmd.Context(), startDate, endDate)
			if err != nil {
				a.handleError(renderer, "recurring.list", wrapError(err, "failed to list recurring transactions"), start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("recurring.list", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, recurring, time.Since(start))
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%-20s %10s %-12s %-12s %s\n", "MERCHANT", "AMOUNT", "FREQUENCY", "NEXT DATE", "STATUS") //nolint:errcheck // best-effort output
			for _, r := range recurring {
				fmt.Fprintf(cmd.OutOrStdout(), "%-20s %10.2f %-12s %-12s %s\n", r.Merchant, r.Amount, r.Frequency, r.NextDate, r.Status) //nolint:errcheck // best-effort output
			}
		},
	}
}

func (a *App) buildRecurringUpdateCommand() *cobra.Command {
	var amount float64

	cmd := &cobra.Command{
		Use:   "update <recurring-id>",
		Short: "Update a recurring transaction",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)
			id := args[0]

			if !a.checkSafety(renderer, "recurring.update", safety.TierMutation, start) {
				return
			}

			if a.Flags.DryRun {
				plan := safety.NewPlan()
				plan.Add("recurring.update", id, nil, map[string]any{"amount": amount})
				a.renderPlan(renderer, "recurring.update", plan, start)
				return
			}

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "recurring.update", wrapError(err, "failed to load service"), start)
				return
			}

			result, err := a.mutate(renderer, "recurring.update", id, start, func() (any, error) {
				return svc.UpdateRecurring(cmd.Context(), id, amount)
			}, "failed to update recurring transaction")
			if err != nil {
				return
			}
			r := result.(*monarch.RecurringTransaction)

			if a.Flags.JSONMode {
				env := output.NewEnvelope("recurring.update", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, r, time.Since(start))
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully updated recurring transaction %s.\n", r.ID) //nolint:errcheck // best-effort output
		},
	}
	cmd.Flags().Float64Var(&amount, "amount", 0, "new recurring amount")
	cmd.MarkFlagRequired("amount") //nolint:errcheck // flag registered above
	return cmd
}

func init() {
	recurringUpdateCmd.Flags().Float64Var(&recurringAmount, "amount", 0, "new recurring amount")
	recurringUpdateCmd.MarkFlagRequired("amount") //nolint:errcheck // flag registered above

	recurringCmd.AddCommand(recurringListCmd)
	recurringCmd.AddCommand(recurringUpdateCmd)
	RootCmd.AddCommand(recurringCmd)
}
