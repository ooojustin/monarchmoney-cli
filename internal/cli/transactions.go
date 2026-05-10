package cli

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/thedavidweng/monarchmoney-cli/internal/audit"
	"github.com/thedavidweng/monarchmoney-cli/internal/errors"
	"github.com/thedavidweng/monarchmoney-cli/internal/monarch"
	"github.com/thedavidweng/monarchmoney-cli/internal/output"
	"github.com/thedavidweng/monarchmoney-cli/internal/safety"
)

func (a *App) buildTransactionsCommands(parent *cobra.Command) {
	var (
		txStartDate string
		txEndDate   string
	)
	resolveDates := func() (string, string) {
		return txStartDate, txEndDate
	}

	txCmd := &cobra.Command{
		Use:   "transactions",
		Short: "Manage Monarch Money transactions",
	}
	txCmd.PersistentFlags().StringVar(&txStartDate, "from", "", "start date (YYYY-MM-DD)")
	txCmd.PersistentFlags().StringVar(&txEndDate, "to", "", "end date (YYYY-MM-DD)")

	txCmd.AddCommand(a.buildTransactionsList(resolveDates))
	txCmd.AddCommand(a.buildTransactionsSearch(resolveDates))
	txCmd.AddCommand(a.buildTransactionsShow())
	txCmd.AddCommand(a.buildTransactionsSummary(resolveDates))
	txCmd.AddCommand(a.buildTransactionsDuplicates())
	txCmd.AddCommand(a.buildTransactionsSplits())
	txCmd.AddCommand(a.buildTransactionsExport(resolveDates))
	txCmd.AddCommand(a.buildTransactionsUpdate())
	txCmd.AddCommand(a.buildTransactionsDelete())
	txCmd.AddCommand(a.buildTransactionsCreate())
	txCmd.AddCommand(a.buildTransactionsSplit())
	txCmd.AddCommand(a.buildTransactionsBulkCategorize())

	tagsCmd := &cobra.Command{Use: "tags", Short: "Manage transaction tags"}
	tagsCmd.AddCommand(a.buildTransactionsTagsSet())
	tagsCmd.AddCommand(a.buildTransactionsTagsAdd())
	tagsCmd.AddCommand(a.buildTransactionsTagsClear())
	txCmd.AddCommand(tagsCmd)

	attachmentsCmd := &cobra.Command{Use: "attachments", Short: "Manage transaction attachments"}
	attachmentsCmd.AddCommand(a.buildTransactionsAttachmentsList())
	attachmentsCmd.AddCommand(a.buildTransactionsAttachmentsUpload())
	attachmentsCmd.AddCommand(a.buildTransactionsAttachmentsDownload())
	txCmd.AddCommand(attachmentsCmd)

	parent.AddCommand(txCmd)
}

func (a *App) buildTransactionsList(resolveDates func() (string, string)) *cobra.Command {
	var (
		limit             int
		offset            int
		filterCategoryIDs []string
		filterAccountIDs  []string
		filterTagIDs      []string
		filterNeedsReview bool
		filterHasNotes    bool
		filterIsSplit     bool
		filterIsRecurring bool
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List transactions",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, a.Flags.JSONMode, a.Flags.Pretty)

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "transactions.list", err.(*errors.Error), start)
				return
			}

			txStartDate, txEndDate := resolveDates()
			opts := monarch.ListTransactionsOptions{
				Limit:       limit,
				Offset:      offset,
				StartDate:   txStartDate,
				EndDate:     txEndDate,
				CategoryIDs: filterCategoryIDs,
				AccountIDs:  filterAccountIDs,
				TagIDs:      filterTagIDs,
			}
			if cmd.Flags().Changed("needs-review") {
				opts.NeedsReview = &filterNeedsReview
			}
			if cmd.Flags().Changed("has-notes") {
				opts.HasNotes = &filterHasNotes
			}
			if cmd.Flags().Changed("is-split") {
				opts.IsSplit = &filterIsSplit
			}
			if cmd.Flags().Changed("is-recurring") {
				opts.IsRecurring = &filterIsRecurring
			}

			txs, total, err := svc.ListTransactions(cmd.Context(), opts)
			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to list transactions", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "transactions.list", cliErr, start)
				return
			}

			if a.Flags.JSONMode {
				data := map[string]interface{}{"transactions": txs, "total": total}
				env := a.envelopeWithWarnings("transactions.list", data, start, "uses legacy Monarch GraphQL root field: allTransactions")
				renderer.RenderSuccess(env)
			} else {
				fmt.Printf("%-12s %-20s %-15s %10s %s\n", "DATE", "MERCHANT", "CATEGORY", "AMOUNT", "NOTES")
				for _, t := range txs {
					fmt.Printf("%-12s %-20s %-15s %10.2f %s\n", t.Date, t.Merchant, t.Category, t.Amount, t.Notes)
				}
				fmt.Printf("\nTotal transactions: %d\n", total)
			}
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 100, "maximum number of transactions to return")
	cmd.Flags().IntVar(&offset, "offset", 0, "number of transactions to skip")
	cmd.Flags().StringSliceVar(&filterCategoryIDs, "category-id", nil, "filter by category ID (repeatable)")
	cmd.Flags().StringSliceVar(&filterAccountIDs, "account-id", nil, "filter by account ID (repeatable)")
	cmd.Flags().StringSliceVar(&filterTagIDs, "tag-id", nil, "filter by tag ID (repeatable)")
	cmd.Flags().BoolVar(&filterNeedsReview, "needs-review", false, "filter for transactions needing review")
	cmd.Flags().BoolVar(&filterHasNotes, "has-notes", false, "filter for transactions with notes")
	cmd.Flags().BoolVar(&filterIsSplit, "is-split", false, "filter for split transactions")
	cmd.Flags().BoolVar(&filterIsRecurring, "is-recurring", false, "filter for recurring transactions")
	return cmd
}

