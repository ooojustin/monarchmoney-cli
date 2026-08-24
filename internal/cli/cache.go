package cli

import (
	stderrors "errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/thedavidweng/monarchmoney-cli/internal/cache"
	"github.com/thedavidweng/monarchmoney-cli/internal/errors"
	"github.com/thedavidweng/monarchmoney-cli/internal/monarch"
	"github.com/thedavidweng/monarchmoney-cli/internal/output"
)

func parseCacheDate(value string) (time.Time, error) {
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed, nil
	}
	return time.Parse("2006-01-02", value)
}

func (a *App) buildCacheCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "cache",
		Short:   "Manage local data cache",
		GroupID: "utility",
		Example: "  monarch cache sync\n  monarch cache search \"grocery\"\n  monarch cache stats",
	}
	cmd.AddCommand(a.buildCacheSyncCommand())
	cmd.AddCommand(a.buildCacheSearchCommand())
	cmd.AddCommand(a.buildCacheStatsCommand())
	cmd.AddCommand(a.buildCacheCleanupCommand())
	return cmd
}

func (a *App) buildCacheSyncCommand() *cobra.Command {
	var from string
	var limit int
	var all bool

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync data from Monarch to local cache",
		Long: `Pull a full-fidelity archive copy of your Monarch data into the local cache:
accounts with type groups and lifecycle flags, transactions with tags, splits,
review state, category groups and raw merchant names, plus investment holdings
and closing balances. A cache created by an older version is rebuilt automatically.`,
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)

			if err := validateDateFlag("from", from); err != nil {
				a.handleError(renderer, "cache.sync", err, start)
				return
			}
			if err := validatePositiveInt("limit", limit); err != nil {
				a.handleError(renderer, "cache.sync", err, start)
				return
			}
			if a.Config.BackupPath != "" {
				if err := validateJournalPath(a.Config.BackupPath, a.Config.CachePath); err != nil {
					a.handleError(renderer, "cache.sync", err, start)
					return
				}
			}

			svc, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "cache.sync", wrapError(err, "failed to load service"), start)
				return
			}

			cacheStore, ok := a.openCacheStore(renderer, "cache.sync", start, true)
			if !ok {
				return
			}
			defer cacheStore.Close()

			renderer.PrintDiagnostic("Syncing accounts...")
			accounts, err := svc.ListAccounts(cmd.Context())
			if err != nil {
				a.handleError(renderer, "cache.sync", wrapError(err, "failed to sync accounts"), start)
				return
			}
			cacheAccs := make([]cache.Account, 0, len(accounts))
			for i := range accounts {
				account := &accounts[i]
				updatedAt, err := parseCacheDate(account.UpdatedAt)
				if err != nil {
					a.handleError(renderer, "cache.sync", errors.New(errors.APISchemaChanged, "failed to parse account updated_at", errors.CatAPI, false, err), start)
					return
				}
				cacheAccs = append(cacheAccs, cache.Account{
					ID:             account.ID,
					DisplayName:    account.DisplayName,
					AccountType:    account.AccountType,
					TypeGroup:      account.TypeGroup,
					DisplayBalance: account.DisplayBalance,
					CurrentBalance: account.CurrentBalance,
					IsManual:       account.IsManual,
					IsHidden:       account.IsHidden,
					IsClosed:       account.IsClosed,
					UpdatedAt:      updatedAt,
				})
			}
			if err := cacheStore.SaveAccounts(cacheAccs); err != nil {
				a.handleError(renderer, "cache.sync", errors.New(errors.InternalError, "failed to save accounts to cache", errors.CatInternal, false, err), start)
				return
			}

			renderer.PrintDiagnostic("Syncing transactions...")
			var txs []monarch.Transaction
			if all {
				txs, err = svc.ListAllTransactions(cmd.Context(), &monarch.ListTransactionsOptions{Limit: limit, StartDate: from})
			} else {
				txs, _, err = svc.ListTransactions(cmd.Context(), &monarch.ListTransactionsOptions{Limit: limit, StartDate: from})
			}
			if err != nil {
				a.handleError(renderer, "cache.sync", wrapError(err, "failed to sync transactions"), start)
				return
			}
			cacheTxs := make([]cache.Transaction, 0, len(txs))
			for i := range txs {
				tx := &txs[i]
				date, err := time.Parse("2006-01-02", tx.Date)
				if err != nil {
					a.handleError(renderer, "cache.sync", errors.New(errors.APISchemaChanged, "failed to parse transaction date", errors.CatAPI, false, err), start)
					return
				}
				cached := cache.Transaction{
					ID:                  tx.ID,
					Date:                date,
					Amount:              tx.Amount,
					Merchant:            tx.Merchant,
					PlaidName:           tx.PlaidName,
					ProviderDescription: tx.DataProviderDescription,
					Category:            tx.Category,
					CategoryGroup:       tx.CategoryGroup.Name,
					CategoryGroupType:   tx.CategoryGroup.Type,
					Notes:               tx.Notes,
					Pending:             tx.Pending,
					ReviewStatus:        tx.ReviewStatus,
					NeedsReview:         tx.NeedsReview,
					GoalID:              tx.Goal.ID,
					GoalName:            tx.Goal.Name,
					AccountID:           tx.AccountID,
				}
				for _, tag := range tx.Tags {
					cached.Tags = append(cached.Tags, cache.Tag{ID: tag.ID, Name: tag.Name})
				}
				for _, split := range tx.Splits {
					cached.Splits = append(cached.Splits, cache.Split{
						ID: split.ID, Amount: split.Amount, Category: split.Category,
						Merchant: split.Merchant, Notes: split.Notes,
					})
				}
				cacheTxs = append(cacheTxs, cached)
			}
			if len(cacheTxs) > 0 {
				if err := cacheStore.SaveTransactions(cacheTxs); err != nil {
					a.handleError(renderer, "cache.sync", errors.New(errors.InternalError, "failed to save transactions to cache", errors.CatInternal, false, err), start)
					return
				}
			}

			renderer.PrintDiagnostic("Syncing holdings...")
			holdings, err := svc.ListHoldings(cmd.Context())
			if err != nil {
				a.handleError(renderer, "cache.sync", wrapError(err, "failed to sync holdings"), start)
				return
			}
			cacheHoldings := make([]cache.Holding, len(holdings))
			for i := range holdings {
				holding := &holdings[i]
				cacheHoldings[i] = cache.Holding{
					ID: holding.ID, Ticker: holding.Ticker, Name: holding.Name,
					Quantity: holding.Quantity, Basis: holding.Basis, Value: holding.Value,
					AccountID: holding.AccountID,
				}
			}
			if err := cacheStore.SaveHoldings(cacheHoldings); err != nil {
				a.handleError(renderer, "cache.sync", errors.New(errors.InternalError, "failed to save holdings to cache", errors.CatInternal, false, err), start)
				return
			}

			if err := cacheStore.RecordSync(len(cacheAccs), len(cacheTxs)); err != nil {
				a.handleError(renderer, "cache.sync", errors.New(errors.InternalError, "failed to record sync metadata", errors.CatInternal, false, err), start)
				return
			}

			var backupPath string
			var backupWarnings []string
			if a.Config.BackupPath != "" {
				renderer.PrintDiagnostic("Regenerating ledger backup...")
				if _, err := writeJournal(cacheStore, a.Config.BackupPath); err != nil {
					message := fmt.Sprintf("ledger backup regeneration failed: %v", err)
					backupWarnings = append(backupWarnings, message)
					renderer.PrintDiagnostic(message)
				} else {
					backupPath = a.Config.BackupPath
				}
			}

			if a.Flags.JSONMode {
				data := map[string]any{
					"status": "sync complete", "accounts": len(cacheAccs),
					"transactions": len(cacheTxs), "holdings": len(cacheHoldings),
				}
				if backupPath != "" {
					data["backup"] = backupPath
				}
				env := output.NewEnvelope("cache.sync", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, data, time.Since(start))
				env.Meta.Warnings = backupWarnings
				renderer.RenderSuccess(env)
				return
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Sync complete. %d accounts, %d transactions, %d holdings.\n", len(cacheAccs), len(cacheTxs), len(cacheHoldings))
			if backupPath != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Ledger backup written to %s.\n", backupPath)
			}
		},
	}

	cmd.Flags().StringVar(&from, "from", "", "sync transactions from date (YYYY-MM-DD)")
	cmd.Flags().IntVar(&limit, "limit", 1000, "max transactions per page (default 1000)")
	cmd.Flags().BoolVar(&all, "all", false, "paginate through all matching transactions")
	return cmd
}

