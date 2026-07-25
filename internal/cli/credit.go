package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/thedavidweng/monarchmoney-cli/internal/monarch"
)

var creditCmd = &cobra.Command{
	Use:     "credit",
	Short:   "Manage credit history",
	GroupID: "core",
	Example: "  monarch credit history --json",
}

var creditHistoryCmd = &cobra.Command{
	Use:   "history",
	Short: "Get credit score history",
	Run: func(cmd *cobra.Command, args []string) {
		run(cmd.Context(), "credit.history", "failed to get credit history",
			func(ctx context.Context, svc *monarch.Service) ([]monarch.CreditRecord, error) {
				return svc.GetCreditHistory(ctx)
			},
			func(history []monarch.CreditRecord) {
				fmt.Printf("%-12s %s\n", "DATE", "SCORE")
				for _, r := range history {
					fmt.Printf("%-12s %d\n", r.Date, r.Score)
				}
			})
	},
}

func init() {
	creditCmd.AddCommand(creditHistoryCmd)
	RootCmd.AddCommand(creditCmd)
}
