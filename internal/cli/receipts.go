package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/thedavidweng/monarchmoney-cli/internal/monarch"
	"github.com/thedavidweng/monarchmoney-cli/internal/output"
	"github.com/thedavidweng/monarchmoney-cli/internal/safety"
)

func (a *App) buildReceiptsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "receipts",
		Short:   "Upload receipts to the Monarch receipt inbox",
		GroupID: "core",
		Example: "  monarch receipts upload receipt.jpg --confirm",
	}
	cmd.AddCommand(a.buildReceiptsUploadCommand())
	return cmd
}

func (a *App) buildReceiptsUploadCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "upload <file>",
		Short: "Upload a receipt to the inbox for AI categorization and matching",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)
			path := args[0]

			if !a.checkSafety(renderer, "receipts.upload", safety.TierMutation, start) {
				return
			}
			if a.Flags.DryRun {
				plan := safety.NewPlan()
				plan.Add("receipts.upload", "", nil, map[string]string{"file": path})
				a.renderPlan(renderer, "receipts.upload", plan, start)
				return
			}

			svc, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "receipts.upload", wrapError(err, "failed to load service"), start)
				return
			}
			var sync *monarch.ReceiptSync
			result, err := a.mutate(renderer, "receipts.upload", "", start, func() (any, error) {
				var uploadErr error
				sync, uploadErr = svc.UploadReceiptToInbox(cmd.Context(), path)
				return sync, uploadErr
			}, "failed to upload receipt")
			if err != nil {
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("receipts.upload", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, result, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Receipt uploaded. Sync %s status: %s\n", sync.ID, sync.Status)
		},
	}
}
