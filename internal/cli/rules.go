package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/thedavidweng/monarchmoney-cli/internal/audit"
	"github.com/thedavidweng/monarchmoney-cli/internal/errors"
	"github.com/thedavidweng/monarchmoney-cli/internal/monarch"
	"github.com/thedavidweng/monarchmoney-cli/internal/output"
	"github.com/thedavidweng/monarchmoney-cli/internal/safety"
)

// ruleFlags binds the rule-creation/update flag set onto a command. Reused by
// both create and update.
type ruleFlags struct {
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

func (rf *ruleFlags) bind(cmd *cobra.Command) {
	cmd.Flags().StringVar(&rf.merchantOperator, "merchant-operator", "", "merchant match operator (eq, contains)")
	cmd.Flags().StringVar(&rf.merchantValue, "merchant-value", "", "merchant name/pattern to match")
	cmd.Flags().StringVar(&rf.amountOperator, "amount-operator", "", "amount comparison (gt, lt, eq, between)")
	cmd.Flags().Float64Var(&rf.amountValue, "amount-value", 0, "amount threshold value")
	cmd.Flags().BoolVar(&rf.amountIsExpense, "amount-is-expense", true, "whether amount is expense")
	cmd.Flags().StringVar(&rf.setCategoryID, "set-category-id", "", "category ID to assign")
	cmd.Flags().StringSliceVar(&rf.addTagIDs, "add-tag-id", nil, "tag IDs to add (repeatable)")
	cmd.Flags().StringSliceVar(&rf.accountIDs, "account-id", nil, "limit rule to account IDs (repeatable)")
	cmd.Flags().BoolVar(&rf.applyToExisting, "apply-to-existing", false, "apply rule to existing transactions")
}

func (a *App) buildRulesCommands(parent *cobra.Command) {
	rulesCmd := &cobra.Command{
		Use:   "rules",
		Short: "Manage transaction auto-categorization rules",
	}
	rulesCmd.AddCommand(a.buildRulesList())
	rulesCmd.AddCommand(a.buildRulesCreate())
	rulesCmd.AddCommand(a.buildRulesUpdate())
	rulesCmd.AddCommand(a.buildRulesDelete())
	parent.AddCommand(rulesCmd)
}

func (a *App) buildRulesList() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all transaction rules",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, jsonMode, pretty)

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "rules.list", err.(*errors.Error), start)
				return
			}

			rules, err := svc.ListRules(cmd.Context())
			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to list rules", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "rules.list", cliErr, start)
				return
			}

			if jsonMode {
				env := output.NewEnvelope("rules.list", profile, output.SchemaVersion, "", rules, time.Since(start))
				renderer.RenderSuccess(env)
			} else {
				fmt.Printf("%-36s %-12s %-20s %s\n", "ID", "OPERATOR", "MATCH", "ACTION")
				for _, r := range rules {
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
					fmt.Printf("%-36s %-12s %-20s %s\n", r.ID, operator, match, action)
				}
				fmt.Printf("\nTotal rules: %d\n", len(rules))
			}
		},
	}
}

func (a *App) buildRulesCreate() *cobra.Command {
	rf := &ruleFlags{}
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a transaction rule",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, jsonMode, pretty)
			logger := audit.NewLogger()

			if err := safety.Check(safety.TierMutation, readOnly, dryRun, confirm); err != nil {
				a.handleError(renderer, "rules.create", err.(*errors.Error), start)
				return
			}

			input := monarch.CreateRuleInput{
				MerchantOperator: rf.merchantOperator,
				MerchantValue:    rf.merchantValue,
				AmountOperator:   rf.amountOperator,
				AmountIsExpense:  rf.amountIsExpense,
				SetCategoryID:    rf.setCategoryID,
				AddTagIDs:        rf.addTagIDs,
				AccountIDs:       rf.accountIDs,
				ApplyToExisting:  rf.applyToExisting,
			}
			if cmd.Flags().Changed("amount-value") {
				input.AmountValue = &rf.amountValue
			}

			if dryRun {
				plan := safety.NewPlan()
				plan.Add("rules.create", "", nil, map[string]interface{}{"input": input})
				env := output.NewEnvelope("rules.create", profile, output.SchemaVersion, "", plan, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "rules.create", err.(*errors.Error), start)
				return
			}

			err = svc.CreateRule(cmd.Context(), input)
			result := "success"
			var errCode string
			if err != nil {
				result = "failure"
				if e, ok := err.(*errors.Error); ok {
					errCode = string(e.Code)
				}
			}
			logger.Log(&audit.Record{Command: "rules.create", DryRun: dryRun, Confirmed: confirm, Profile: profile, Result: result, ErrorCode: errCode})

			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to create rule", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "rules.create", cliErr, start)
				return
			}

			if jsonMode {
				env := output.NewEnvelope("rules.create", profile, output.SchemaVersion, "", map[string]string{"status": "created"}, time.Since(start))
				renderer.RenderSuccess(env)
			} else {
				fmt.Println("Successfully created rule.")
			}
		},
	}
	rf.bind(cmd)
	return cmd
}