func (a *App) buildTransactionsSearch(resolveDates func() (string, string)) *cobra.Command {
	var (
		limit  int
		offset int
	)
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search transactions",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, a.Flags.JSONMode, a.Flags.Pretty)

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "transactions.search", err.(*errors.Error), start)
				return
			}

			txStartDate, txEndDate := resolveDates()
			txs, total, err := svc.ListTransactions(cmd.Context(), monarch.ListTransactionsOptions{
				Limit: limit, Offset: offset, Search: args[0], StartDate: txStartDate, EndDate: txEndDate,
			})
			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to search transactions", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "transactions.search", cliErr, start)
				return
			}

			if a.Flags.JSONMode {
				data := map[string]interface{}{"transactions": txs, "total": total}
				env := a.envelopeWithWarnings("transactions.search", data, start, "uses legacy Monarch GraphQL root field: allTransactions")
				renderer.RenderSuccess(env)
			} else {
				fmt.Printf("%-12s %-20s %-15s %10s %s\n", "DATE", "MERCHANT", "CATEGORY", "AMOUNT", "NOTES")
				for _, t := range txs {
					fmt.Printf("%-12s %-20s %-15s %10.2f %s\n", t.Date, t.Merchant, t.Category, t.Amount, t.Notes)
				}
				fmt.Printf("\nTotal matches: %d\n", total)
			}
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 100, "maximum number of transactions to return")
	cmd.Flags().IntVar(&offset, "offset", 0, "number of transactions to skip")
	return cmd
}

func (a *App) buildTransactionsShow() *cobra.Command {
	return &cobra.Command{
		Use:   "show <transaction-id>",
		Short: "Show detailed information for a transaction",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, a.Flags.JSONMode, a.Flags.Pretty)

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "transactions.show", err.(*errors.Error), start)
				return
			}

			tx, err := svc.GetTransaction(cmd.Context(), args[0])
			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to get transaction", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "transactions.show", cliErr, start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("transactions.show", a.Flags.Profile, output.SchemaVersion, "", tx, time.Since(start))
				renderer.RenderSuccess(env)
			} else {
				fmt.Printf("ID:       %s\n", tx.ID)
				fmt.Printf("Date:     %s\n", tx.Date)
				fmt.Printf("Merchant: %s\n", tx.Merchant)
				fmt.Printf("Category: %s\n", tx.Category)
				fmt.Printf("Amount:   %.2f\n", tx.Amount)
				fmt.Printf("Notes:    %s\n", tx.Notes)
			}
		},
	}
}

func (a *App) buildTransactionsSummary(resolveDates func() (string, string)) *cobra.Command {
	return &cobra.Command{
		Use:   "summary",
		Short: "Get transaction summary",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, a.Flags.JSONMode, a.Flags.Pretty)

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "transactions.summary", err.(*errors.Error), start)
				return
			}

			txStartDate, txEndDate := resolveDates()
			summary, err := svc.GetTransactionsSummary(cmd.Context(), txStartDate, txEndDate)
			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to get transaction summary", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "transactions.summary", cliErr, start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("transactions.summary", a.Flags.Profile, output.SchemaVersion, "", summary, time.Since(start))
				renderer.RenderSuccess(env)
			} else {
				fmt.Println("Transaction Summary")
			}
		},
	}
}

