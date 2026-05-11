package cli

import (
	"bufio"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/thedavidweng/monarchmoney-cli/internal/audit"
	"github.com/thedavidweng/monarchmoney-cli/internal/errors"
	"github.com/thedavidweng/monarchmoney-cli/internal/output"
	"github.com/thedavidweng/monarchmoney-cli/internal/safety"
)

func (a *App) buildCategoriesCommands(parent *cobra.Command) {
	categoriesCmd := &cobra.Command{
		Use:   "categories",
		Short: "Manage Monarch Money categories",
	}
	categoriesCmd.AddCommand(a.buildCategoriesList())
	categoriesCmd.AddCommand(a.buildCategoriesGroups())
	categoriesCmd.AddCommand(a.buildCategoriesCreate())
	categoriesCmd.AddCommand(a.buildCategoriesDelete())
	categoriesCmd.AddCommand(a.buildCategoriesDeleteMany())
	parent.AddCommand(categoriesCmd)
}

func (a *App) buildCategoriesList() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all categories",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, a.Flags.JSONMode, a.Flags.Pretty)

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "categories.list", err.(*errors.Error), start)
				return
			}

			cats, err := svc.ListCategories(cmd.Context())
			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to list categories", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "categories.list", cliErr, start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("categories.list", a.Flags.Profile, output.SchemaVersion, "", cats, time.Since(start))
				a.renderSuccess(renderer, env, start)
			} else {
				writeText(a.Deps.Stdout, "%-20s %-30s %s\n", "ID", "NAME", "GROUP")
				for _, c := range cats {
					writeText(a.Deps.Stdout, "%-20s %-30s %s\n", c.ID, c.Name, c.GroupName)
				}
			}
		},
	}
}

func (a *App) buildCategoriesGroups() *cobra.Command {
	return &cobra.Command{
		Use:   "groups",
		Short: "List all category groups",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, a.Flags.JSONMode, a.Flags.Pretty)

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "categories.groups", err.(*errors.Error), start)
				return
			}

			groups, err := svc.ListCategoryGroups(cmd.Context())
			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to list category groups", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "categories.groups", cliErr, start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("categories.groups", a.Flags.Profile, output.SchemaVersion, "", groups, time.Since(start))
				a.renderSuccess(renderer, env, start)
			} else {
				writeText(a.Deps.Stdout, "%-20s %-30s %s\n", "ID", "NAME", "TYPE")
				for _, g := range groups {
					writeText(a.Deps.Stdout, "%-20s %-30s %s\n", g.ID, g.Name, g.Type)
				}
			}
		},
	}
}

func (a *App) buildCategoriesCreate() *cobra.Command {
	var (
		categoryName    string
		categoryGroupID string
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a category",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, a.Flags.JSONMode, a.Flags.Pretty)
			logger := a.Deps.NewAuditLogger()

			if err := safety.Check(safety.TierMutation, a.Flags.ReadOnly, a.Flags.DryRun, a.Flags.Confirm); err != nil {
				a.handleError(renderer, "categories.create", err.(*errors.Error), start)
				return
			}

			if a.Flags.DryRun {
				plan := safety.NewPlan()
				plan.Add("categories.create", "", nil, map[string]string{"name": categoryName, "groupId": categoryGroupID})
				env := output.NewEnvelope("categories.create", a.Flags.Profile, output.SchemaVersion, "", plan, time.Since(start))
				a.renderSuccess(renderer, env, start)
				return
			}

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "categories.create", err.(*errors.Error), start)
				return
			}

			cat, err := svc.CreateCategory(cmd.Context(), categoryName, categoryGroupID)
			result := "success"
			var errCode string
			if err != nil {
				result = "failure"
				if e, ok := err.(*errors.Error); ok {
					errCode = string(e.Code)
				}
			}

			a.logAudit(logger, &audit.Record{
				Command:   "categories.create",
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
					cliErr = errors.New(errors.APIError, "failed to create category", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "categories.create", cliErr, start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("categories.create", a.Flags.Profile, output.SchemaVersion, "", cat, time.Since(start))
				a.renderSuccess(renderer, env, start)
			} else {
				writeText(a.Deps.Stdout, "Successfully created category %s (%s).\n", cat.Name, cat.ID)
			}
		},
	}
	cmd.Flags().StringVar(&categoryName, "name", "", "category name")
	cmd.Flags().StringVar(&categoryGroupID, "group", "", "category group ID")
	mustMarkFlagRequired(cmd, "name")
	mustMarkFlagRequired(cmd, "group")
	return cmd
}

