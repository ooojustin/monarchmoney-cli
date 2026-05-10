package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/thedavidweng/monarchmoney-cli/internal/audit"
	"github.com/thedavidweng/monarchmoney-cli/internal/errors"
	"github.com/thedavidweng/monarchmoney-cli/internal/output"
	"github.com/thedavidweng/monarchmoney-cli/internal/safety"
)

func (a *App) buildRecurringCommands(parent *cobra.Command) {
	recurringCmd := &cobra.Command{
		Use:   "recurring",
		Short: "Manage recurring transactions",
	}
	recurringCmd.AddCommand(a.buildRecurringList())
	recurringCmd.AddCommand(a.buildRecurringUpdate())
	parent.AddCommand(recurringCmd)
}

func (a *App) buildRecurringList() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List recurring transactions",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, a.Flags.JSONMode, a.Flags.Pretty)

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "recurring.list", err.(*errors.Error), start)
				return
			}

			// Default to current month
			now := time.Now()
			startDate := now.Format("2006-01-02")
			endDate := time.Date(now.Year(), now.Month()+2, 0, 0, 0, 0, 0, time.UTC).Format("2006-01-02")

			recurring, err := svc.ListRecurring(cmd.Context(), startDate, endDate)
			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to list recurring transactions", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "recurring.list", cliErr, start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("recurring.list", a.Flags.Profile, output.SchemaVersion, "", recurring, time.Since(start))
				renderer.RenderSuccess(env)
			} else {
				fmt.Printf("%-20s %10s %-12s %-12s %s\n", "MERCHANT", "AMOUNT", "FREQUENCY", "NEXT DATE", "STATUS")
				for _, r := range recurring {
					fmt.Printf("%-20s %10.2f %-12s %-12s %s\n", r.Merchant, r.Amount, r.Frequency, r.NextDate, r.Status)
				}
			}
		},
	}
}

func (a *App) buildRecurringUpdate() *cobra.Command {
	var recurringAmount float64
	cmd := &cobra.Command{
		Use:   "update <recurring-id>",
		Short: "Update a recurring transaction",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, a.Flags.JSONMode, a.Flags.Pretty)
			logger := a.Deps.NewAuditLogger()
			id := args[0]

			if err := safety.Check(safety.TierMutation, a.Flags.ReadOnly, a.Flags.DryRun, a.Flags.Confirm); err != nil {
				a.handleError(renderer, "recurring.update", err.(*errors.Error), start)
				return
			}

			if a.Flags.DryRun {
				plan := safety.NewPlan()
				plan.Add("recurring.update", id, nil, map[string]interface{}{"amount": recurringAmount})
				env := output.NewEnvelope("recurring.update", a.Flags.Profile, output.SchemaVersion, "", plan, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "recurring.update", err.(*errors.Error), start)
				return
			}

			r, err := svc.UpdateRecurring(cmd.Context(), id, recurringAmount)
			result := "success"
			var errCode string
			if err != nil {
				result = "failure"
				if e, ok := err.(*errors.Error); ok {
					errCode = string(e.Code)
				}
			}

			logger.Log(&audit.Record{
				Command:    "recurring.update",
				ResourceID: id,
				DryRun:     a.Flags.DryRun,
				Confirmed:  a.Flags.Confirm,
				Profile:    a.Flags.Profile,
				Result:     result,
				ErrorCode:  errCode,
			})

			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to update recurring transaction", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "recurring.update", cliErr, start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("recurring.update", a.Flags.Profile, output.SchemaVersion, "", r, time.Since(start))
				renderer.RenderSuccess(env)
			} else {
				fmt.Printf("Successfully updated recurring transaction %s.\n", r.ID)
			}
		},
	}
	cmd.Flags().Float64Var(&recurringAmount, "amount", 0, "new recurring amount")
	cmd.MarkFlagRequired("amount")
	return cmd
}
