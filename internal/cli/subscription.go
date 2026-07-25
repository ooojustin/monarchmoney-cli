package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/thedavidweng/monarchmoney-cli/internal/monarch"
)

var subscriptionCmd = &cobra.Command{
	Use:     "subscription",
	Short:   "Manage subscription details",
	GroupID: "core",
	Example: "  monarch subscription show --json",
}

var subscriptionShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show subscription details",
	Run: func(cmd *cobra.Command, args []string) {
		runWarn(cmd.Context(), "subscription.show", "failed to get subscription details",
			[]string{"uses legacy Monarch GraphQL root field: subscription"},
			func(ctx context.Context, svc *monarch.Service) (*monarch.Subscription, error) {
				return svc.GetSubscriptionDetails(ctx)
			},
			func(sub *monarch.Subscription) {
				fmt.Printf("ID:                      %s\n", sub.ID)
				fmt.Printf("Payment Source:          %s\n", sub.PaymentSource)
				fmt.Printf("Referral Code:           %s\n", sub.ReferralCode)
				fmt.Printf("On Free Trial:           %v\n", sub.IsOnFreeTrial)
				fmt.Printf("Has Premium Entitlement: %v\n", sub.HasPremiumEntitlement)
			})
	},
}

func init() {
	subscriptionCmd.AddCommand(subscriptionShowCmd)
	RootCmd.AddCommand(subscriptionCmd)
}
