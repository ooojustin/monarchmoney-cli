package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/thedavidweng/monarchmoney-cli/internal/audit"
	"github.com/thedavidweng/monarchmoney-cli/internal/errors"
	"github.com/thedavidweng/monarchmoney-cli/internal/monarch"
	"github.com/thedavidweng/monarchmoney-cli/internal/output"
	"github.com/thedavidweng/monarchmoney-cli/internal/safety"
)

var (
	accountName    string
	accountBalance float64
	accountType    string
	historyFrom    string
	historyTo      string
	refreshWait    bool
	timeframe      string
	balanceAtDate  string
	accountIDs     []string
)

var accountsCmd = &cobra.Command{
	Use:     "accounts",
	Short:   "Manage Monarch Money accounts",
	GroupID: "core",
	Example: "  monarch accounts list --json\n  monarch accounts show <id>\n  monarch accounts refresh --confirm",
}

var accountsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all accounts",
	Run: func(cmd *cobra.Command, args []string) {
		run(cmd.Context(), "accounts.list", "failed to list accounts",
			func(ctx context.Context, svc *monarch.Service) ([]monarch.Account, error) {
				return svc.ListAccounts(ctx)
			},
			func(accounts []monarch.Account) {
				fmt.Printf("%-20s %-15s %-15s %s\n", "ID", "NAME", "TYPE", "BALANCE")
				for _, a := range accounts {
					fmt.Printf("%-20s %-15s %-15s %.2f\n", a.ID, a.DisplayName, a.AccountType, a.DisplayBalance)
				}
			})
	},
}

var accountsHoldingsCmd = &cobra.Command{
	Use:   "holdings <account-id>",
	Short: "List holdings for an account",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		run(cmd.Context(), "accounts.holdings", "failed to get holdings",
			func(ctx context.Context, svc *monarch.Service) ([]monarch.Holding, error) {
				return svc.GetAccountHoldings(ctx, args[0])
			},
			func(holdings []monarch.Holding) {
				fmt.Printf("%-20s %12s %12s %12s\n", "ID", "QUANTITY", "BASIS", "TOTAL VALUE")
				for _, h := range holdings {
					fmt.Printf("%-20s %12.2f %12.2f %12.2f\n", h.ID, h.Quantity, h.Basis, h.TotalValue)
				}
			})
	},
}

var accountsBalanceAtCmd = &cobra.Command{
	Use:   "balance-at",
	Short: "Get account balances at a specific date",
	Long:  "Get display balances for all accounts, or selected accounts, as of a specific date.",
	Example: `  monarch accounts balance-at --date 2026-05-10
  monarch accounts balance-at --date 2026-05-10 --account-id acc_123 --account-id acc_456 --json --pretty`,
	Run: func(cmd *cobra.Command, args []string) {
		run(cmd.Context(), "accounts.balance-at", "failed to get account balances",
			func(ctx context.Context, svc *monarch.Service) ([]monarch.AccountBalanceAt, error) {
				if _, err := time.Parse("2006-01-02", balanceAtDate); err != nil {
					return nil, errors.New(errors.InvalidArguments, "date must use YYYY-MM-DD", errors.CatValidation, false, err)
				}
				return svc.GetAccountBalancesAt(ctx, balanceAtDate, accountIDs)
			},
			func(balances []monarch.AccountBalanceAt) {
				fmt.Printf("%-20s %-30s %-15s %12s\n", "ID", "NAME", "TYPE", "BALANCE")
				for _, balance := range balances {
					fmt.Printf("%-20s %-30s %-15s %12.2f\n", balance.ID, balance.DisplayName, balance.AccountType, balance.DisplayBalance)
				}
			})
	},
}

var accountsHistoryCmd = &cobra.Command{
	Use:   "history <account-id>",
	Short: "Get balance history for an account",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		runWarn(cmd.Context(), "accounts.history", "failed to get history",
			[]string{"uses aggregateSnapshots for account history; per-account snapshots are not currently available"},
			func(ctx context.Context, svc *monarch.Service) ([]monarch.HistoryRecord, error) {
				return svc.GetAccountHistory(ctx, args[0], historyFrom, historyTo)
			},
			func(history []monarch.HistoryRecord) {
				fmt.Printf("%-12s %10s\n", "DATE", "AMOUNT")
				for _, r := range history {
					fmt.Printf("%-12s %10.2f\n", r.Date, r.Amount)
				}
			})
	},
}

