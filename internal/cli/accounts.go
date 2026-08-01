package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/thedavidweng/monarchmoney-cli/internal/errors"
	"github.com/thedavidweng/monarchmoney-cli/internal/monarch"
	"github.com/thedavidweng/monarchmoney-cli/internal/output"
	"github.com/thedavidweng/monarchmoney-cli/internal/safety"
)

func (a *App) buildAccountsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "accounts",
		Short:   "Manage Monarch Money accounts",
		GroupID: "core",
		Example: "  monarch accounts list --json\n  monarch accounts show <id>\n  monarch accounts refresh --confirm",
	}
	cmd.AddCommand(a.buildAccountsListCommand())
	cmd.AddCommand(a.buildAccountsShowCommand())
	cmd.AddCommand(a.buildAccountsTypesCommand())
	cmd.AddCommand(a.buildAccountsHoldingsCommand())
	cmd.AddCommand(a.buildAccountsBalanceAtCommand())
	cmd.AddCommand(a.buildAccountsHistoryCommand())
	cmd.AddCommand(a.buildAccountsRefreshCommand())
	cmd.AddCommand(a.buildAccountsRefreshStatusCommand())
	cmd.AddCommand(a.buildAccountsUpdateCommand())
	cmd.AddCommand(a.buildAccountsDeleteCommand())
	cmd.AddCommand(a.buildAccountsCreateManualCommand())
	cmd.AddCommand(a.buildAccountsUploadHistoryCommand())
	cmd.AddCommand(a.buildAccountsRecentBalancesCommand())
	cmd.AddCommand(a.buildAccountsSnapshotsCommand())
	cmd.AddCommand(a.buildAccountsAggregateSnapshotsCommand())
	return cmd
}

func (a *App) buildAccountsListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all accounts",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)

			svc, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "accounts.list", wrapError(err, "failed to load service"), start)
				return
			}

			accounts, err := svc.ListAccounts(cmd.Context())
			if err != nil {
				a.handleError(renderer, "accounts.list", wrapError(err, "failed to list accounts"), start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("accounts.list", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, accounts, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-15s %-15s %s\n", "ID", "NAME", "TYPE", "BALANCE")
			for i := range accounts {
				account := &accounts[i]
				fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-15s %-15s %.2f\n", account.ID, account.DisplayName, account.AccountType, account.DisplayBalance)
			}
		},
	}
}

func (a *App) buildAccountsHoldingsCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "holdings <account-id>",
		Short: "List holdings for an account",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)

			svc, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "accounts.holdings", wrapError(err, "failed to load service"), start)
				return
			}

			holdings, err := svc.GetAccountHoldings(cmd.Context(), args[0])
			if err != nil {
				a.handleError(renderer, "accounts.holdings", wrapError(err, "failed to get holdings"), start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("accounts.holdings", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, holdings, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%-20s %12s %12s %12s\n", "ID", "QUANTITY", "BASIS", "TOTAL VALUE")
			for _, h := range holdings {
				fmt.Fprintf(cmd.OutOrStdout(), "%-20s %12.2f %12.2f %12.2f\n", h.ID, h.Quantity, h.Basis, h.TotalValue)
			}
		},
	}
}

