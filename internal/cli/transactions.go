package cli

import (
	"context"
	"encoding/json"
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

var jsonUnmarshal = json.Unmarshal

var (
	limit        int
	offset       int
	format       string
	outputFile   string
	txNotes      string
	txCategoryID string
	attachmentID string
	txStartDate  string
	txEndDate    string
	txAmount     float64
	txMerchant   string
	txDate       string
	txAccountID  string
	splitFile    string
	tagIDs       []string

	filterCategoryIDs []string
	filterAccountIDs  []string
	filterTagIDs      []string
	filterNeedsReview bool
	filterHasNotes    bool
	filterIsSplit     bool
	filterIsRecurring bool
	filterPending     bool
	filterHideReports bool
	filterGoalIDs     []string

	txHideFromReports bool
	txNeedsReview     bool
	txMarkReviewed    bool

	bulkTxIDs        []string
	bulkCategoryID   string
	bulkMarkReviewed bool
)

var transactionsCmd = &cobra.Command{
	Use:     "transactions",
	Short:   "Manage Monarch Money transactions",
	GroupID: "core",
	Example: "  monarch transactions list --limit 10 --json\n  monarch transactions search \"Amazon\"\n  monarch transactions update <id> --category cat_food --dry-run",
}

var transactionsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List transactions",
	Run: func(cmd *cobra.Command, args []string) {
		var txs []monarch.Transaction
		var total int
		runWarn(cmd.Context(), "transactions.list", "failed to list transactions",
			[]string{"uses legacy Monarch GraphQL root field: allTransactions"},
			func(ctx context.Context, svc *monarch.Service) (map[string]any, error) {
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
				if cmd.Flags().Changed("pending") {
					opts.Pending = &filterPending
				}
				if cmd.Flags().Changed("hide-from-reports") {
					opts.HideFromReports = &filterHideReports
				}
				opts.GoalIDs = filterGoalIDs

				t, tot, err := svc.ListTransactions(ctx, &opts)
				if err != nil {
					return nil, err
				}
				txs, total = t, tot
				return map[string]any{"transactions": t, "total": tot}, nil
			},
			func(_ map[string]any) {
				fmt.Printf("%-12s %-20s %-15s %10s %s\n", "DATE", "MERCHANT", "CATEGORY", "AMOUNT", "NOTES")
				for _, t := range txs {
					fmt.Printf("%-12s %-20s %-15s %10.2f %s\n", t.Date, t.Merchant, t.Category, t.Amount, t.Notes)
				}
				fmt.Printf("\nTotal transactions: %d\n", total)
			})
	},
}

var transactionsSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search transactions",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var txs []monarch.Transaction
		var total int
		runWarn(cmd.Context(), "transactions.search", "failed to search transactions",
			[]string{"uses legacy Monarch GraphQL root field: allTransactions"},
			func(ctx context.Context, svc *monarch.Service) (map[string]any, error) {
				t, tot, err := svc.ListTransactions(ctx, &monarch.ListTransactionsOptions{
					Limit:     limit,
					Offset:    offset,
					Search:    args[0],
					StartDate: txStartDate,
					EndDate:   txEndDate,
				})
				if err != nil {
					return nil, err
				}
				txs, total = t, tot
				return map[string]any{"transactions": t, "total": tot}, nil
			},
			func(_ map[string]any) {
				fmt.Printf("%-12s %-20s %-15s %10s %s\n", "DATE", "MERCHANT", "CATEGORY", "AMOUNT", "NOTES")
				for _, t := range txs {
					fmt.Printf("%-12s %-20s %-15s %10.2f %s\n", t.Date, t.Merchant, t.Category, t.Amount, t.Notes)
				}
				fmt.Printf("\nTotal matches: %d\n", total)
			})
	},
}

var transactionsDuplicatesCmd = &cobra.Command{
	Use:   "duplicates",
	Short: "Find duplicate transactions",
	Run: func(cmd *cobra.Command, args []string) {
		runWarn(cmd.Context(), "transactions.duplicates", "failed to find duplicates",
			[]string{"uses legacy Monarch GraphQL root field: allTransactions"},
			func(ctx context.Context, svc *monarch.Service) ([]monarch.Transaction, error) {
				now := time.Now()
				startDate := now.Format("2006-01-02")
				endDate := time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
				return svc.GetDuplicateTransactions(ctx, startDate, endDate)
			},
			func(txs []monarch.Transaction) {
				fmt.Printf("%-12s %-20s %10s %s\n", "DATE", "MERCHANT", "AMOUNT", "ID")
				for _, t := range txs {
					fmt.Printf("%-12s %-20s %10.2f %s\n", t.Date, t.Merchant, t.Amount, t.ID)
				}
			})
	},
}

var transactionsSplitsCmd = &cobra.Command{
	Use:   "splits <transaction-id>",
	Short: "Get splits for a transaction",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		run(cmd.Context(), "transactions.splits", "failed to get splits",
			func(ctx context.Context, svc *monarch.Service) ([]monarch.TransactionSplit, error) {
				return svc.GetTransactionSplits(ctx, args[0])
			},
			func(splits []monarch.TransactionSplit) {
				fmt.Printf("%-20s %10s %s\n", "CATEGORY", "AMOUNT", "NOTES")
				for _, s := range splits {
					fmt.Printf("%-20s %10.2f %s\n", s.Category, s.Amount, s.Notes)
				}
			})
	},
}

var transactionsUpdateCmd = &cobra.Command{
	Use:   "update <transaction-id>",
	Short: "Update a transaction",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		runMutation(cmd, "transactions.update", "failed to update transaction", safety.TierMutation, func() (mutation, *errors.Error) {
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
			var tx *monarch.Transaction
			return mutation{
				resourceID: id,
				planAfter:  map[string]any{"notes": notes, "categoryId": categoryID, "amount": amount, "date": date, "merchant": merchantName, "hideFromReports": hideFromReports, "needsReview": needsReview},
				do: func(ctx context.Context, svc *monarch.Service) (any, error) {
					updated, err := svc.UpdateTransaction(ctx, id, notes, categoryID, amount, date, merchantName, hideFromReports, needsReview)
					if err != nil {
						return nil, err
					}
					tx = updated
					return updated, nil
				},
				human: func() { fmt.Printf("Successfully updated transaction %s.\n", tx.ID) },
			}, nil
		})
	},
}

var transactionsDeleteCmd = &cobra.Command{
	Use:   "delete <transaction-id>",
	Short: "Delete a transaction",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		runMutation(cmd, "transactions.delete", "failed to delete transaction", safety.TierDestructive, func() (mutation, *errors.Error) {
			return mutation{
				resourceID: id,
				do: func(ctx context.Context, svc *monarch.Service) (any, error) {
					if err := svc.DeleteTransaction(ctx, id); err != nil {
						return nil, err
					}
					return map[string]string{"status": "deleted"}, nil
				},
				human: func() { fmt.Printf("Successfully deleted transaction %s.\n", id) },
			}, nil
		})
	},
}

var transactionsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a transaction",
	Run: func(cmd *cobra.Command, args []string) {
		runMutation(cmd, "transactions.create", "failed to create transaction", safety.TierMutation, func() (mutation, *errors.Error) {
			if txDate == "" {
				txDate = time.Now().Format("2006-01-02")
			}
			var tx *monarch.Transaction
			return mutation{
				planAfter: map[string]any{"amount": txAmount, "merchant": txMerchant, "date": txDate, "categoryId": txCategoryID},
				do: func(ctx context.Context, svc *monarch.Service) (any, error) {
					created, err := svc.CreateTransaction(ctx, txAmount, txMerchant, txDate, txCategoryID, txAccountID, txNotes)
					if err != nil {
						return nil, err
					}
					tx = created
					return created, nil
				},
				human: func() { fmt.Printf("Successfully created transaction %s.\n", tx.ID) },
			}, nil
		})
	},
}

