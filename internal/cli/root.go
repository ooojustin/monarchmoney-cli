package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/thedavidweng/monarchmoney-cli/internal/config"
	"github.com/thedavidweng/monarchmoney-cli/internal/output"
	"github.com/thedavidweng/monarchmoney-cli/internal/version"
)

// Global flag values populated by cobra's PersistentPreRun from viper. These
// are intentionally still package-level: they're shared with config.Load and
// command bodies that read them at runtime. Migrating them off globals is a
// follow-up; this refactor focuses on dependencies, not flag state.
var (
	cfgFile  string
	jsonMode bool
	pretty   bool
	compact  bool
	full     bool
	events   bool
	readOnly bool
	dryRun   bool
	confirm  bool
	timeout  time.Duration
	profile  string
	noColor  bool
	verbose  bool
	debug    bool
)

// buildRoot constructs a fresh root cobra.Command and registers global flags
// against it. Called once by App.New per App instance.
func (a *App) buildRoot() *cobra.Command {
	root := &cobra.Command{
		Use:     "monarch",
		Short:   "A local, agent-friendly CLI for Monarch Money",
		Version: version.Version,
		Long: `monarchmoney-cli is a single-binary command line tool for working with
Monarch Money data from your terminal, scripts, and local agents.`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			jsonMode = viper.GetBool("json")
			pretty = viper.GetBool("pretty")
			compact = viper.GetBool("compact")
			full = viper.GetBool("full")
			events = viper.GetBool("events")
			readOnly = viper.GetBool("read-only")
			dryRun = viper.GetBool("dry-run")
			confirm = viper.GetBool("confirm")
			timeout = viper.GetDuration("timeout")
			profile = viper.GetString("profile")
			noColor = viper.GetBool("no-color")
			verbose = viper.GetBool("verbose")
			debug = viper.GetBool("debug")
		},
	}

	root.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.monarchmoney-cli/config.yaml)")
	root.PersistentFlags().BoolVar(&jsonMode, "json", false, "emit machine-readable JSON")
	root.PersistentFlags().BoolVar(&pretty, "pretty", false, "pretty-print JSON output")
	root.PersistentFlags().BoolVar(&compact, "compact", false, "return compact output fields")
	root.PersistentFlags().BoolVar(&full, "full", false, "return full normalized output fields")
	root.PersistentFlags().BoolVar(&events, "events", false, "emit NDJSON progress events for long-running commands")
	root.PersistentFlags().BoolVar(&readOnly, "read-only", false, "block remote writes")
	root.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "preview a remote write without executing it")
	root.PersistentFlags().BoolVar(&confirm, "confirm", false, "explicitly execute a remote write")
	root.PersistentFlags().DurationVar(&timeout, "timeout", 30*time.Second, "set command timeout")
	root.PersistentFlags().StringVar(&profile, "profile", "default", "use a named profile")
	root.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable colored output")
	root.PersistentFlags().BoolVar(&verbose, "verbose", false, "print more diagnostics to stderr")
	root.PersistentFlags().BoolVar(&debug, "debug", false, "print debug diagnostics to stderr with secrets redacted")

	viper.BindPFlag("json", root.PersistentFlags().Lookup("json"))
	viper.BindPFlag("pretty", root.PersistentFlags().Lookup("pretty"))
	viper.BindPFlag("compact", root.PersistentFlags().Lookup("compact"))
	viper.BindPFlag("full", root.PersistentFlags().Lookup("full"))
	viper.BindPFlag("events", root.PersistentFlags().Lookup("events"))
	viper.BindPFlag("read-only", root.PersistentFlags().Lookup("read-only"))
	viper.BindPFlag("dry-run", root.PersistentFlags().Lookup("dry-run"))
	viper.BindPFlag("confirm", root.PersistentFlags().Lookup("confirm"))
	viper.BindPFlag("timeout", root.PersistentFlags().Lookup("timeout"))
	viper.BindPFlag("profile", root.PersistentFlags().Lookup("profile"))
	viper.BindPFlag("no-color", root.PersistentFlags().Lookup("no-color"))
	viper.BindPFlag("verbose", root.PersistentFlags().Lookup("verbose"))
	viper.BindPFlag("debug", root.PersistentFlags().Lookup("debug"))

	cobra.OnInitialize(initConfig)

	root.AddCommand(a.buildVersion())

	return root
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
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

	if err := viper.ReadInConfig(); err == nil {
		if verbose {
			fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
		}
	}
}

func (a *App) buildVersion() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version number of monarch",
		Run: func(cmd *cobra.Command, args []string) {
			if err := writeVersion(cmd.OutOrStdout(), profile, jsonMode, pretty, time.Duration(0)); err != nil {
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
