package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/thedavidweng/monarchmoney-cli/internal/monarch"
)

var institutionsCmd = &cobra.Command{
	Use:     "institutions",
	Short:   "Manage financial institutions",
	GroupID: "core",
	Example: "  monarch institutions list --json",
}

var institutionsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all institutions",
	Run: func(cmd *cobra.Command, args []string) {
		run(cmd.Context(), "institutions.list", "failed to list institutions",
			func(ctx context.Context, svc *monarch.Service) ([]monarch.Institution, error) {
				return svc.ListInstitutions(ctx)
			},
			func(insts []monarch.Institution) {
				fmt.Printf("%-20s %-30s %s\n", "ID", "NAME", "URL")
				for _, inst := range insts {
					fmt.Printf("%-20s %-30s %s\n", inst.ID, inst.Name, inst.URL)
				}
			})
	},
}

func init() {
	institutionsCmd.AddCommand(institutionsListCmd)
	RootCmd.AddCommand(institutionsCmd)
}
