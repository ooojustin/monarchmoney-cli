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
	categoryName            string
	categoryGroupID         string
	categoryFile            string
	categoryIcon            string
	categoryBudgetVar       string
	categoryExcludeBudget   bool
	categoryGroupName       string
	categoryRolloverEnabled bool
	categoryRolloverMonth   string
	categoryRolloverBalance float64
	categoryRolloverType    string
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

var categoriesUpdateCmd = &cobra.Command{
	Use:   "update <category-id>",
	Short: "Update a category",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		runMutation(cmd, "categories.update", "failed to update category", safety.TierMutation, func() (mutation, *errors.Error) {
			opts := monarch.UpdateCategoryOptions{}
			if cmd.Flags().Changed("name") {
				opts.Name = &categoryName
			}
			if cmd.Flags().Changed("icon") {
				opts.Icon = &categoryIcon
			}
			if cmd.Flags().Changed("budget-variability") {
				opts.BudgetVariability = &categoryBudgetVar
			}
			if cmd.Flags().Changed("exclude-from-budget") {
				opts.ExcludeFromBudget = &categoryExcludeBudget
			}
			var cat *monarch.Category
			return mutation{
				resourceID: id,
				planAfter:  map[string]any{"name": categoryName, "icon": categoryIcon},
				do: func(ctx context.Context, svc *monarch.Service) (any, error) {
					c, err := svc.UpdateCategory(ctx, id, opts)
					if err != nil {
						return nil, err
					}
					cat = c
					return c, nil
				},
				human: func() { fmt.Printf("Successfully updated category %s (%s).\n", cat.Name, cat.ID) },
			}, nil
		})
	},
}

var categoriesRolloverCmd = &cobra.Command{
	Use:   "rollover <category-id>",
	Short: "Show rollover settings for a category",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		run(cmd.Context(), "categories.rollover", "failed to get category rollover",
			func(ctx context.Context, svc *monarch.Service) (*monarch.CategoryRollover, error) {
				return svc.GetCategoryRollover(ctx, id)
			},
			func(r *monarch.CategoryRollover) {
				fmt.Printf("Category:  %s\n", r.Name)
				if r.StartMonth == "" {
					fmt.Println("Rollover:  not enabled")
					return
				}
				fmt.Printf("Start Month:      %s\n", r.StartMonth)
				fmt.Printf("Starting Balance: %.2f\n", r.StartingBalance)
				fmt.Printf("Type:             %s\n", r.Type)
				fmt.Printf("Frequency:        %s\n", r.Frequency)
				if r.TargetAmount > 0 {
					fmt.Printf("Target Amount:    %.2f\n", r.TargetAmount)
				}
			})
	},
}

var categoriesGroupUpdateCmd = &cobra.Command{
	Use:   "groups update <group-id>",
	Short: "Update a category group",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		runMutation(cmd, "categories.groups.update", "failed to update category group", safety.TierMutation, func() (mutation, *errors.Error) {
			opts := monarch.UpdateCategoryGroupOptions{}
			if cmd.Flags().Changed("name") {
				opts.Name = &categoryGroupName
			}
			if cmd.Flags().Changed("budget-variability") {
				opts.BudgetVariability = &categoryBudgetVar
			}
			if cmd.Flags().Changed("rollover-enabled") {
				opts.RolloverEnabled = &categoryRolloverEnabled
			}
			if cmd.Flags().Changed("rollover-month") {
				opts.RolloverStartMonth = &categoryRolloverMonth
			}
			if cmd.Flags().Changed("rollover-balance") {
				opts.RolloverStartingBalance = &categoryRolloverBalance
			}
			if cmd.Flags().Changed("rollover-type") {
				opts.RolloverType = &categoryRolloverType
			}
			var group *monarch.CategoryGroup
			return mutation{
				resourceID: id,
				planAfter:  map[string]any{"name": categoryGroupName},
				do: func(ctx context.Context, svc *monarch.Service) (any, error) {
					g, err := svc.UpdateCategoryGroup(ctx, id, opts)
					if err != nil {
						return nil, err
					}
					group = g
					return g, nil
				},
				human: func() { fmt.Printf("Successfully updated category group %s (%s).\n", group.Name, group.ID) },
			}, nil
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

	categoriesUpdateCmd.Flags().StringVar(&categoryName, "name", "", "new category name")
	categoriesUpdateCmd.Flags().StringVar(&categoryIcon, "icon", "", "category icon")
	categoriesUpdateCmd.Flags().StringVar(&categoryBudgetVar, "budget-variability", "", "budget variability (fixed or flexible)")
	categoriesUpdateCmd.Flags().BoolVar(&categoryExcludeBudget, "exclude-from-budget", false, "exclude from budget")

	categoriesGroupUpdateCmd.Flags().StringVar(&categoryGroupName, "name", "", "new group name")
	categoriesGroupUpdateCmd.Flags().StringVar(&categoryBudgetVar, "budget-variability", "", "budget variability (fixed or flexible)")
	categoriesGroupUpdateCmd.Flags().BoolVar(&categoryRolloverEnabled, "rollover-enabled", false, "enable rollover")
	categoriesGroupUpdateCmd.Flags().StringVar(&categoryRolloverMonth, "rollover-month", "", "rollover start month (YYYY-MM-DD)")
	categoriesGroupUpdateCmd.Flags().Float64Var(&categoryRolloverBalance, "rollover-balance", 0, "rollover starting balance")
	categoriesGroupUpdateCmd.Flags().StringVar(&categoryRolloverType, "rollover-type", "", "rollover type (e.g., monthly)")

	categoriesCmd.AddCommand(categoriesListCmd)
	categoriesCmd.AddCommand(categoriesGroupsCmd)
	categoriesCmd.AddCommand(categoriesCreateCmd)
	categoriesCmd.AddCommand(categoriesUpdateCmd)
	categoriesCmd.AddCommand(categoriesRolloverCmd)
	categoriesCmd.AddCommand(categoriesDeleteCmd)
	categoriesCmd.AddCommand(categoriesDeleteManyCmd)
	categoriesGroupsCmd.AddCommand(categoriesGroupUpdateCmd)
	RootCmd.AddCommand(categoriesCmd)
}
