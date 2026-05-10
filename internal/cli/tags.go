package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/thedavidweng/monarchmoney-cli/internal/audit"
	"github.com/thedavidweng/monarchmoney-cli/internal/errors"
	"github.com/thedavidweng/monarchmoney-cli/internal/output"
	"github.com/thedavidweng/monarchmoney-cli/internal/safety"
)

func (a *App) buildTagsCommands(parent *cobra.Command) {
	tagsCmd := &cobra.Command{
		Use:   "tags",
		Short: "Manage Monarch Money tags",
	}
	tagsCmd.AddCommand(a.buildTagsList())
	tagsCmd.AddCommand(a.buildTagsCreate())
	parent.AddCommand(tagsCmd)
}

func (a *App) buildTagsList() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all tags",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, a.Flags.JSONMode, a.Flags.Pretty)

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "tags.list", err.(*errors.Error), start)
				return
			}

			tags, err := svc.ListTags(cmd.Context())
			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to list tags", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "tags.list", cliErr, start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("tags.list", a.Flags.Profile, output.SchemaVersion, "", tags, time.Since(start))
				a.renderSuccess(renderer, env, start)
			} else {
				fmt.Printf("%-20s %-20s %s\n", "ID", "NAME", "COLOR")
				for _, t := range tags {
					fmt.Printf("%-20s %-20s %s\n", t.ID, t.Name, t.Color)
				}
			}
		},
	}
}

func (a *App) buildTagsCreate() *cobra.Command {
	var (
		tagName  string
		tagColor string
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a tag",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, a.Flags.JSONMode, a.Flags.Pretty)
			logger := a.Deps.NewAuditLogger()

			if err := safety.Check(safety.TierMutation, a.Flags.ReadOnly, a.Flags.DryRun, a.Flags.Confirm); err != nil {
				a.handleError(renderer, "tags.create", err.(*errors.Error), start)
				return
			}

			if a.Flags.DryRun {
				plan := safety.NewPlan()
				plan.Add("tags.create", "", nil, map[string]string{"name": tagName, "color": tagColor})
				env := output.NewEnvelope("tags.create", a.Flags.Profile, output.SchemaVersion, "", plan, time.Since(start))
				a.renderSuccess(renderer, env, start)
				return
			}

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "tags.create", err.(*errors.Error), start)
				return
			}

			tag, err := svc.CreateTag(cmd.Context(), tagName, tagColor)
			result := "success"
			var errCode string
			if err != nil {
				result = "failure"
				if e, ok := err.(*errors.Error); ok {
					errCode = string(e.Code)
				}
			}

			a.logAudit(logger, &audit.Record{
				Command:   "tags.create",
				DryRun:    a.Flags.DryRun,
				Confirmed: a.Flags.Confirm,
				Profile:   a.Flags.Profile,
				Result:    result,
				ErrorCode: errCode,
			})

			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to create tag", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "tags.create", cliErr, start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("tags.create", a.Flags.Profile, output.SchemaVersion, "", tag, time.Since(start))
				a.renderSuccess(renderer, env, start)
			} else {
				fmt.Printf("Successfully created tag %s (%s).\n", tag.Name, tag.ID)
			}
		},
	}
	cmd.Flags().StringVar(&tagName, "name", "", "tag name")
	cmd.Flags().StringVar(&tagColor, "color", "#000000", "tag color")
	mustMarkFlagRequired(cmd, "name")
	return cmd
}
