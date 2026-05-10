package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/thedavidweng/monarchmoney-cli/internal/cache"
	"github.com/thedavidweng/monarchmoney-cli/internal/config"
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

func (a *App) buildCacheCommands(parent *cobra.Command) {
	cacheCmd := &cobra.Command{
		Use:   "cache",
		Short: "Manage local data cache",
	}
	cacheCmd.AddCommand(a.buildCacheSync())
	cacheCmd.AddCommand(a.buildCacheSearch())
	cacheCmd.AddCommand(a.buildCacheStats())
	cacheCmd.AddCommand(a.buildCacheCleanup())
	parent.AddCommand(cacheCmd)
}

func (a *App) buildCacheSync() *cobra.Command {
	var syncFrom string
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync data from Monarch to local cache",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, jsonMode, pretty)

			if syncFrom != "" {
				if _, err := time.Parse("2006-01-02", syncFrom); err != nil {
					a.handleError(renderer, "cache.sync", errors.New(errors.InvalidArguments, "--from must be a date in YYYY-MM-DD format", errors.CatValidation, false, err), start)
					return
				}
			}

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "cache.sync", err.(*errors.Error), start)
				return
			}

			cfg, _ := config.Load()
			cacheStore, err := cache.NewStore(cfg.CachePath)
			if err != nil {
				a.handleError(renderer, "cache.sync", errors.New(errors.InternalError, "failed to open cache", errors.CatInternal, false, err), start)
				return
			}

			renderer.PrintDiagnostic("Syncing accounts...")
			accounts, err := svc.ListAccounts(cmd.Context())
			if err != nil {
				a.handleError(renderer, "cache.sync", errors.New(errors.APIError, fmt.Sprintf("failed to sync accounts: %v", err), errors.CatAPI, false, err), start)
				return
			}
			var cacheAccs []cache.Account
			for _, acc := range accounts {
				updatedAt, err := parseCacheDate(acc.UpdatedAt)
				if err != nil {
					a.handleError(renderer, "cache.sync", errors.New(errors.APISchemaChanged, "failed to parse account updated_at", errors.CatAPI, false, err), start)
					return
				}
				cacheAccs = append(cacheAccs, cache.Account{
					ID:             acc.ID,
					DisplayName:    acc.DisplayName,
					AccountType:    acc.AccountType,
					DisplayBalance: acc.DisplayBalance,
					UpdatedAt:      updatedAt,
				})
			}
			if err := cacheStore.SaveAccounts(cacheAccs); err != nil {
				a.handleError(renderer, "cache.sync", errors.New(errors.InternalError, "failed to save accounts to cache", errors.CatInternal, false, err), start)
				return
			}

			renderer.PrintDiagnostic("Syncing transactions...")
			txs, _, err := svc.ListTransactions(cmd.Context(), monarch.ListTransactionsOptions{Limit: 1000, StartDate: syncFrom})
			if err != nil {
				a.handleError(renderer, "cache.sync", errors.New(errors.APIError, fmt.Sprintf("failed to sync transactions: %v", err), errors.CatAPI, false, err), start)
				return
			}
			var cacheTxs []cache.Transaction
			for _, t := range txs {
				date, err := time.Parse("2006-01-02", t.Date)
				if err != nil {
					a.handleError(renderer, "cache.sync", errors.New(errors.APISchemaChanged, "failed to parse transaction date", errors.CatAPI, false, err), start)
					return
				}
				cacheTxs = append(cacheTxs, cache.Transaction{
					ID:        t.ID,
					Date:      date,
					Amount:    t.Amount,
					Merchant:  t.Merchant,
					Category:  t.Category,
					Notes:     t.Notes,
					AccountID: t.AccountID,
				})
			}
			if err := cacheStore.SaveTransactions(cacheTxs); err != nil {
				a.handleError(renderer, "cache.sync", errors.New(errors.InternalError, "failed to save transactions to cache", errors.CatInternal, false, err), start)
				return
			}

			if jsonMode {
				env := output.NewEnvelope("cache.sync", profile, output.SchemaVersion, "", map[string]string{"status": "sync complete"}, time.Since(start))
				renderer.RenderSuccess(env)
			} else {
				fmt.Println("Sync complete.")
			}
		},
	}
	cmd.Flags().StringVar(&syncFrom, "from", "", "sync transactions from date (YYYY-MM-DD)")
	return cmd
}

