package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/thedavidweng/monarchmoney-cli/internal/monarch"
	"github.com/thedavidweng/monarchmoney-cli/internal/output"
	"github.com/thedavidweng/monarchmoney-cli/internal/safety"
)

type appRuleFlags struct {
	merchantOperator string
	merchantValue    string
	amountOperator   string
	amountValue      float64
	amountIsExpense  bool
	setCategoryID    string
	addTagIDs        []string
	accountIDs       []string
	applyToExisting  bool
}

func (f *appRuleFlags) bind(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.merchantOperator, "merchant-operator", "", "merchant match operator (eq, contains)")
	cmd.Flags().StringVar(&f.merchantValue, "merchant-value", "", "merchant name/pattern to match")
	cmd.Flags().StringVar(&f.amountOperator, "amount-operator", "", "amount comparison (gt, lt, eq, between)")
	cmd.Flags().Float64Var(&f.amountValue, "amount-value", 0, "amount threshold value")
	cmd.Flags().BoolVar(&f.amountIsExpense, "amount-is-expense", true, "whether amount is expense")
	cmd.Flags().StringVar(&f.setCategoryID, "set-category-id", "", "category ID to assign")
	cmd.Flags().StringSliceVar(&f.addTagIDs, "add-tag-id", nil, "tag IDs to add (repeatable)")
	cmd.Flags().StringSliceVar(&f.accountIDs, "account-id", nil, "limit rule to account IDs (repeatable)")
	cmd.Flags().BoolVar(&f.applyToExisting, "apply-to-existing", false, "apply rule to existing transactions")
}

func (f *appRuleFlags) createInput(cmd *cobra.Command) monarch.CreateRuleInput {
	input := monarch.CreateRuleInput{
		MerchantOperator: f.merchantOperator,
		MerchantValue:    f.merchantValue,
		AmountOperator:   f.amountOperator,
		AmountIsExpense:  f.amountIsExpense,
		SetCategoryID:    f.setCategoryID,
		AddTagIDs:        f.addTagIDs,
		AccountIDs:       f.accountIDs,
		ApplyToExisting:  f.applyToExisting,
	}
	if cmd.Flags().Changed("amount-value") {
		input.AmountValue = &f.amountValue
	}
	return input
}

func (f *appRuleFlags) updateInput(cmd *cobra.Command, id string) monarch.UpdateRuleInput {
	input := monarch.UpdateRuleInput{
		ID:               id,
		MerchantOperator: f.merchantOperator,
		MerchantValue:    f.merchantValue,
		AmountOperator:   f.amountOperator,
		AmountIsExpense:  f.amountIsExpense,
		SetCategoryID:    f.setCategoryID,
		AddTagIDs:        f.addTagIDs,
		AccountIDs:       f.accountIDs,
		ApplyToExisting:  f.applyToExisting,
	}
	if cmd.Flags().Changed("amount-value") {
		input.AmountValue = &f.amountValue
	}
	return input
}

func (a *App) buildRulesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "rules",
		Short:   "Manage transaction auto-categorization rules",
		GroupID: "core",
		Example: "  monarch rules list --json\n  monarch rules create --trigger-value \"Uber\" --category-id <id> --confirm",
	}
	cmd.AddCommand(a.buildRulesListCommand())
	cmd.AddCommand(a.buildRulesCreateCommand())
	cmd.AddCommand(a.buildRulesUpdateCommand())
	cmd.AddCommand(a.buildRulesDeleteCommand())
	return cmd
}

func (a *App) buildRulesListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all transaction rules",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)

			svc, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "rules.list", wrapError(err, "failed to load service"), start)
				return
			}

			rules, err := svc.ListRules(cmd.Context())
			if err != nil {
				a.handleError(renderer, "rules.list", wrapError(err, "failed to list rules"), start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("rules.list", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, rules, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%-36s %-12s %-20s %s\n", "ID", "OPERATOR", "MATCH", "ACTION")
			for i := range rules {
				r := &rules[i]
				match := ""
				if len(r.MerchantNameCriteria) > 0 {
					match = r.MerchantNameCriteria[0].Value
				}
				action := ""
				if r.SetCategoryAction != nil {
					action = "→ " + r.SetCategoryAction.Name
				}
				operator := ""
				if len(r.MerchantNameCriteria) > 0 {
					operator = r.MerchantNameCriteria[0].Operator
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-36s %-12s %-20s %s\n", r.ID, operator, match, action)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nTotal rules: %d\n", len(rules))
		},
	}
}

func (a *App) buildRulesCreateCommand() *cobra.Command {
	var flags appRuleFlags

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a transaction rule",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)

			if !a.checkSafety(renderer, "rules.create", safety.TierMutation, start) {
				return
			}

			input := flags.createInput(cmd)
			if a.Flags.DryRun {
				plan := safety.NewPlan()
				plan.Add("rules.create", "", nil, map[string]any{"input": input})
				a.renderPlan(renderer, "rules.create", plan, start)
				return
			}

			svc, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "rules.create", wrapError(err, "failed to load service"), start)
				return
			}

			if _, err := a.mutate(renderer, "rules.create", "", start, func() (any, error) {
				return nil, svc.CreateRule(cmd.Context(), &input)
			}, "failed to create rule"); err != nil {
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("rules.create", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, map[string]string{"status": "created"}, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Successfully created rule.")
		},
	}
	flags.bind(cmd)
	return cmd
}

func (a *App) buildRulesUpdateCommand() *cobra.Command {
	var flags appRuleFlags

	cmd := &cobra.Command{
		Use:   "update <rule-id>",
		Short: "Update a transaction rule",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)
			id := args[0]

			if !a.checkSafety(renderer, "rules.update", safety.TierMutation, start) {
				return
			}

			input := flags.updateInput(cmd, id)
			if a.Flags.DryRun {
				plan := safety.NewPlan()
				plan.Add("rules.update", id, nil, map[string]any{"input": input})
				a.renderPlan(renderer, "rules.update", plan, start)
				return
			}

			svc, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "rules.update", wrapError(err, "failed to load service"), start)
				return
			}

			if _, err := a.mutate(renderer, "rules.update", id, start, func() (any, error) {
				return nil, svc.UpdateRule(cmd.Context(), &input)
			}, "failed to update rule"); err != nil {
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("rules.update", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, map[string]string{"status": "updated"}, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully updated rule %s.\n", id)
		},
	}
	flags.bind(cmd)
	return cmd
}

func (a *App) buildRulesDeleteCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <rule-id>",
		Short: "Delete a transaction rule",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)
			id := args[0]

			if !a.checkSafety(renderer, "rules.delete", safety.TierDestructive, start) {
				return
			}

			if a.Flags.DryRun {
				plan := safety.NewPlan()
				plan.Add("rules.delete", id, nil, nil)
				a.renderPlan(renderer, "rules.delete", plan, start)
				return
			}

			svc, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "rules.delete", wrapError(err, "failed to load service"), start)
				return
			}

			if _, err := a.mutate(renderer, "rules.delete", id, start, func() (any, error) {
				return nil, svc.DeleteRule(cmd.Context(), id)
			}, "failed to delete rule"); err != nil {
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("rules.delete", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, map[string]string{"status": "deleted"}, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully deleted rule %s.\n", id)
		},
	}
}