func (a *App) buildTransactionsDuplicates() *cobra.Command {
	return &cobra.Command{
		Use:   "duplicates",
		Short: "Find duplicate transactions",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, a.Flags.JSONMode, a.Flags.Pretty)

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "transactions.duplicates", err.(*errors.Error), start)
				return
			}

			now := time.Now()
			startDate := now.Format("2006-01-02")
			endDate := time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, time.UTC).Format("2006-01-02")

			txs, err := svc.GetDuplicateTransactions(cmd.Context(), startDate, endDate)
			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to find duplicates", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "transactions.duplicates", cliErr, start)
				return
			}

			if a.Flags.JSONMode {
				env := a.envelopeWithWarnings("transactions.duplicates", txs, start, "uses legacy Monarch GraphQL root field: allTransactions")
				renderer.RenderSuccess(env)
			} else {
				fmt.Printf("%-12s %-20s %10s %s\n", "DATE", "MERCHANT", "AMOUNT", "ID")
				for _, t := range txs {
					fmt.Printf("%-12s %-20s %10.2f %s\n", t.Date, t.Merchant, t.Amount, t.ID)
				}
			}
		},
	}
}

func (a *App) buildTransactionsSplits() *cobra.Command {
	return &cobra.Command{
		Use:   "splits <transaction-id>",
		Short: "Get splits for a transaction",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, a.Flags.JSONMode, a.Flags.Pretty)

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "transactions.splits", err.(*errors.Error), start)
				return
			}

			splits, err := svc.GetTransactionSplits(cmd.Context(), args[0])
			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to get splits", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "transactions.splits", cliErr, start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("transactions.splits", a.Flags.Profile, output.SchemaVersion, "", splits, time.Since(start))
				renderer.RenderSuccess(env)
			} else {
				fmt.Printf("%-20s %10s %s\n", "CATEGORY", "AMOUNT", "NOTES")
				for _, s := range splits {
					fmt.Printf("%-20s %10.2f %s\n", s.Category, s.Amount, s.Notes)
				}
			}
		},
	}
}

func (a *App) buildTransactionsExport(resolveDates func() (string, string)) *cobra.Command {
	var (
		limit      int
		offset     int
		format     string
		outputFile string
	)
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export transactions",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, a.Flags.JSONMode, a.Flags.Pretty)

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "transactions.export", err.(*errors.Error), start)
				return
			}

			txStartDate, txEndDate := resolveDates()
			txs, _, err := svc.ListTransactions(cmd.Context(), monarch.ListTransactionsOptions{
				Limit: limit, Offset: offset, StartDate: txStartDate, EndDate: txEndDate,
			})
			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to list transactions", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "transactions.export", cliErr, start)
				return
			}

			var out io.Writer = a.Deps.Stdout
			if outputFile != "" {
				f, err := os.Create(outputFile)
				if err != nil {
					a.handleError(renderer, "transactions.export", errors.New(errors.InternalError, "failed to create output file", errors.CatInternal, false, err), start)
					return
				}
				defer f.Close()
				out = f
			}

			if format == "csv" {
				if err := monarch.ExportTransactionsCSV(txs, out); err != nil {
					a.handleError(renderer, "transactions.export", errors.New(errors.InternalError, "failed to export CSV", errors.CatInternal, false, err), start)
					return
				}
			} else {
				env := output.NewEnvelope("transactions.export", a.Flags.Profile, output.SchemaVersion, "", txs, time.Since(start))
				renderer.RenderSuccess(env)
			}
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 1000, "maximum number of transactions to export")
	cmd.Flags().IntVar(&offset, "offset", 0, "number of transactions to skip")
	cmd.Flags().StringVar(&format, "format", "json", "export format (json or csv)")
	cmd.Flags().StringVar(&outputFile, "output", "", "output file path")
	return cmd
}

