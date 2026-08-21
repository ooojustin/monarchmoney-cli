package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/thedavidweng/monarchmoney-cli/internal/errors"
	"github.com/thedavidweng/monarchmoney-cli/internal/monarch"
	"github.com/thedavidweng/monarchmoney-cli/internal/safety"
)

var receiptsCmd = &cobra.Command{
	Use:     "receipts",
	Short:   "Upload receipts to the Monarch receipt inbox",
	GroupID: "core",
	Example: "  monarch receipts upload receipt.jpg --confirm",
}

var receiptsUploadCmd = &cobra.Command{
	Use:   "upload <file>",
	Short: "Upload a receipt to the inbox for AI categorization and matching",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		path := args[0]
		runMutation(cmd, "receipts.upload", "failed to upload receipt", safety.TierMutation, func() (mutation, *errors.Error) {
			var sync *monarch.ReceiptSync
			return mutation{
				planAfter: map[string]string{"file": path},
				do: func(ctx context.Context, svc *monarch.Service) (any, error) {
					result, err := svc.UploadReceiptToInbox(ctx, path)
					if err != nil {
						return nil, err
					}
					sync = result
					return result, nil
				},
				human: func() {
					fmt.Printf("Receipt uploaded. Sync %s status: %s\n", sync.ID, sync.Status)
				},
			}, nil
		})
	},
}

func init() {
	receiptsCmd.AddCommand(receiptsUploadCmd)
	RootCmd.AddCommand(receiptsCmd)
}