func (a *App) buildAccountsBalanceAtCommand() *cobra.Command {
	var (
		date string
		ids  []string
	)

	cmd := &cobra.Command{
		Use:   "balance-at",
		Short: "Get account balances at a specific date",
		Long:  "Get display balances for all accounts, or selected accounts, as of a specific date.",
		Example: `  monarch accounts balance-at --date 2026-05-10
  monarch accounts balance-at --date 2026-05-10 --account-id acc_123 --account-id acc_456 --json --pretty`,
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)

			if _, err := time.Parse("2006-01-02", date); err != nil {
				a.handleError(renderer, "accounts.balance-at", errors.New(errors.InvalidArguments, "date must use YYYY-MM-DD", errors.CatValidation, false, err), start)
				return
			}

			svc, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "accounts.balance-at", wrapError(err, "failed to load service"), start)
				return
			}

			balances, err := svc.GetAccountBalancesAt(cmd.Context(), date, ids)
			if err != nil {
				a.handleError(renderer, "accounts.balance-at", wrapError(err, "failed to get account balances"), start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("accounts.balance-at", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, balances, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-30s %-15s %12s\n", "ID", "NAME", "TYPE", "BALANCE")
			for _, balance := range balances {
				fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-30s %-15s %12.2f\n", balance.ID, balance.DisplayName, balance.AccountType, balance.DisplayBalance)
			}
		},
	}
	cmd.Flags().StringVar(&date, "date", "", "balance date (YYYY-MM-DD)")
	cmd.Flags().StringSliceVar(&ids, "account-id", nil, "account id to include (repeatable)")
	cmd.MarkFlagRequired("date") //nolint:errcheck // flag registered above
	return cmd
}

func (a *App) buildAccountsHistoryCommand() *cobra.Command {
	var (
		from string
		to   string
	)

	cmd := &cobra.Command{
		Use:   "history <account-id>",
		Short: "Get balance history for an account",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)

			svc, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "accounts.history", wrapError(err, "failed to load service"), start)
				return
			}

			history, err := svc.GetAccountHistory(cmd.Context(), args[0], from, to)
			if err != nil {
				a.handleError(renderer, "accounts.history", wrapError(err, "failed to get history"), start)
				return
			}

			if a.Flags.JSONMode {
				env := a.envelopeWithWarnings("accounts.history", history, start, "uses aggregateSnapshots for account history; per-account snapshots are not currently available")
				renderer.RenderSuccess(env)
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%-12s %10s\n", "DATE", "AMOUNT")
			for _, r := range history {
				fmt.Fprintf(cmd.OutOrStdout(), "%-12s %10.2f\n", r.Date, r.Amount)
			}
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&to, "to", "", "end date (YYYY-MM-DD)")
	return cmd
}

func (a *App) buildAccountsRefreshCommand() *cobra.Command {
	var wait bool

	cmd := &cobra.Command{
		Use:   "refresh [account-id...]",
		Short: "Request a refresh of all accounts (or specific ones)",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)

			if !a.checkSafety(renderer, "accounts.refresh", safety.TierRemoteAction, start) {
				return
			}

			if a.Flags.DryRun {
				plan := safety.NewPlan()
				plan.Add("accounts.refresh", "", nil, map[string]any{"account_ids": args})
				a.renderPlan(renderer, "accounts.refresh", plan, start)
				return
			}

			svc, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "accounts.refresh", wrapError(err, "failed to load service"), start)
				return
			}

			if _, err := a.mutate(renderer, "accounts.refresh", "", start, func() (any, error) {
				return nil, svc.RefreshAccounts(cmd.Context(), args)
			}, "failed to refresh accounts"); err != nil {
				return
			}

			if wait {
				renderer.PrintDiagnostic("Waiting for refresh to complete...")
				ticker := time.NewTicker(2 * time.Second)
				defer ticker.Stop()

				for {
					select {
					case <-cmd.Context().Done():
						a.handleError(renderer, "accounts.refresh", errors.New(errors.InternalError, "context canceled", errors.CatInternal, false, cmd.Context().Err()), start)
						return
					case <-ticker.C:
						status, err := svc.GetAccountsRefreshStatus(cmd.Context())
						if err != nil {
							renderer.PrintDiagnostic(fmt.Sprintf("Warning: failed to check refresh status: %v", err))
							continue
						}

						if a.Flags.Events {
							env := output.NewEnvelope("accounts.refresh.progress", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, status, time.Since(start))
							renderer.RenderSuccess(env)
						}

						complete, ok := status["is_complete"].(bool)
						if !ok {
							a.handleError(renderer, "accounts.refresh", errors.New(errors.APISchemaChanged, "refresh status is missing boolean is_complete", errors.CatAPI, false, nil), start)
							return
						}
						if complete {
							goto complete
						}
					}
				}
			}

		complete:
			if a.Flags.JSONMode {
				status := "refresh requested"
				if wait {
					status = "refresh complete"
				}
				env := output.NewEnvelope("accounts.refresh", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, map[string]string{"status": status}, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}

			if wait {
				fmt.Fprintln(cmd.OutOrStdout(), "Refresh complete.")
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "Refresh requested successfully.")
			}
		},
	}
	cmd.Flags().BoolVar(&wait, "wait", false, "wait for refresh to complete")
	return cmd
}

