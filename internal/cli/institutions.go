package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/thedavidweng/monarchmoney-cli/internal/monarch"
	"github.com/thedavidweng/monarchmoney-cli/internal/output"
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

func (a *App) buildInstitutionsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "institutions",
		Short:   "Manage financial institutions",
		GroupID: "core",
		Example: "  monarch institutions list --json",
	}
	cmd.AddCommand(a.buildInstitutionsListCommand())
	return cmd
}

func (a *App) buildInstitutionsListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all institutions",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)

			svc, _, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "institutions.list", wrapError(err, "failed to load service"), start)
				return
			}

			insts, err := svc.ListInstitutions(cmd.Context())
			if err != nil {
				a.handleError(renderer, "institutions.list", wrapError(err, "failed to list institutions"), start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("institutions.list", a.Flags.Profile, output.SchemaVersion, "", insts, time.Since(start))
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-30s %s\n", "ID", "NAME", "URL") //nolint:errcheck // best-effort output
			for _, inst := range insts {
				fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-30s %s\n", inst.ID, inst.Name, inst.URL) //nolint:errcheck // best-effort output
			}
		},
	}
}

func init() {
	institutionsCmd.AddCommand(institutionsListCmd)
	RootCmd.AddCommand(institutionsCmd)
}