func (a *App) buildCacheSearchCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "search <query>",
		Short: "Search transactions in local cache",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)

			cacheStore, ok := a.openCacheStore(renderer, "cache.search", start, false)
			if !ok {
				return
			}
			defer cacheStore.Close()

			txs, err := cacheStore.SearchTransactions(args[0])
			if err != nil {
				a.handleError(renderer, "cache.search", errors.New(errors.InternalError, "search failed", errors.CatInternal, false, err), start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("cache.search", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, txs, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-12s %-20s %-15s %10s %s\n", "DATE", "MERCHANT", "CATEGORY", "AMOUNT", "NOTES")
			for i := range txs {
				tx := &txs[i]
				fmt.Fprintf(cmd.OutOrStdout(), "%-12s %-20s %-15s %10.2f %s\n", tx.Date.Format("2006-01-02"), tx.Merchant, tx.Category, tx.Amount, tx.Notes)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nTotal matches: %d\n", len(txs))
		},
	}
}

func (a *App) buildCacheStatsCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "stats",
		Short: "Show cache statistics",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)

			cacheStore, ok := a.openCacheStore(renderer, "cache.stats", start, false)
			if !ok {
				return
			}
			defer cacheStore.Close()

			stats, _ := cacheStore.GetStats()

			if a.Flags.JSONMode {
				env := output.NewEnvelope("cache.stats", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, stats, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Cache Statistics")
			for key, value := range stats {
				switch val := value.(type) {
				case int64:
					fmt.Fprintf(cmd.OutOrStdout(), "%s: %d\n", key, val)
				case string:
					fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", key, val)
				default:
					fmt.Fprintf(cmd.OutOrStdout(), "%s: %v\n", key, val)
				}
			}
		},
	}
}

