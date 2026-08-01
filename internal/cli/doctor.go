package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/thedavidweng/monarchmoney-cli/internal/config"
	"github.com/thedavidweng/monarchmoney-cli/internal/doctor"
	"github.com/thedavidweng/monarchmoney-cli/internal/output"
)

func (a *App) buildDoctorCommand() *cobra.Command {
	var checkConnectivity bool
	command := &cobra.Command{
		Use:     "doctor",
		Short:   "Check local configuration and connectivity",
		GroupID: "utility",
		Example: "  monarch doctor",
		Run: func(cmd *cobra.Command, _ []string) {
			start := time.Now()
			cfg := a.Config
			if cfg == nil {
				cfg = config.Default()
			}
			result := doctor.Check(cmd.Context(), &doctor.Options{
				Connect:       checkConnectivity,
				ConfigPath:    a.configPath(),
				ConfigError:   a.ConfigErr,
				SessionPath:   a.sessionPath(),
				APIEndpoint:   cfg.APIEndpoint,
				Timeout:       a.Flags.Timeout,
				HTTPTransport: a.Deps.HTTPTransport,
			})
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)
			if a.Flags.JSONMode {
				env := output.NewEnvelope("doctor", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, result, time.Since(start))
				renderer.RenderSuccess(env)
				return
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Monarch Money CLI Doctor")
			fmt.Fprintf(cmd.OutOrStdout(), "Version: %s\n", result.Version)
			fmt.Fprintf(cmd.OutOrStdout(), "OS/Arch: %s/%s\n", result.OS, result.Arch)
			fmt.Fprintf(cmd.OutOrStdout(), "Config Path: %s (Exists: %v, Valid: %v)\n", result.Config.Path, result.Config.Exists, result.Config.Valid)
			fmt.Fprintf(cmd.OutOrStdout(), "Session Path: %s (Exists: %v, Auth: %v, PermOK: %v)\n", result.Session.Path, result.Session.Exists, result.Session.Authenticated, result.Session.PermissionOK)
			if checkConnectivity {
				fmt.Fprintf(cmd.OutOrStdout(), "API Connected: %v\n", result.Network.APIReachable)
			}
		},
	}
	command.Flags().BoolVar(&checkConnectivity, "connect", false, "check API connectivity (requires auth)")
	return command
}
