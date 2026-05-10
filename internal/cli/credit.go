package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/thedavidweng/monarchmoney-cli/internal/errors"
	"github.com/thedavidweng/monarchmoney-cli/internal/output"
)

func (a *App) buildCreditCommands(parent *cobra.Command) {
	creditCmd := &cobra.Command{
		Use:   "credit",
		Short: "Manage credit history",
	}
	creditCmd.AddCommand(a.buildCreditHistory())
	parent.AddCommand(creditCmd)
}

func (a *App) buildCreditHistory() *cobra.Command {
	return &cobra.Command{
		Use:   "history",
		Short: "Get credit score history",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, a.Flags.JSONMode, a.Flags.Pretty)

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "credit.history", err.(*errors.Error), start)
				return
			}

			history, err := svc.GetCreditHistory(cmd.Context())
			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to get credit history", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "credit.history", cliErr, start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("credit.history", a.Flags.Profile, output.SchemaVersion, "", history, time.Since(start))
				renderer.RenderSuccess(env)
			} else {
				fmt.Printf("%-12s %s\n", "DATE", "SCORE")
				for _, r := range history {
					fmt.Printf("%-12s %d\n", r.Date, r.Score)
				}
			}
		},
	}
}
