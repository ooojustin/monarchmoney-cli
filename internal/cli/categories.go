package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/thedavidweng/monarchmoney-cli/internal/errors"
	"github.com/thedavidweng/monarchmoney-cli/internal/monarch"
	"github.com/thedavidweng/monarchmoney-cli/internal/safety"
)

var (
	categoryName    string
	categoryGroupID string
	categoryFile    string
)

var categoriesCmd = &cobra.Command{
	Use:     "categories",
	Short:   "Manage Monarch Money categories",
	GroupID: "core",
	Example: "  monarch categories list --json\n  monarch categories create --name \"Coffee\" --group <group-id> --confirm",
}

var categoriesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all categories",
	Run: func(cmd *cobra.Command, args []string) {
		run(cmd.Context(), "categories.list", "failed to list categories",
			func(ctx context.Context, svc *monarch.Service) ([]monarch.Category, error) {
				return svc.ListCategories(ctx)
			},
			func(cats []monarch.Category) {
				fmt.Printf("%-20s %-30s %s\n", "ID", "NAME", "GROUP")
				for _, c := range cats {
					fmt.Printf("%-20s %-30s %s\n", c.ID, c.Name, c.GroupName)
				}
			})
	},
}

var categoriesCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a category",
	Run: func(cmd *cobra.Command, args []string) {
		runMutation(cmd, "categories.create", "failed to create category", safety.TierMutation, func() (mutation, *errors.Error) {
			var cat *monarch.Category
			return mutation{
				planAfter: map[string]string{"name": categoryName, "groupId": categoryGroupID},
				do: func(ctx context.Context, svc *monarch.Service) (any, error) {
					c, err := svc.CreateCategory(ctx, categoryName, categoryGroupID)
					if err != nil {
						return nil, err
					}
					cat = c
					return c, nil
				},
				human: func() { fmt.Printf("Successfully created category %s (%s).\n", cat.Name, cat.ID) },
			}, nil
		})
	},
}

var categoriesDeleteCmd = &cobra.Command{
	Use:   "delete <category-id>",
	Short: "Delete a category",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		runMutation(cmd, "categories.delete", "failed to delete category", safety.TierDestructive, func() (mutation, *errors.Error) {
			return mutation{
				resourceID: id,
				do: func(ctx context.Context, svc *monarch.Service) (any, error) {
					if err := svc.DeleteCategory(ctx, id); err != nil {
						return nil, err
					}
					return map[string]string{"status": "deleted"}, nil
				},
				human: func() { fmt.Printf("Successfully deleted category %s.\n", id) },
			}, nil
		})
	},
}

var categoriesDeleteManyCmd = &cobra.Command{
	Use:   "delete-many",
	Short: "Delete multiple categories from a file",
	Run: func(cmd *cobra.Command, args []string) {
		runMutation(cmd, "categories.delete-many", "failed to delete categories", safety.TierDestructive, func() (mutation, *errors.Error) {
			if categoryFile == "" {
				return mutation{}, errors.New(errors.InvalidArguments, "--file is required", errors.CatValidation, false, nil)
			}
			f, err := os.Open(categoryFile)
			if err != nil {
				return mutation{}, errors.New(errors.InternalError, "failed to open file", errors.CatInternal, false, err)
			}
			defer func() {
				if cerr := f.Close(); cerr != nil {
					fmt.Fprintf(os.Stderr, "warning: failed to close file: %v\n", cerr)
				}
			}()

			var ids []string
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				id := strings.TrimSpace(scanner.Text())
				if id != "" {
					ids = append(ids, id)
				}
			}
			return mutation{
				planAfter: map[string]any{"ids": ids},
				do: func(ctx context.Context, svc *monarch.Service) (any, error) {
					if err := svc.DeleteCategories(ctx, ids); err != nil {
						return nil, err
					}
					return map[string]string{"status": "categories deleted"}, nil
				},
				human: func() { fmt.Printf("Successfully deleted %d categories.\n", len(ids)) },
			}, nil
		})
	},
}

var categoriesGroupsCmd = &cobra.Command{
	Use:   "groups",
	Short: "List all category groups",
	Run: func(cmd *cobra.Command, args []string) {
		run(cmd.Context(), "categories.groups", "failed to list category groups",
			func(ctx context.Context, svc *monarch.Service) ([]monarch.CategoryGroup, error) {
				return svc.ListCategoryGroups(ctx)
			},
			func(groups []monarch.CategoryGroup) {
				fmt.Printf("%-20s %-30s %s\n", "ID", "NAME", "TYPE")
				for _, g := range groups {
					fmt.Printf("%-20s %-30s %s\n", g.ID, g.Name, g.Type)
				}
			})
	},
}

func init() {
	categoriesCreateCmd.Flags().StringVar(&categoryName, "name", "", "category name")
	categoriesCreateCmd.Flags().StringVar(&categoryGroupID, "group", "", "category group ID")
	categoriesCreateCmd.MarkFlagRequired("name")  //nolint:errcheck // flag registered above
	categoriesCreateCmd.MarkFlagRequired("group") //nolint:errcheck // flag registered above

	categoriesDeleteManyCmd.Flags().StringVar(&categoryFile, "file", "", "file with category IDs (one per line)")
	categoriesDeleteManyCmd.MarkFlagRequired("file") //nolint:errcheck // flag registered above

	categoriesCmd.AddCommand(categoriesListCmd)
	categoriesCmd.AddCommand(categoriesGroupsCmd)
	categoriesCmd.AddCommand(categoriesCreateCmd)
	categoriesCmd.AddCommand(categoriesDeleteCmd)
	categoriesCmd.AddCommand(categoriesDeleteManyCmd)
	RootCmd.AddCommand(categoriesCmd)
}