func (a *App) buildTransactionsUpdate() *cobra.Command {
	var (
		txNotes           string
		txCategoryID      string
		txAmount          float64
		txDate            string
		txMerchant        string
		txHideFromReports bool
		txNeedsReview     bool
		txMarkReviewed    bool
	)
	cmd := &cobra.Command{
		Use:   "update <transaction-id>",
		Short: "Update a transaction",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, a.Flags.JSONMode, a.Flags.Pretty)
			logger := a.Deps.NewAuditLogger()
			id := args[0]

			if err := safety.Check(safety.TierMutation, a.Flags.ReadOnly, a.Flags.DryRun, a.Flags.Confirm); err != nil {
				a.handleError(renderer, "transactions.update", err.(*errors.Error), start)
				return
			}

			var notes *string
			if cmd.Flags().Changed("notes") {
				notes = &txNotes
			}
			var categoryID *string
			if cmd.Flags().Changed("category") {
				categoryID = &txCategoryID
			}
			var amount *float64
			if cmd.Flags().Changed("amount") {
				amount = &txAmount
			}
			var date *string
			if cmd.Flags().Changed("date") {
				date = &txDate
			}
			var merchantName *string
			if cmd.Flags().Changed("merchant") {
				merchantName = &txMerchant
			}
			var hideFromReports *bool
			if cmd.Flags().Changed("hide-from-reports") {
				hideFromReports = &txHideFromReports
			}
			var needsReview *bool
			if cmd.Flags().Changed("needs-review") {
				needsReview = &txNeedsReview
			}
			if txMarkReviewed {
				f := false
				needsReview = &f
			}

			if a.Flags.DryRun {
				plan := safety.NewPlan()
				plan.Add("transactions.update", id, nil, map[string]interface{}{"notes": notes, "categoryId": categoryID, "amount": amount, "date": date, "merchant": merchantName, "hideFromReports": hideFromReports, "needsReview": needsReview})
				env := output.NewEnvelope("transactions.update", a.Flags.Profile, output.SchemaVersion, "", plan, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "transactions.update", err.(*errors.Error), start)
				return
			}

			tx, err := svc.UpdateTransaction(cmd.Context(), id, notes, categoryID, amount, date, merchantName, hideFromReports, needsReview)
			result := "success"
			var errCode string
			if err != nil {
				result = "failure"
				if e, ok := err.(*errors.Error); ok {
					errCode = string(e.Code)
				}
			}
			logger.Log(&audit.Record{Command: "transactions.update", ResourceID: id, DryRun: a.Flags.DryRun, Confirmed: a.Flags.Confirm, Profile: a.Flags.Profile, Result: result, ErrorCode: errCode})

			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to update transaction", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "transactions.update", cliErr, start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("transactions.update", a.Flags.Profile, output.SchemaVersion, "", tx, time.Since(start))
				renderer.RenderSuccess(env)
			} else {
				fmt.Printf("Successfully updated transaction %s.\n", tx.ID)
			}
		},
	}
	cmd.Flags().StringVar(&txNotes, "notes", "", "transaction notes")
	cmd.Flags().StringVar(&txCategoryID, "category", "", "transaction category ID")
	cmd.Flags().Float64Var(&txAmount, "amount", 0, "transaction amount")
	cmd.Flags().StringVar(&txDate, "date", "", "transaction date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&txMerchant, "merchant", "", "merchant name")
	cmd.Flags().BoolVar(&txHideFromReports, "hide-from-reports", false, "hide transaction from reports")
	cmd.Flags().BoolVar(&txNeedsReview, "needs-review", false, "mark transaction as needing review")
	cmd.Flags().BoolVar(&txMarkReviewed, "mark-reviewed", false, "mark transaction as reviewed (shortcut for --needs-review=false)")
	return cmd
}

func (a *App) buildTransactionsDelete() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <transaction-id>",
		Short: "Delete a transaction",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, a.Flags.JSONMode, a.Flags.Pretty)
			logger := a.Deps.NewAuditLogger()
			id := args[0]

			if err := safety.Check(safety.TierDestructive, a.Flags.ReadOnly, a.Flags.DryRun, a.Flags.Confirm); err != nil {
				a.handleError(renderer, "transactions.delete", err.(*errors.Error), start)
				return
			}

			if a.Flags.DryRun {
				plan := safety.NewPlan()
				plan.Add("transactions.delete", id, nil, nil)
				env := output.NewEnvelope("transactions.delete", a.Flags.Profile, output.SchemaVersion, "", plan, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "transactions.delete", err.(*errors.Error), start)
				return
			}

			err = svc.DeleteTransaction(cmd.Context(), id)
			result := "success"
			var errCode string
			if err != nil {
				result = "failure"
				if e, ok := err.(*errors.Error); ok {
					errCode = string(e.Code)
				}
			}
			logger.Log(&audit.Record{Command: "transactions.delete", ResourceID: id, DryRun: a.Flags.DryRun, Confirmed: a.Flags.Confirm, Profile: a.Flags.Profile, Result: result, ErrorCode: errCode})

			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to delete transaction", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "transactions.delete", cliErr, start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("transactions.delete", a.Flags.Profile, output.SchemaVersion, "", map[string]string{"status": "deleted"}, time.Since(start))
				renderer.RenderSuccess(env)
			} else {
				fmt.Printf("Successfully deleted transaction %s.\n", id)
			}
		},
	}
}

