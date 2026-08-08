package cli

import (
	"encoding/json"
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

func (f *appTransactionListFlags) options(cmd *cobra.Command, startDate, endDate string) monarch.ListTransactionsOptions {
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

func (f *appTransactionExportFlags) options(cmd *cobra.Command, startDate, endDate string) monarch.ListTransactionsOptions {
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

type appTransactionUpdateValues struct {
	notes           *string
	categoryID      *string
	amount          *float64
	date            *string
	merchant        *string
	hideFromReports *bool
	needsReview     *bool
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

func (f *appTransactionUpdateFlags) values(cmd *cobra.Command) appTransactionUpdateValues {
	var values appTransactionUpdateValues
	if cmd.Flags().Changed("notes") {
		values.notes = &f.notes
	}
	if cmd.Flags().Changed("category") {
		values.categoryID = &f.categoryID
	}
	if cmd.Flags().Changed("amount") {
		values.amount = &f.amount
	}
	if cmd.Flags().Changed("date") {
		values.date = &f.date
	}
	if cmd.Flags().Changed("merchant") {
		values.merchant = &f.merchant
	}
	if cmd.Flags().Changed("hide-from-reports") {
		values.hideFromReports = &f.hideFromReports
	}
	if cmd.Flags().Changed("needs-review") {
		values.needsReview = &f.needsReview
	}
	if f.markReviewed {
		reviewed := false
		values.needsReview = &reviewed
	}
	return values
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

			if err := validateDateRange(*startDate, *endDate); err != nil {
				a.handleError(renderer, "transactions.list", err, start)
				return
			}
			if err := validatePositiveInt("limit", flags.limit); err != nil {
				a.handleError(renderer, "transactions.list", err, start)
				return
			}
			if err := validateNonNegativeInt("offset", flags.offset); err != nil {
				a.handleError(renderer, "transactions.list", err, start)
				return
			}

			svc, err := a.loadService()
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
				renderer.RenderSuccess(env)
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%-12s %-20s %-15s %10s %s\n", "DATE", "MERCHANT", "CATEGORY", "AMOUNT", "NOTES")
			for i := range txs {
				t := &txs[i]
				fmt.Fprintf(cmd.OutOrStdout(), "%-12s %-20s %-15s %10.2f %s\n", t.Date, t.Merchant, t.Category, t.Amount, t.Notes)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nTotal transactions: %d\n", total)
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

			if err := validateDateRange(*startDate, *endDate); err != nil {
				a.handleError(renderer, "transactions.search", err, start)
				return
			}
			if err := validatePositiveInt("limit", limitValue); err != nil {
				a.handleError(renderer, "transactions.search", err, start)
				return
			}
			if err := validateNonNegativeInt("offset", offsetValue); err != nil {
				a.handleError(renderer, "transactions.search", err, start)
				return
			}

			svc, err := a.loadService()
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
				renderer.RenderSuccess(env)
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%-12s %-20s %-15s %10s %s\n", "DATE", "MERCHANT", "CATEGORY", "AMOUNT", "NOTES")
			for i := range txs {
				t := &txs[i]
				fmt.Fprintf(cmd.OutOrStdout(), "%-12s %-20s %-15s %10.2f %s\n", t.Date, t.Merchant, t.Category, t.Amount, t.Notes)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nTotal matches: %d\n", total)
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

			svc, err := a.loadService()
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
				renderer.RenderSuccess(env)
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%-12s %-20s %10s %s\n", "DATE", "MERCHANT", "AMOUNT", "ID")
			for i := range txs {
				t := &txs[i]
				fmt.Fprintf(cmd.OutOrStdout(), "%-12s %-20s %10.2f %s\n", t.Date, t.Merchant, t.Amount, t.ID)
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

			svc, err := a.loadService()
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
				renderer.RenderSuccess(env)
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%-20s %10s %s\n", "CATEGORY", "AMOUNT", "NOTES")
			for _, s := range splits {
				fmt.Fprintf(cmd.OutOrStdout(), "%-20s %10.2f %s\n", s.Category, s.Amount, s.Notes)
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

			values := flags.values(cmd)
			if a.Flags.DryRun {
				plan := safety.NewPlan()
				plan.Add("transactions.update", id, nil, map[string]any{"notes": values.notes, "categoryId": values.categoryID, "amount": values.amount, "date": values.date, "merchant": values.merchant, "hideFromReports": values.hideFromReports, "needsReview": values.needsReview})
				a.renderPlan(renderer, "transactions.update", plan, start)
				return
			}

			svc, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "transactions.update", wrapError(err, "failed to load service"), start)
				return
			}

			result, err := a.mutate(renderer, "transactions.update", id, start, func() (any, error) {
				return svc.UpdateTransaction(cmd.Context(), id, values.notes, values.categoryID, values.amount, values.date, values.merchant, values.hideFromReports, values.needsReview)
			}, "failed to update transaction")
			if err != nil {
				return
			}
			tx, ok := result.(*monarch.Transaction)
			if !ok || tx == nil {
				a.handleError(renderer, "transactions.update", errors.New(errors.InternalError, "unexpected transaction update result", errors.CatInternal, false, nil), start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("transactions.update", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, tx, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully updated transaction %s.\n", tx.ID)
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

			svc, err := a.loadService()
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
				renderer.RenderSuccess(env)
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully deleted transaction %s.\n", id)
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

			svc, err := a.loadService()
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
			tx, ok := result.(*monarch.Transaction)
			if !ok || tx == nil {
				a.handleError(renderer, "transactions.create", errors.New(errors.InternalError, "unexpected transaction create result", errors.CatInternal, false, nil), start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("transactions.create", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, tx, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully created transaction %s.\n", tx.ID)
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
			if err := json.Unmarshal(data, &splits); err != nil {
				a.handleError(renderer, "transactions.split", errors.New(errors.ValidationFailed, "invalid split JSON: "+err.Error(), errors.CatValidation, false, err), start)
				return
			}

			if a.Flags.DryRun {
				plan := safety.NewPlan()
				plan.Add("transactions.split", id, nil, map[string]any{"splits": splits})
				a.renderPlan(renderer, "transactions.split", plan, start)
				return
			}

			svc, err := a.loadService()
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
				renderer.RenderSuccess(env)
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully split transaction %s.\n", id)
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
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode || flags.format == "json", a.Flags.Pretty)
			if flags.format != "json" && flags.format != "csv" {
				a.handleError(renderer, "transactions.export", errors.New(errors.InvalidArguments, "--format must be json or csv", errors.CatValidation, false, nil), start)
				return
			}

			if err := validateDateRange(*startDate, *endDate); err != nil {
				a.handleError(renderer, "transactions.export", err, start)
				return
			}
			if err := validatePositiveInt("limit", flags.limit); err != nil {
				a.handleError(renderer, "transactions.export", err, start)
				return
			}
			if err := validateNonNegativeInt("offset", flags.offset); err != nil {
				a.handleError(renderer, "transactions.export", err, start)
				return
			}

			svc, err := a.loadService()
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
			var file *os.File
			if flags.outputFile != "" {
				file, err = os.Create(flags.outputFile)
				if err != nil {
					a.handleError(renderer, "transactions.export", errors.New(errors.InternalError, "failed to create output file", errors.CatInternal, false, err), start)
					return
				}
				out = file
			}

			if flags.format == "csv" {
				if err := monarch.ExportTransactionsCSV(txs, out); err != nil {
					if file != nil {
						_ = file.Close()
					}
					a.handleError(renderer, "transactions.export", errors.New(errors.InternalError, "failed to export CSV", errors.CatInternal, false, err), start)
					return
				}
				if file != nil {
					if err := file.Close(); err != nil {
						a.handleError(renderer, "transactions.export", errors.New(errors.InternalError, "failed to close output file", errors.CatInternal, false, err), start)
					}
				}
				return
			}

			env := output.NewEnvelope("transactions.export", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, txs, time.Since(start))
			encoder := json.NewEncoder(out)
			if a.Flags.Pretty {
				encoder.SetIndent("", "  ")
			}
			if err := encoder.Encode(env); err != nil {
				if file != nil {
					_ = file.Close()
				}
				a.handleError(renderer, "transactions.export", errors.New(errors.InternalError, "failed to export JSON", errors.CatInternal, false, err), start)
				return
			}
			if file != nil {
				if err := file.Close(); err != nil {
					a.handleError(renderer, "transactions.export", errors.New(errors.InternalError, "failed to close output file", errors.CatInternal, false, err), start)
				}
			}
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

			svc, err := a.loadService()
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
				renderer.RenderSuccess(env)
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully set tags for transaction %s.\n", id)
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

			svc, err := a.loadService()
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
				renderer.RenderSuccess(env)
				return
			}

			if len(attachments) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No attachments found.")
				return
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-36s %-20s %s\n", "ID", "FILENAME", "SIZE")
			for _, attachment := range attachments {
				fmt.Fprintf(cmd.OutOrStdout(), "%-36s %-20s %d bytes\n", attachment.ID, attachment.Filename, attachment.SizeBytes)
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

			svc, err := a.loadService()
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
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to close file: %v\n", cerr)
				}
			}()

			if err := svc.DownloadAttachment(cmd.Context(), targetURL, f); err != nil {
				a.handleError(renderer, "transactions.attachments.download", wrapError(err, "failed to download attachment"), start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("transactions.attachments.download", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, map[string]string{"status": "downloaded", "path": outPath}, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Downloaded attachment to %s\n", outPath)
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

			svc, err := a.loadService()
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
				renderer.RenderSuccess(env)
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "ID:       %s\n", tx.ID)
			fmt.Fprintf(cmd.OutOrStdout(), "Date:     %s\n", tx.Date)
			fmt.Fprintf(cmd.OutOrStdout(), "Merchant: %s\n", tx.Merchant)
			fmt.Fprintf(cmd.OutOrStdout(), "Category: %s\n", tx.Category)
			fmt.Fprintf(cmd.OutOrStdout(), "Amount:   %.2f\n", tx.Amount)
			fmt.Fprintf(cmd.OutOrStdout(), "Notes:    %s\n", tx.Notes)
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

			if err := validateDateRange(*startDate, *endDate); err != nil {
				a.handleError(renderer, "transactions.summary", err, start)
				return
			}

			svc, err := a.loadService()
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
				renderer.RenderSuccess(env)
				return
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Transaction Summary")
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

			svc, err := a.loadService()
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
				renderer.RenderSuccess(env)
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully cleared tags for transaction %s.\n", id)
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

			svc, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "transactions.tags.add", wrapError(err, "failed to load service"), start)
				return
			}

			tx, err := svc.GetTransaction(cmd.Context(), id)
			if err != nil {
				a.handleError(renderer, "transactions.tags.add", wrapError(err, "failed to fetch current transaction"), start)
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
				renderer.RenderSuccess(env)
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully added tags to transaction %s.\n", id)
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

			svc, err := a.loadService()
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
			a.recordAudit(&audit.Record{Command: "transactions.bulk-categorize", DryRun: a.Flags.DryRun, Confirmed: a.Flags.Confirm, Profile: a.Flags.Profile, Result: result})

			if a.Flags.JSONMode {
				data := map[string]any{"total": len(ids), "successful": successes, "failed": len(failures), "errors": failures}
				env := output.NewEnvelope("transactions.bulk-categorize", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, data, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Bulk categorize: %d/%d successful.\n", successes, len(ids))
			for _, failure := range failures {
				fmt.Fprintf(cmd.OutOrStdout(), "  FAILED: %s\n", failure)
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
		Short: "Report that attachment upload is unavailable",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)
			a.handleError(renderer, "transactions.attachments.upload", errors.New(errors.FEATURE_UNAVAILABLE, "transaction attachment upload is unavailable in the current Monarch API", errors.CatAPI, false, nil), start)
		},
	}
}
