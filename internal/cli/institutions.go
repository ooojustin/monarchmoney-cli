package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/thedavidweng/monarchmoney-cli/internal/errors"
	"github.com/thedavidweng/monarchmoney-cli/internal/output"
)

func (a *App) buildInstitutionsCommands(parent *cobra.Command) {
	institutionsCmd := &cobra.Command{
		Use:   "institutions",
		Short: "Manage financial institutions",
	}
	institutionsCmd.AddCommand(a.buildInstitutionsList())
	parent.AddCommand(institutionsCmd)
}

func (a *App) buildInstitutionsList() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all institutions",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, jsonMode, pretty)

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "institutions.list", err.(*errors.Error), start)
				return
			}

			insts, err := svc.ListInstitutions(cmd.Context())
			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to list institutions", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "institutions.list", cliErr, start)
				return
			}

			if jsonMode {
				env := output.NewEnvelope("institutions.list", profile, output.SchemaVersion, "", insts, time.Since(start))
				renderer.RenderSuccess(env)
			} else {
				fmt.Printf("%-20s %-30s %s\n", "ID", "NAME", "URL")
				for _, inst := range insts {
					fmt.Printf("%-20s %-30s %s\n", inst.ID, inst.Name, inst.URL)
				}
			}
		},
	}
}
