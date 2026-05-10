package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/thedavidweng/monarchmoney-cli/internal/audit"
	"github.com/thedavidweng/monarchmoney-cli/internal/errors"
	"github.com/thedavidweng/monarchmoney-cli/internal/output"
	"github.com/thedavidweng/monarchmoney-cli/internal/safety"
)

func (a *App) buildAccountsCommands(parent *cobra.Command) {
	accountsCmd := &cobra.Command{
		Use:   "accounts",
		Short: "Manage Monarch Money accounts",
	}
	accountsCmd.AddCommand(a.buildAccountsList())
	accountsCmd.AddCommand(a.buildAccountsShow())
	accountsCmd.AddCommand(a.buildAccountsTypes())
	accountsCmd.AddCommand(a.buildAccountsHoldings())
	accountsCmd.AddCommand(a.buildAccountsBalanceAt())
	accountsCmd.AddCommand(a.buildAccountsHistory())
	accountsCmd.AddCommand(a.buildAccountsRefresh())
	accountsCmd.AddCommand(a.buildAccountsRefreshStatus())
	accountsCmd.AddCommand(a.buildAccountsUpdate())
	accountsCmd.AddCommand(a.buildAccountsDelete())
	accountsCmd.AddCommand(a.buildAccountsCreateManual())
	accountsCmd.AddCommand(a.buildAccountsUploadHistory())
	accountsCmd.AddCommand(a.buildAccountsRecentBalances())
	accountsCmd.AddCommand(a.buildAccountsSnapshots())
	accountsCmd.AddCommand(a.buildAccountsAggregateSnapshots())
	parent.AddCommand(accountsCmd)

	parent.AddCommand(a.buildNetworthCommand())
}

func (a *App) buildAccountsList() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all accounts",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, a.Flags.JSONMode, a.Flags.Pretty)

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "accounts.list", err.(*errors.Error), start)
				return
			}

			accounts, err := svc.ListAccounts(cmd.Context())
			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to list accounts", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "accounts.list", cliErr, start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("accounts.list", a.Flags.Profile, output.SchemaVersion, "", accounts, time.Since(start))
				a.renderSuccess(renderer, env, start)
			} else {
				fmt.Printf("%-20s %-15s %-15s %s\n", "ID", "NAME", "TYPE", "BALANCE")
				for _, ac := range accounts {
					fmt.Printf("%-20s %-15s %-15s %.2f\n", ac.ID, ac.DisplayName, ac.AccountType, ac.DisplayBalance)
				}
			}
		},
	}
}

func (a *App) buildAccountsShow() *cobra.Command {
	return &cobra.Command{
		Use:   "show <account-id>",
		Short: "Show detailed information for an account",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, a.Flags.JSONMode, a.Flags.Pretty)

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "accounts.show", err.(*errors.Error), start)
				return
			}

			acc, err := svc.GetAccount(cmd.Context(), args[0])
			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to get account", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "accounts.show", cliErr, start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("accounts.show", a.Flags.Profile, output.SchemaVersion, "", acc, time.Since(start))
				a.renderSuccess(renderer, env, start)
			} else {
				fmt.Printf("ID:       %s\n", acc.ID)
				fmt.Printf("Name:     %s\n", acc.DisplayName)
				fmt.Printf("Type:     %s\n", acc.AccountType)
				fmt.Printf("Balance:  %.2f\n", acc.DisplayBalance)
				fmt.Printf("Updated:  %s\n", acc.UpdatedAt)
			}
		},
	}
}

func (a *App) buildAccountsTypes() *cobra.Command {
	return &cobra.Command{
		Use:   "types",
		Short: "List all available account types",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, a.Flags.JSONMode, a.Flags.Pretty)

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "accounts.types", err.(*errors.Error), start)
				return
			}

			types, err := svc.GetAccountTypes(cmd.Context())
			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to get account types", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "accounts.types", cliErr, start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("accounts.types", a.Flags.Profile, output.SchemaVersion, "", types, time.Since(start))
				a.renderSuccess(renderer, env, start)
			} else {
				for _, t := range types {
					fmt.Println(t)
				}
			}
		},
	}
}

