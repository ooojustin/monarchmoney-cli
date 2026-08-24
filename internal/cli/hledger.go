package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/thedavidweng/monarchmoney-cli/internal/cache"
	"github.com/thedavidweng/monarchmoney-cli/internal/errors"
	"github.com/thedavidweng/monarchmoney-cli/internal/hledger"
	"github.com/thedavidweng/monarchmoney-cli/internal/output"
)

func (a *App) buildHledgerCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "hledger",
		Short:   "Plain-text ledger backups for hledger",
		GroupID: "utility",
		Example: "  monarch hledger backup",
	}
	cmd.AddCommand(a.buildHledgerBackupCommand())
	return cmd
}

func (a *App) buildHledgerBackupCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "backup [FILE]",
		Short: "Regenerate a complete hledger journal from the local cache",
		Long: `Regenerate a complete hledger journal from the local cache.

The journal is rewritten from scratch on every run as a disposable derived
artifact with zero sync state; Monarch stays the source of truth. Keep
handwritten annotations in your own journal that includes this file.

The journal covers all accounts (including hidden and closed ones), the full
transaction history, closing balance assertions for every account, and
investment holdings as opening positions. Pending transactions are excluded.
Accounts whose cached history does not reconcile to their Monarch balance
(typical when an institution's feed predates your history) get a deterministic
opening-balance entry through equity:monarch:opening; audit those with
'hledger reg equity:monarch:opening'.

Reads only from the local cache, never the network. Run 'monarch cache sync
--all' first for archive-complete history. Configure 'backup_path' in your
config file to make every 'cache sync' regenerate this journal automatically.
The journal path must differ from the cache_path.`,
		Args: cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)

			path := "./monarch.journal"
			if len(args) == 1 {
				path = args[0]
			}
			if err := validateJournalPath(path, a.Config.CachePath); err != nil {
				a.handleError(renderer, "hledger.backup", err, start)
				return
			}

			store, ok := a.openCacheStore(renderer, "hledger.backup", start, false)
			if !ok {
				return
			}
			defer store.Close()

			result, err := writeJournal(store, path)
			if err != nil {
				a.handleError(renderer, "hledger.backup", errors.New(errors.InternalError, "failed to write journal", errors.CatInternal, false, err), start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("hledger.backup", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, map[string]any{
					"status":       "backup complete",
					"file":         path,
					"accounts":     result.accounts,
					"transactions": result.transactions,
					"holdings":     result.holdings,
				}, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Wrote %s (%d accounts, %d transactions, %d holdings).\n", path, result.accounts, result.transactions, result.holdings)
		},
	}
}

type backupResult struct {
	accounts     int
	transactions int
	holdings     int
}

func writeJournal(store *cache.Store, path string) (backupResult, error) {
	accounts, err := store.Accounts()
	if err != nil {
		return backupResult{}, err
	}
	txs, err := store.Transactions()
	if err != nil {
		return backupResult{}, err
	}
	holdings, err := store.Holdings()
	if err != nil {
		return backupResult{}, err
	}

	journal := hledger.Generate(&hledger.Data{
		Accounts:     accounts,
		Transactions: txs,
		Holdings:     holdings,
		Anchor:       backupAnchor(store, txs),
	})
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return backupResult{}, err
	}
	tmpPath := tmp.Name()
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err := tmp.WriteString(journal); err != nil {
		_ = tmp.Close()
		return backupResult{}, err
	}
	if err := tmp.Close(); err != nil {
		return backupResult{}, err
	}
	if err := os.Chmod(tmpPath, 0o600); err != nil {
		return backupResult{}, err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return backupResult{}, err
	}
	committed = true
	return backupResult{accounts: len(accounts), transactions: len(txs), holdings: len(holdings)}, nil
}

func validateJournalPath(journalPath, cachePath string) *errors.Error {
	journalAbs, err := filepath.Abs(journalPath)
	if err != nil {
		return errors.New(errors.InvalidArguments, "invalid journal path", errors.CatValidation, false, err)
	}
	cacheAbs, err := filepath.Abs(cachePath)
	if err != nil {
		return errors.New(errors.InvalidArguments, "invalid cache path", errors.CatValidation, false, err)
	}
	same := journalAbs == cacheAbs
	journalInfo, journalErr := os.Stat(journalPath)
	cacheInfo, cacheErr := os.Stat(cachePath)
	if journalErr == nil && cacheErr == nil {
		same = same || os.SameFile(journalInfo, cacheInfo)
	}
	if same {
		return errors.New(errors.InvalidArguments, "journal path must differ from cache path", errors.CatValidation, false, nil)
	}
	return nil
}

func backupAnchor(store *cache.Store, txs []cache.Transaction) time.Time {
	var latest time.Time
	for i := range txs {
		if txs[i].Date.After(latest) {
			latest = txs[i].Date
		}
	}
	if !latest.IsZero() {
		return latest.AddDate(0, 0, 1)
	}
	meta, err := store.LastSync()
	if err != nil || meta == nil {
		return time.Time{}
	}
	y, m, d := meta.SyncedAt.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