func (a *App) buildCategoriesDelete() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <category-id>",
		Short: "Delete a category",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, a.Flags.JSONMode, a.Flags.Pretty)
			logger := a.Deps.NewAuditLogger()
			id := args[0]

			if err := safety.Check(safety.TierDestructive, a.Flags.ReadOnly, a.Flags.DryRun, a.Flags.Confirm); err != nil {
				a.handleError(renderer, "categories.delete", err.(*errors.Error), start)
				return
			}

			if a.Flags.DryRun {
				plan := safety.NewPlan()
				plan.Add("categories.delete", id, nil, nil)
				env := output.NewEnvelope("categories.delete", a.Flags.Profile, output.SchemaVersion, "", plan, time.Since(start))
				a.renderSuccess(renderer, env, start)
				return
			}

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "categories.delete", err.(*errors.Error), start)
				return
			}

			err = svc.DeleteCategory(cmd.Context(), id)
			result := "success"
			var errCode string
			if err != nil {
				result = "failure"
				if e, ok := err.(*errors.Error); ok {
					errCode = string(e.Code)
				}
			}

			a.logAudit(logger, &audit.Record{
				Command:    "categories.delete",
				ResourceID: id,
				DryRun:     a.Flags.DryRun,
				Confirmed:  a.Flags.Confirm,
				Profile:    a.Flags.Profile,
				Result:     result,
				ErrorCode:  errCode,
			})

			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to delete category", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "categories.delete", cliErr, start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("categories.delete", a.Flags.Profile, output.SchemaVersion, "", map[string]string{"status": "deleted"}, time.Since(start))
				a.renderSuccess(renderer, env, start)
			} else {
				writeText(a.Deps.Stdout, "Successfully deleted category %s.\n", id)
			}
		},
	}
}

func (a *App) buildCategoriesDeleteMany() *cobra.Command {
	var categoryFile string
	cmd := &cobra.Command{
		Use:   "delete-many",
		Short: "Delete multiple categories from a file",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, a.Flags.JSONMode, a.Flags.Pretty)
			logger := a.Deps.NewAuditLogger()

			if err := safety.Check(safety.TierDestructive, a.Flags.ReadOnly, a.Flags.DryRun, a.Flags.Confirm); err != nil {
				a.handleError(renderer, "categories.delete-many", err.(*errors.Error), start)
				return
			}

			if categoryFile == "" {
				a.handleError(renderer, "categories.delete-many", errors.New(errors.InvalidArguments, "--file is required", errors.CatValidation, false, nil), start)
				return
			}

			f, err := os.Open(categoryFile)
			if err != nil {
				a.handleError(renderer, "categories.delete-many", errors.New(errors.InternalError, "failed to open file", errors.CatInternal, false, err), start)
				return
			}
			defer func() {
				_ = f.Close()
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
				plan.Add("categories.delete-many", "", nil, map[string]interface{}{"ids": ids})
				env := output.NewEnvelope("categories.delete-many", a.Flags.Profile, output.SchemaVersion, "", plan, time.Since(start))
				a.renderSuccess(renderer, env, start)
				return
			}

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "categories.delete-many", err.(*errors.Error), start)
				return
			}

			err = svc.DeleteCategories(cmd.Context(), ids)
			result := "success"
			var errCode string
			if err != nil {
				result = "failure"
				if e, ok := err.(*errors.Error); ok {
					errCode = string(e.Code)
				}
			}

			a.logAudit(logger, &audit.Record{
				Command:   "categories.delete-many",
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
					cliErr = errors.New(errors.APIError, "failed to delete categories", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "categories.delete-many", cliErr, start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("categories.delete-many", a.Flags.Profile, output.SchemaVersion, "", map[string]string{"status": "categories deleted"}, time.Since(start))
				a.renderSuccess(renderer, env, start)
			} else {
				writeText(a.Deps.Stdout, "Successfully deleted %d categories.\n", len(ids))
			}
		},
	}
	cmd.Flags().StringVar(&categoryFile, "file", "", "file with category IDs (one per line)")
	mustMarkFlagRequired(cmd, "file")
	return cmd
}