func (a *App) buildAccountsUpdateCommand() *cobra.Command {
	var (
		nameValue    string
		balanceValue float64
	)

	cmd := &cobra.Command{
		Use:   "update <account-id>",
		Short: "Update an account",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)
			id := args[0]

			if !a.checkSafety(renderer, "accounts.update", safety.TierMutation, start) {
				return
			}

			var name *string
			if cmd.Flags().Changed("name") {
				name = &nameValue
			}
			var balance *float64
			if cmd.Flags().Changed("balance") {
				balance = &balanceValue
			}

			if a.Flags.DryRun {
				plan := safety.NewPlan()
				plan.Add("accounts.update", id, nil, map[string]any{"name": name, "balance": balance})
				a.renderPlan(renderer, "accounts.update", plan, start)
				return
			}

			svc, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "accounts.update", wrapError(err, "failed to load service"), start)
				return
			}

			result, err := a.mutate(renderer, "accounts.update", id, start, func() (any, error) {
				return svc.UpdateAccount(cmd.Context(), id, name, balance)
			}, "failed to update account")
			if err != nil {
				return
			}
			acc, ok := result.(*monarch.Account)
			if !ok || acc == nil {
				a.handleError(renderer, "accounts.update", errors.New(errors.InternalError, "unexpected account update result", errors.CatInternal, false, nil), start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("accounts.update", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, acc, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully updated account %s.\n", acc.ID)
		},
	}
	cmd.Flags().StringVar(&nameValue, "name", "", "new account name")
	cmd.Flags().Float64Var(&balanceValue, "balance", 0, "new account balance")
	return cmd
}

func (a *App) buildAccountsDeleteCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <account-id>",
		Short: "Delete an account",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)
			id := args[0]

			if !a.checkSafety(renderer, "accounts.delete", safety.TierDestructive, start) {
				return
			}

			if a.Flags.DryRun {
				plan := safety.NewPlan()
				plan.Add("accounts.delete", id, nil, nil)
				a.renderPlan(renderer, "accounts.delete", plan, start)
				return
			}

			svc, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "accounts.delete", wrapError(err, "failed to load service"), start)
				return
			}

			if _, err := a.mutate(renderer, "accounts.delete", id, start, func() (any, error) {
				return nil, svc.DeleteAccount(cmd.Context(), id)
			}, "failed to delete account"); err != nil {
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("accounts.delete", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, map[string]string{"status": "deleted"}, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully deleted account %s.\n", id)
		},
	}
}