func (a *App) buildTransactionsCreate() *cobra.Command {
	var (
		txAmount     float64
		txMerchant   string
		txDate       string
		txCategoryID string
		txAccountID  string
		txNotes      string
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a transaction",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, a.Flags.JSONMode, a.Flags.Pretty)
			logger := a.Deps.NewAuditLogger()

			if err := safety.Check(safety.TierMutation, a.Flags.ReadOnly, a.Flags.DryRun, a.Flags.Confirm); err != nil {
				a.handleError(renderer, "transactions.create", err.(*errors.Error), start)
				return
			}

			if txDate == "" {
				txDate = time.Now().Format("2006-01-02")
			}

			if a.Flags.DryRun {
				plan := safety.NewPlan()
				plan.Add("transactions.create", "", nil, map[string]interface{}{"amount": txAmount, "merchant": txMerchant, "date": txDate, "categoryId": txCategoryID})
				env := output.NewEnvelope("transactions.create", a.Flags.Profile, output.SchemaVersion, "", plan, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "transactions.create", err.(*errors.Error), start)
				return
			}

			tx, err := svc.CreateTransaction(cmd.Context(), txAmount, txMerchant, txDate, txCategoryID, txAccountID, txNotes)
			result := "success"
			var errCode string
			if err != nil {
				result = "failure"
				if e, ok := err.(*errors.Error); ok {
					errCode = string(e.Code)
				}
			}
			logger.Log(&audit.Record{Command: "transactions.create", DryRun: a.Flags.DryRun, Confirmed: a.Flags.Confirm, Profile: a.Flags.Profile, Result: result, ErrorCode: errCode})

			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to create transaction", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "transactions.create", cliErr, start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("transactions.create", a.Flags.Profile, output.SchemaVersion, "", tx, time.Since(start))
				renderer.RenderSuccess(env)
			} else {
				fmt.Printf("Successfully created transaction %s.\n", tx.ID)
			}
		},
	}
	cmd.Flags().Float64Var(&txAmount, "amount", 0, "transaction amount")
	cmd.Flags().StringVar(&txMerchant, "merchant", "", "merchant name")
	cmd.Flags().StringVar(&txDate, "date", "", "transaction date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&txCategoryID, "category", "", "category ID")
	cmd.Flags().StringVar(&txAccountID, "account", "", "account ID")
	cmd.Flags().StringVar(&txNotes, "notes", "", "transaction notes")
	cmd.MarkFlagRequired("amount")
	cmd.MarkFlagRequired("merchant")
	cmd.MarkFlagRequired("category")
	return cmd
}

func (a *App) buildTransactionsSplit() *cobra.Command {
	var splitFile string
	cmd := &cobra.Command{
		Use:   "split <transaction-id>",
		Short: "Split a transaction",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, a.Flags.JSONMode, a.Flags.Pretty)
			logger := a.Deps.NewAuditLogger()
			id := args[0]

			if err := safety.Check(safety.TierMutation, a.Flags.ReadOnly, a.Flags.DryRun, a.Flags.Confirm); err != nil {
				a.handleError(renderer, "transactions.split", err.(*errors.Error), start)
				return
			}

			data, err := os.ReadFile(splitFile)
			if err != nil {
				a.handleError(renderer, "transactions.split", errors.New(errors.ValidationFailed, "failed to read split file: "+err.Error(), errors.CatValidation, false, err), start)
				return
			}

			var splits []monarch.SplitInput
			if err := a.Deps.JSONUnmarshal(data, &splits); err != nil {
				a.handleError(renderer, "transactions.split", errors.New(errors.ValidationFailed, "invalid split JSON: "+err.Error(), errors.CatValidation, false, err), start)
				return
			}

			if a.Flags.DryRun {
				plan := safety.NewPlan()
				plan.Add("transactions.split", id, nil, map[string]interface{}{"splits": splits})
				env := output.NewEnvelope("transactions.split", a.Flags.Profile, output.SchemaVersion, "", plan, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "transactions.split", err.(*errors.Error), start)
				return
			}

			err = svc.UpdateTransactionSplits(cmd.Context(), id, splits)
			result := "success"
			var errCode string
			if err != nil {
				result = "failure"
				if e, ok := err.(*errors.Error); ok {
					errCode = string(e.Code)
				}
			}
			logger.Log(&audit.Record{Command: "transactions.split", ResourceID: id, DryRun: a.Flags.DryRun, Confirmed: a.Flags.Confirm, Profile: a.Flags.Profile, Result: result, ErrorCode: errCode})

			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to split transaction", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "transactions.split", cliErr, start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("transactions.split", a.Flags.Profile, output.SchemaVersion, "", map[string]string{"status": "split updated"}, time.Since(start))
				renderer.RenderSuccess(env)
			} else {
				fmt.Printf("Successfully split transaction %s.\n", id)
			}
		},
	}
	cmd.Flags().StringVar(&splitFile, "file", "", "JSON file with split data")
	cmd.MarkFlagRequired("file")
	return cmd
}

