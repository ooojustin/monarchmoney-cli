package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/thedavidweng/monarchmoney-cli/internal/config"
	"github.com/thedavidweng/monarchmoney-cli/internal/doctor"
	"github.com/thedavidweng/monarchmoney-cli/internal/output"
)

var connect bool

var doctorCmd = &cobra.Command{
	Use:     "doctor",
	Short:   "Check local configuration and connectivity",
	GroupID: "utility",
	Example: "  monarch doctor",
	Run: func(cmd *cobra.Command, args []string) {
		start := time.Now()
		configPath := cfgFile
		if configPath == "" {
			configPath = config.DefaultConfigPath()
		}
		cfg, cfgErr := config.Load(configPath)
		res := doctor.Check(cmd.Context(), doctor.Options{
			Connect:     connect,
			ConfigPath:  configPath,
			ConfigError: cfgErr,
			SessionPath: cfg.SessionPath,
			APIEndpoint: cfg.APIEndpoint,
			Timeout:     cfg.Timeout,
		})

		renderer := output.NewRenderer(nil, nil, jsonMode, pretty)

		if jsonMode {
			env := output.NewEnvelope("doctor", profile, output.SchemaVersion, requestID, res, time.Since(start))
			renderer.RenderSuccess(env)
		} else {
			fmt.Println("Monarch Money CLI Doctor")
			fmt.Printf("Version: %s\n", res.Version)
			fmt.Printf("OS/Arch: %s/%s\n", res.OS, res.Arch)
			fmt.Printf("Config Path: %s (Exists: %v)\n", res.Config.Path, res.Config.Exists)
			fmt.Printf("Session Path: %s (Exists: %v, Auth: %v, PermOK: %v)\n", res.Session.Path, res.Session.Exists, res.Session.Authenticated, res.Session.PermissionOK)
			if connect {
				fmt.Printf("API Connected: %v\n", res.Network.APIReachable)
			}
		}
	},
}

func init() {
	doctorCmd.Flags().BoolVar(&connect, "connect", false, "check API connectivity (requires auth)")
	RootCmd.AddCommand(doctorCmd)
}

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
			result := doctor.Check(cmd.Context(), doctor.Options{
				Connect:       checkConnectivity,
				ConfigPath:    a.configPath(),
				ConfigError:   a.ConfigErr,
				SessionPath:   cfg.SessionPath,
				APIEndpoint:   cfg.APIEndpoint,
				Timeout:       a.Flags.Timeout,
				HTTPTransport: a.Deps.HTTPTransport,
			})
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)
			if a.Flags.JSONMode {
				env := output.NewEnvelope("doctor", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, result, time.Since(start))
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Monarch Money CLI Doctor")                                                                                                                                    //nolint:errcheck // best-effort output
			fmt.Fprintf(cmd.OutOrStdout(), "Version: %s\n", result.Version)                                                                                                                                //nolint:errcheck // best-effort output
			fmt.Fprintf(cmd.OutOrStdout(), "OS/Arch: %s/%s\n", result.OS, result.Arch)                                                                                                                     //nolint:errcheck // best-effort output
			fmt.Fprintf(cmd.OutOrStdout(), "Config Path: %s (Exists: %v, Valid: %v)\n", result.Config.Path, result.Config.Exists, result.Config.Valid)                                                     //nolint:errcheck // best-effort output
			fmt.Fprintf(cmd.OutOrStdout(), "Session Path: %s (Exists: %v, Auth: %v, PermOK: %v)\n", result.Session.Path, result.Session.Exists, result.Session.Authenticated, result.Session.PermissionOK) //nolint:errcheck // best-effort output
			if checkConnectivity {
				fmt.Fprintf(cmd.OutOrStdout(), "API Connected: %v\n", result.Network.APIReachable) //nolint:errcheck // best-effort output
			}
		},
	}
	command.Flags().BoolVar(&checkConnectivity, "connect", false, "check API connectivity (requires auth)")
	return command
}
