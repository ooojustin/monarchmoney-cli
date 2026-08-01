package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/thedavidweng/monarchmoney-cli/internal/monarch"
	"github.com/thedavidweng/monarchmoney-cli/internal/output"
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

func (a *App) buildCreditCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "credit",
		Short:   "Manage credit history",
		GroupID: "core",
		Example: "  monarch credit history --json",
	}
	cmd.AddCommand(a.buildCreditHistoryCommand())
	return cmd
}

func (a *App) buildCreditHistoryCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "history",
		Short: "Get credit score history",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "credit.history", wrapError(err, "failed to load service"), start)
				return
			}

			history, err := svc.GetCreditHistory(cmd.Context())
			if err != nil {
				a.handleError(renderer, "credit.history", wrapError(err, "failed to get credit history"), start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("credit.history", a.Flags.Profile, output.SchemaVersion, "", history, time.Since(start))
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%-12s %s\n", "DATE", "SCORE") //nolint:errcheck // best-effort output
			for _, r := range history {
				fmt.Fprintf(cmd.OutOrStdout(), "%-12s %d\n", r.Date, r.Score) //nolint:errcheck // best-effort output
			}
		},
	}
}

func init() {
	creditCmd.AddCommand(creditHistoryCmd)
	RootCmd.AddCommand(creditCmd)
}
