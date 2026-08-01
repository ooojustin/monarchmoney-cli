package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/thedavidweng/monarchmoney-cli/internal/errors"
	"github.com/thedavidweng/monarchmoney-cli/internal/monarch"
	"github.com/thedavidweng/monarchmoney-cli/internal/output"
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

func (a *App) buildCategoriesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "categories",
		Short:   "Manage Monarch Money categories",
		GroupID: "core",
		Example: "  monarch categories list --json\n  monarch categories create --name \"Coffee\" --group <group-id> --confirm",
	}
	cmd.AddCommand(a.buildCategoriesListCommand())
	cmd.AddCommand(a.buildCategoriesGroupsCommand())
	cmd.AddCommand(a.buildCategoriesCreateCommand())
	cmd.AddCommand(a.buildCategoriesUpdateCommand())
	cmd.AddCommand(a.buildCategoriesRolloverCommand())
	cmd.AddCommand(a.buildCategoriesDeleteCommand())
	cmd.AddCommand(a.buildCategoriesDeleteManyCommand())
	return cmd
}

func (a *App) buildCategoriesListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all categories",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)

			svc, _, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "categories.list", wrapError(err, "failed to load service"), start)
				return
			}

			cats, err := svc.ListCategories(cmd.Context())
			if err != nil {
				a.handleError(renderer, "categories.list", wrapError(err, "failed to list categories"), start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("categories.list", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, cats, time.Since(start))
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-30s %s\n", "ID", "NAME", "GROUP") //nolint:errcheck // best-effort output
			for _, c := range cats {
				fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-30s %s\n", c.ID, c.Name, c.GroupName) //nolint:errcheck // best-effort output
			}
		},
	}
}

func (a *App) buildCategoriesGroupsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "groups",
		Short: "List all category groups",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)

			svc, _, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "categories.groups", wrapError(err, "failed to load service"), start)
				return
			}

			groups, err := svc.ListCategoryGroups(cmd.Context())
			if err != nil {
				a.handleError(renderer, "categories.groups", wrapError(err, "failed to list category groups"), start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("categories.groups", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, groups, time.Since(start))
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-30s %s\n", "ID", "NAME", "TYPE") //nolint:errcheck // best-effort output
			for _, g := range groups {
				fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-30s %s\n", g.ID, g.Name, g.Type) //nolint:errcheck // best-effort output
			}
		},
	}
	cmd.AddCommand(a.buildCategoriesGroupUpdateCommand())
	return cmd
}

func (a *App) buildCategoriesUpdateCommand() *cobra.Command {
	var (
		name              string
		icon              string
		budgetVariability string
		excludeFromBudget bool
	)
	cmd := &cobra.Command{
		Use:   "update <category-id>",
		Short: "Update a category",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)
			if !a.checkSafety(renderer, "categories.update", safety.TierMutation, start) {
				return
			}
			id := args[0]
			opts := monarch.UpdateCategoryOptions{}
			if cmd.Flags().Changed("name") {
				opts.Name = &name
			}
			if cmd.Flags().Changed("icon") {
				opts.Icon = &icon
			}
			if cmd.Flags().Changed("budget-variability") {
				opts.BudgetVariability = &budgetVariability
			}
			if cmd.Flags().Changed("exclude-from-budget") {
				opts.ExcludeFromBudget = &excludeFromBudget
			}
			if a.Flags.DryRun {
				plan := safety.NewPlan()
				plan.Add("categories.update", id, nil, opts)
				a.renderPlan(renderer, "categories.update", plan, start)
				return
			}
			svc, _, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "categories.update", wrapError(err, "failed to load service"), start)
				return
			}
			result, err := a.mutate(renderer, "categories.update", id, start, func() (any, error) {
				return svc.UpdateCategory(cmd.Context(), id, opts)
			}, "failed to update category")
			if err != nil {
				return
			}
			category := result.(*monarch.Category)
			if a.Flags.JSONMode {
				renderer.RenderSuccess(output.NewEnvelope("categories.update", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, category, time.Since(start)))
				return
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Successfully updated category %s (%s).\n", category.Name, category.ID) //nolint:errcheck
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "new category name")
	cmd.Flags().StringVar(&icon, "icon", "", "category icon")
	cmd.Flags().StringVar(&budgetVariability, "budget-variability", "", "budget variability (fixed or flexible)")
	cmd.Flags().BoolVar(&excludeFromBudget, "exclude-from-budget", false, "exclude from budget")
	return cmd
}

func (a *App) buildCategoriesRolloverCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "rollover <category-id>",
		Short: "Show rollover settings for a category",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)
			svc, _, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "categories.rollover", wrapError(err, "failed to load service"), start)
				return
			}
			rollover, err := svc.GetCategoryRollover(cmd.Context(), args[0])
			if err != nil {
				a.handleError(renderer, "categories.rollover", wrapError(err, "failed to get category rollover"), start)
				return
			}
			if a.Flags.JSONMode {
				renderer.RenderSuccess(output.NewEnvelope("categories.rollover", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, rollover, time.Since(start)))
				return
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Category:  %s\n", rollover.Name) //nolint:errcheck
			if rollover.StartMonth == "" {
				fmt.Fprintln(cmd.OutOrStdout(), "Rollover:  not enabled") //nolint:errcheck
				return
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Start Month:      %s\nStarting Balance: %.2f\nType:             %s\nFrequency:        %s\n", rollover.StartMonth, rollover.StartingBalance, rollover.Type, rollover.Frequency) //nolint:errcheck
			if rollover.TargetAmount > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Target Amount:    %.2f\n", rollover.TargetAmount) //nolint:errcheck
			}
		},
	}
}

func (a *App) buildCategoriesGroupUpdateCommand() *cobra.Command {
	var (
		name              string
		budgetVariability string
		rolloverEnabled   bool
		rolloverMonth     string
		rolloverBalance   float64
		rolloverType      string
	)
	cmd := &cobra.Command{
		Use:   "update <group-id>",
		Short: "Update a category group",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)
			if !a.checkSafety(renderer, "categories.groups.update", safety.TierMutation, start) {
				return
			}
			id := args[0]
			opts := monarch.UpdateCategoryGroupOptions{}
			if cmd.Flags().Changed("name") {
				opts.Name = &name
			}
			if cmd.Flags().Changed("budget-variability") {
				opts.BudgetVariability = &budgetVariability
			}
			if cmd.Flags().Changed("rollover-enabled") {
				opts.RolloverEnabled = &rolloverEnabled
			}
			if cmd.Flags().Changed("rollover-month") {
				opts.RolloverStartMonth = &rolloverMonth
			}
			if cmd.Flags().Changed("rollover-balance") {
				opts.RolloverStartingBalance = &rolloverBalance
			}
			if cmd.Flags().Changed("rollover-type") {
				opts.RolloverType = &rolloverType
			}
			if a.Flags.DryRun {
				plan := safety.NewPlan()
				plan.Add("categories.groups.update", id, nil, opts)
				a.renderPlan(renderer, "categories.groups.update", plan, start)
				return
			}
			svc, _, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "categories.groups.update", wrapError(err, "failed to load service"), start)
				return
			}
			result, err := a.mutate(renderer, "categories.groups.update", id, start, func() (any, error) {
				return svc.UpdateCategoryGroup(cmd.Context(), id, opts)
			}, "failed to update category group")
			if err != nil {
				return
			}
			group := result.(*monarch.CategoryGroup)
			if a.Flags.JSONMode {
				renderer.RenderSuccess(output.NewEnvelope("categories.groups.update", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, group, time.Since(start)))
				return
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Successfully updated category group %s (%s).\n", group.Name, group.ID) //nolint:errcheck
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "new group name")
	cmd.Flags().StringVar(&budgetVariability, "budget-variability", "", "budget variability (fixed or flexible)")
	cmd.Flags().BoolVar(&rolloverEnabled, "rollover-enabled", false, "enable rollover")
	cmd.Flags().StringVar(&rolloverMonth, "rollover-month", "", "rollover start month (YYYY-MM-DD)")
	cmd.Flags().Float64Var(&rolloverBalance, "rollover-balance", 0, "rollover starting balance")
	cmd.Flags().StringVar(&rolloverType, "rollover-type", "", "rollover type (e.g., monthly)")
	return cmd
}