func (a *App) buildCacheSearch() *cobra.Command {
	return &cobra.Command{
		Use:   "search <query>",
		Short: "Search transactions in local cache",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, jsonMode, pretty)

			cfg, _ := config.Load()
			cacheStore, err := cache.NewStore(cfg.CachePath)
			if err != nil {
				a.handleError(renderer, "cache.search", errors.New(errors.InternalError, "failed to open cache", errors.CatInternal, false, err), start)
				return
			}

			txs, err := cacheStore.SearchTransactions(args[0])
			if err != nil {
				a.handleError(renderer, "cache.search", errors.New(errors.InternalError, "search failed", errors.CatInternal, false, err), start)
				return
			}

			if jsonMode {
				env := output.NewEnvelope("cache.search", profile, output.SchemaVersion, "", txs, time.Since(start))
				renderer.RenderSuccess(env)
			} else {
				fmt.Printf("%-12s %-20s %-15s %10s %s\n", "DATE", "MERCHANT", "CATEGORY", "AMOUNT", "NOTES")
				for _, t := range txs {
					fmt.Printf("%-12s %-20s %-15s %10.2f %s\n", t.Date.Format("2006-01-02"), t.Merchant, t.Category, t.Amount, t.Notes)
				}
				fmt.Printf("\nTotal matches: %d\n", len(txs))
			}
		},
	}
}

func (a *App) buildCacheStats() *cobra.Command {
	return &cobra.Command{
		Use:   "stats",
		Short: "Show cache statistics",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, jsonMode, pretty)

			cfg, _ := config.Load()
			cacheStore, err := cache.NewStore(cfg.CachePath)
			if err != nil {
				a.handleError(renderer, "cache.stats", errors.New(errors.InternalError, "failed to open cache", errors.CatInternal, false, err), start)
				return
			}

			stats, _ := cacheStore.GetStats()

			if jsonMode {
				env := output.NewEnvelope("cache.stats", profile, output.SchemaVersion, "", stats, time.Since(start))
				renderer.RenderSuccess(env)
			} else {
				fmt.Println("Cache Statistics")
				for k, v := range stats {
					fmt.Printf("%s: %d\n", k, v)
				}
			}
		},
	}
}

func (a *App) buildCacheCleanup() *cobra.Command {
	var cleanupBefore string
	cmd := &cobra.Command{
		Use:   "cleanup",
		Short: "Clean up old transactions from cache",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, jsonMode, pretty)

			if cleanupBefore == "" {
				a.handleError(renderer, "cache.cleanup", errors.New(errors.InvalidArguments, "--before is required", errors.CatValidation, false, nil), start)
				return
			}
			if _, err := time.Parse("2006-01-02", cleanupBefore); err != nil {
				a.handleError(renderer, "cache.cleanup", errors.New(errors.InvalidArguments, "--before must be a date in YYYY-MM-DD format", errors.CatValidation, false, err), start)
				return
			}

			cfg, _ := config.Load()
			store, err := cache.NewStore(cfg.CachePath)
			if err != nil {
				a.handleError(renderer, "cache.cleanup", errors.New(errors.InternalError, "failed to open cache", errors.CatInternal, false, err), start)
				return
			}

			affected, err := store.Cleanup(cleanupBefore)
			if err != nil {
				a.handleError(renderer, "cache.cleanup", errors.New(errors.InternalError, "failed to cleanup cache", errors.CatInternal, false, err), start)
				return
			}

			if jsonMode {
				env := output.NewEnvelope("cache.cleanup", profile, output.SchemaVersion, "", map[string]int64{"deleted": affected}, time.Since(start))
				renderer.RenderSuccess(env)
			} else {
				fmt.Printf("Deleted %d transactions from cache.\n", affected)
			}
		},
	}
	cmd.Flags().StringVar(&cleanupBefore, "before", "", "delete transactions before date (YYYY-MM-DD)")
	cmd.MarkFlagRequired("before")
	return cmd
}
