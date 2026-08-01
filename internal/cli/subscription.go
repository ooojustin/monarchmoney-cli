package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/thedavidweng/monarchmoney-cli/internal/output"
)

func (a *App) buildSubscriptionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "subscription",
		Short:   "Manage subscription details",
		GroupID: "core",
		Example: "  monarch subscription show --json",
	}
	cmd.AddCommand(a.buildSubscriptionShowCommand())
	return cmd
}

func (a *App) buildSubscriptionShowCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show subscription details",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)

			svc, err := a.loadService()
			if err != nil {
				a.handleError(renderer, "subscription.show", wrapError(err, "failed to load service"), start)
				return
			}

			sub, err := svc.GetSubscriptionDetails(cmd.Context())
			if err != nil {
				a.handleError(renderer, "subscription.show", wrapError(err, "failed to get subscription details"), start)
				return
			}

			if a.Flags.JSONMode {
				env := a.envelopeWithWarnings("subscription.show", sub, start, "uses legacy Monarch GraphQL root field: subscription")
				renderer.RenderSuccess(env)
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "ID:                      %s\n", sub.ID)
			fmt.Fprintf(cmd.OutOrStdout(), "Payment Source:          %s\n", sub.PaymentSource)
			fmt.Fprintf(cmd.OutOrStdout(), "Referral Code:           %s\n", sub.ReferralCode)
			fmt.Fprintf(cmd.OutOrStdout(), "On Free Trial:           %v\n", sub.IsOnFreeTrial)
			fmt.Fprintf(cmd.OutOrStdout(), "Has Premium Entitlement: %v\n", sub.HasPremiumEntitlement)
		},
	}
}