var transactionsSplitCmd = &cobra.Command{
	Use:   "split <transaction-id>",
	Short: "Split a transaction",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		runMutation(cmd, "transactions.split", "failed to split transaction", safety.TierMutation, func() (mutation, *errors.Error) {
			data, err := os.ReadFile(splitFile)
			if err != nil {
				return mutation{}, errors.New(errors.ValidationFailed, "failed to read split file: "+err.Error(), errors.CatValidation, false, err)
			}
			var splits []monarch.SplitInput
			if err := jsonUnmarshal(data, &splits); err != nil {
				return mutation{}, errors.New(errors.ValidationFailed, "invalid split JSON: "+err.Error(), errors.CatValidation, false, err)
			}
			return mutation{
				resourceID: id,
				planAfter:  map[string]any{"splits": splits},
				do: func(ctx context.Context, svc *monarch.Service) (any, error) {
					if err := svc.UpdateTransactionSplits(ctx, id, splits); err != nil {
						return nil, err
					}
					return map[string]string{"status": "split updated"}, nil
				},
				human: func() { fmt.Printf("Successfully split transaction %s.\n", id) },
			}, nil
		})
	},
}

var transactionsExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export transactions",
	Run: func(cmd *cobra.Command, args []string) {
		start := time.Now()
		renderer := output.NewRenderer(nil, nil, jsonMode, pretty)

		deps, ok := newDeps(renderer, "transactions.export", start)
		if !ok {
			return
		}
		svc := deps.Service

		opts := monarch.ListTransactionsOptions{
			Limit:     limit,
			Offset:    offset,
			StartDate: txStartDate,
			EndDate:   txEndDate,
			GoalIDs:   filterGoalIDs,
		}
		if cmd.Flags().Changed("pending") {
			opts.Pending = &filterPending
		}
		if cmd.Flags().Changed("hide-from-reports") {
			opts.HideFromReports = &filterHideReports
		}

		txs, _, err := svc.ListTransactions(cmd.Context(), &opts)
		if err != nil {
			handleError(renderer, "transactions.export", wrapError(err, "failed to list transactions"), start)
			return
		}

		var out io.Writer = os.Stdout
		if outputFile != "" {
			f, err := os.Create(outputFile)
			if err != nil {
				handleError(renderer, "transactions.export", errors.New(errors.InternalError, "failed to create output file", errors.CatInternal, false, err), start)
				return
			}
			defer func() {
				if cerr := f.Close(); cerr != nil {
					fmt.Fprintf(os.Stderr, "warning: failed to close file: %v\n", cerr) //nolint:errcheck // best-effort output
				}
			}()
			out = f
		}

		if format == "csv" {
			if err := monarch.ExportTransactionsCSV(txs, out); err != nil {
				handleError(renderer, "transactions.export", errors.New(errors.InternalError, "failed to export CSV", errors.CatInternal, false, err), start)
				return
			}
		} else {
			env := output.NewEnvelope("transactions.export", profile, output.SchemaVersion, requestID, txs, time.Since(start))
			renderer.RenderSuccess(env)
		}
	},
}

var transactionsTagsCmd = &cobra.Command{
	Use:   "tags",
	Short: "Manage transaction tags",
}

var transactionsTagsSetCmd = &cobra.Command{
	Use:   "set <transaction-id>",
	Short: "Set tags for a transaction",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		runMutation(cmd, "transactions.tags.set", "failed to set transaction tags", safety.TierMutation, func() (mutation, *errors.Error) {
			return mutation{
				resourceID: id,
				planAfter:  map[string]any{"tag_ids": tagIDs},
				do: func(ctx context.Context, svc *monarch.Service) (any, error) {
					if err := svc.SetTransactionTags(ctx, id, tagIDs); err != nil {
						return nil, err
					}
					return map[string]string{"status": "tags set"}, nil
				},
				human: func() { fmt.Printf("Successfully set tags for transaction %s.\n", id) },
			}, nil
		})
	},
}

var transactionsAttachmentsCmd = &cobra.Command{
	Use:   "attachments",
	Short: "Manage transaction attachments",
}

var transactionsAttachmentsListCmd = &cobra.Command{
	Use:   "list <transaction-id>",
	Short: "List attachments for a transaction",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		run(cmd.Context(), "transactions.attachments.list", "failed to list attachments",
			func(ctx context.Context, svc *monarch.Service) ([]monarch.Attachment, error) {
				return svc.ListTransactionAttachments(ctx, args[0])
			},
			func(attachments []monarch.Attachment) {
				if len(attachments) == 0 {
					fmt.Println("No attachments found.")
					return
				}
				fmt.Printf("%-36s %-20s %s\n", "ID", "FILENAME", "SIZE")
				for _, a := range attachments {
					fmt.Printf("%-36s %-20s %d bytes\n", a.ID, a.Filename, a.SizeBytes)
				}
			})
	},
}

var transactionsAttachmentsDownloadCmd = &cobra.Command{
	Use:   "download <transaction-id>",
	Short: "Download an attachment for a transaction",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var outPath string
		run(cmd.Context(), "transactions.attachments.download", "failed to download attachment",
			func(ctx context.Context, svc *monarch.Service) (map[string]string, error) {
				if attachmentID == "" {
					return nil, errors.New(errors.InvalidArguments, "--id flag is required", errors.CatValidation, false, nil)
				}
				attachments, err := svc.ListTransactionAttachments(ctx, args[0])
				if err != nil {
					return nil, wrapError(err, "failed to list attachments")
				}
				var targetURL, targetFilename string
				for _, a := range attachments {
					if a.ID == attachmentID {
						targetURL = a.URL
						targetFilename = a.Filename
						break
					}
				}
				if targetURL == "" {
					return nil, errors.New(errors.ResourceNotFound, "attachment not found", errors.CatAPI, false, nil)
				}
				outPath = outputFile
				if outPath == "" {
					outPath = targetFilename
				}
				f, err := os.Create(outPath)
				if err != nil {
					return nil, errors.New(errors.InternalError, "failed to create output file: "+err.Error(), errors.CatInternal, false, err)
				}
				defer func() {
					if cerr := f.Close(); cerr != nil {
						fmt.Fprintf(os.Stderr, "warning: failed to close file: %v\n", cerr)
					}
				}()
				if err := svc.DownloadAttachment(ctx, targetURL, f); err != nil {
					return nil, wrapError(err, "failed to download attachment")
				}
				return map[string]string{"status": "downloaded", "path": outPath}, nil
			},
			func(_ map[string]string) {
				fmt.Printf("Downloaded attachment to %s\n", outPath)
			})
	},
}

var transactionsShowCmd = &cobra.Command{
	Use:   "show <transaction-id>",
	Short: "Show detailed information for a transaction",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		run(cmd.Context(), "transactions.show", "failed to get transaction",
			func(ctx context.Context, svc *monarch.Service) (*monarch.Transaction, error) {
				return svc.GetTransaction(ctx, args[0])
			},
			func(tx *monarch.Transaction) {
				fmt.Printf("ID:       %s\n", tx.ID)
				fmt.Printf("Date:     %s\n", tx.Date)
				fmt.Printf("Merchant: %s\n", tx.Merchant)
				fmt.Printf("Category: %s\n", tx.Category)
				fmt.Printf("Amount:   %.2f\n", tx.Amount)
				fmt.Printf("Notes:    %s\n", tx.Notes)
			})
	},
}

var transactionsSummaryCmd = &cobra.Command{
	Use:   "summary",
	Short: "Get transaction summary",
	Run: func(cmd *cobra.Command, args []string) {
		run(cmd.Context(), "transactions.summary", "failed to get transaction summary",
			func(ctx context.Context, svc *monarch.Service) (*monarch.TransactionSummaryResult, error) {
				return svc.GetTransactionsSummary(ctx, txStartDate, txEndDate)
			},
			func(_ *monarch.TransactionSummaryResult) {
				fmt.Println("Transaction Summary")
			})
	},
}