var accountsRefreshCmd = &cobra.Command{
	Use:   "refresh [account-id...]",
	Short: "Request a refresh of all accounts (or specific ones)",
	Run: func(cmd *cobra.Command, args []string) {
		start := time.Now()
		renderer := output.NewRenderer(nil, nil, jsonMode, pretty)
		logger := audit.NewLogger()

		if err := safety.Check(safety.TierRemoteAction, readOnly, dryRun, confirm); err != nil {
			handleError(renderer, "accounts.refresh", err, start)
			return
		}

		if dryRun {
			plan := safety.NewPlan()
			plan.Add("accounts.refresh", "", nil, map[string]any{"account_ids": args})
			env := output.NewEnvelope("accounts.refresh", profile, output.SchemaVersion, requestID, plan, time.Since(start))
			renderer.RenderSuccess(env)
			return
		}

		deps, ok := newDeps(renderer, "accounts.refresh", start)
		if !ok {
			return
		}
		svc := deps.Service

		err := svc.RefreshAccounts(cmd.Context(), args)
		result := "success"
		var errCode string
		if err != nil {
			result = "failure"
			if e, ok := err.(*errors.Error); ok {
				errCode = string(e.Code)
			}
		}

		logger.Log(&audit.Record{ //nolint:errcheck // best-effort audit
			Command:   "accounts.refresh",
			DryRun:    dryRun,
			Confirmed: confirm,
			Profile:   profile,
			Result:    result,
			ErrorCode: errCode,
		})

		if err != nil {
			handleError(renderer, "accounts.refresh", wrapError(err, "failed to refresh accounts"), start)
			return
		}

		if refreshWait {
			renderer.PrintDiagnostic("Waiting for refresh to complete...")
			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-cmd.Context().Done():
					handleError(renderer, "accounts.refresh", errors.New(errors.InternalError, "context canceled", errors.CatInternal, false, cmd.Context().Err()), start)
					return
				case <-ticker.C:
					status, err := svc.GetAccountsRefreshStatus(cmd.Context())
					if err != nil {
						renderer.PrintDiagnostic(fmt.Sprintf("Warning: failed to check refresh status: %v", err))
						continue
					}

					if events {
						env := output.NewEnvelope("accounts.refresh.progress", profile, output.SchemaVersion, requestID, status, time.Since(start))
						renderer.RenderSuccess(env)
					}

					if done, _ := status["is_complete"].(bool); done {
						goto complete
					}
				}
			}
		}

	complete:
		if jsonMode {
			var status string
			if refreshWait {
				status = "refresh complete"
			} else {
				status = "refresh requested"
			}
			env := output.NewEnvelope("accounts.refresh", profile, output.SchemaVersion, requestID, map[string]string{"status": status}, time.Since(start))
			renderer.RenderSuccess(env)
		} else {
			if refreshWait {
				fmt.Println("Refresh complete.")
			} else {
				fmt.Println("Refresh requested successfully.")
			}
		}
	},
}

var accountsUpdateCmd = &cobra.Command{
	Use:   "update <account-id>",
	Short: "Update an account",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		runMutation(cmd, "accounts.update", "failed to update account", safety.TierMutation, func() (mutation, *errors.Error) {
			var name *string
			if cmd.Flags().Changed("name") {
				name = &accountName
			}
			var balance *float64
			if cmd.Flags().Changed("balance") {
				balance = &accountBalance
			}
			var acc *monarch.Account
			return mutation{
				resourceID: id,
				planAfter:  map[string]any{"name": name, "balance": balance},
				do: func(ctx context.Context, svc *monarch.Service) (any, error) {
					a, err := svc.UpdateAccount(ctx, id, name, balance)
					if err != nil {
						return nil, err
					}
					acc = a
					return a, nil
				},
				human: func() { fmt.Printf("Successfully updated account %s.\n", acc.ID) },
			}, nil
		})
	},
}