func (a *App) buildCacheCleanupCommand() *cobra.Command {
	var before string
	cmd := &cobra.Command{
		Use:   "cleanup",
		Short: "Clean up old transactions from cache",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)

			if err := validateRequiredDateFlag("before", before); err != nil {
				a.handleError(renderer, "cache.cleanup", err, start)
				return
			}

			cacheStore, ok := a.openCacheStore(renderer, "cache.cleanup", start, false)
			if !ok {
				return
			}
			defer cacheStore.Close()

			affected, err := cacheStore.Cleanup(before)
			if err != nil {
				a.handleError(renderer, "cache.cleanup", errors.New(errors.InternalError, "failed to cleanup cache", errors.CatInternal, false, err), start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("cache.cleanup", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, map[string]int64{"deleted": affected}, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted %d transactions from cache.\n", affected)
		},
	}

	cmd.Flags().StringVar(&before, "before", "", "delete transactions before date (YYYY-MM-DD)")
	cmd.MarkFlagRequired("before") //nolint:errcheck // flag registered above
	return cmd
}

func (a *App) openCacheStore(renderer *output.Renderer, command string, start time.Time, rebuild bool) (*cache.Store, bool) {
	if a.Config == nil {
		a.handleError(renderer, command, errors.New(errors.InternalError, "configuration not initialized", errors.CatInternal, false, nil), start)
		return nil, false
	}

	cacheStore, err := cache.NewStore(a.Config.CachePath)
	if rebuild && stderrors.Is(err, cache.ErrSchemaOutdated) {
		renderer.PrintDiagnostic("Cache schema outdated; rebuilding...")
		cacheStore, err = cache.RebuildStore(a.Config.CachePath)
	}
	if err != nil {
		message := "failed to open cache"
		if stderrors.Is(err, cache.ErrSchemaOutdated) {
			message = "cache schema is outdated; run 'monarch cache sync' to rebuild it"
		}
		a.handleError(renderer, command, errors.New(errors.InternalError, message, errors.CatInternal, false, err), start)
		return nil, false
	}
	return cacheStore, true
}