var transactionsTagsClearCmd = &cobra.Command{
	Use:   "clear <transaction-id>",
	Short: "Clear all tags for a transaction",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := args[0]
		runMutation(cmd, "transactions.tags.clear", "failed to clear transaction tags", safety.TierMutation, func() (mutation, *errors.Error) {
			return mutation{
				resourceID: id,
				do: func(ctx context.Context, svc *monarch.Service) (any, error) {
					if err := svc.SetTransactionTags(ctx, id, []string{}); err != nil {
						return nil, err
					}
					return map[string]string{"status": "tags cleared"}, nil
				},
				human: func() { fmt.Printf("Successfully cleared tags for transaction %s.\n", id) },
			}, nil
		})
	},
}

var transactionsTagsAddCmd = &cobra.Command{
	Use:   "add <transaction-id>",
	Short: "Add tags to a transaction (appending to existing tags)",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		start := time.Now()
		renderer := output.NewRenderer(nil, nil, jsonMode, pretty)
		logger := audit.NewLogger()
		id := args[0]

		if err := safety.Check(safety.TierMutation, readOnly, dryRun, confirm); err != nil {
			handleError(renderer, "transactions.tags.add", err, start)
			return
		}

		if len(tagIDs) == 0 {
			handleError(renderer, "transactions.tags.add", errors.New(errors.InvalidArguments, "--tag is required", errors.CatValidation, false, nil), start)
			return
		}

		deps, ok := newDeps(renderer, "transactions.tags.add", start)
		if !ok {
			return
		}
		svc := deps.Service

		tx, err := svc.GetTransaction(cmd.Context(), id)
		if err != nil {
			handleError(renderer, "transactions.tags.add", errors.New(errors.APIError, "failed to fetch current transaction", errors.CatAPI, false, err), start)
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

		if dryRun {
			plan := safety.NewPlan()
			plan.Add("transactions.tags.add", id, nil, map[string]any{"tag_ids": newTagIDs})
			env := output.NewEnvelope("transactions.tags.add", profile, output.SchemaVersion, requestID, plan, time.Since(start))
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

		logger.Log(&audit.Record{ //nolint:errcheck // best-effort audit
			Command:    "transactions.tags.add",
			ResourceID: id,
			DryRun:     dryRun,
			Confirmed:  confirm,
			Profile:    profile,
			Result:     result,
			ErrorCode:  errCode,
		})

		if err != nil {
			handleError(renderer, "transactions.tags.add", wrapError(err, "failed to add transaction tags"), start)
			return
		}

		if jsonMode {
			env := output.NewEnvelope("transactions.tags.add", profile, output.SchemaVersion, requestID, map[string]string{"status": "tags added"}, time.Since(start))
			renderer.RenderSuccess(env)
		} else {
			fmt.Printf("Successfully added tags to transaction %s.\n", id)
		}
	},
}

var transactionsBulkCategorizeCmd = &cobra.Command{
	Use:   "bulk-categorize",
	Short: "Apply a category to multiple transactions",
	Run: func(cmd *cobra.Command, args []string) {
		start := time.Now()
		renderer := output.NewRenderer(nil, nil, jsonMode, pretty)
		logger := audit.NewLogger()

		if len(bulkTxIDs) == 0 {
			handleError(renderer, "transactions.bulk-categorize", errors.New(errors.InvalidArguments, "at least one --id is required", errors.CatValidation, false, nil), start)
			return
		}
		if bulkCategoryID == "" {
			handleError(renderer, "transactions.bulk-categorize", errors.New(errors.InvalidArguments, "--category-id is required", errors.CatValidation, false, nil), start)
			return
		}

		if err := safety.Check(safety.TierMutation, readOnly, dryRun, confirm); err != nil {
			handleError(renderer, "transactions.bulk-categorize", err, start)
			return
		}

		if dryRun {
			plan := safety.NewPlan()
			for _, id := range bulkTxIDs {
				plan.Add("transactions.update", id, nil, map[string]any{"categoryId": bulkCategoryID, "markReviewed": bulkMarkReviewed})
			}
			env := output.NewEnvelope("transactions.bulk-categorize", profile, output.SchemaVersion, requestID, plan, time.Since(start))
			renderer.RenderSuccess(env)
			return
		}

		deps, ok := newDeps(renderer, "transactions.bulk-categorize", start)
		if !ok {
			return
		}
		svc := deps.Service

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
		logger.Log(&audit.Record{Command: "transactions.bulk-categorize", DryRun: dryRun, Confirmed: confirm, Profile: profile, Result: result}) //nolint:errcheck // best-effort audit

		if jsonMode {
			data := map[string]any{"total": len(bulkTxIDs), "successful": successes, "failed": len(failures), "errors": failures}
			env := output.NewEnvelope("transactions.bulk-categorize", profile, output.SchemaVersion, requestID, data, time.Since(start))
			renderer.RenderSuccess(env)
		} else {
			fmt.Printf("Bulk categorize: %d/%d successful.\n", successes, len(bulkTxIDs))
			for _, f := range failures {
				fmt.Printf("  FAILED: %s\n", f)
			}
		}
	},
}

var transactionsAttachmentsUploadCmd = &cobra.Command{
	Use:   "upload <transaction-id> <file>",
	Short: "Upload an attachment for a transaction",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		start := time.Now()
		renderer := output.NewRenderer(nil, nil, jsonMode, pretty)
		handleError(renderer, "transactions.attachments.upload", errors.New(errors.FEATURE_UNAVAILABLE, "transaction attachment upload is unavailable in the current Monarch API", errors.CatAPI, false, nil), start)
	},
}

type appTransactionListFlags struct {
	limit           int
	offset          int
	categoryIDs     []string
	accountIDs      []string
	tagIDs          []string
	needsReview     bool
	hasNotes        bool
	isSplit         bool
	isRecurring     bool
	pending         bool
	hideFromReports bool
	goalIDs         []string
}

func (f *appTransactionListFlags) bind(cmd *cobra.Command) {
	cmd.Flags().IntVar(&f.limit, "limit", 100, "maximum number of transactions to return")
	cmd.Flags().IntVar(&f.offset, "offset", 0, "number of transactions to skip")
	cmd.Flags().StringSliceVar(&f.categoryIDs, "category-id", nil, "filter by category ID (repeatable)")
	cmd.Flags().StringSliceVar(&f.accountIDs, "account-id", nil, "filter by account ID (repeatable)")
	cmd.Flags().StringSliceVar(&f.tagIDs, "tag-id", nil, "filter by tag ID (repeatable)")
	cmd.Flags().BoolVar(&f.needsReview, "needs-review", false, "filter for transactions needing review")
	cmd.Flags().BoolVar(&f.hasNotes, "has-notes", false, "filter for transactions with notes")
	cmd.Flags().BoolVar(&f.isSplit, "is-split", false, "filter for split transactions")
	cmd.Flags().BoolVar(&f.isRecurring, "is-recurring", false, "filter for recurring transactions")
	cmd.Flags().BoolVar(&f.pending, "pending", false, "filter by pending status")
	cmd.Flags().BoolVar(&f.hideFromReports, "hide-from-reports", false, "filter by hide-from-reports status")
	cmd.Flags().StringSliceVar(&f.goalIDs, "goal-id", nil, "filter by goal ID (repeatable)")
}

func (f appTransactionListFlags) options(cmd *cobra.Command, startDate, endDate string) monarch.ListTransactionsOptions {
	opts := monarch.ListTransactionsOptions{
		Limit:       f.limit,
		Offset:      f.offset,
		StartDate:   startDate,
		EndDate:     endDate,
		CategoryIDs: f.categoryIDs,
		AccountIDs:  f.accountIDs,
		TagIDs:      f.tagIDs,
		GoalIDs:     f.goalIDs,
	}
	if cmd.Flags().Changed("needs-review") {
		opts.NeedsReview = &f.needsReview
	}
	if cmd.Flags().Changed("has-notes") {
		opts.HasNotes = &f.hasNotes
	}
	if cmd.Flags().Changed("is-split") {
		opts.IsSplit = &f.isSplit
	}
	if cmd.Flags().Changed("is-recurring") {
		opts.IsRecurring = &f.isRecurring
	}
	if cmd.Flags().Changed("pending") {
		opts.Pending = &f.pending
	}
	if cmd.Flags().Changed("hide-from-reports") {
		opts.HideFromReports = &f.hideFromReports
	}
	return opts
}