func (a *App) buildTransactionsTagsSet() *cobra.Command {
	var tagIDs []string
	cmd := &cobra.Command{
		Use:   "set <transaction-id>",
		Short: "Set tags for a transaction",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, a.Flags.JSONMode, a.Flags.Pretty)
			logger := a.Deps.NewAuditLogger()
			id := args[0]

			if err := safety.Check(safety.TierMutation, a.Flags.ReadOnly, a.Flags.DryRun, a.Flags.Confirm); err != nil {
				a.handleError(renderer, "transactions.tags.set", err.(*errors.Error), start)
				return
			}

			if a.Flags.DryRun {
				plan := safety.NewPlan()
				plan.Add("transactions.tags.set", id, nil, map[string]interface{}{"tag_ids": tagIDs})
				env := output.NewEnvelope("transactions.tags.set", a.Flags.Profile, output.SchemaVersion, "", plan, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "transactions.tags.set", err.(*errors.Error), start)
				return
			}

			err = svc.SetTransactionTags(cmd.Context(), id, tagIDs)
			result := "success"
			var errCode string
			if err != nil {
				result = "failure"
				if e, ok := err.(*errors.Error); ok {
					errCode = string(e.Code)
				}
			}
			logger.Log(&audit.Record{Command: "transactions.tags.set", ResourceID: id, DryRun: a.Flags.DryRun, Confirmed: a.Flags.Confirm, Profile: a.Flags.Profile, Result: result, ErrorCode: errCode})

			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to set transaction tags", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "transactions.tags.set", cliErr, start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("transactions.tags.set", a.Flags.Profile, output.SchemaVersion, "", map[string]string{"status": "tags set"}, time.Since(start))
				renderer.RenderSuccess(env)
			} else {
				fmt.Printf("Successfully set tags for transaction %s.\n", id)
			}
		},
	}
	cmd.Flags().StringSliceVar(&tagIDs, "tag", []string{}, "tag IDs to set")
	cmd.MarkFlagRequired("tag")
	return cmd
}

func (a *App) buildTransactionsTagsAdd() *cobra.Command {
	var tagIDs []string
	cmd := &cobra.Command{
		Use:   "add <transaction-id>",
		Short: "Add tags to a transaction (appending to existing tags)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, a.Flags.JSONMode, a.Flags.Pretty)
			logger := a.Deps.NewAuditLogger()
			id := args[0]

			if err := safety.Check(safety.TierMutation, a.Flags.ReadOnly, a.Flags.DryRun, a.Flags.Confirm); err != nil {
				a.handleError(renderer, "transactions.tags.add", err.(*errors.Error), start)
				return
			}

			if len(tagIDs) == 0 {
				a.handleError(renderer, "transactions.tags.add", errors.New(errors.InvalidArguments, "--tag is required", errors.CatValidation, false, nil), start)
				return
			}

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "transactions.tags.add", err.(*errors.Error), start)
				return
			}

			tx, err := svc.GetTransaction(cmd.Context(), id)
			if err != nil {
				a.handleError(renderer, "transactions.tags.add", errors.New(errors.APIError, "failed to fetch current transaction", errors.CatAPI, false, err), start)
				return
			}

			existingTagIDs := make(map[string]bool)
			newTagIDs := []string{}

			for _, t := range tx.Tags {
				existingTagIDs[t.ID] = true
				newTagIDs = append(newTagIDs, t.ID)
			}

			for _, tid := range tagIDs {
				if !existingTagIDs[tid] {
					newTagIDs = append(newTagIDs, tid)
				}
			}

			if a.Flags.DryRun {
				plan := safety.NewPlan()
				plan.Add("transactions.tags.add", id, nil, map[string]interface{}{"tag_ids": newTagIDs})
				env := output.NewEnvelope("transactions.tags.add", a.Flags.Profile, output.SchemaVersion, "", plan, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}

			err = svc.SetTransactionTags(cmd.Context(), id, newTagIDs)
			result := "success"
			var errCode string
			if err != nil {
				result = "failure"
				if e, ok := err.(*errors.Error); ok {
					errCode = string(e.Code)
				}
			}
			logger.Log(&audit.Record{Command: "transactions.tags.add", ResourceID: id, DryRun: a.Flags.DryRun, Confirmed: a.Flags.Confirm, Profile: a.Flags.Profile, Result: result, ErrorCode: errCode})

			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to add transaction tags", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "transactions.tags.add", cliErr, start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("transactions.tags.add", a.Flags.Profile, output.SchemaVersion, "", map[string]string{"status": "tags added"}, time.Since(start))
				renderer.RenderSuccess(env)
			} else {
				fmt.Printf("Successfully added tags to transaction %s.\n", id)
			}
		},
	}
	cmd.Flags().StringSliceVar(&tagIDs, "tag", []string{}, "tag IDs to add")
	cmd.MarkFlagRequired("tag")
	return cmd
}

