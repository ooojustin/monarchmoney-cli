package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/thedavidweng/monarchmoney-cli/internal/config"
	"github.com/thedavidweng/monarchmoney-cli/internal/errors"
	"github.com/thedavidweng/monarchmoney-cli/internal/output"
	"github.com/thedavidweng/monarchmoney-cli/internal/version"
)

var (
	cfgFile   string
	jsonMode  bool
	pretty    bool
	events    bool
	readOnly  bool
	dryRun    bool
	confirm   bool
	timeout   time.Duration
	profile   string
	verbose   bool
	requestID string
)

var RootCmd = &cobra.Command{
	Use:     "monarch",
	Short:   "A local, agent-friendly CLI for Monarch Money",
	Version: version.Version,
	Long: `monarchmoney-cli is a single-binary command line tool for working with
Monarch Money data from your terminal, scripts, and local agents.`,
	Example: `  monarch accounts list --json
  monarch transactions search "Amazon" --from 2024-01-01
  monarch transactions update tx_123 --category cat_food --dry-run
  monarch cashflow spending --from 2024-01-01 --to 2024-01-31
  monarch rules list --json`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		requestID = uuid.NewString()
		jsonMode = viper.GetBool("json")
		pretty = viper.GetBool("pretty")
		events = viper.GetBool("events")
		readOnly = viper.GetBool("read-only")
		dryRun = viper.GetBool("dry-run")
		confirm = viper.GetBool("confirm")
		timeout = viper.GetDuration("timeout")
		profile = viper.GetString("profile")
		verbose = viper.GetBool("verbose")
	},
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func Execute() {
	if err := RootCmd.Execute(); err != nil {
		if e, ok := err.(*errors.Error); ok {
			fmt.Println(err)
			os.Exit(e.ExitCode())
		}
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	RootCmd.AddGroup(&cobra.Group{ID: "core", Title: "Core Commands"})
	RootCmd.AddGroup(&cobra.Group{ID: "analysis", Title: "Analysis & Insights"})
	RootCmd.AddGroup(&cobra.Group{ID: "utility", Title: "Utilities"})

	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.monarchmoney-cli/config.yaml)")
	RootCmd.PersistentFlags().BoolVar(&jsonMode, "json", false, "emit machine-readable JSON")
	RootCmd.PersistentFlags().BoolVar(&pretty, "pretty", false, "pretty-print JSON output")
	RootCmd.PersistentFlags().BoolVar(&events, "events", false, "emit NDJSON progress events (accounts refresh --wait)")
	RootCmd.PersistentFlags().BoolVar(&readOnly, "read-only", false, "block remote writes")
	RootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "preview a remote write without executing it")
	RootCmd.PersistentFlags().BoolVar(&confirm, "confirm", false, "explicitly execute a remote write")
	RootCmd.PersistentFlags().DurationVar(&timeout, "timeout", 30*time.Second, "set command timeout")
	RootCmd.PersistentFlags().StringVar(&profile, "profile", "default", "use a named profile")
	RootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "print more diagnostics to stderr")

	must(viper.BindPFlag("json", RootCmd.PersistentFlags().Lookup("json")))
	must(viper.BindPFlag("pretty", RootCmd.PersistentFlags().Lookup("pretty")))
	must(viper.BindPFlag("events", RootCmd.PersistentFlags().Lookup("events")))
	must(viper.BindPFlag("read-only", RootCmd.PersistentFlags().Lookup("read-only")))
	must(viper.BindPFlag("dry-run", RootCmd.PersistentFlags().Lookup("dry-run")))
	must(viper.BindPFlag("confirm", RootCmd.PersistentFlags().Lookup("confirm")))
	must(viper.BindPFlag("timeout", RootCmd.PersistentFlags().Lookup("timeout")))
	must(viper.BindPFlag("profile", RootCmd.PersistentFlags().Lookup("profile")))
	must(viper.BindPFlag("verbose", RootCmd.PersistentFlags().Lookup("verbose")))

	RootCmd.AddCommand(versionCmd)
}

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

	must(viper.BindEnv("read-only", "MONARCH_READONLY"))
	must(viper.BindEnv("profile", "MONARCH_PROFILE"))
	must(viper.BindEnv("timeout", "MONARCH_TIMEOUT"))
	must(viper.BindEnv("config", "MONARCH_CONFIG"))

	if err := viper.ReadInConfig(); err == nil {
		if verbose {
			fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
		}
	}
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of monarch",
	Run: func(cmd *cobra.Command, args []string) {
		if err := writeVersion(cmd.OutOrStdout(), profile, jsonMode, pretty, time.Duration(0)); err != nil {
			fmt.Fprintln(cmd.ErrOrStderr(), err)
			os.Exit(1)
		}
	},
}

type versionPayload struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
	BuiltBy string `json:"built_by"`
}

func writeVersion(out io.Writer, profileName string, jsonOut, prettyOut bool, duration time.Duration) error {
	if jsonOut {
		renderer := output.NewRenderer(out, nil, true, prettyOut)
		env := output.NewEnvelope("version", profileName, output.SchemaVersion, requestID, versionPayload{
			Version: version.Version,
			Commit:  version.Commit,
			Date:    version.Date,
			BuiltBy: version.BuiltBy,
		}, duration)
		renderer.RenderSuccess(env)
		return nil
	}

	fmt.Fprint(out, monarchBanner)
	fmt.Fprintln(out)
	_, err := fmt.Fprintf(out, "monarch version %s (commit: %s, date: %s, built by: %s)\n", version.Version, version.Commit, version.Date, version.BuiltBy)
	return err
}
