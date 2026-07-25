package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

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
		start := time.Now()
		renderer := output.NewRenderer(nil, nil, jsonMode, pretty)

		if err := safety.Check(safety.TierMutation, readOnly, dryRun, confirm); err != nil {
			handleError(renderer, "tags.create", err, start)
			return
		}

		if dryRun {
			plan := safety.NewPlan()
			plan.Add("tags.create", "", nil, map[string]string{"name": tagName, "color": tagColor})
			env := output.NewEnvelope("tags.create", profile, output.SchemaVersion, requestID, plan, time.Since(start))
			renderer.RenderSuccess(env)
			return
		}

		deps, ok := newDeps(renderer, "tags.create", start)
		if !ok {
			return
		}

		result, err := deps.Mutate("tags.create", "", func() (any, error) {
			return deps.Service.CreateTag(cmd.Context(), tagName, tagColor)
		}, "failed to create tag")
		if err != nil {
			return
		}
		tag, _ := result.(*monarch.Tag)

		if jsonMode {
			env := output.NewEnvelope("tags.create", profile, output.SchemaVersion, requestID, tag, time.Since(start))
			renderer.RenderSuccess(env)
		} else {
			fmt.Printf("Successfully created tag %s (%s).\n", tag.Name, tag.ID)
		}
	},
}

func init() {
	tagsCreateCmd.Flags().StringVar(&tagName, "name", "", "tag name")
	tagsCreateCmd.Flags().StringVar(&tagColor, "color", "#000000", "tag color")
	tagsCreateCmd.MarkFlagRequired("name") //nolint:errcheck // flag registered above

	tagsCmd.AddCommand(tagsListCmd)
	tagsCmd.AddCommand(tagsCreateCmd)
	RootCmd.AddCommand(tagsCmd)
}