type appTransactionExportFlags struct {
	limit           int
	offset          int
	format          string
	outputFile      string
	pending         bool
	hideFromReports bool
	goalIDs         []string
}

func (f *appTransactionExportFlags) bind(cmd *cobra.Command) {
	cmd.Flags().IntVar(&f.limit, "limit", 1000, "maximum number of transactions to export")
	cmd.Flags().IntVar(&f.offset, "offset", 0, "number of transactions to skip")
	cmd.Flags().StringVar(&f.format, "format", "json", "export format (json or csv)")
	must(cmd.RegisterFlagCompletionFunc("format", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"json", "csv"}, cobra.ShellCompDirectiveNoFileComp
	}))
	cmd.Flags().StringVar(&f.outputFile, "output", "", "output file path")
	cmd.Flags().BoolVar(&f.pending, "pending", false, "filter by pending status")
	cmd.Flags().BoolVar(&f.hideFromReports, "hide-from-reports", false, "filter by hide-from-reports status")
	cmd.Flags().StringSliceVar(&f.goalIDs, "goal-id", nil, "filter by goal ID (repeatable)")
}

func (f appTransactionExportFlags) options(cmd *cobra.Command, startDate, endDate string) monarch.ListTransactionsOptions {
	opts := monarch.ListTransactionsOptions{
		Limit:     f.limit,
		Offset:    f.offset,
		StartDate: startDate,
		EndDate:   endDate,
		GoalIDs:   f.goalIDs,
	}
	if cmd.Flags().Changed("pending") {
		opts.Pending = &f.pending
	}
	if cmd.Flags().Changed("hide-from-reports") {
		opts.HideFromReports = &f.hideFromReports
	}
	return opts
}

type appTransactionUpdateFlags struct {
	notes           string
	categoryID      string
	amount          float64
	date            string
	merchant        string
	hideFromReports bool
	needsReview     bool
	markReviewed    bool
}

func (f *appTransactionUpdateFlags) bind(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.notes, "notes", "", "transaction notes")
	cmd.Flags().StringVar(&f.categoryID, "category", "", "transaction category ID")
	cmd.Flags().Float64Var(&f.amount, "amount", 0, "transaction amount")
	cmd.Flags().StringVar(&f.date, "date", "", "transaction date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&f.merchant, "merchant", "", "merchant name")
	cmd.Flags().BoolVar(&f.hideFromReports, "hide-from-reports", false, "hide transaction from reports")
	cmd.Flags().BoolVar(&f.needsReview, "needs-review", false, "mark transaction as needing review")
	cmd.Flags().BoolVar(&f.markReviewed, "mark-reviewed", false, "mark transaction as reviewed (shortcut for --needs-review=false)")
}

func (f appTransactionUpdateFlags) values(cmd *cobra.Command) (*string, *string, *float64, *string, *string, *bool, *bool) {
	var notes *string
	if cmd.Flags().Changed("notes") {
		notes = &f.notes
	}
	var categoryID *string
	if cmd.Flags().Changed("category") {
		categoryID = &f.categoryID
	}
	var amount *float64
	if cmd.Flags().Changed("amount") {
		amount = &f.amount
	}
	var date *string
	if cmd.Flags().Changed("date") {
		date = &f.date
	}
	var merchant *string
	if cmd.Flags().Changed("merchant") {
		merchant = &f.merchant
	}
	var hideFromReports *bool
	if cmd.Flags().Changed("hide-from-reports") {
		hideFromReports = &f.hideFromReports
	}
	var needsReview *bool
	if cmd.Flags().Changed("needs-review") {
		needsReview = &f.needsReview
	}
	if f.markReviewed {
		reviewed := false
		needsReview = &reviewed
	}
	return notes, categoryID, amount, date, merchant, hideFromReports, needsReview
}

type appTransactionCreateFlags struct {
	amount     float64
	merchant   string
	date       string
	categoryID string
	accountID  string
	notes      string
}

func (f *appTransactionCreateFlags) bind(cmd *cobra.Command) {
	cmd.Flags().Float64Var(&f.amount, "amount", 0, "transaction amount")
	cmd.Flags().StringVar(&f.merchant, "merchant", "", "merchant name")
	cmd.Flags().StringVar(&f.date, "date", "", "transaction date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&f.categoryID, "category", "", "category ID")
	cmd.Flags().StringVar(&f.accountID, "account", "", "account ID")
	cmd.Flags().StringVar(&f.notes, "notes", "", "transaction notes")
	cmd.MarkFlagRequired("amount")   //nolint:errcheck // flag registered above
	cmd.MarkFlagRequired("merchant") //nolint:errcheck // flag registered above
	cmd.MarkFlagRequired("category") //nolint:errcheck // flag registered above
}

func (a *App) buildTransactionsCommand() *cobra.Command {
	var (
		startDate string
		endDate   string
	)

	cmd := &cobra.Command{
		Use:     "transactions",
		Short:   "Manage Monarch Money transactions",
		GroupID: "core",
		Example: "  monarch transactions list --limit 10 --json\n  monarch transactions search \"Amazon\"\n  monarch transactions update <id> --category cat_food --dry-run",
	}
	cmd.PersistentFlags().StringVar(&startDate, "from", "", "start date (YYYY-MM-DD)")
	cmd.PersistentFlags().StringVar(&endDate, "to", "", "end date (YYYY-MM-DD)")

	cmd.AddCommand(a.buildTransactionsTagsCommand())
	cmd.AddCommand(a.buildTransactionsAttachmentsCommand())
	cmd.AddCommand(a.buildTransactionsListCommand(&startDate, &endDate))
	cmd.AddCommand(a.buildTransactionsSearchCommand(&startDate, &endDate))
	cmd.AddCommand(a.buildTransactionsShowCommand())
	cmd.AddCommand(a.buildTransactionsSummaryCommand(&startDate, &endDate))
	cmd.AddCommand(a.buildTransactionsDuplicatesCommand())
	cmd.AddCommand(a.buildTransactionsSplitsCommand())
	cmd.AddCommand(a.buildTransactionsExportCommand(&startDate, &endDate))
	cmd.AddCommand(a.buildTransactionsUpdateCommand())
	cmd.AddCommand(a.buildTransactionsDeleteCommand())
	cmd.AddCommand(a.buildTransactionsCreateCommand())
	cmd.AddCommand(a.buildTransactionsSplitCommand())
	cmd.AddCommand(a.buildTransactionsBulkCategorizeCommand())
	return cmd
}

func (a *App) buildTransactionsListCommand(startDate, endDate *string) *cobra.Command {
	var flags appTransactionListFlags

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List transactions",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "transactions.list", wrapError(err, "failed to load service"), start)
				return
			}

			options := flags.options(cmd, *startDate, *endDate)
			txs, total, err := svc.ListTransactions(cmd.Context(), &options)
			if err != nil {
				a.handleError(renderer, "transactions.list", wrapError(err, "failed to list transactions"), start)
				return
			}

			if a.Flags.JSONMode {
				data := map[string]any{"transactions": txs, "total": total}
				env := a.envelopeWithWarnings("transactions.list", data, start, "uses legacy Monarch GraphQL root field: allTransactions")
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%-12s %-20s %-15s %10s %s\n", "DATE", "MERCHANT", "CATEGORY", "AMOUNT", "NOTES") //nolint:errcheck // best-effort output
			for _, t := range txs {
				fmt.Fprintf(cmd.OutOrStdout(), "%-12s %-20s %-15s %10.2f %s\n", t.Date, t.Merchant, t.Category, t.Amount, t.Notes) //nolint:errcheck // best-effort output
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nTotal transactions: %d\n", total) //nolint:errcheck // best-effort output
		},
	}
	flags.bind(cmd)
	return cmd
}

