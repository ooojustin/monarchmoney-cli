package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/thedavidweng/monarchmoney-cli/internal/errors"
	"github.com/thedavidweng/monarchmoney-cli/internal/monarch"
	"github.com/thedavidweng/monarchmoney-cli/internal/output"
	"github.com/thedavidweng/monarchmoney-cli/internal/safety"
)

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

			svc, err := a.loadService()
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
				renderer.RenderSuccess(env)
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%-20s %10s %-12s %-12s %s\n", "MERCHANT", "AMOUNT", "FREQUENCY", "NEXT DATE", "STATUS")
			for _, r := range recurring {
				fmt.Fprintf(cmd.OutOrStdout(), "%-20s %10.2f %-12s %-12s %s\n", r.Merchant, r.Amount, r.Frequency, r.NextDate, r.Status)
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

			svc, err := a.loadService()
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
			r, ok := result.(*monarch.RecurringTransaction)
			if !ok || r == nil {
				a.handleError(renderer, "recurring.update", errors.New(errors.InternalError, "unexpected recurring update result", errors.CatInternal, false, nil), start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("recurring.update", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, r, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully updated recurring transaction %s.\n", r.ID)
		},
	}
	cmd.Flags().Float64Var(&amount, "amount", 0, "new recurring amount")
	cmd.MarkFlagRequired("amount") //nolint:errcheck // flag registered above
	return cmd
}