func (a *App) buildAccountsHoldings() *cobra.Command {
	return &cobra.Command{
		Use:   "holdings <account-id>",
		Short: "List holdings for an account",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, a.Flags.JSONMode, a.Flags.Pretty)

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "accounts.holdings", err.(*errors.Error), start)
				return
			}

			holdings, err := svc.GetAccountHoldings(cmd.Context(), args[0])
			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to get holdings", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "accounts.holdings", cliErr, start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("accounts.holdings", a.Flags.Profile, output.SchemaVersion, "", holdings, time.Since(start))
				a.renderSuccess(renderer, env, start)
			} else {
				fmt.Printf("%-20s %12s %12s %12s\n", "ID", "QUANTITY", "BASIS", "TOTAL VALUE")
				for _, h := range holdings {
					fmt.Printf("%-20s %12.2f %12.2f %12.2f\n", h.ID, h.Quantity, h.Basis, h.TotalValue)
				}
			}
		},
	}
}

func (a *App) buildAccountsBalanceAt() *cobra.Command {
	var (
		balanceAtDate string
		accountIDs    []string
	)
	cmd := &cobra.Command{
		Use:   "balance-at",
		Short: "Get account balances at a specific date",
		Long:  "Get display balances for all accounts, or selected accounts, as of a specific date.",
		Example: `  monarch accounts balance-at --date 2026-05-10
  monarch accounts balance-at --date 2026-05-10 --account-id acc_123 --account-id acc_456 --json --pretty`,
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, a.Flags.JSONMode, a.Flags.Pretty)

			if _, err := time.Parse("2006-01-02", balanceAtDate); err != nil {
				a.handleError(renderer, "accounts.balance-at", errors.New(errors.InvalidArguments, "date must use YYYY-MM-DD", errors.CatValidation, false, err), start)
				return
			}

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "accounts.balance-at", err.(*errors.Error), start)
				return
			}

			balances, err := svc.GetAccountBalancesAt(cmd.Context(), balanceAtDate, accountIDs)
			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to get account balances", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "accounts.balance-at", cliErr, start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("accounts.balance-at", a.Flags.Profile, output.SchemaVersion, "", balances, time.Since(start))
				a.renderSuccess(renderer, env, start)
			} else {
				fmt.Printf("%-20s %-30s %-15s %12s\n", "ID", "NAME", "TYPE", "BALANCE")
				for _, balance := range balances {
					fmt.Printf("%-20s %-30s %-15s %12.2f\n", balance.ID, balance.DisplayName, balance.AccountType, balance.DisplayBalance)
				}
			}
		},
	}
	cmd.Flags().StringVar(&balanceAtDate, "date", "", "balance date (YYYY-MM-DD)")
	cmd.Flags().StringSliceVar(&accountIDs, "account-id", nil, "account id to include (repeatable)")
	mustMarkFlagRequired(cmd, "date")
	return cmd
}

func (a *App) buildAccountsHistory() *cobra.Command {
	var (
		historyFrom string
		historyTo   string
	)
	cmd := &cobra.Command{
		Use:   "history <account-id>",
		Short: "Get balance history for an account",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, a.Flags.JSONMode, a.Flags.Pretty)

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "accounts.history", err.(*errors.Error), start)
				return
			}

			history, err := svc.GetAccountHistory(cmd.Context(), args[0], historyFrom, historyTo)
			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to get history", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "accounts.history", cliErr, start)
				return
			}

			if a.Flags.JSONMode {
				env := a.envelopeWithWarnings("accounts.history", history, start, "uses aggregateSnapshots for account history; per-account snapshots are not currently available")
				a.renderSuccess(renderer, env, start)
			} else {
				fmt.Printf("%-12s %10s\n", "DATE", "AMOUNT")
				for _, r := range history {
					fmt.Printf("%-12s %10.2f\n", r.Date, r.Amount)
				}
			}
		},
	}
	cmd.Flags().StringVar(&historyFrom, "from", "", "start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&historyTo, "to", "", "end date (YYYY-MM-DD)")
	return cmd
}