func (a *App) buildCategoriesCreateCommand() *cobra.Command {
	var (
		name    string
		groupID string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a category",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)

			if !a.checkSafety(renderer, "categories.create", safety.TierMutation, start) {
				return
			}

			if a.Flags.DryRun {
				plan := safety.NewPlan()
				plan.Add("categories.create", "", nil, map[string]string{"name": name, "groupId": groupID})
				a.renderPlan(renderer, "categories.create", plan, start)
				return
			}

			svc, _, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "categories.create", wrapError(err, "failed to load service"), start)
				return
			}

			result, err := a.mutate(renderer, "categories.create", "", start, func() (any, error) {
				return svc.CreateCategory(cmd.Context(), name, groupID)
			}, "failed to create category")
			if err != nil {
				return
			}
			cat := result.(*monarch.Category)

			if a.Flags.JSONMode {
				env := output.NewEnvelope("categories.create", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, cat, time.Since(start))
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully created category %s (%s).\n", cat.Name, cat.ID) //nolint:errcheck // best-effort output
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "category name")
	cmd.Flags().StringVar(&groupID, "group", "", "category group ID")
	cmd.MarkFlagRequired("name")  //nolint:errcheck // flag registered above
	cmd.MarkFlagRequired("group") //nolint:errcheck // flag registered above
	return cmd
}

func (a *App) buildCategoriesDeleteCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <category-id>",
		Short: "Delete a category",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)
			id := args[0]

			if !a.checkSafety(renderer, "categories.delete", safety.TierDestructive, start) {
				return
			}

			if a.Flags.DryRun {
				plan := safety.NewPlan()
				plan.Add("categories.delete", id, nil, nil)
				a.renderPlan(renderer, "categories.delete", plan, start)
				return
			}

			svc, _, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "categories.delete", wrapError(err, "failed to load service"), start)
				return
			}

			if _, err := a.mutate(renderer, "categories.delete", id, start, func() (any, error) {
				return nil, svc.DeleteCategory(cmd.Context(), id)
			}, "failed to delete category"); err != nil {
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("categories.delete", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, map[string]string{"status": "deleted"}, time.Since(start))
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully deleted category %s.\n", id) //nolint:errcheck // best-effort output
		},
	}
}

func (a *App) buildCategoriesDeleteManyCommand() *cobra.Command {
	var file string

	cmd := &cobra.Command{
		Use:   "delete-many",
		Short: "Delete multiple categories from a file",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)

			if !a.checkSafety(renderer, "categories.delete-many", safety.TierDestructive, start) {
				return
			}

			if file == "" {
				a.handleError(renderer, "categories.delete-many", errors.New(errors.InvalidArguments, "--file is required", errors.CatValidation, false, nil), start)
				return
			}

			f, err := os.Open(file)
			if err != nil {
				a.handleError(renderer, "categories.delete-many", errors.New(errors.InternalError, "failed to open file", errors.CatInternal, false, err), start)
				return
			}
			defer func() {
				if cerr := f.Close(); cerr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to close file: %v\n", cerr) //nolint:errcheck // best-effort warning
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

			if a.Flags.DryRun {
				plan := safety.NewPlan()
				plan.Add("categories.delete-many", "", nil, map[string]any{"ids": ids})
				a.renderPlan(renderer, "categories.delete-many", plan, start)
				return
			}

			svc, _, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "categories.delete-many", wrapError(err, "failed to load service"), start)
				return
			}

			if _, err := a.mutate(renderer, "categories.delete-many", "", start, func() (any, error) {
				return nil, svc.DeleteCategories(cmd.Context(), ids)
			}, "failed to delete categories"); err != nil {
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("categories.delete-many", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, map[string]string{"status": "categories deleted"}, time.Since(start))
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully deleted %d categories.\n", len(ids)) //nolint:errcheck // best-effort output
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "file with category IDs (one per line)")
	cmd.MarkFlagRequired("file") //nolint:errcheck // flag registered above
	return cmd
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
