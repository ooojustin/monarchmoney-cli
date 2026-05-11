package cli

import (
	"time"

	"github.com/spf13/cobra"
	"github.com/thedavidweng/monarchmoney-cli/internal/errors"
	"github.com/thedavidweng/monarchmoney-cli/internal/output"
)

func (a *App) buildSubscriptionCommands(parent *cobra.Command) {
	subCmd := &cobra.Command{
		Use:   "subscription",
		Short: "Manage subscription details",
	}
	subCmd.AddCommand(a.buildSubscriptionShow())
	parent.AddCommand(subCmd)
}

func (a *App) buildSubscriptionShow() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show subscription details",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, a.Flags.JSONMode, a.Flags.Pretty)

			svc, _, err := a.Deps.LoadService()
			if err != nil {
				a.handleError(renderer, "subscription.show", err.(*errors.Error), start)
				return
			}

			sub, err := svc.GetSubscriptionDetails(cmd.Context())
			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.APIError, "failed to get subscription details", errors.CatAPI, false, err)
				}
				a.handleError(renderer, "subscription.show", cliErr, start)
				return
			}

			if a.Flags.JSONMode {
				env := a.envelopeWithWarnings("subscription.show", sub, start, "uses legacy Monarch GraphQL root field: subscription")
				a.renderSuccess(renderer, env, start)
			} else {
				writeText(a.Deps.Stdout, "ID:                      %s\n", sub.ID)
				writeText(a.Deps.Stdout, "Payment Source:          %s\n", sub.PaymentSource)
				writeText(a.Deps.Stdout, "Referral Code:           %s\n", sub.ReferralCode)
				writeText(a.Deps.Stdout, "On Free Trial:           %v\n", sub.IsOnFreeTrial)
				writeText(a.Deps.Stdout, "Has Premium Entitlement: %v\n", sub.HasPremiumEntitlement)
			}
		},
	}
}