func (a *App) buildTransactionsSearchCommand(startDate, endDate *string) *cobra.Command {
	var (
		limitValue  int
		offsetValue int
	)

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search transactions",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "transactions.search", wrapError(err, "failed to load service"), start)
				return
			}

			txs, total, err := svc.ListTransactions(cmd.Context(), &monarch.ListTransactionsOptions{
				Limit:     limitValue,
				Offset:    offsetValue,
				Search:    args[0],
				StartDate: *startDate,
				EndDate:   *endDate,
			})
			if err != nil {
				a.handleError(renderer, "transactions.search", wrapError(err, "failed to search transactions"), start)
				return
			}

			if a.Flags.JSONMode {
				data := map[string]any{"transactions": txs, "total": total}
				env := a.envelopeWithWarnings("transactions.search", data, start, "uses legacy Monarch GraphQL root field: allTransactions")
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%-12s %-20s %-15s %10s %s\n", "DATE", "MERCHANT", "CATEGORY", "AMOUNT", "NOTES") //nolint:errcheck // best-effort output
			for _, t := range txs {
				fmt.Fprintf(cmd.OutOrStdout(), "%-12s %-20s %-15s %10.2f %s\n", t.Date, t.Merchant, t.Category, t.Amount, t.Notes) //nolint:errcheck // best-effort output
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nTotal matches: %d\n", total) //nolint:errcheck // best-effort output
		},
	}
	cmd.Flags().IntVar(&limitValue, "limit", 100, "maximum number of transactions to return")
	cmd.Flags().IntVar(&offsetValue, "offset", 0, "number of transactions to skip")
	return cmd
}

func (a *App) buildTransactionsDuplicatesCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "duplicates",
		Short: "Find duplicate transactions",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "transactions.duplicates", wrapError(err, "failed to load service"), start)
				return
			}

			now := time.Now()
			startDate := now.Format("2006-01-02")
			endDate := time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, time.UTC).Format("2006-01-02")

			txs, err := svc.GetDuplicateTransactions(cmd.Context(), startDate, endDate)
			if err != nil {
				a.handleError(renderer, "transactions.duplicates", wrapError(err, "failed to find duplicates"), start)
				return
			}

			if a.Flags.JSONMode {
				env := a.envelopeWithWarnings("transactions.duplicates", txs, start, "uses legacy Monarch GraphQL root field: allTransactions")
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%-12s %-20s %10s %s\n", "DATE", "MERCHANT", "AMOUNT", "ID") //nolint:errcheck // best-effort output
			for _, t := range txs {
				fmt.Fprintf(cmd.OutOrStdout(), "%-12s %-20s %10.2f %s\n", t.Date, t.Merchant, t.Amount, t.ID) //nolint:errcheck // best-effort output
			}
		},
	}
}

func (a *App) buildTransactionsSplitsCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "splits <transaction-id>",
		Short: "Get splits for a transaction",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "transactions.splits", wrapError(err, "failed to load service"), start)
				return
			}

			splits, err := svc.GetTransactionSplits(cmd.Context(), args[0])
			if err != nil {
				a.handleError(renderer, "transactions.splits", wrapError(err, "failed to get splits"), start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("transactions.splits", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, splits, time.Since(start))
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%-20s %10s %s\n", "CATEGORY", "AMOUNT", "NOTES") //nolint:errcheck // best-effort output
			for _, s := range splits {
				fmt.Fprintf(cmd.OutOrStdout(), "%-20s %10.2f %s\n", s.Category, s.Amount, s.Notes) //nolint:errcheck // best-effort output
			}
		},
	}
}

func (a *App) buildTransactionsUpdateCommand() *cobra.Command {
	var flags appTransactionUpdateFlags

	cmd := &cobra.Command{
		Use:   "update <transaction-id>",
		Short: "Update a transaction",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)
			id := args[0]

			if !a.checkSafety(renderer, "transactions.update", safety.TierMutation, start) {
				return
			}

			notes, categoryID, amount, date, merchant, hideFromReports, needsReview := flags.values(cmd)
			if a.Flags.DryRun {
				plan := safety.NewPlan()
				plan.Add("transactions.update", id, nil, map[string]any{"notes": notes, "categoryId": categoryID, "amount": amount, "date": date, "merchant": merchant, "hideFromReports": hideFromReports, "needsReview": needsReview})
				a.renderPlan(renderer, "transactions.update", plan, start)
				return
			}

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "transactions.update", wrapError(err, "failed to load service"), start)
				return
			}

			result, err := a.mutate(renderer, "transactions.update", id, start, func() (any, error) {
				return svc.UpdateTransaction(cmd.Context(), id, notes, categoryID, amount, date, merchant, hideFromReports, needsReview)
			}, "failed to update transaction")
			if err != nil {
				return
			}
			tx := result.(*monarch.Transaction)

			if a.Flags.JSONMode {
				env := output.NewEnvelope("transactions.update", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, tx, time.Since(start))
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully updated transaction %s.\n", tx.ID) //nolint:errcheck // best-effort output
		},
	}
	flags.bind(cmd)
	return cmd
}

func (a *App) buildTransactionsDeleteCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <transaction-id>",
		Short: "Delete a transaction",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)
			id := args[0]

			if !a.checkSafety(renderer, "transactions.delete", safety.TierDestructive, start) {
				return
			}

			if a.Flags.DryRun {
				plan := safety.NewPlan()
				plan.Add("transactions.delete", id, nil, nil)
				a.renderPlan(renderer, "transactions.delete", plan, start)
				return
			}

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "transactions.delete", wrapError(err, "failed to load service"), start)
				return
			}

			if _, err := a.mutate(renderer, "transactions.delete", id, start, func() (any, error) {
				return nil, svc.DeleteTransaction(cmd.Context(), id)
			}, "failed to delete transaction"); err != nil {
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("transactions.delete", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, map[string]string{"status": "deleted"}, time.Since(start))
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully deleted transaction %s.\n", id) //nolint:errcheck // best-effort output
		},
	}
}

func (a *App) buildTransactionsCreateCommand() *cobra.Command {
	var flags appTransactionCreateFlags

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a transaction",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)

			if !a.checkSafety(renderer, "transactions.create", safety.TierMutation, start) {
				return
			}

			if flags.date == "" {
				flags.date = time.Now().Format("2006-01-02")
			}

			if a.Flags.DryRun {
				plan := safety.NewPlan()
				plan.Add("transactions.create", "", nil, map[string]any{"amount": flags.amount, "merchant": flags.merchant, "date": flags.date, "categoryId": flags.categoryID})
				a.renderPlan(renderer, "transactions.create", plan, start)
				return
			}

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "transactions.create", wrapError(err, "failed to load service"), start)
				return
			}

			result, err := a.mutate(renderer, "transactions.create", "", start, func() (any, error) {
				return svc.CreateTransaction(cmd.Context(), flags.amount, flags.merchant, flags.date, flags.categoryID, flags.accountID, flags.notes)
			}, "failed to create transaction")
			if err != nil {
				return
			}
			tx := result.(*monarch.Transaction)

			if a.Flags.JSONMode {
				env := output.NewEnvelope("transactions.create", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, tx, time.Since(start))
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully created transaction %s.\n", tx.ID) //nolint:errcheck // best-effort output
		},
	}
	flags.bind(cmd)
	return cmd
}