func (a *App) buildAccountsCreateManualCommand() *cobra.Command {
	var (
		name    string
		accType string
		balance float64
	)

	cmd := &cobra.Command{
		Use:   "create-manual",
		Short: "Create a manual account",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)

			if !a.checkSafety(renderer, "accounts.create-manual", safety.TierMutation, start) {
				return
			}

			if a.Flags.DryRun {
				plan := safety.NewPlan()
				plan.Add("accounts.create-manual", "", nil, map[string]any{"name": name, "type": accType, "balance": balance})
				a.renderPlan(renderer, "accounts.create-manual", plan, start)
				return
			}

			svc, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "accounts.create-manual", wrapError(err, "failed to load service"), start)
				return
			}

			result, err := a.mutate(renderer, "accounts.create-manual", "", start, func() (any, error) {
				return svc.CreateManualAccount(cmd.Context(), name, accType, balance)
			}, "failed to create manual account")
			if err != nil {
				return
			}
			acc, ok := result.(*monarch.Account)
			if !ok || acc == nil {
				a.handleError(renderer, "accounts.create-manual", errors.New(errors.InternalError, "unexpected account creation result", errors.CatInternal, false, nil), start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("accounts.create-manual", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, acc, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully created manual account %s (%s).\n", acc.DisplayName, acc.ID)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "account name")
	cmd.Flags().StringVar(&accType, "type", "cash", "account type (e.g. cash, credit, investment)")
	cmd.Flags().Float64Var(&balance, "balance", 0, "initial balance")
	cmd.MarkFlagRequired("name") //nolint:errcheck // flag registered above
	return cmd
}

func (a *App) buildAccountsUploadHistoryCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "upload-history <account-id> <file>",
		Short: "Upload balance history for an account",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)
			id := args[0]
			path := args[1]

			if !a.checkSafety(renderer, "accounts.upload-history", safety.TierMutation, start) {
				return
			}

			if a.Flags.DryRun {
				plan := safety.NewPlan()
				plan.Add("accounts.upload-history", id, nil, map[string]string{"file": path})
				a.renderPlan(renderer, "accounts.upload-history", plan, start)
				return
			}

			f, err := os.Open(path)
			if err != nil {
				a.handleError(renderer, "accounts.upload-history", errors.New(errors.InternalError, "failed to open file", errors.CatInternal, false, err), start)
				return
			}
			defer func() {
				if cerr := f.Close(); cerr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to close file: %v\n", cerr)
				}
			}()

			svc, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "accounts.upload-history", wrapError(err, "failed to load service"), start)
				return
			}

			if _, err := a.mutate(renderer, "accounts.upload-history", id, start, func() (any, error) {
				return nil, svc.UploadAccountBalanceHistory(cmd.Context(), id, f)
			}, "failed to upload history"); err != nil {
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("accounts.upload-history", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, map[string]string{"status": "uploaded"}, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully uploaded history for account %s.\n", id)
		},
	}
}

func (a *App) buildAccountsShowCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "show <account-id>",
		Short: "Show detailed information for an account",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)

			svc, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "accounts.show", wrapError(err, "failed to load service"), start)
				return
			}

			acc, err := svc.GetAccount(cmd.Context(), args[0])
			if err != nil {
				a.handleError(renderer, "accounts.show", wrapError(err, "failed to get account"), start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("accounts.show", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, acc, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "ID:       %s\n", acc.ID)
			fmt.Fprintf(cmd.OutOrStdout(), "Name:     %s\n", acc.DisplayName)
			fmt.Fprintf(cmd.OutOrStdout(), "Type:     %s\n", acc.AccountType)
			fmt.Fprintf(cmd.OutOrStdout(), "Balance:  %.2f\n", acc.DisplayBalance)
			fmt.Fprintf(cmd.OutOrStdout(), "Updated:  %s\n", acc.UpdatedAt)
		},
	}
}

func (a *App) buildAccountsTypesCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "types",
		Short: "List all available account types",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)

			svc, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "accounts.types", wrapError(err, "failed to load service"), start)
				return
			}

			types, err := svc.GetAccountTypes(cmd.Context())
			if err != nil {
				a.handleError(renderer, "accounts.types", wrapError(err, "failed to get account types"), start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("accounts.types", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, types, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}

			for _, accountType := range types {
				fmt.Fprintln(cmd.OutOrStdout(), accountType)
			}
		},
	}
}

