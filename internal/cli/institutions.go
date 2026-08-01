package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/thedavidweng/monarchmoney-cli/internal/output"
)

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

			svc, err := a.loadService()
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
				env := output.NewEnvelope("institutions.list", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, insts, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-30s %s\n", "ID", "NAME", "URL")
			for _, inst := range insts {
				fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-30s %s\n", inst.ID, inst.Name, inst.URL)
			}
		},
	}
}
