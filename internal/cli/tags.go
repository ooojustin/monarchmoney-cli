package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/thedavidweng/monarchmoney-cli/internal/errors"
	"github.com/thedavidweng/monarchmoney-cli/internal/monarch"
	"github.com/thedavidweng/monarchmoney-cli/internal/output"
	"github.com/thedavidweng/monarchmoney-cli/internal/safety"
)

func (a *App) buildTagsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "tags",
		Short:   "Manage Monarch Money tags",
		GroupID: "core",
		Example: "  monarch tags list --json\n  monarch tags create --name \"reimbursable\" --confirm",
	}
	cmd.AddCommand(a.buildTagsListCommand())
	cmd.AddCommand(a.buildTagsCreateCommand())
	return cmd
}

func (a *App) buildTagsListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all tags",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)

			svc, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "tags.list", wrapError(err, "failed to load service"), start)
				return
			}

			tags, err := svc.ListTags(cmd.Context())
			if err != nil {
				a.handleError(renderer, "tags.list", wrapError(err, "failed to list tags"), start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("tags.list", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, tags, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-20s %s\n", "ID", "NAME", "COLOR")
			for _, t := range tags {
				fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-20s %s\n", t.ID, t.Name, t.Color)
			}
		},
	}
}

func (a *App) buildTagsCreateCommand() *cobra.Command {
	var (
		name  string
		color string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a tag",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)

			if !a.checkSafety(renderer, "tags.create", safety.TierMutation, start) {
				return
			}

			if a.Flags.DryRun {
				plan := safety.NewPlan()
				plan.Add("tags.create", "", nil, map[string]string{"name": name, "color": color})
				a.renderPlan(renderer, "tags.create", plan, start)
				return
			}

			svc, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "tags.create", wrapError(err, "failed to load service"), start)
				return
			}

			result, err := a.mutate(renderer, "tags.create", "", start, func() (any, error) {
				return svc.CreateTag(cmd.Context(), name, color)
			}, "failed to create tag")
			if err != nil {
				return
			}
			tag, ok := result.(*monarch.Tag)
			if !ok || tag == nil {
				a.handleError(renderer, "tags.create", errors.New(errors.InternalError, "unexpected tag creation result", errors.CatInternal, false, nil), start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("tags.create", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, tag, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully created tag %s (%s).\n", tag.Name, tag.ID)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "tag name")
	cmd.Flags().StringVar(&color, "color", "#000000", "tag color")
	cmd.MarkFlagRequired("name") //nolint:errcheck // flag registered above
	return cmd
}