var accountsDeleteCmd = &cobra.Command{
	Use:   "delete <account-id>",
	Short: "Delete an account",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		runMutation(cmd, "accounts.delete", "failed to delete account", safety.TierDestructive, func() (mutation, *errors.Error) {
			return mutation{
				resourceID: id,
				do: func(ctx context.Context, svc *monarch.Service) (any, error) {
					if err := svc.DeleteAccount(ctx, id); err != nil {
						return nil, err
					}
					return map[string]string{"status": "deleted"}, nil
				},
				human: func() { fmt.Printf("Successfully deleted account %s.\n", id) },
			}, nil
		})
	},
}

var accountsCreateManualCmd = &cobra.Command{
	Use:   "create-manual",
	Short: "Create a manual account",
	Run: func(cmd *cobra.Command, args []string) {
		runMutation(cmd, "accounts.create-manual", "failed to create manual account", safety.TierMutation, func() (mutation, *errors.Error) {
			var acc *monarch.Account
			return mutation{
				planAfter: map[string]any{"name": accountName, "type": accountType, "balance": accountBalance},
				do: func(ctx context.Context, svc *monarch.Service) (any, error) {
					a, err := svc.CreateManualAccount(ctx, accountName, accountType, accountBalance)
					if err != nil {
						return nil, err
					}
					acc = a
					return a, nil
				},
				human: func() {
					fmt.Printf("Successfully created manual account %s (%s).\n", acc.DisplayName, acc.ID)
				},
			}, nil
		})
	},
}

var accountsUploadHistoryCmd = &cobra.Command{
	Use:   "upload-history <account-id> <file>",
	Short: "Upload balance history for an account",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		path := args[1]
		runMutation(cmd, "accounts.upload-history", "failed to upload history", safety.TierMutation, func() (mutation, *errors.Error) {
			return mutation{
				resourceID: id,
				planAfter:  map[string]string{"file": path},
				do: func(ctx context.Context, svc *monarch.Service) (any, error) {
					f, err := os.Open(path)
					if err != nil {
						return nil, errors.New(errors.InternalError, "failed to open file", errors.CatInternal, false, err)
					}
					defer func() {
						if cerr := f.Close(); cerr != nil {
							fmt.Fprintf(os.Stderr, "warning: failed to close file: %v\n", cerr)
						}
					}()
					if err := svc.UploadAccountBalanceHistory(ctx, id, f); err != nil {
						return nil, err
					}
					return map[string]string{"status": "uploaded"}, nil
				},
				human: func() { fmt.Printf("Successfully uploaded history for account %s.\n", id) },
			}, nil
		})
	},
}

var accountsShowCmd = &cobra.Command{
	Use:   "show <account-id>",
	Short: "Show detailed information for an account",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		run(cmd.Context(), "accounts.show", "failed to get account",
			func(ctx context.Context, svc *monarch.Service) (*monarch.Account, error) {
				return svc.GetAccount(ctx, args[0])
			},
			func(acc *monarch.Account) {
				fmt.Printf("ID:       %s\n", acc.ID)
				fmt.Printf("Name:     %s\n", acc.DisplayName)
				fmt.Printf("Type:     %s\n", acc.AccountType)
				fmt.Printf("Balance:  %.2f\n", acc.DisplayBalance)
				fmt.Printf("Updated:  %s\n", acc.UpdatedAt)
			})
	},
}

var accountsTypesCmd = &cobra.Command{
	Use:   "types",
	Short: "List all available account types",
	Run: func(cmd *cobra.Command, args []string) {
		run(cmd.Context(), "accounts.types", "failed to get account types",
			func(ctx context.Context, svc *monarch.Service) ([]string, error) {
				return svc.GetAccountTypes(ctx)
			},
			func(types []string) {
				for _, t := range types {
					fmt.Println(t)
				}
			})
	},
}

