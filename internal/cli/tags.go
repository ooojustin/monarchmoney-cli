package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/thedavidweng/monarchmoney-cli/internal/errors"
	"github.com/thedavidweng/monarchmoney-cli/internal/monarch"
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

func init() {
	tagsCreateCmd.Flags().StringVar(&tagName, "name", "", "tag name")
	tagsCreateCmd.Flags().StringVar(&tagColor, "color", "#000000", "tag color")
	tagsCreateCmd.MarkFlagRequired("name") //nolint:errcheck // flag registered above

	tagsCmd.AddCommand(tagsListCmd)
	tagsCmd.AddCommand(tagsCreateCmd)
	RootCmd.AddCommand(tagsCmd)
}