func (a *App) buildTransactionsSplitCommand() *cobra.Command {
	var file string

	cmd := &cobra.Command{
		Use:   "split <transaction-id>",
		Short: "Split a transaction",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)
			id := args[0]

			if !a.checkSafety(renderer, "transactions.split", safety.TierMutation, start) {
				return
			}

			data, err := os.ReadFile(file)
			if err != nil {
				a.handleError(renderer, "transactions.split", errors.New(errors.ValidationFailed, "failed to read split file: "+err.Error(), errors.CatValidation, false, err), start)
				return
			}

			var splits []monarch.SplitInput
			if err := jsonUnmarshal(data, &splits); err != nil {
				a.handleError(renderer, "transactions.split", errors.New(errors.ValidationFailed, "invalid split JSON: "+err.Error(), errors.CatValidation, false, err), start)
				return
			}

			if a.Flags.DryRun {
				plan := safety.NewPlan()
				plan.Add("transactions.split", id, nil, map[string]any{"splits": splits})
				a.renderPlan(renderer, "transactions.split", plan, start)
				return
			}

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "transactions.split", wrapError(err, "failed to load service"), start)
				return
			}

			if _, err := a.mutate(renderer, "transactions.split", id, start, func() (any, error) {
				return nil, svc.UpdateTransactionSplits(cmd.Context(), id, splits)
			}, "failed to split transaction"); err != nil {
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("transactions.split", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, map[string]string{"status": "split updated"}, time.Since(start))
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully split transaction %s.\n", id) //nolint:errcheck // best-effort output
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "JSON file with split data")
	cmd.MarkFlagRequired("file") //nolint:errcheck // flag registered above
	return cmd
}

func (a *App) buildTransactionsExportCommand(startDate, endDate *string) *cobra.Command {
	var flags appTransactionExportFlags

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export transactions",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "transactions.export", wrapError(err, "failed to load service"), start)
				return
			}

			options := flags.options(cmd, *startDate, *endDate)
			txs, _, err := svc.ListTransactions(cmd.Context(), &options)
			if err != nil {
				a.handleError(renderer, "transactions.export", wrapError(err, "failed to list transactions"), start)
				return
			}

			out := cmd.OutOrStdout()
			if flags.outputFile != "" {
				f, err := os.Create(flags.outputFile)
				if err != nil {
					a.handleError(renderer, "transactions.export", errors.New(errors.InternalError, "failed to create output file", errors.CatInternal, false, err), start)
					return
				}
				defer func() {
					if cerr := f.Close(); cerr != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to close file: %v\n", cerr) //nolint:errcheck // best-effort warning
					}
				}()
				out = f
			}

			if flags.format == "csv" {
				if err := monarch.ExportTransactionsCSV(txs, out); err != nil {
					a.handleError(renderer, "transactions.export", errors.New(errors.InternalError, "failed to export CSV", errors.CatInternal, false, err), start)
					return
				}
				return
			}

			env := output.NewEnvelope("transactions.export", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, txs, time.Since(start))
			renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
		},
	}
	flags.bind(cmd)
	return cmd
}

func (a *App) buildTransactionsTagsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tags",
		Short: "Manage transaction tags",
	}
	cmd.AddCommand(a.buildTransactionsTagsSetCommand())
	cmd.AddCommand(a.buildTransactionsTagsAddCommand())
	cmd.AddCommand(a.buildTransactionsTagsClearCommand())
	return cmd
}

func (a *App) buildTransactionsTagsSetCommand() *cobra.Command {
	var tags []string

	cmd := &cobra.Command{
		Use:   "set <transaction-id>",
		Short: "Set tags for a transaction",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)
			id := args[0]

			if !a.checkSafety(renderer, "transactions.tags.set", safety.TierMutation, start) {
				return
			}

			if a.Flags.DryRun {
				plan := safety.NewPlan()
				plan.Add("transactions.tags.set", id, nil, map[string]any{"tag_ids": tags})
				a.renderPlan(renderer, "transactions.tags.set", plan, start)
				return
			}

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "transactions.tags.set", wrapError(err, "failed to load service"), start)
				return
			}

			if _, err := a.mutate(renderer, "transactions.tags.set", id, start, func() (any, error) {
				return nil, svc.SetTransactionTags(cmd.Context(), id, tags)
			}, "failed to set transaction tags"); err != nil {
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("transactions.tags.set", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, map[string]string{"status": "tags set"}, time.Since(start))
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully set tags for transaction %s.\n", id) //nolint:errcheck // best-effort output
		},
	}
	cmd.Flags().StringSliceVar(&tags, "tag", []string{}, "tag IDs to set")
	cmd.MarkFlagRequired("tag") //nolint:errcheck // flag registered above
	return cmd
}

func (a *App) buildTransactionsAttachmentsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "attachments",
		Short: "Manage transaction attachments",
	}
	cmd.AddCommand(a.buildTransactionsAttachmentsListCommand())
	cmd.AddCommand(a.buildTransactionsAttachmentsUploadCommand())
	cmd.AddCommand(a.buildTransactionsAttachmentsDownloadCommand())
	return cmd
}

func (a *App) buildTransactionsAttachmentsListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list <transaction-id>",
		Short: "List attachments for a transaction",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "transactions.attachments.list", wrapError(err, "failed to load service"), start)
				return
			}

			attachments, err := svc.ListTransactionAttachments(cmd.Context(), args[0])
			if err != nil {
				a.handleError(renderer, "transactions.attachments.list", wrapError(err, "failed to list attachments"), start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("transactions.attachments.list", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, attachments, time.Since(start))
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}

			if len(attachments) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No attachments found.") //nolint:errcheck // best-effort output
				return
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-36s %-20s %s\n", "ID", "FILENAME", "SIZE") //nolint:errcheck // best-effort output
			for _, attachment := range attachments {
				fmt.Fprintf(cmd.OutOrStdout(), "%-36s %-20s %d bytes\n", attachment.ID, attachment.Filename, attachment.SizeBytes) //nolint:errcheck // best-effort output
			}
		},
	}
}

func (a *App) buildTransactionsAttachmentsDownloadCommand() *cobra.Command {
	var (
		id         string
		outputPath string
	)

	cmd := &cobra.Command{
		Use:   "download <transaction-id>",
		Short: "Download an attachment for a transaction",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)

			if id == "" {
				a.handleError(renderer, "transactions.attachments.download", errors.New(errors.InvalidArguments, "--id flag is required", errors.CatValidation, false, nil), start)
				return
			}

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "transactions.attachments.download", wrapError(err, "failed to load service"), start)
				return
			}

			attachments, err := svc.ListTransactionAttachments(cmd.Context(), args[0])
			if err != nil {
				a.handleError(renderer, "transactions.attachments.download", wrapError(err, "failed to list attachments"), start)
				return
			}

			var targetURL, targetFilename string
			for _, attachment := range attachments {
				if attachment.ID == id {
					targetURL = attachment.URL
					targetFilename = attachment.Filename
					break
				}
			}
			if targetURL == "" {
				a.handleError(renderer, "transactions.attachments.download", errors.New(errors.ResourceNotFound, "attachment not found", errors.CatAPI, false, nil), start)
				return
			}

			outPath := outputPath
			if outPath == "" {
				outPath = targetFilename
			}

			f, err := os.Create(outPath)
			if err != nil {
				a.handleError(renderer, "transactions.attachments.download", errors.New(errors.InternalError, "failed to create output file: "+err.Error(), errors.CatInternal, false, err), start)
				return
			}
			defer func() {
				if cerr := f.Close(); cerr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to close file: %v\n", cerr) //nolint:errcheck // best-effort warning
				}
			}()

			if err := svc.DownloadAttachment(cmd.Context(), targetURL, f); err != nil {
				a.handleError(renderer, "transactions.attachments.download", wrapError(err, "failed to download attachment"), start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("transactions.attachments.download", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, map[string]string{"status": "downloaded", "path": outPath}, time.Since(start))
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Downloaded attachment to %s\n", outPath) //nolint:errcheck // best-effort output
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "attachment ID")
	cmd.Flags().StringVar(&outputPath, "output", "", "output file path")
	return cmd
}

