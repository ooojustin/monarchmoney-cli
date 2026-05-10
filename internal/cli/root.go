package cli

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/thedavidweng/monarchmoney-cli/internal/config"
	"github.com/thedavidweng/monarchmoney-cli/internal/output"
	"github.com/thedavidweng/monarchmoney-cli/internal/version"
)

// globalRegistration serializes the parts of buildRoot that touch
// process-global state: viper's flag bindings and cobra's OnInitialize hook.
// Production constructs exactly one App so the mutex is uncontended; parallel
// tests construct one App per test and the mutex prevents concurrent map
// writes inside viper. Per-App state (App.Flags, the cobra command tree)
// stays lock-free.
var globalRegistration sync.Mutex

// buildRoot constructs a fresh root cobra.Command and registers global flags
// against it. Called once by App.New per App instance.
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
			a.Flags.JSONMode = viper.GetBool("json")
			a.Flags.Pretty = viper.GetBool("pretty")
			a.Flags.ReadOnly = viper.GetBool("read-only")
			a.Flags.DryRun = viper.GetBool("dry-run")
			a.Flags.Confirm = viper.GetBool("confirm")
			a.Flags.Profile = viper.GetString("profile")
		},
	}

	root.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.monarchmoney-cli/config.yaml)")
	root.PersistentFlags().BoolVar(&a.Flags.JSONMode, "json", false, "emit machine-readable JSON")
	root.PersistentFlags().BoolVar(&a.Flags.Pretty, "pretty", false, "pretty-print JSON output")
	root.PersistentFlags().BoolVar(&a.Flags.ReadOnly, "read-only", false, "block remote writes")
	root.PersistentFlags().BoolVar(&a.Flags.DryRun, "dry-run", false, "preview a remote write without executing it")
	root.PersistentFlags().BoolVar(&a.Flags.Confirm, "confirm", false, "explicitly execute a remote write")
	root.PersistentFlags().DurationVar(&timeout, "timeout", 30*time.Second, "set command timeout")
	root.PersistentFlags().StringVar(&a.Flags.Profile, "profile", "default", "use a named profile")

	viper.BindPFlag("json", root.PersistentFlags().Lookup("json"))
	viper.BindPFlag("pretty", root.PersistentFlags().Lookup("pretty"))
	viper.BindPFlag("read-only", root.PersistentFlags().Lookup("read-only"))
	viper.BindPFlag("dry-run", root.PersistentFlags().Lookup("dry-run"))
	viper.BindPFlag("confirm", root.PersistentFlags().Lookup("confirm"))
	viper.BindPFlag("timeout", root.PersistentFlags().Lookup("timeout"))
	viper.BindPFlag("profile", root.PersistentFlags().Lookup("profile"))

	cobra.OnInitialize(func() {
		if cfgFile != "" {
			viper.SetConfigFile(cfgFile)
		} else {
			viper.AddConfigPath(config.DefaultDir())
			viper.SetConfigType("yaml")
			viper.SetConfigName("config")
		}

		viper.SetEnvPrefix("MONARCH")
		viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
		viper.AutomaticEnv()

		viper.BindEnv("read-only", "MONARCH_READONLY")
		viper.BindEnv("profile", "MONARCH_PROFILE")
		viper.BindEnv("timeout", "MONARCH_TIMEOUT")
		viper.BindEnv("config", "MONARCH_CONFIG")

		_ = viper.ReadInConfig()
	})

	root.AddCommand(a.buildVersion())

	return root
}

func (a *App) buildVersion() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version number of monarch",
		Run: func(cmd *cobra.Command, args []string) {
			if err := writeVersion(cmd.OutOrStdout(), a.Flags.Profile, a.Flags.JSONMode, a.Flags.Pretty, time.Duration(0)); err != nil {
				fmt.Fprintln(cmd.ErrOrStderr(), err)
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

	_, err := fmt.Fprintf(out, "monarch version %s (commit: %s, date: %s)\n", version.Version, version.Commit, version.Date)
	return err
}