func (a *App) buildRulesUpdate() *cobra.Command {
	rf := &ruleFlags{}
	cmd := &cobra.Command{
		Use:   "update <rule-id>",
		Short: "Update a transaction rule",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, jsonMode, pretty)
			logger := audit.NewLogger()
			id := args[0]

			if err := safety.Check(safety.TierMutation, readOnly, dryRun, confirm); err != nil {
				a.handleError(renderer, "rules.update", err.(*errors.Error), start)
				return
			}

			input := monarch.UpdateRuleInput{
				ID:               id,
				MerchantOperator: rf.merchantOperator,
				MerchantValue:    rf.merchantValue,
				AmountOperator:   rf.amountOperator,
				AmountIsExpense:  rf.amountIsExpense,
				SetCategoryID:    rf.setCategoryID,
				AddTagIDs:        rf.addTagIDs,
				AccountIDs:       rf.accountIDs,
				ApplyToExisting:  rf.applyToExisting,
			}
			if cmd.Flags().Changed("amount-value") {
				input.AmountValue = &rf.amountValue
			}

			if dryRun {
				plan := safety.NewPlan()
				plan.Add("rules.update", id, nil, map[string]interface{}{"input": input})
				env := output.NewEnvelope("rules.update", profile, output.SchemaVersion, "", plan, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "rules.update", err.(*errors.Error), start)
				return
			}

			err = svc.UpdateRule(cmd.Context(), input)
			result := "success"
			var errCode string
			if err != nil {
				result = "failure"
				if e, ok := err.(*errors.Error); ok {
					errCode = string(e.Code)
				}
			}
			logger.Log(&audit.Record{Command: "rules.update", ResourceID: id, DryRun: dryRun, Confirmed: confirm, Profile: profile, Result: result, ErrorCode: errCode})

			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to update rule", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "rules.update", cliErr, start)
				return
			}

			if jsonMode {
				env := output.NewEnvelope("rules.update", profile, output.SchemaVersion, "", map[string]string{"status": "updated"}, time.Since(start))
				renderer.RenderSuccess(env)
			} else {
				fmt.Printf("Successfully updated rule %s.\n", id)
			}
		},
	}
	rf.bind(cmd)
	return cmd
}

func (a *App) buildRulesDelete() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <rule-id>",
		Short: "Delete a transaction rule",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, jsonMode, pretty)
			logger := audit.NewLogger()
			id := args[0]

			if err := safety.Check(safety.TierDestructive, readOnly, dryRun, confirm); err != nil {
				a.handleError(renderer, "rules.delete", err.(*errors.Error), start)
				return
			}

			if dryRun {
				plan := safety.NewPlan()
				plan.Add("rules.delete", id, nil, nil)
				env := output.NewEnvelope("rules.delete", profile, output.SchemaVersion, "", plan, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "rules.delete", err.(*errors.Error), start)
				return
			}

			err = svc.DeleteRule(cmd.Context(), id)
			result := "success"
			var errCode string
			if err != nil {
				result = "failure"
				if e, ok := err.(*errors.Error); ok {
					errCode = string(e.Code)
				}
			}
			logger.Log(&audit.Record{Command: "rules.delete", ResourceID: id, DryRun: dryRun, Confirmed: confirm, Profile: profile, Result: result, ErrorCode: errCode})

			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to delete rule", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "rules.delete", cliErr, start)
				return
			}

			if jsonMode {
				env := output.NewEnvelope("rules.delete", profile, output.SchemaVersion, "", map[string]string{"status": "deleted"}, time.Since(start))
				renderer.RenderSuccess(env)
			} else {
				fmt.Printf("Successfully deleted rule %s.\n", id)
			}
		},
	}
}