func (a *App) buildTransactionsShowCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "show <transaction-id>",
		Short: "Show detailed information for a transaction",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "transactions.show", wrapError(err, "failed to load service"), start)
				return
			}

			tx, err := svc.GetTransaction(cmd.Context(), args[0])
			if err != nil {
				a.handleError(renderer, "transactions.show", wrapError(err, "failed to get transaction"), start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("transactions.show", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, tx, time.Since(start))
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "ID:       %s\n", tx.ID)       //nolint:errcheck // best-effort output
			fmt.Fprintf(cmd.OutOrStdout(), "Date:     %s\n", tx.Date)     //nolint:errcheck // best-effort output
			fmt.Fprintf(cmd.OutOrStdout(), "Merchant: %s\n", tx.Merchant) //nolint:errcheck // best-effort output
			fmt.Fprintf(cmd.OutOrStdout(), "Category: %s\n", tx.Category) //nolint:errcheck // best-effort output
			fmt.Fprintf(cmd.OutOrStdout(), "Amount:   %.2f\n", tx.Amount) //nolint:errcheck // best-effort output
			fmt.Fprintf(cmd.OutOrStdout(), "Notes:    %s\n", tx.Notes)    //nolint:errcheck // best-effort output
		},
	}
}

func (a *App) buildTransactionsSummaryCommand(startDate, endDate *string) *cobra.Command {
	return &cobra.Command{
		Use:   "summary",
		Short: "Get transaction summary",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "transactions.summary", wrapError(err, "failed to load service"), start)
				return
			}

			summary, err := svc.GetTransactionsSummary(cmd.Context(), *startDate, *endDate)
			if err != nil {
				a.handleError(renderer, "transactions.summary", wrapError(err, "failed to get transaction summary"), start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("transactions.summary", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, summary, time.Since(start))
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Transaction Summary") //nolint:errcheck // best-effort output
		},
	}
}

func (a *App) buildTransactionsTagsClearCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "clear <transaction-id>",
		Short: "Clear all tags for a transaction",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)
			id := args[0]

			if !a.checkSafety(renderer, "transactions.tags.clear", safety.TierMutation, start) {
				return
			}

			if a.Flags.DryRun {
				plan := safety.NewPlan()
				plan.Add("transactions.tags.clear", id, nil, nil)
				a.renderPlan(renderer, "transactions.tags.clear", plan, start)
				return
			}

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "transactions.tags.clear", wrapError(err, "failed to load service"), start)
				return
			}

			if _, err := a.mutate(renderer, "transactions.tags.clear", id, start, func() (any, error) {
				return nil, svc.SetTransactionTags(cmd.Context(), id, []string{})
			}, "failed to clear transaction tags"); err != nil {
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("transactions.tags.clear", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, map[string]string{"status": "tags cleared"}, time.Since(start))
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully cleared tags for transaction %s.\n", id) //nolint:errcheck // best-effort output
		},
	}
}

func (a *App) buildTransactionsTagsAddCommand() *cobra.Command {
	var tags []string

	cmd := &cobra.Command{
		Use:   "add <transaction-id>",
		Short: "Add tags to a transaction (appending to existing tags)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)
			id := args[0]

			if !a.checkSafety(renderer, "transactions.tags.add", safety.TierMutation, start) {
				return
			}

			if len(tags) == 0 {
				a.handleError(renderer, "transactions.tags.add", errors.New(errors.InvalidArguments, "--tag is required", errors.CatValidation, false, nil), start)
				return
			}

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "transactions.tags.add", wrapError(err, "failed to load service"), start)
				return
			}

			tx, err := svc.GetTransaction(cmd.Context(), id)
			if err != nil {
				a.handleError(renderer, "transactions.tags.add", errors.New(errors.APIError, "failed to fetch current transaction", errors.CatAPI, false, err), start)
				return
			}

			existingTagIDs := make(map[string]bool)
			newTagIDs := []string{}
			for _, tag := range tx.Tags {
				existingTagIDs[tag.ID] = true
				newTagIDs = append(newTagIDs, tag.ID)
			}
			for _, tagID := range tags {
				if !existingTagIDs[tagID] {
					newTagIDs = append(newTagIDs, tagID)
				}
			}

			if a.Flags.DryRun {
				plan := safety.NewPlan()
				plan.Add("transactions.tags.add", id, nil, map[string]any{"tag_ids": newTagIDs})
				a.renderPlan(renderer, "transactions.tags.add", plan, start)
				return
			}

			if _, err := a.mutate(renderer, "transactions.tags.add", id, start, func() (any, error) {
				return nil, svc.SetTransactionTags(cmd.Context(), id, newTagIDs)
			}, "failed to add transaction tags"); err != nil {
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("transactions.tags.add", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, map[string]string{"status": "tags added"}, time.Since(start))
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully added tags to transaction %s.\n", id) //nolint:errcheck // best-effort output
		},
	}
	cmd.Flags().StringSliceVar(&tags, "tag", []string{}, "tag IDs to add")
	cmd.MarkFlagRequired("tag") //nolint:errcheck // flag registered above
	return cmd
}

func (a *App) buildTransactionsBulkCategorizeCommand() *cobra.Command {
	var (
		ids          []string
		categoryID   string
		markReviewed bool
	)

	cmd := &cobra.Command{
		Use:   "bulk-categorize",
		Short: "Apply a category to multiple transactions",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)

			if len(ids) == 0 {
				a.handleError(renderer, "transactions.bulk-categorize", errors.New(errors.InvalidArguments, "at least one --id is required", errors.CatValidation, false, nil), start)
				return
			}
			if categoryID == "" {
				a.handleError(renderer, "transactions.bulk-categorize", errors.New(errors.InvalidArguments, "--category-id is required", errors.CatValidation, false, nil), start)
				return
			}

			if !a.checkSafety(renderer, "transactions.bulk-categorize", safety.TierMutation, start) {
				return
			}

			if a.Flags.DryRun {
				plan := safety.NewPlan()
				for _, id := range ids {
					plan.Add("transactions.update", id, nil, map[string]any{"categoryId": categoryID, "markReviewed": markReviewed})
				}
				a.renderPlan(renderer, "transactions.bulk-categorize", plan, start)
				return
			}

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "transactions.bulk-categorize", wrapError(err, "failed to load service"), start)
				return
			}

			var needsReview *bool
			if markReviewed {
				reviewed := false
				needsReview = &reviewed
			}

			successes := 0
			var failures []string
			for _, txID := range ids {
				_, err := svc.UpdateTransaction(cmd.Context(), txID, nil, &categoryID, nil, nil, nil, nil, needsReview)
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
			a.Deps.WriteAudit(&audit.Record{Command: "transactions.bulk-categorize", DryRun: a.Flags.DryRun, Confirmed: a.Flags.Confirm, Profile: a.Flags.Profile, Result: result}) //nolint:errcheck // best-effort audit

			if a.Flags.JSONMode {
				data := map[string]any{"total": len(ids), "successful": successes, "failed": len(failures), "errors": failures}
				env := output.NewEnvelope("transactions.bulk-categorize", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, data, time.Since(start))
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Bulk categorize: %d/%d successful.\n", successes, len(ids)) //nolint:errcheck // best-effort output
			for _, failure := range failures {
				fmt.Fprintf(cmd.OutOrStdout(), "  FAILED: %s\n", failure) //nolint:errcheck // best-effort output
			}
		},
	}
	cmd.Flags().StringSliceVar(&ids, "id", nil, "transaction IDs to categorize (repeatable)")
	cmd.Flags().StringVar(&categoryID, "category-id", "", "category ID to apply")
	cmd.Flags().BoolVar(&markReviewed, "mark-reviewed", true, "also mark transactions as reviewed")
	return cmd
}

func (a *App) buildTransactionsAttachmentsUploadCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "upload <transaction-id> <file>",
		Short: "Upload an attachment for a transaction",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)
			a.handleError(renderer, "transactions.attachments.upload", errors.New(errors.FEATURE_UNAVAILABLE, "transaction attachment upload is unavailable in the current Monarch API", errors.CatAPI, false, nil), start)
		},
	}
}