func (a *App) buildAccountsRefreshStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "refresh-status",
		Short: "Check the status of the latest account refresh",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)

			svc, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "accounts.refresh-status", wrapError(err, "failed to load service"), start)
				return
			}

			status, err := svc.GetAccountsRefreshStatus(cmd.Context())
			if err != nil {
				a.handleError(renderer, "accounts.refresh-status", wrapError(err, "failed to get refresh status"), start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("accounts.refresh-status", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, status, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Complete:   %v\n", status["is_complete"])
			fmt.Fprintf(cmd.OutOrStdout(), "Status:     %s\n", status["status"])
			fmt.Fprintf(cmd.OutOrStdout(), "Start Time: %s\n", status["start_time"])
			fmt.Fprintf(cmd.OutOrStdout(), "End Time:   %s\n", status["end_time"])
		},
	}
}

func (a *App) buildAccountsRecentBalancesCommand() *cobra.Command {
	var from string

	cmd := &cobra.Command{
		Use:   "recent-balances",
		Short: "Get recent daily balances for all accounts",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)

			effectiveFrom := from
			if effectiveFrom == "" {
				effectiveFrom = time.Now().AddDate(0, 0, -31).Format("2006-01-02")
			}

			svc, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "accounts.recent-balances", wrapError(err, "failed to load service"), start)
				return
			}

			res, err := svc.GetAccountRecentBalances(cmd.Context(), effectiveFrom)
			if err != nil {
				a.handleError(renderer, "accounts.recent-balances", wrapError(err, "failed to get recent balances"), start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("accounts.recent-balances", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, res, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Recent daily balances fetched.")
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "start date (YYYY-MM-DD)")
	return cmd
}

func (a *App) buildAccountsSnapshotsCommand() *cobra.Command {
	var (
		from         string
		timeframeOpt string
	)

	cmd := &cobra.Command{
		Use:   "snapshots",
		Short: "Get net value snapshots by account type",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)

			effectiveFrom := from
			if effectiveFrom == "" {
				effectiveFrom = time.Now().AddDate(-1, 0, 0).Format("2006-01-02")
			}

			svc, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "accounts.snapshots", wrapError(err, "failed to load service"), start)
				return
			}

			res, err := svc.GetSnapshotsByAccountType(cmd.Context(), effectiveFrom, timeframeOpt)
			if err != nil {
				a.handleError(renderer, "accounts.snapshots", wrapError(err, "failed to get snapshots"), start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("accounts.snapshots", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, res, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Account type snapshots fetched.")
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&timeframeOpt, "timeframe", "month", "granularity (month or year)")
	return cmd
}

func (a *App) buildAccountsAggregateSnapshotsCommand() *cobra.Command {
	var (
		from    string
		to      string
		accType string
	)

	cmd := &cobra.Command{
		Use:   "aggregate-snapshots",
		Short: "Get aggregate net value snapshots",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)

			svc, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "accounts.aggregate-snapshots", wrapError(err, "failed to load service"), start)
				return
			}

			res, err := svc.GetAggregateSnapshots(cmd.Context(), from, to, accType)
			if err != nil {
				a.handleError(renderer, "accounts.aggregate-snapshots", wrapError(err, "failed to get aggregate snapshots"), start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("accounts.aggregate-snapshots", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, res, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Aggregate snapshots fetched.")
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&to, "to", "", "end date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&accType, "type", "", "filter by account type")
	return cmd
}

func (a *App) buildNetworthCommand() *cobra.Command {
	var (
		from    string
		to      string
		accType string
	)

	cmd := &cobra.Command{
		Use:   "networth",
		Short: "Get net worth history over time",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)

			svc, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "networth", wrapError(err, "failed to load service"), start)
				return
			}

			res, err := svc.GetAggregateSnapshots(cmd.Context(), from, to, accType)
			if err != nil {
				a.handleError(renderer, "networth", wrapError(err, "failed to get net worth data"), start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("networth", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, res, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Net worth snapshots fetched.")
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&to, "to", "", "end date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&accType, "type", "", "filter by account type")
	return cmd
}
