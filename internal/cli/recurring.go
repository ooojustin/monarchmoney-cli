package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/thedavidweng/monarchmoney-cli/internal/errors"
	"github.com/thedavidweng/monarchmoney-cli/internal/monarch"
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

func init() {
	recurringUpdateCmd.Flags().Float64Var(&recurringAmount, "amount", 0, "new recurring amount")
	recurringUpdateCmd.MarkFlagRequired("amount") //nolint:errcheck // flag registered above

	recurringCmd.AddCommand(recurringListCmd)
	recurringCmd.AddCommand(recurringUpdateCmd)
	RootCmd.AddCommand(recurringCmd)
}
