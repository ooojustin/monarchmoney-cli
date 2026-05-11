package cli

import (
	"time"

	"github.com/spf13/cobra"
	"github.com/thedavidweng/monarchmoney-cli/internal/doctor"
	"github.com/thedavidweng/monarchmoney-cli/internal/output"
)

func (a *App) buildDoctorCommand(parent *cobra.Command) {
	var connect bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check local configuration and connectivity",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			res := doctor.Check(cmd.Context(), connect, doctor.Options{
				ConfigPath:      a.Deps.ConfigPath(),
				SessionPath:     a.Deps.SessionPath(),
				GraphQLEndpoint: a.Deps.GraphQLEndpoint(),
				Timeout:         a.Deps.Timeout(),
				HTTPTransport:   a.Deps.HTTPTransport,
			})

			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, a.Flags.JSONMode, a.Flags.Pretty)

			if a.Flags.JSONMode {
				env := output.NewEnvelope("doctor", a.Flags.Profile, output.SchemaVersion, "", res, time.Since(start))
				a.renderSuccess(renderer, env, start)
			} else {
				printlnText(a.Deps.Stdout, "Monarch Money CLI Doctor")
				writeText(a.Deps.Stdout, "Version: %s\n", res.Version)
				writeText(a.Deps.Stdout, "OS/Arch: %s/%s\n", res.OS, res.Arch)
				writeText(a.Deps.Stdout, "Config Path: %s (Exists: %v)\n", res.Config.Path, res.Config.Exists)
				writeText(a.Deps.Stdout, "Session Path: %s (Exists: %v, Auth: %v, PermOK: %v)\n", res.Session.Path, res.Session.Exists, res.Session.Authenticated, res.Session.PermissionOK)
				if connect {
					writeText(a.Deps.Stdout, "API Connected: %v\n", res.Network.APIReachable)
				}
			}
		},
	}
	cmd.Flags().BoolVar(&connect, "connect", false, "check API connectivity (requires auth)")
	parent.AddCommand(cmd)
}