func (a *App) buildAccountsRefresh() *cobra.Command {
	var (
		refreshWait bool
		emitEvents  bool
	)
	cmd := &cobra.Command{
		Use:   "refresh [account-id...]",
		Short: "Request a refresh of all accounts (or specific ones)",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, a.Flags.JSONMode, a.Flags.Pretty)
			logger := a.Deps.NewAuditLogger()

			if err := safety.Check(safety.TierRemoteAction, a.Flags.ReadOnly, a.Flags.DryRun, a.Flags.Confirm); err != nil {
				a.handleError(renderer, "accounts.refresh", err.(*errors.Error), start)
				return
			}

			if a.Flags.DryRun {
				plan := safety.NewPlan()
				plan.Add("accounts.refresh", "", nil, map[string]interface{}{"account_ids": args})
				env := output.NewEnvelope("accounts.refresh", a.Flags.Profile, output.SchemaVersion, "", plan, time.Since(start))
				a.renderSuccess(renderer, env, start)
				return
			}

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "accounts.refresh", err.(*errors.Error), start)
				return
			}

			err = svc.RefreshAccounts(cmd.Context(), args)
			result := "success"
			var errCode string
			if err != nil {
				result = "failure"
				if e, ok := err.(*errors.Error); ok {
					errCode = string(e.Code)
				}
			}

			a.logAudit(logger, &audit.Record{Command: "accounts.refresh", DryRun: a.Flags.DryRun, Confirmed: a.Flags.Confirm, Profile: a.Flags.Profile, Result: result, ErrorCode: errCode})

			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to refresh accounts", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "accounts.refresh", cliErr, start)
				return
			}

			if refreshWait {
				renderer.PrintDiagnostic("Waiting for refresh to complete...")
				ticker := time.NewTicker(2 * time.Second)
				defer ticker.Stop()

				for {
					select {
					case <-cmd.Context().Done():
						a.handleError(renderer, "accounts.refresh", errors.New(errors.InternalError, "context cancelled", errors.CatInternal, false, cmd.Context().Err()), start)
						return
					case <-ticker.C:
						status, err := svc.GetAccountsRefreshStatus(cmd.Context())
						if err != nil {
							renderer.PrintDiagnostic(fmt.Sprintf("Warning: failed to check refresh status: %v", err))
							continue
						}

						if emitEvents {
							env := output.NewEnvelope("accounts.refresh.progress", a.Flags.Profile, output.SchemaVersion, "", status, time.Since(start))
							a.renderSuccess(renderer, env, start)
						}

						if complete, ok := status["is_complete"].(bool); ok && complete {
							goto complete
						}
					}
				}
			}

		complete:
			if a.Flags.JSONMode {
				var status string
				if refreshWait {
					status = "refresh complete"
				} else {
					status = "refresh requested"
				}
				env := output.NewEnvelope("accounts.refresh", a.Flags.Profile, output.SchemaVersion, "", map[string]string{"status": status}, time.Since(start))
				a.renderSuccess(renderer, env, start)
			} else {
				if refreshWait {
					fmt.Println("Refresh complete.")
				} else {
					fmt.Println("Refresh requested successfully.")
				}
			}
		},
	}
	cmd.Flags().BoolVar(&refreshWait, "wait", false, "wait for refresh to complete")
	cmd.Flags().BoolVar(&emitEvents, "events", false, "emit NDJSON progress events while waiting")
	return cmd
}

func (a *App) buildAccountsRefreshStatus() *cobra.Command {
	return &cobra.Command{
		Use:   "refresh-status",
		Short: "Check the status of the latest account refresh",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, a.Flags.JSONMode, a.Flags.Pretty)

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "accounts.refresh-status", err.(*errors.Error), start)
				return
			}

			status, err := svc.GetAccountsRefreshStatus(cmd.Context())
			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to get refresh status", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "accounts.refresh-status", cliErr, start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("accounts.refresh-status", a.Flags.Profile, output.SchemaVersion, "", status, time.Since(start))
				a.renderSuccess(renderer, env, start)
			} else {
				fmt.Printf("Complete:   %v\n", status["is_complete"])
				fmt.Printf("Status:     %s\n", status["status"])
				fmt.Printf("Start Time: %s\n", status["start_time"])
				fmt.Printf("End Time:   %s\n", status["end_time"])
			}
		},
	}
}

