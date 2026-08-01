package cli

import (
	"bufio"
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

			svc, err := a.loadService()
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
				renderer.RenderSuccess(env)
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-30s %s\n", "ID", "NAME", "GROUP")
			for _, c := range cats {
				fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-30s %s\n", c.ID, c.Name, c.GroupName)
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

			svc, err := a.loadService()
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
				renderer.RenderSuccess(env)
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-30s %s\n", "ID", "NAME", "TYPE")
			for _, g := range groups {
				fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-30s %s\n", g.ID, g.Name, g.Type)
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
			svc, err := a.loadService()
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
			category, ok := result.(*monarch.Category)
			if !ok || category == nil {
				a.handleError(renderer, "categories.update", errors.New(errors.InternalError, "unexpected category update result", errors.CatInternal, false, nil), start)
				return
			}
			if a.Flags.JSONMode {
				renderer.RenderSuccess(output.NewEnvelope("categories.update", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, category, time.Since(start)))
				return
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Successfully updated category %s (%s).\n", category.Name, category.ID)
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
			svc, err := a.loadService()
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
			fmt.Fprintf(cmd.OutOrStdout(), "Category:  %s\n", rollover.Name)
			if rollover.StartMonth == "" {
				fmt.Fprintln(cmd.OutOrStdout(), "Rollover:  not enabled")
				return
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Start Month:      %s\nStarting Balance: %.2f\nType:             %s\nFrequency:        %s\n", rollover.StartMonth, rollover.StartingBalance, rollover.Type, rollover.Frequency)
			if rollover.TargetAmount > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Target Amount:    %.2f\n", rollover.TargetAmount)
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
			svc, err := a.loadService()
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
			group, ok := result.(*monarch.CategoryGroup)
			if !ok || group == nil {
				a.handleError(renderer, "categories.groups.update", errors.New(errors.InternalError, "unexpected category group update result", errors.CatInternal, false, nil), start)
				return
			}
			if a.Flags.JSONMode {
				renderer.RenderSuccess(output.NewEnvelope("categories.groups.update", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, group, time.Since(start)))
				return
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Successfully updated category group %s (%s).\n", group.Name, group.ID)
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

			svc, err := a.loadService()
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
			cat, ok := result.(*monarch.Category)
			if !ok || cat == nil {
				a.handleError(renderer, "categories.create", errors.New(errors.InternalError, "unexpected category creation result", errors.CatInternal, false, nil), start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("categories.create", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, cat, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully created category %s (%s).\n", cat.Name, cat.ID)
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

			svc, err := a.loadService()
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
				renderer.RenderSuccess(env)
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully deleted category %s.\n", id)
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
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to close file: %v\n", cerr)
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

			svc, err := a.loadService()
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
				renderer.RenderSuccess(env)
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully deleted %d categories.\n", len(ids))
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "file with category IDs (one per line)")
	cmd.MarkFlagRequired("file") //nolint:errcheck // flag registered above
	return cmd
}
