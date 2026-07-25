package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/thedavidweng/monarchmoney-cli/internal/monarch"
)

var goalsCmd = &cobra.Command{
	Use:     "goals",
	Short:   "Manage Monarch Money goals",
	GroupID: "core",
	Example: "  monarch goals list --json",
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
				fmt.Printf("%-20s %s\n", "ID", "NAME")
				for _, goal := range goals {
					fmt.Printf("%-20s %s\n", goal.ID, goal.Name)
				}
			})
	},
}

func init() {
	goalsCmd.AddCommand(goalsListCmd)
	RootCmd.AddCommand(goalsCmd)
}