var accountsRefreshStatusCmd = &cobra.Command{
	Use:   "refresh-status",
	Short: "Check the status of the latest account refresh",
	Run: func(cmd *cobra.Command, args []string) {
		run(cmd.Context(), "accounts.refresh-status", "failed to get refresh status",
			func(ctx context.Context, svc *monarch.Service) (map[string]any, error) {
				return svc.GetAccountsRefreshStatus(ctx)
			},
			func(status map[string]any) {
				fmt.Printf("Complete:   %v\n", status["is_complete"])
				fmt.Printf("Status:     %s\n", status["status"])
				fmt.Printf("Start Time: %s\n", status["start_time"])
				fmt.Printf("End Time:   %s\n", status["end_time"])
			})
	},
}

var accountsRecentBalancesCmd = &cobra.Command{
	Use:   "recent-balances",
	Short: "Get recent daily balances for all accounts",
	Run: func(cmd *cobra.Command, args []string) {
		run(cmd.Context(), "accounts.recent-balances", "failed to get recent balances",
			func(ctx context.Context, svc *monarch.Service) ([]monarch.AccountRecentBalance, error) {
				if historyFrom == "" {
					historyFrom = time.Now().AddDate(0, 0, -31).Format("2006-01-02")
				}
				return svc.GetAccountRecentBalances(ctx, historyFrom)
			},
			func(_ []monarch.AccountRecentBalance) {
				fmt.Println("Recent daily balances fetched.")
			})
	},
}

var accountsSnapshotsCmd = &cobra.Command{
	Use:   "snapshots",
	Short: "Get net value snapshots by account type",
	Run: func(cmd *cobra.Command, args []string) {
		run(cmd.Context(), "accounts.snapshots", "failed to get snapshots",
			func(ctx context.Context, svc *monarch.Service) (any, error) {
				if historyFrom == "" {
					historyFrom = time.Now().AddDate(-1, 0, 0).Format("2006-01-02")
				}
				return svc.GetSnapshotsByAccountType(ctx, historyFrom, timeframe)
			},
			func(_ any) {
				fmt.Println("Account type snapshots fetched.")
			})
	},
}

var accountsAggregateSnapshotsCmd = &cobra.Command{
	Use:   "aggregate-snapshots",
	Short: "Get aggregate net value snapshots",
	Run: func(cmd *cobra.Command, args []string) {
		run(cmd.Context(), "accounts.aggregate-snapshots", "failed to get aggregate snapshots",
			func(ctx context.Context, svc *monarch.Service) (any, error) {
				return svc.GetAggregateSnapshots(ctx, historyFrom, historyTo, accountType)
			},
			func(_ any) {
				fmt.Println("Aggregate snapshots fetched.")
			})
	},
}

