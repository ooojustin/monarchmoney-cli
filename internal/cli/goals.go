package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/thedavidweng/monarchmoney-cli/internal/errors"
	"github.com/thedavidweng/monarchmoney-cli/internal/output"
)

func (a *App) buildGoalsCommands(parent *cobra.Command) {
	goalsCmd := &cobra.Command{
		Use:   "goals",
		Short: "Manage Monarch Money goals",
	}
	goalsCmd.AddCommand(a.buildGoalsList())
	parent.AddCommand(goalsCmd)
}

func (a *App) buildGoalsList() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List goals",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, a.Flags.JSONMode, a.Flags.Pretty)

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "goals.list", err.(*errors.Error), start)
				return
			}

			goals, err := svc.ListGoals(cmd.Context())
			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to list goals", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "goals.list", cliErr, start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("goals.list", a.Flags.Profile, output.SchemaVersion, "", goals, time.Since(start))
				a.renderSuccess(renderer, env, start)
			} else {
				fmt.Printf("%-20s %s\n", "ID", "NAME")
				for _, goal := range goals {
					fmt.Printf("%-20s %s\n", goal.ID, goal.Name)
				}
			}
		},
	}
}
