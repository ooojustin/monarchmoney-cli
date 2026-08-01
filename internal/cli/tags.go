package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/thedavidweng/monarchmoney-cli/internal/errors"
	"github.com/thedavidweng/monarchmoney-cli/internal/monarch"
	"github.com/thedavidweng/monarchmoney-cli/internal/output"
	"github.com/thedavidweng/monarchmoney-cli/internal/safety"
)

var (
	tagName  string
	tagColor string
)

var tagsCmd = &cobra.Command{
	Use:     "tags",
	Short:   "Manage Monarch Money tags",
	GroupID: "core",
	Example: "  monarch tags list --json\n  monarch tags create --name \"reimbursable\" --confirm",
}

var tagsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all tags",
	Run: func(cmd *cobra.Command, args []string) {
		run(cmd.Context(), "tags.list", "failed to list tags",
			func(ctx context.Context, svc *monarch.Service) ([]monarch.Tag, error) {
				return svc.ListTags(ctx)
			},
			func(tags []monarch.Tag) {
				fmt.Printf("%-20s %-20s %s\n", "ID", "NAME", "COLOR")
				for _, t := range tags {
					fmt.Printf("%-20s %-20s %s\n", t.ID, t.Name, t.Color)
				}
			})
	},
}

var tagsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a tag",
	Run: func(cmd *cobra.Command, args []string) {
		runMutation(cmd, "tags.create", "failed to create tag", safety.TierMutation, func() (mutation, *errors.Error) {
			var tag *monarch.Tag
			return mutation{
				planAfter: map[string]string{"name": tagName, "color": tagColor},
				do: func(ctx context.Context, svc *monarch.Service) (any, error) {
					t, err := svc.CreateTag(ctx, tagName, tagColor)
					if err != nil {
						return nil, err
					}
					tag = t
					return t, nil
				},
				human: func() { fmt.Printf("Successfully created tag %s (%s).\n", tag.Name, tag.ID) },
			}, nil
		})
	},
}

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

			svc, _, err := a.Deps.LoadService()
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
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-20s %s\n", "ID", "NAME", "COLOR") //nolint:errcheck // best-effort output
			for _, t := range tags {
				fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-20s %s\n", t.ID, t.Name, t.Color) //nolint:errcheck // best-effort output
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

			svc, _, err := a.Deps.LoadService()
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
			tag := result.(*monarch.Tag)

			if a.Flags.JSONMode {
				env := output.NewEnvelope("tags.create", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, tag, time.Since(start))
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully created tag %s (%s).\n", tag.Name, tag.ID) //nolint:errcheck // best-effort output
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "tag name")
	cmd.Flags().StringVar(&color, "color", "#000000", "tag color")
	cmd.MarkFlagRequired("name") //nolint:errcheck // flag registered above
	return cmd
}

func init() {
	tagsCreateCmd.Flags().StringVar(&tagName, "name", "", "tag name")
	tagsCreateCmd.Flags().StringVar(&tagColor, "color", "#000000", "tag color")
	tagsCreateCmd.MarkFlagRequired("name") //nolint:errcheck // flag registered above

	tagsCmd.AddCommand(tagsListCmd)
	tagsCmd.AddCommand(tagsCreateCmd)
	RootCmd.AddCommand(tagsCmd)
}