func (a *App) buildTransactionsTagsClear() *cobra.Command {
	return &cobra.Command{
		Use:   "clear <transaction-id>",
		Short: "Clear all tags for a transaction",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, a.Flags.JSONMode, a.Flags.Pretty)
			logger := a.Deps.NewAuditLogger()
			id := args[0]

			if err := safety.Check(safety.TierMutation, a.Flags.ReadOnly, a.Flags.DryRun, a.Flags.Confirm); err != nil {
				a.handleError(renderer, "transactions.tags.clear", err.(*errors.Error), start)
				return
			}

			if a.Flags.DryRun {
				plan := safety.NewPlan()
				plan.Add("transactions.tags.clear", id, nil, nil)
				env := output.NewEnvelope("transactions.tags.clear", a.Flags.Profile, output.SchemaVersion, "", plan, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "transactions.tags.clear", err.(*errors.Error), start)
				return
			}

			err = svc.SetTransactionTags(cmd.Context(), id, []string{})
			result := "success"
			var errCode string
			if err != nil {
				result = "failure"
				if e, ok := err.(*errors.Error); ok {
					errCode = string(e.Code)
				}
			}
			logger.Log(&audit.Record{Command: "transactions.tags.clear", ResourceID: id, DryRun: a.Flags.DryRun, Confirmed: a.Flags.Confirm, Profile: a.Flags.Profile, Result: result, ErrorCode: errCode})

			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to clear transaction tags", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "transactions.tags.clear", cliErr, start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("transactions.tags.clear", a.Flags.Profile, output.SchemaVersion, "", map[string]string{"status": "tags cleared"}, time.Since(start))
				renderer.RenderSuccess(env)
			} else {
				fmt.Printf("Successfully cleared tags for transaction %s.\n", id)
			}
		},
	}
}

func (a *App) buildTransactionsBulkCategorize() *cobra.Command {
	var (
		bulkTxIDs        []string
		bulkCategoryID   string
		bulkMarkReviewed bool
	)
	cmd := &cobra.Command{
		Use:   "bulk-categorize",
		Short: "Apply a category to multiple transactions",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, a.Flags.JSONMode, a.Flags.Pretty)
			logger := a.Deps.NewAuditLogger()

			if len(bulkTxIDs) == 0 {
				a.handleError(renderer, "transactions.bulk-categorize", errors.New(errors.InvalidArguments, "at least one --id is required", errors.CatValidation, false, nil), start)
				return
			}
			if bulkCategoryID == "" {
				a.handleError(renderer, "transactions.bulk-categorize", errors.New(errors.InvalidArguments, "--category-id is required", errors.CatValidation, false, nil), start)
				return
			}

			if err := safety.Check(safety.TierMutation, a.Flags.ReadOnly, a.Flags.DryRun, a.Flags.Confirm); err != nil {
				a.handleError(renderer, "transactions.bulk-categorize", err.(*errors.Error), start)
				return
			}

			if a.Flags.DryRun {
				plan := safety.NewPlan()
				for _, id := range bulkTxIDs {
					plan.Add("transactions.update", id, nil, map[string]interface{}{"categoryId": bulkCategoryID, "markReviewed": bulkMarkReviewed})
				}
				env := output.NewEnvelope("transactions.bulk-categorize", a.Flags.Profile, output.SchemaVersion, "", plan, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "transactions.bulk-categorize", err.(*errors.Error), start)
				return
			}

			var needsReview *bool
			if bulkMarkReviewed {
				f := false
				needsReview = &f
			}

			successes := 0
			var failures []string
			for _, txID := range bulkTxIDs {
				_, err := svc.UpdateTransaction(cmd.Context(), txID, nil, &bulkCategoryID, nil, nil, nil, nil, needsReview)
				if err != nil {
					failures = append(failures, txID+": "+err.Error())
				} else {
					successes++
				}
			}

			result := "success"
			if len(failures) > 0 && successes == 0 {
				result = "failure"
			} else if len(failures) > 0 {
				result = "partial"
			}
			logger.Log(&audit.Record{Command: "transactions.bulk-categorize", DryRun: a.Flags.DryRun, Confirmed: a.Flags.Confirm, Profile: a.Flags.Profile, Result: result})

			if a.Flags.JSONMode {
				data := map[string]interface{}{"total": len(bulkTxIDs), "successful": successes, "failed": len(failures), "errors": failures}
				env := output.NewEnvelope("transactions.bulk-categorize", a.Flags.Profile, output.SchemaVersion, "", data, time.Since(start))
				renderer.RenderSuccess(env)
			} else {
				fmt.Printf("Bulk categorize: %d/%d successful.\n", successes, len(bulkTxIDs))
				for _, f := range failures {
					fmt.Printf("  FAILED: %s\n", f)
				}
			}
		},
	}
	cmd.Flags().StringSliceVar(&bulkTxIDs, "id", nil, "transaction IDs to categorize (repeatable)")
	cmd.Flags().StringVar(&bulkCategoryID, "category-id", "", "category ID to apply")
	cmd.Flags().BoolVar(&bulkMarkReviewed, "mark-reviewed", true, "also mark transactions as reviewed")
	return cmd
}