func (a *App) buildAccountsUpdate() *cobra.Command {
	var (
		accountName    string
		accountBalance float64
	)
	cmd := &cobra.Command{
		Use:   "update <account-id>",
		Short: "Update an account",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, a.Flags.JSONMode, a.Flags.Pretty)
			logger := a.Deps.NewAuditLogger()
			id := args[0]

			if err := safety.Check(safety.TierMutation, a.Flags.ReadOnly, a.Flags.DryRun, a.Flags.Confirm); err != nil {
				a.handleError(renderer, "accounts.update", err.(*errors.Error), start)
				return
			}

			var name *string
			if cmd.Flags().Changed("name") {
				name = &accountName
			}
			var balance *float64
			if cmd.Flags().Changed("balance") {
				balance = &accountBalance
			}

			if a.Flags.DryRun {
				plan := safety.NewPlan()
				plan.Add("accounts.update", id, nil, map[string]interface{}{"name": name, "balance": balance})
				env := output.NewEnvelope("accounts.update", a.Flags.Profile, output.SchemaVersion, "", plan, time.Since(start))
				a.renderSuccess(renderer, env, start)
				return
			}

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "accounts.update", err.(*errors.Error), start)
				return
			}

			acc, err := svc.UpdateAccount(cmd.Context(), id, name, balance)
			result := "success"
			var errCode string
			if err != nil {
				result = "failure"
				if e, ok := err.(*errors.Error); ok {
					errCode = string(e.Code)
				}
			}
			a.logAudit(logger, &audit.Record{Command: "accounts.update", ResourceID: id, DryRun: a.Flags.DryRun, Confirmed: a.Flags.Confirm, Profile: a.Flags.Profile, Result: result, ErrorCode: errCode})

			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to update account", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "accounts.update", cliErr, start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("accounts.update", a.Flags.Profile, output.SchemaVersion, "", acc, time.Since(start))
				a.renderSuccess(renderer, env, start)
			} else {
				fmt.Printf("Successfully updated account %s.\n", acc.ID)
			}
		},
	}
	cmd.Flags().StringVar(&accountName, "name", "", "new account name")
	cmd.Flags().Float64Var(&accountBalance, "balance", 0, "new account balance")
	return cmd
}

func (a *App) buildAccountsDelete() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <account-id>",
		Short: "Delete an account",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, a.Flags.JSONMode, a.Flags.Pretty)
			logger := a.Deps.NewAuditLogger()
			id := args[0]

			if err := safety.Check(safety.TierDestructive, a.Flags.ReadOnly, a.Flags.DryRun, a.Flags.Confirm); err != nil {
				a.handleError(renderer, "accounts.delete", err.(*errors.Error), start)
				return
			}

			if a.Flags.DryRun {
				plan := safety.NewPlan()
				plan.Add("accounts.delete", id, nil, nil)
				env := output.NewEnvelope("accounts.delete", a.Flags.Profile, output.SchemaVersion, "", plan, time.Since(start))
				a.renderSuccess(renderer, env, start)
				return
			}

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "accounts.delete", err.(*errors.Error), start)
				return
			}

			err = svc.DeleteAccount(cmd.Context(), id)
			result := "success"
			var errCode string
			if err != nil {
				result = "failure"
				if e, ok := err.(*errors.Error); ok {
					errCode = string(e.Code)
				}
			}
			a.logAudit(logger, &audit.Record{Command: "accounts.delete", ResourceID: id, DryRun: a.Flags.DryRun, Confirmed: a.Flags.Confirm, Profile: a.Flags.Profile, Result: result, ErrorCode: errCode})

			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to delete account", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "accounts.delete", cliErr, start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("accounts.delete", a.Flags.Profile, output.SchemaVersion, "", map[string]string{"status": "deleted"}, time.Since(start))
				a.renderSuccess(renderer, env, start)
			} else {
				fmt.Printf("Successfully deleted account %s.\n", id)
			}
		},
	}
}

