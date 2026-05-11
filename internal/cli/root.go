package cli

import (
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"
	"github.com/thedavidweng/monarchmoney-cli/internal/config"
	"github.com/thedavidweng/monarchmoney-cli/internal/output"
	"github.com/thedavidweng/monarchmoney-cli/internal/version"
)

// buildRoot constructs a fresh root cobra.Command and registers global flags
// against it. Called once by App.New per App instance. All viper calls go
// through a.Deps.Viper, so each App owns its configuration state and parallel
// construction is race-free.
func (a *App) buildRoot() *cobra.Command {
	var (
		cfgFile string
		timeout time.Duration
	)

	root := &cobra.Command{
		Use:     "monarch",
		Short:   "A local, agent-friendly CLI for Monarch Money",
		Version: version.Version,
		Long: `monarchmoney-cli is a single-binary command line tool for working with
Monarch Money data from your terminal, scripts, and local agents.`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			v := a.Deps.Viper

			// Resolve config file from flag or default locations and merge
			// it into the per-App viper. Errors are intentionally ignored:
			// running without a config file is supported. Done here rather
			// than via Cobra's global initialization hook so each App's setup is fully
			// scoped to its own viper, avoiding the global hook list.
			configFile := cfgFile
			if configFile == "" {
				configFile = v.GetString("config")
			}
			if configFile != "" {
				v.SetConfigFile(configFile)
			} else {
				v.AddConfigPath(config.DefaultDir())
				v.SetConfigType("yaml")
				v.SetConfigName("config")
			}
			_ = v.ReadInConfig()

			a.Flags.JSONMode = v.GetBool("json")
			a.Flags.Pretty = v.GetBool("pretty")
			a.Flags.ReadOnly = v.GetBool("read_only")
			a.Flags.DryRun = v.GetBool("dry-run")
			a.Flags.Confirm = v.GetBool("confirm")
			a.Flags.Profile = v.GetString("profile")
		},
	}
	root.SetOut(a.Deps.Stdout)
	root.SetErr(a.Deps.Stderr)
	root.SetIn(a.Deps.Stdin)

	root.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.monarchmoney-cli/config.yaml)")
	root.PersistentFlags().BoolVar(&a.Flags.JSONMode, "json", false, "emit machine-readable JSON")
	root.PersistentFlags().BoolVar(&a.Flags.Pretty, "pretty", false, "pretty-print JSON output")
	root.PersistentFlags().BoolVar(&a.Flags.ReadOnly, "read-only", false, "block remote writes")
	root.PersistentFlags().BoolVar(&a.Flags.DryRun, "dry-run", false, "preview a remote write without executing it")
	root.PersistentFlags().BoolVar(&a.Flags.Confirm, "confirm", false, "explicitly execute a remote write")
	root.PersistentFlags().DurationVar(&timeout, "timeout", 30*time.Second, "set command timeout")
	root.PersistentFlags().StringVar(&a.Flags.Profile, "profile", "default", "use a named profile")

	mustBindPFlag(a.Deps.Viper, "json", root.PersistentFlags().Lookup("json"))
	mustBindPFlag(a.Deps.Viper, "pretty", root.PersistentFlags().Lookup("pretty"))
	mustBindPFlag(a.Deps.Viper, "read_only", root.PersistentFlags().Lookup("read-only"))
	mustBindPFlag(a.Deps.Viper, "dry-run", root.PersistentFlags().Lookup("dry-run"))
	mustBindPFlag(a.Deps.Viper, "confirm", root.PersistentFlags().Lookup("confirm"))
	mustBindPFlag(a.Deps.Viper, "timeout", root.PersistentFlags().Lookup("timeout"))
	mustBindPFlag(a.Deps.Viper, "profile", root.PersistentFlags().Lookup("profile"))
	mustBindPFlag(a.Deps.Viper, "config", root.PersistentFlags().Lookup("config"))

	root.AddCommand(a.buildVersion())

	return root
}

func (a *App) buildVersion() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version number of monarch",
		Run: func(cmd *cobra.Command, args []string) {
			if err := writeVersion(cmd.OutOrStdout(), a.Flags.Profile, a.Flags.JSONMode, a.Flags.Pretty, time.Duration(0)); err != nil {
				printlnText(cmd.ErrOrStderr(), err)
				a.Deps.Exit(1)
			}
		},
	}
}

type versionPayload struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
}

func writeVersion(out io.Writer, profileName string, jsonOut, prettyOut bool, duration time.Duration) error {
	if jsonOut {
		renderer := output.NewRenderer(out, nil, true, prettyOut)
		env := output.NewEnvelope("version", profileName, output.SchemaVersion, "", versionPayload{
			Version: version.Version,
			Commit:  version.Commit,
			Date:    version.Date,
		}, duration)
		return renderer.RenderSuccess(env)
	}

	if _, err := fmt.Fprint(out, monarchBanner); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out); err != nil {
		return err
	}
	_, err := fmt.Fprintf(out, "monarch version %s (commit: %s, date: %s)\n", version.Version, version.Commit, version.Date)
	return err
}