func (a *App) buildTransactionsAttachmentsList() *cobra.Command {
	return &cobra.Command{
		Use:   "list <transaction-id>",
		Short: "List attachments for a transaction",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, a.Flags.JSONMode, a.Flags.Pretty)

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "transactions.attachments.list", err.(*errors.Error), start)
				return
			}

			attachments, err := svc.ListTransactionAttachments(cmd.Context(), args[0])
			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to list attachments", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "transactions.attachments.list", cliErr, start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("transactions.attachments.list", a.Flags.Profile, output.SchemaVersion, "", attachments, time.Since(start))
				renderer.RenderSuccess(env)
			} else {
				if len(attachments) == 0 {
					fmt.Println("No attachments found.")
				} else {
					fmt.Printf("%-36s %-20s %s\n", "ID", "FILENAME", "SIZE")
					for _, at := range attachments {
						fmt.Printf("%-36s %-20s %d bytes\n", at.ID, at.Filename, at.SizeBytes)
					}
				}
			}
		},
	}
}

func (a *App) buildTransactionsAttachmentsDownload() *cobra.Command {
	var (
		attachmentID string
		outputFile   string
	)
	cmd := &cobra.Command{
		Use:   "download <transaction-id>",
		Short: "Download an attachment for a transaction",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, a.Flags.JSONMode, a.Flags.Pretty)

			if attachmentID == "" {
				a.handleError(renderer, "transactions.attachments.download", errors.New(errors.InvalidArguments, "--id flag is required", errors.CatValidation, false, nil), start)
				return
			}

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "transactions.attachments.download", err.(*errors.Error), start)
				return
			}

			attachments, err := svc.ListTransactionAttachments(cmd.Context(), args[0])
			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to list attachments", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "transactions.attachments.download", cliErr, start)
				return
			}

			var targetURL, targetFilename string
			for _, at := range attachments {
				if at.ID == attachmentID {
					targetURL = at.URL
					targetFilename = at.Filename
					break
				}
			}
			if targetURL == "" {
				a.handleError(renderer, "transactions.attachments.download", errors.New(errors.ResourceNotFound, "attachment not found", errors.CatAPI, false, nil), start)
				return
			}

			outPath := outputFile
			if outPath == "" {
				outPath = targetFilename
			}

			f, err := os.Create(outPath)
			if err != nil {
				a.handleError(renderer, "transactions.attachments.download", errors.New(errors.InternalError, "failed to create output file: "+err.Error(), errors.CatInternal, false, err), start)
				return
			}
			defer f.Close()

			if err := svc.DownloadAttachment(cmd.Context(), targetURL, f); err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to download attachment", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "transactions.attachments.download", cliErr, start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("transactions.attachments.download", a.Flags.Profile, output.SchemaVersion, "", map[string]string{"status": "downloaded", "path": outPath}, time.Since(start))
				renderer.RenderSuccess(env)
			} else {
				fmt.Printf("Downloaded attachment to %s\n", outPath)
			}
		},
	}
	cmd.Flags().StringVar(&attachmentID, "id", "", "attachment ID")
	cmd.Flags().StringVar(&outputFile, "output", "", "output file path")
	return cmd
}

func (a *App) buildTransactionsAttachmentsUpload() *cobra.Command {
	return &cobra.Command{
		Use:   "upload <transaction-id> <file>",
		Short: "Upload an attachment for a transaction",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, a.Flags.JSONMode, a.Flags.Pretty)
			a.handleError(renderer, "transactions.attachments.upload", errors.New(errors.FEATURE_UNAVAILABLE, "transaction attachment upload is unavailable in the current Monarch API", errors.CatAPI, false, nil), start)
		},
	}
}