func (a *App) buildAccountsCreateManual() *cobra.Command {
	var (
		accountName    string
		accountType    string
		accountBalance float64
	)
	cmd := &cobra.Command{
		Use:   "create-manual",
		Short: "Create a manual account",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, a.Flags.JSONMode, a.Flags.Pretty)
			logger := a.Deps.NewAuditLogger()

			if err := safety.Check(safety.TierMutation, a.Flags.ReadOnly, a.Flags.DryRun, a.Flags.Confirm); err != nil {
				a.handleError(renderer, "accounts.create-manual", err.(*errors.Error), start)
				return
			}

			if a.Flags.DryRun {
				plan := safety.NewPlan()
				plan.Add("accounts.create-manual", "", nil, map[string]interface{}{"name": accountName, "type": accountType, "balance": accountBalance})
				env := output.NewEnvelope("accounts.create-manual", a.Flags.Profile, output.SchemaVersion, "", plan, time.Since(start))
				a.renderSuccess(renderer, env, start)
				return
			}

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "accounts.create-manual", err.(*errors.Error), start)
				return
			}

			acc, err := svc.CreateManualAccount(cmd.Context(), accountName, accountType, accountBalance)
			result := "success"
			var errCode string
			if err != nil {
				result = "failure"
				if e, ok := err.(*errors.Error); ok {
					errCode = string(e.Code)
				}
			}
			a.logAudit(logger, &audit.Record{Command: "accounts.create-manual", DryRun: a.Flags.DryRun, Confirmed: a.Flags.Confirm, Profile: a.Flags.Profile, Result: result, ErrorCode: errCode})

			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to create manual account", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "accounts.create-manual", cliErr, start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("accounts.create-manual", a.Flags.Profile, output.SchemaVersion, "", acc, time.Since(start))
				a.renderSuccess(renderer, env, start)
			} else {
				fmt.Printf("Successfully created manual account %s (%s).\n", acc.DisplayName, acc.ID)
			}
		},
	}
	cmd.Flags().StringVar(&accountName, "name", "", "account name")
	cmd.Flags().StringVar(&accountType, "type", "cash", "account type (e.g. cash, credit, investment)")
	cmd.Flags().Float64Var(&accountBalance, "balance", 0, "initial balance")
	mustMarkFlagRequired(cmd, "name")
	return cmd
}

func (a *App) buildAccountsUploadHistory() *cobra.Command {
	return &cobra.Command{
		Use:   "upload-history <account-id> <file>",
		Short: "Upload balance history for an account",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, a.Flags.JSONMode, a.Flags.Pretty)
			logger := a.Deps.NewAuditLogger()
			id := args[0]
			path := args[1]

			if err := safety.Check(safety.TierMutation, a.Flags.ReadOnly, a.Flags.DryRun, a.Flags.Confirm); err != nil {
				a.handleError(renderer, "accounts.upload-history", err.(*errors.Error), start)
				return
			}

			if a.Flags.DryRun {
				plan := safety.NewPlan()
				plan.Add("accounts.upload-history", id, nil, map[string]string{"file": path})
				env := output.NewEnvelope("accounts.upload-history", a.Flags.Profile, output.SchemaVersion, "", plan, time.Since(start))
				a.renderSuccess(renderer, env, start)
				return
			}

			f, err := os.Open(path)
			if err != nil {
				a.handleError(renderer, "accounts.upload-history", errors.New(errors.InternalError, "failed to open file", errors.CatInternal, false, err), start)
				return
			}
			defer func() {
				_ = f.Close()
			}()

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "accounts.upload-history", err.(*errors.Error), start)
				return
			}

			err = svc.UploadAccountBalanceHistory(cmd.Context(), id, f)
			result := "success"
			var errCode string
			if err != nil {
				result = "failure"
				if e, ok := err.(*errors.Error); ok {
					errCode = string(e.Code)
				}
			}
			a.logAudit(logger, &audit.Record{Command: "accounts.upload-history", ResourceID: id, DryRun: a.Flags.DryRun, Confirmed: a.Flags.Confirm, Profile: a.Flags.Profile, Result: result, ErrorCode: errCode})

			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to upload history", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "accounts.upload-history", cliErr, start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("accounts.upload-history", a.Flags.Profile, output.SchemaVersion, "", map[string]string{"status": "uploaded"}, time.Since(start))
				a.renderSuccess(renderer, env, start)
			} else {
				fmt.Printf("Successfully uploaded history for account %s.\n", id)
			}
		},
	}
}