var networthCmd = &cobra.Command{
	Use:   "networth",
	Short: "Get net worth history over time",
	Run: func(cmd *cobra.Command, args []string) {
		run(cmd.Context(), "networth", "failed to get net worth data",
			func(ctx context.Context, svc *monarch.Service) (any, error) {
				return svc.GetAggregateSnapshots(ctx, historyFrom, historyTo, accountType)
			},
			func(_ any) {
				fmt.Println("Net worth snapshots fetched.")
			})
	},
}

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

			svc, _, err := a.loadService()
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
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-15s %-15s %s\n", "ID", "NAME", "TYPE", "BALANCE") //nolint:errcheck // best-effort output
			for _, account := range accounts {
				fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-15s %-15s %.2f\n", account.ID, account.DisplayName, account.AccountType, account.DisplayBalance) //nolint:errcheck // best-effort output
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

			svc, _, err := a.loadService()
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
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%-20s %12s %12s %12s\n", "ID", "QUANTITY", "BASIS", "TOTAL VALUE") //nolint:errcheck // best-effort output
			for _, h := range holdings {
				fmt.Fprintf(cmd.OutOrStdout(), "%-20s %12.2f %12.2f %12.2f\n", h.ID, h.Quantity, h.Basis, h.TotalValue) //nolint:errcheck // best-effort output
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

			svc, _, err := a.loadService()
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
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-30s %-15s %12s\n", "ID", "NAME", "TYPE", "BALANCE") //nolint:errcheck // best-effort output
			for _, balance := range balances {
				fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-30s %-15s %12.2f\n", balance.ID, balance.DisplayName, balance.AccountType, balance.DisplayBalance) //nolint:errcheck // best-effort output
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

			svc, _, err := a.loadService()
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
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%-12s %10s\n", "DATE", "AMOUNT") //nolint:errcheck // best-effort output
			for _, r := range history {
				fmt.Fprintf(cmd.OutOrStdout(), "%-12s %10.2f\n", r.Date, r.Amount) //nolint:errcheck // best-effort output
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

			svc, _, err := a.loadService()
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
						a.handleError(renderer, "accounts.refresh", errors.New(errors.InternalError, "context cancelled", errors.CatInternal, false, cmd.Context().Err()), start)
						return
					case <-ticker.C:
						status, err := svc.GetAccountsRefreshStatus(cmd.Context())
						if err != nil {
							renderer.PrintDiagnostic(fmt.Sprintf("Warning: failed to check refresh status: %v", err))
							continue
						}

						if a.Flags.Events {
							env := output.NewEnvelope("accounts.refresh.progress", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, status, time.Since(start))
							renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
						}

						if status["is_complete"].(bool) {
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
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}

			if wait {
				fmt.Fprintln(cmd.OutOrStdout(), "Refresh complete.") //nolint:errcheck // best-effort output
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "Refresh requested successfully.") //nolint:errcheck // best-effort output
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

			svc, _, err := a.loadService()
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
			acc := result.(*monarch.Account)

			if a.Flags.JSONMode {
				env := output.NewEnvelope("accounts.update", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, acc, time.Since(start))
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully updated account %s.\n", acc.ID) //nolint:errcheck // best-effort output
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

			svc, _, err := a.loadService()
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
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully deleted account %s.\n", id) //nolint:errcheck // best-effort output
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

			svc, _, err := a.loadService()
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
			acc := result.(*monarch.Account)

			if a.Flags.JSONMode {
				env := output.NewEnvelope("accounts.create-manual", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, acc, time.Since(start))
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully created manual account %s (%s).\n", acc.DisplayName, acc.ID) //nolint:errcheck // best-effort output
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
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to close file: %v\n", cerr) //nolint:errcheck // best-effort warning
				}
			}()

			svc, _, err := a.loadService()
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
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully uploaded history for account %s.\n", id) //nolint:errcheck // best-effort output
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

			svc, _, err := a.loadService()
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
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "ID:       %s\n", acc.ID)               //nolint:errcheck // best-effort output
			fmt.Fprintf(cmd.OutOrStdout(), "Name:     %s\n", acc.DisplayName)      //nolint:errcheck // best-effort output
			fmt.Fprintf(cmd.OutOrStdout(), "Type:     %s\n", acc.AccountType)      //nolint:errcheck // best-effort output
			fmt.Fprintf(cmd.OutOrStdout(), "Balance:  %.2f\n", acc.DisplayBalance) //nolint:errcheck // best-effort output
			fmt.Fprintf(cmd.OutOrStdout(), "Updated:  %s\n", acc.UpdatedAt)        //nolint:errcheck // best-effort output
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

			svc, _, err := a.loadService()
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
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}

			for _, accountType := range types {
				fmt.Fprintln(cmd.OutOrStdout(), accountType) //nolint:errcheck // best-effort output
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

			svc, _, err := a.loadService()
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
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Complete:   %v\n", status["is_complete"]) //nolint:errcheck // best-effort output
			fmt.Fprintf(cmd.OutOrStdout(), "Status:     %s\n", status["status"])      //nolint:errcheck // best-effort output
			fmt.Fprintf(cmd.OutOrStdout(), "Start Time: %s\n", status["start_time"])  //nolint:errcheck // best-effort output
			fmt.Fprintf(cmd.OutOrStdout(), "End Time:   %s\n", status["end_time"])    //nolint:errcheck // best-effort output
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

			svc, _, err := a.loadService()
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
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Recent daily balances fetched.") //nolint:errcheck // best-effort output
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

			svc, _, err := a.loadService()
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
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Account type snapshots fetched.") //nolint:errcheck // best-effort output
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

			svc, _, err := a.loadService()
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
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Aggregate snapshots fetched.") //nolint:errcheck // best-effort output
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

			svc, _, err := a.loadService()
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
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Net worth snapshots fetched.") //nolint:errcheck // best-effort output
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&to, "to", "", "end date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&accType, "type", "", "filter by account type")
	return cmd
}

func init() {
	accountsCreateManualCmd.Flags().StringVar(&accountName, "name", "", "account name")
	accountsCreateManualCmd.Flags().StringVar(&accountType, "type", "cash", "account type (e.g. cash, credit, investment)")
	accountsCreateManualCmd.Flags().Float64Var(&accountBalance, "balance", 0, "initial balance")
	accountsCreateManualCmd.MarkFlagRequired("name") //nolint:errcheck // flag registered above

	accountsUpdateCmd.Flags().StringVar(&accountName, "name", "", "new account name")
	accountsUpdateCmd.Flags().Float64Var(&accountBalance, "balance", 0, "new account balance")

	accountsHistoryCmd.Flags().StringVar(&historyFrom, "from", "", "start date (YYYY-MM-DD)")
	accountsHistoryCmd.Flags().StringVar(&historyTo, "to", "", "end date (YYYY-MM-DD)")

	accountsBalanceAtCmd.Flags().StringVar(&balanceAtDate, "date", "", "balance date (YYYY-MM-DD)")
	accountsBalanceAtCmd.Flags().StringSliceVar(&accountIDs, "account-id", nil, "account id to include (repeatable)")
	accountsBalanceAtCmd.MarkFlagRequired("date") //nolint:errcheck // flag registered above

	accountsRefreshCmd.Flags().BoolVar(&refreshWait, "wait", false, "wait for refresh to complete")

	accountsRecentBalancesCmd.Flags().StringVar(&historyFrom, "from", "", "start date (YYYY-MM-DD)")

	accountsSnapshotsCmd.Flags().StringVar(&historyFrom, "from", "", "start date (YYYY-MM-DD)")
	accountsSnapshotsCmd.Flags().StringVar(&timeframe, "timeframe", "month", "granularity (month or year)")

	accountsAggregateSnapshotsCmd.Flags().StringVar(&historyFrom, "from", "", "start date (YYYY-MM-DD)")
	accountsAggregateSnapshotsCmd.Flags().StringVar(&historyTo, "to", "", "end date (YYYY-MM-DD)")
	accountsAggregateSnapshotsCmd.Flags().StringVar(&accountType, "type", "", "filter by account type")

	accountsCmd.AddCommand(accountsListCmd)
	accountsCmd.AddCommand(accountsShowCmd)
	accountsCmd.AddCommand(accountsTypesCmd)
	accountsCmd.AddCommand(accountsHoldingsCmd)
	accountsCmd.AddCommand(accountsBalanceAtCmd)
	accountsCmd.AddCommand(accountsHistoryCmd)
	accountsCmd.AddCommand(accountsRefreshCmd)
	accountsCmd.AddCommand(accountsRefreshStatusCmd)
	accountsCmd.AddCommand(accountsUpdateCmd)
	accountsCmd.AddCommand(accountsDeleteCmd)
	accountsCmd.AddCommand(accountsCreateManualCmd)
	accountsCmd.AddCommand(accountsUploadHistoryCmd)
	accountsCmd.AddCommand(accountsRecentBalancesCmd)
	accountsCmd.AddCommand(accountsSnapshotsCmd)
	accountsCmd.AddCommand(accountsAggregateSnapshotsCmd)
	RootCmd.AddCommand(accountsCmd)

	networthCmd.Flags().StringVar(&historyFrom, "from", "", "start date (YYYY-MM-DD)")
	networthCmd.Flags().StringVar(&historyTo, "to", "", "end date (YYYY-MM-DD)")
	networthCmd.Flags().StringVar(&accountType, "type", "", "filter by account type")
	RootCmd.AddCommand(networthCmd)
}