func init() {
	transactionsCmd.PersistentFlags().StringVar(&txStartDate, "from", "", "start date (YYYY-MM-DD)")
	transactionsCmd.PersistentFlags().StringVar(&txEndDate, "to", "", "end date (YYYY-MM-DD)")

	transactionsListCmd.Flags().IntVar(&limit, "limit", 100, "maximum number of transactions to return")
	transactionsListCmd.Flags().IntVar(&offset, "offset", 0, "number of transactions to skip")
	transactionsListCmd.Flags().StringSliceVar(&filterCategoryIDs, "category-id", nil, "filter by category ID (repeatable)")
	transactionsListCmd.Flags().StringSliceVar(&filterAccountIDs, "account-id", nil, "filter by account ID (repeatable)")
	transactionsListCmd.Flags().StringSliceVar(&filterTagIDs, "tag-id", nil, "filter by tag ID (repeatable)")
	transactionsListCmd.Flags().BoolVar(&filterNeedsReview, "needs-review", false, "filter for transactions needing review")
	transactionsListCmd.Flags().BoolVar(&filterHasNotes, "has-notes", false, "filter for transactions with notes")
	transactionsListCmd.Flags().BoolVar(&filterIsSplit, "is-split", false, "filter for split transactions")
	transactionsListCmd.Flags().BoolVar(&filterIsRecurring, "is-recurring", false, "filter for recurring transactions")
	transactionsListCmd.Flags().BoolVar(&filterPending, "pending", false, "filter by pending status")
	transactionsListCmd.Flags().BoolVar(&filterHideReports, "hide-from-reports", false, "filter by hide-from-reports status")
	transactionsListCmd.Flags().StringSliceVar(&filterGoalIDs, "goal-id", nil, "filter by goal ID (repeatable)")

	transactionsSearchCmd.Flags().IntVar(&limit, "limit", 100, "maximum number of transactions to return")
	transactionsSearchCmd.Flags().IntVar(&offset, "offset", 0, "number of transactions to skip")

	transactionsExportCmd.Flags().IntVar(&limit, "limit", 1000, "maximum number of transactions to export")
	transactionsExportCmd.Flags().IntVar(&offset, "offset", 0, "number of transactions to skip")
	transactionsExportCmd.Flags().StringVar(&format, "format", "json", "export format (json or csv)")
	must(transactionsExportCmd.RegisterFlagCompletionFunc("format", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"json", "csv"}, cobra.ShellCompDirectiveNoFileComp
	}))
	transactionsExportCmd.Flags().StringVar(&outputFile, "output", "", "output file path")
	transactionsExportCmd.Flags().BoolVar(&filterPending, "pending", false, "filter by pending status")
	transactionsExportCmd.Flags().BoolVar(&filterHideReports, "hide-from-reports", false, "filter by hide-from-reports status")
	transactionsExportCmd.Flags().StringSliceVar(&filterGoalIDs, "goal-id", nil, "filter by goal ID (repeatable)")

	transactionsUpdateCmd.Flags().StringVar(&txNotes, "notes", "", "transaction notes")
	transactionsUpdateCmd.Flags().StringVar(&txCategoryID, "category", "", "transaction category ID")
	transactionsUpdateCmd.Flags().Float64Var(&txAmount, "amount", 0, "transaction amount")
	transactionsUpdateCmd.Flags().StringVar(&txDate, "date", "", "transaction date (YYYY-MM-DD)")
	transactionsUpdateCmd.Flags().StringVar(&txMerchant, "merchant", "", "merchant name")
	transactionsUpdateCmd.Flags().BoolVar(&txHideFromReports, "hide-from-reports", false, "hide transaction from reports")
	transactionsUpdateCmd.Flags().BoolVar(&txNeedsReview, "needs-review", false, "mark transaction as needing review")
	transactionsUpdateCmd.Flags().BoolVar(&txMarkReviewed, "mark-reviewed", false, "mark transaction as reviewed (shortcut for --needs-review=false)")

	transactionsCreateCmd.Flags().Float64Var(&txAmount, "amount", 0, "transaction amount")
	transactionsCreateCmd.Flags().StringVar(&txMerchant, "merchant", "", "merchant name")
	transactionsCreateCmd.Flags().StringVar(&txDate, "date", "", "transaction date (YYYY-MM-DD)")
	transactionsCreateCmd.Flags().StringVar(&txCategoryID, "category", "", "category ID")
	transactionsCreateCmd.Flags().StringVar(&txAccountID, "account", "", "account ID")
	transactionsCreateCmd.Flags().StringVar(&txNotes, "notes", "", "transaction notes")
	transactionsCreateCmd.MarkFlagRequired("amount")   //nolint:errcheck // flag registered above
	transactionsCreateCmd.MarkFlagRequired("merchant") //nolint:errcheck // flag registered above
	transactionsCreateCmd.MarkFlagRequired("category") //nolint:errcheck // flag registered above

	transactionsSplitCmd.Flags().StringVar(&splitFile, "file", "", "JSON file with split data")
	transactionsSplitCmd.MarkFlagRequired("file") //nolint:errcheck // flag registered above

	transactionsTagsSetCmd.Flags().StringSliceVar(&tagIDs, "tag", []string{}, "tag IDs to set")
	transactionsTagsSetCmd.MarkFlagRequired("tag") //nolint:errcheck // flag registered above

	transactionsTagsAddCmd.Flags().StringSliceVar(&tagIDs, "tag", []string{}, "tag IDs to add")
	transactionsTagsAddCmd.MarkFlagRequired("tag") //nolint:errcheck // flag registered above

	transactionsAttachmentsDownloadCmd.Flags().StringVar(&attachmentID, "id", "", "attachment ID")
	transactionsAttachmentsDownloadCmd.Flags().StringVar(&outputFile, "output", "", "output file path")

	transactionsBulkCategorizeCmd.Flags().StringSliceVar(&bulkTxIDs, "id", nil, "transaction IDs to categorize (repeatable)")
	transactionsBulkCategorizeCmd.Flags().StringVar(&bulkCategoryID, "category-id", "", "category ID to apply")
	transactionsBulkCategorizeCmd.Flags().BoolVar(&bulkMarkReviewed, "mark-reviewed", true, "also mark transactions as reviewed")

	transactionsTagsCmd.AddCommand(transactionsTagsSetCmd)
	transactionsTagsCmd.AddCommand(transactionsTagsAddCmd)
	transactionsTagsCmd.AddCommand(transactionsTagsClearCmd)
	transactionsCmd.AddCommand(transactionsTagsCmd)

	transactionsAttachmentsCmd.AddCommand(transactionsAttachmentsListCmd)
	transactionsAttachmentsCmd.AddCommand(transactionsAttachmentsUploadCmd)
	transactionsAttachmentsCmd.AddCommand(transactionsAttachmentsDownloadCmd)
	transactionsCmd.AddCommand(transactionsAttachmentsCmd)

	transactionsCmd.AddCommand(transactionsListCmd)
	transactionsCmd.AddCommand(transactionsSearchCmd)
	transactionsCmd.AddCommand(transactionsShowCmd)
	transactionsCmd.AddCommand(transactionsSummaryCmd)
	transactionsCmd.AddCommand(transactionsDuplicatesCmd)
	transactionsCmd.AddCommand(transactionsSplitsCmd)
	transactionsCmd.AddCommand(transactionsExportCmd)
	transactionsCmd.AddCommand(transactionsUpdateCmd)
	transactionsCmd.AddCommand(transactionsDeleteCmd)
	transactionsCmd.AddCommand(transactionsCreateCmd)
	transactionsCmd.AddCommand(transactionsSplitCmd)
	transactionsCmd.AddCommand(transactionsBulkCategorizeCmd)
	RootCmd.AddCommand(transactionsCmd)
}