func (a *App) buildAccountsRecentBalances() *cobra.Command {
	var historyFrom string
	cmd := &cobra.Command{
		Use:   "recent-balances",
		Short: "Get recent daily balances for all accounts",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, a.Flags.JSONMode, a.Flags.Pretty)

			if historyFrom == "" {
				historyFrom = time.Now().AddDate(0, 0, -31).Format("2006-01-02")
			}

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "accounts.recent-balances", err.(*errors.Error), start)
				return
			}

			res, err := svc.GetAccountRecentBalances(cmd.Context(), historyFrom)
			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to get recent balances", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "accounts.recent-balances", cliErr, start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("accounts.recent-balances", a.Flags.Profile, output.SchemaVersion, "", res, time.Since(start))
				a.renderSuccess(renderer, env, start)
			} else {
				fmt.Println("Recent daily balances fetched.")
			}
		},
	}
	cmd.Flags().StringVar(&historyFrom, "from", "", "start date (YYYY-MM-DD)")
	return cmd
}

func (a *App) buildAccountsSnapshots() *cobra.Command {
	var (
		historyFrom string
		timeframe   string
	)
	cmd := &cobra.Command{
		Use:   "snapshots",
		Short: "Get net value snapshots by account type",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, a.Flags.JSONMode, a.Flags.Pretty)

			if historyFrom == "" {
				historyFrom = time.Now().AddDate(-1, 0, 0).Format("2006-01-02")
			}

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "accounts.snapshots", err.(*errors.Error), start)
				return
			}

			res, err := svc.GetSnapshotsByAccountType(cmd.Context(), historyFrom, timeframe)
			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to get snapshots", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "accounts.snapshots", cliErr, start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("accounts.snapshots", a.Flags.Profile, output.SchemaVersion, "", res, time.Since(start))
				a.renderSuccess(renderer, env, start)
			} else {
				fmt.Println("Account type snapshots fetched.")
			}
		},
	}
	cmd.Flags().StringVar(&historyFrom, "from", "", "start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&timeframe, "timeframe", "month", "granularity (month or year)")
	return cmd
}

func (a *App) buildAccountsAggregateSnapshots() *cobra.Command {
	var (
		historyFrom string
		historyTo   string
		accountType string
	)
	cmd := &cobra.Command{
		Use:   "aggregate-snapshots",
		Short: "Get aggregate net value snapshots",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, a.Flags.JSONMode, a.Flags.Pretty)

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "accounts.aggregate-snapshots", err.(*errors.Error), start)
				return
			}

			res, err := svc.GetAggregateSnapshots(cmd.Context(), historyFrom, historyTo, accountType)
			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to get aggregate snapshots", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "accounts.aggregate-snapshots", cliErr, start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("accounts.aggregate-snapshots", a.Flags.Profile, output.SchemaVersion, "", res, time.Since(start))
				a.renderSuccess(renderer, env, start)
			} else {
				fmt.Println("Aggregate snapshots fetched.")
			}
		},
	}
	cmd.Flags().StringVar(&historyFrom, "from", "", "start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&historyTo, "to", "", "end date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&accountType, "type", "", "filter by account type")
	return cmd
}

func (a *App) buildNetworthCommand() *cobra.Command {
	var (
		historyFrom string
		historyTo   string
		accountType string
	)
	cmd := &cobra.Command{
		Use:   "networth",
		Short: "Get net worth history over time",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, a.Flags.JSONMode, a.Flags.Pretty)

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "networth", err.(*errors.Error), start)
				return
			}

			res, err := svc.GetAggregateSnapshots(cmd.Context(), historyFrom, historyTo, accountType)
			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to get net worth data", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "networth", cliErr, start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("networth", a.Flags.Profile, output.SchemaVersion, "", res, time.Since(start))
				a.renderSuccess(renderer, env, start)
			} else {
				fmt.Println("Net worth snapshots fetched.")
			}
		},
	}
	cmd.Flags().StringVar(&historyFrom, "from", "", "start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&historyTo, "to", "", "end date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&accountType, "type", "", "filter by account type")
	return cmd
}
