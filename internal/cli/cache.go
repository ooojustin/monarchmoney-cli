package cli

import (
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
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)

			if from != "" {
				if _, err := time.Parse("2006-01-02", from); err != nil {
					a.handleError(renderer, "cache.sync", errors.New(errors.InvalidArguments, "--from must be a date in YYYY-MM-DD format", errors.CatValidation, false, err), start)
					return
				}
			}

			svc, _, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "cache.sync", wrapError(err, "failed to load service"), start)
				return
			}

			cacheStore, ok := a.openCacheStore(renderer, "cache.sync", start)
			if !ok {
				return
			}
			defer cacheStore.Close() //nolint:errcheck // best-effort close

			renderer.PrintDiagnostic("Syncing accounts...")
			accounts, err := svc.ListAccounts(cmd.Context())
			if err != nil {
				a.handleError(renderer, "cache.sync", errors.New(errors.APIError, fmt.Sprintf("failed to sync accounts: %v", err), errors.CatAPI, false, err), start)
				return
			}
			cacheAccs := make([]cache.Account, 0, len(accounts))
			for _, account := range accounts {
				updatedAt, err := parseCacheDate(account.UpdatedAt)
				if err != nil {
					a.handleError(renderer, "cache.sync", errors.New(errors.APISchemaChanged, "failed to parse account updated_at", errors.CatAPI, false, err), start)
					return
				}
				cacheAccs = append(cacheAccs, cache.Account{
					ID:             account.ID,
					DisplayName:    account.DisplayName,
					AccountType:    account.AccountType,
					DisplayBalance: account.DisplayBalance,
					UpdatedAt:      updatedAt,
				})
			}
			if err := cacheStore.SaveAccounts(cacheAccs); err != nil {
				a.handleError(renderer, "cache.sync", errors.New(errors.InternalError, "failed to save accounts to cache", errors.CatInternal, false, err), start)
				return
			}

			renderer.PrintDiagnostic("Syncing transactions...")
			pageLimit := limit
			if pageLimit <= 0 {
				pageLimit = 1000
			}
			var txs []monarch.Transaction
			if all {
				txs, err = svc.ListAllTransactions(cmd.Context(), &monarch.ListTransactionsOptions{Limit: pageLimit, StartDate: from})
			} else {
				txs, _, err = svc.ListTransactions(cmd.Context(), &monarch.ListTransactionsOptions{Limit: pageLimit, StartDate: from})
			}
			if err != nil {
				a.handleError(renderer, "cache.sync", errors.New(errors.APIError, fmt.Sprintf("failed to sync transactions: %v", err), errors.CatAPI, false, err), start)
				return
			}
			cacheTxs := make([]cache.Transaction, 0, len(txs))
			for _, tx := range txs {
				date, err := time.Parse("2006-01-02", tx.Date)
				if err != nil {
					a.handleError(renderer, "cache.sync", errors.New(errors.APISchemaChanged, "failed to parse transaction date", errors.CatAPI, false, err), start)
					return
				}
				cacheTxs = append(cacheTxs, cache.Transaction{
					ID:        tx.ID,
					Date:      date,
					Amount:    tx.Amount,
					Merchant:  tx.Merchant,
					Category:  tx.Category,
					Notes:     tx.Notes,
					AccountID: tx.AccountID,
				})
			}
			if err := cacheStore.SaveTransactions(cacheTxs); err != nil {
				a.handleError(renderer, "cache.sync", errors.New(errors.InternalError, "failed to save transactions to cache", errors.CatInternal, false, err), start)
				return
			}

			cacheStore.RecordSync(len(cacheAccs), len(cacheTxs)) //nolint:errcheck // best-effort sync record

			if a.Flags.JSONMode {
				env := output.NewEnvelope("cache.sync", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, map[string]any{"status": "sync complete", "accounts": len(cacheAccs), "transactions": len(cacheTxs)}, time.Since(start))
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Sync complete. %d accounts, %d transactions.\n", len(cacheAccs), len(cacheTxs)) //nolint:errcheck // best-effort stdout
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

			cacheStore, ok := a.openCacheStore(renderer, "cache.search", start)
			if !ok {
				return
			}
			defer cacheStore.Close() //nolint:errcheck // best-effort close

			txs, err := cacheStore.SearchTransactions(args[0])
			if err != nil {
				a.handleError(renderer, "cache.search", errors.New(errors.InternalError, "search failed", errors.CatInternal, false, err), start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("cache.search", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, txs, time.Since(start))
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-12s %-20s %-15s %10s %s\n", "DATE", "MERCHANT", "CATEGORY", "AMOUNT", "NOTES") //nolint:errcheck // best-effort stdout
			for _, tx := range txs {
				fmt.Fprintf(cmd.OutOrStdout(), "%-12s %-20s %-15s %10.2f %s\n", tx.Date.Format("2006-01-02"), tx.Merchant, tx.Category, tx.Amount, tx.Notes) //nolint:errcheck // best-effort stdout
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nTotal matches: %d\n", len(txs)) //nolint:errcheck // best-effort stdout
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

			cacheStore, ok := a.openCacheStore(renderer, "cache.stats", start)
			if !ok {
				return
			}
			defer cacheStore.Close() //nolint:errcheck // best-effort close

			stats, _ := cacheStore.GetStats()

			if a.Flags.JSONMode {
				env := output.NewEnvelope("cache.stats", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, stats, time.Since(start))
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Cache Statistics") //nolint:errcheck // best-effort stdout
			for key, value := range stats {
				switch val := value.(type) {
				case int64:
					fmt.Fprintf(cmd.OutOrStdout(), "%s: %d\n", key, val) //nolint:errcheck // best-effort stdout
				case string:
					fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", key, val) //nolint:errcheck // best-effort stdout
				default:
					fmt.Fprintf(cmd.OutOrStdout(), "%s: %v\n", key, val) //nolint:errcheck // best-effort stdout
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

			if before == "" {
				a.handleError(renderer, "cache.cleanup", errors.New(errors.InvalidArguments, "--before is required", errors.CatValidation, false, nil), start)
				return
			}
			if _, err := time.Parse("2006-01-02", before); err != nil {
				a.handleError(renderer, "cache.cleanup", errors.New(errors.InvalidArguments, "--before must be a date in YYYY-MM-DD format", errors.CatValidation, false, err), start)
				return
			}

			cacheStore, ok := a.openCacheStore(renderer, "cache.cleanup", start)
			if !ok {
				return
			}
			defer cacheStore.Close() //nolint:errcheck // best-effort close

			affected, err := cacheStore.Cleanup(before)
			if err != nil {
				a.handleError(renderer, "cache.cleanup", errors.New(errors.InternalError, "failed to cleanup cache", errors.CatInternal, false, err), start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("cache.cleanup", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, map[string]int64{"deleted": affected}, time.Since(start))
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted %d transactions from cache.\n", affected) //nolint:errcheck // best-effort stdout
		},
	}

	cmd.Flags().StringVar(&before, "before", "", "delete transactions before date (YYYY-MM-DD)")
	cmd.MarkFlagRequired("before") //nolint:errcheck // flag registered above
	return cmd
}

func (a *App) openCacheStore(renderer *output.Renderer, command string, start time.Time) (*cache.Store, bool) {
	if a.Config == nil {
		a.handleError(renderer, command, errors.New(errors.InternalError, "configuration not initialized", errors.CatInternal, false, nil), start)
		return nil, false
	}

	cacheStore, err := cache.NewStore(a.Config.CachePath)
	if err != nil {
		a.handleError(renderer, command, errors.New(errors.InternalError, "failed to open cache", errors.CatInternal, false, err), start)
		return nil, false
	}
	return cacheStore, true
}
