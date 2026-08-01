package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/thedavidweng/monarchmoney-cli/internal/audit"
	"github.com/thedavidweng/monarchmoney-cli/internal/errors"
	"github.com/thedavidweng/monarchmoney-cli/internal/output"
)

var auditCmd = &cobra.Command{
	Use:     "audit",
	Short:   "Manage audit logs",
	GroupID: "utility",
	Example: "  monarch audit cleanup --older-than 30",
}

var auditCleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Remove audit log files older than N days",
	Run: func(cmd *cobra.Command, args []string) {
		start := time.Now()
		renderer := output.NewRenderer(nil, nil, jsonMode, pretty)

		if auditCleanupDays <= 0 {
			handleError(renderer, "audit.cleanup", errors.New(errors.InvalidArguments, "--older-than must be a positive number of days", errors.CatValidation, false, nil), start)
			return
		}

		logger := audit.NewLogger()
		removed, err := logger.Cleanup(auditCleanupDays)
		if err != nil {
			handleError(renderer, "audit.cleanup", errors.New(errors.InternalError, "failed to cleanup audit logs", errors.CatInternal, false, err), start)
			return
		}

		if jsonMode {
			env := output.NewEnvelope("audit.cleanup", profile, output.SchemaVersion, requestID, map[string]any{"removed": removed, "older_than_days": auditCleanupDays}, time.Since(start))
			renderer.RenderSuccess(env)
		} else {
			fmt.Printf("Removed %d audit log file(s) older than %d days.\n", removed, auditCleanupDays)
		}
	},
}

var auditCleanupDays int

func init() {
	auditCleanupCmd.Flags().IntVar(&auditCleanupDays, "older-than", 30, "remove logs older than N days (default 30)")

	auditCmd.AddCommand(auditCleanupCmd)
	RootCmd.AddCommand(auditCmd)
}

func (a *App) buildAuditCommand() *cobra.Command {
	var olderThanDays int
	auditCommand := &cobra.Command{
		Use:     "audit",
		Short:   "Manage audit logs",
		GroupID: "utility",
		Example: "  monarch audit cleanup --older-than 30",
	}
	cleanupCommand := &cobra.Command{
		Use:   "cleanup",
		Short: "Remove audit log files older than N days",
		Run: func(cmd *cobra.Command, _ []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)
			if olderThanDays <= 0 {
				a.handleError(renderer, "audit.cleanup", errors.New(errors.InvalidArguments, "--older-than must be a positive number of days", errors.CatValidation, false, nil), start)
				return
			}

			removed, err := a.Deps.CleanupAudit(olderThanDays)
			if err != nil {
				a.handleError(renderer, "audit.cleanup", errors.New(errors.InternalError, "failed to cleanup audit logs", errors.CatInternal, false, err), start)
				return
			}
			if a.Flags.JSONMode {
				env := output.NewEnvelope("audit.cleanup", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, map[string]any{"removed": removed, "older_than_days": olderThanDays}, time.Since(start))
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed %d audit log file(s) older than %d days.\n", removed, olderThanDays) //nolint:errcheck // best-effort output
		},
	}
	cleanupCommand.Flags().IntVar(&olderThanDays, "older-than", 30, "remove logs older than N days (default 30)")
	auditCommand.AddCommand(cleanupCommand)
	return auditCommand
}
