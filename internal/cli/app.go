package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/thedavidweng/monarchmoney-cli/internal/audit"
	"github.com/thedavidweng/monarchmoney-cli/internal/auth"
	"github.com/thedavidweng/monarchmoney-cli/internal/config"
	clierrors "github.com/thedavidweng/monarchmoney-cli/internal/errors"
	"github.com/thedavidweng/monarchmoney-cli/internal/graphql"
	"github.com/thedavidweng/monarchmoney-cli/internal/monarch"
	"github.com/thedavidweng/monarchmoney-cli/internal/output"
	"github.com/thedavidweng/monarchmoney-cli/internal/version"
	"golang.org/x/term"
)

type GraphQLClient interface {
	Do(context.Context, *graphql.Request, any) error
	DoMutation(context.Context, *graphql.Request, any) error
	TokenValue() string
}

type App struct {
	Deps      Deps
	Flags     Flags
	Config    *config.Config
	ConfigErr error
	Root      *cobra.Command
}

type Flags struct {
	Config    string
	JSONMode  bool
	Pretty    bool
	Events    bool
	ReadOnly  bool
	DryRun    bool
	Confirm   bool
	Timeout   time.Duration
	Profile   string
	Verbose   bool
	RequestID string
}

type Deps struct {
	LoadConfig    func(path string) (*config.Config, error)
	Getenv        func(string) string
	NewRequestID  func() string
	HTTPTransport http.RoundTripper

	ConfigPath   func() string
	SessionPath  func() string
	NewStore     func(path string) *auth.Store
	Authenticate func(email, password, mfaCode, mfaSecret string) (*auth.Session, error)
	NewClient    func(endpoint, token string, timeout time.Duration) *graphql.Client
	NewService   func(client GraphQLClient) *monarch.Service
	LoadService  func() (*monarch.Service, *auth.Session, error)
	WriteAudit   func(record *audit.Record) error

	Stdout       io.Writer
	Stderr       io.Writer
	Stdin        io.Reader
	ReadPassword func(fd int) ([]byte, error)
	IsTerminal   func(fd int) bool
	Exit         func(code int)

	JSONUnmarshal func([]byte, any) error
}

func DefaultDeps() Deps {
	return Deps{
		LoadConfig:   config.Load,
		Getenv:       os.Getenv,
		NewRequestID: uuid.NewString,
		NewStore:     auth.NewStore,
		Authenticate: auth.Authenticate,
		WriteAudit: func(record *audit.Record) error {
			return audit.NewLogger().Log(record)
		},
		Stdout:        os.Stdout,
		Stderr:        os.Stderr,
		Stdin:         os.Stdin,
		ReadPassword:  term.ReadPassword,
		IsTerminal:    term.IsTerminal,
		Exit:          os.Exit,
		JSONUnmarshal: json.Unmarshal,
	}
}

func New(deps Deps) *App {
	a := &App{Deps: deps}
	defaults := DefaultDeps()
	if a.Deps.LoadConfig == nil {
		a.Deps.LoadConfig = defaults.LoadConfig
	}
	if a.Deps.Getenv == nil {
		a.Deps.Getenv = defaults.Getenv
	}
	if a.Deps.NewRequestID == nil {
		a.Deps.NewRequestID = defaults.NewRequestID
	}
	if a.Deps.NewStore == nil {
		a.Deps.NewStore = defaults.NewStore
	}
	if a.Deps.Authenticate == nil {
		a.Deps.Authenticate = defaults.Authenticate
	}
	if a.Deps.Stdout == nil {
		a.Deps.Stdout = defaults.Stdout
	}
	if a.Deps.Stderr == nil {
		a.Deps.Stderr = defaults.Stderr
	}
	if a.Deps.Stdin == nil {
		a.Deps.Stdin = defaults.Stdin
	}
	if a.Deps.ReadPassword == nil {
		a.Deps.ReadPassword = defaults.ReadPassword
	}
	if a.Deps.IsTerminal == nil {
		a.Deps.IsTerminal = defaults.IsTerminal
	}
	if a.Deps.Exit == nil {
		a.Deps.Exit = defaults.Exit
	}
	if a.Deps.WriteAudit == nil {
		a.Deps.WriteAudit = defaults.WriteAudit
	}
	if a.Deps.JSONUnmarshal == nil {
		a.Deps.JSONUnmarshal = defaults.JSONUnmarshal
	}
	if a.Deps.ConfigPath == nil {
		a.Deps.ConfigPath = func() string {
			if a.Flags.Config != "" {
				return a.Flags.Config
			}
			if path := a.Deps.Getenv("MONARCH_CONFIG"); path != "" {
				return path
			}
			return config.DefaultConfigPath()
		}
	}
	if a.Deps.SessionPath == nil {
		a.Deps.SessionPath = func() string {
			if a.Config != nil && a.Config.SessionPath != "" {
				return a.Config.SessionPath
			}
			cfg, _ := a.Deps.LoadConfig(a.Deps.ConfigPath())
			if cfg != nil && cfg.SessionPath != "" {
				return cfg.SessionPath
			}
			return config.DefaultSessionPath()
		}
	}
	if a.Deps.NewClient == nil {
		a.Deps.NewClient = func(endpoint, token string, timeout time.Duration) *graphql.Client {
			return graphql.NewClient(endpoint, token, timeout, graphql.WithHTTPTransport(a.Deps.HTTPTransport))
		}
	}
	if a.Deps.NewService == nil {
		a.Deps.NewService = func(client GraphQLClient) *monarch.Service {
			return monarch.NewService(client, monarch.WithHTTPTransport(a.Deps.HTTPTransport))
		}
	}
	if a.Deps.LoadService == nil {
		a.Deps.LoadService = a.loadService
	}

	a.Root = a.buildRoot()
	return a
}

func (a *App) Execute() error {
	return a.Root.Execute()
}

func (a *App) buildRoot() *cobra.Command {
	root := &cobra.Command{
		Use:     "monarch",
		Short:   "A local, agent-friendly CLI for Monarch Money",
		Version: version.GetVersion(),
		Long: `monarchmoney-cli is a single-binary command line tool for working with
Monarch Money data from your terminal, scripts, and local agents.`,
		Example: `  monarch accounts list --json
  monarch transactions search "Amazon" --from 2024-01-01
  monarch transactions update tx_123 --category cat_food --dry-run
  monarch cashflow spending --from 2024-01-01 --to 2024-01-31
  monarch rules list --json`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			a.prepareRuntime(cmd)
		},
	}
	root.SetOut(a.Deps.Stdout)
	root.SetErr(a.Deps.Stderr)
	root.SetIn(a.Deps.Stdin)

	root.AddGroup(&cobra.Group{ID: "core", Title: "Core Commands"})
	root.AddGroup(&cobra.Group{ID: "analysis", Title: "Analysis & Insights"})
	root.AddGroup(&cobra.Group{ID: "utility", Title: "Utilities"})

	root.PersistentFlags().StringVar(&a.Flags.Config, "config", "", "config file (default is $HOME/.monarchmoney-cli/config.yaml)")
	root.PersistentFlags().BoolVar(&a.Flags.JSONMode, "json", false, "emit machine-readable JSON")
	root.PersistentFlags().BoolVar(&a.Flags.Pretty, "pretty", false, "pretty-print JSON output")
	root.PersistentFlags().BoolVar(&a.Flags.Events, "events", false, "emit NDJSON progress events (accounts refresh --wait)")
	root.PersistentFlags().BoolVar(&a.Flags.ReadOnly, "read-only", false, "block remote writes")
	root.PersistentFlags().BoolVar(&a.Flags.DryRun, "dry-run", false, "preview a remote write without executing it")
	root.PersistentFlags().BoolVar(&a.Flags.Confirm, "confirm", false, "explicitly execute a remote write")
	root.PersistentFlags().DurationVar(&a.Flags.Timeout, "timeout", 30*time.Second, "set command timeout")
	root.PersistentFlags().StringVar(&a.Flags.Profile, "profile", "default", "use a named profile")
	root.PersistentFlags().BoolVar(&a.Flags.Verbose, "verbose", false, "print more diagnostics to stderr")
	root.AddCommand(a.buildVersion())
	root.AddCommand(a.buildCreditCommand())
	root.AddCommand(a.buildSubscriptionCommand())
	root.AddCommand(a.buildInstitutionsCommand())
	root.AddCommand(a.buildGoalsCommand())
	root.AddCommand(a.buildCashflowCommand())
	root.AddCommand(a.buildInvestmentsCommand())
	root.AddCommand(a.buildCategoriesCommand())
	root.AddCommand(a.buildRecurringCommand())
	root.AddCommand(a.buildTagsCommand())
	root.AddCommand(a.buildRulesCommand())
	root.AddCommand(a.buildBudgetsCommand())
	return root
}

func (a *App) prepareRuntime(cmd *cobra.Command) {
	if !persistentFlagChanged(cmd, "config") {
		a.Flags.Config = a.Deps.Getenv("MONARCH_CONFIG")
	}
	a.Config, a.ConfigErr = a.Deps.LoadConfig(a.Deps.ConfigPath())
	if a.Config == nil {
		a.Config, _ = config.Load("")
	}
	if !persistentFlagChanged(cmd, "profile") {
		a.Flags.Profile = a.Config.Profile
	}
	if !persistentFlagChanged(cmd, "timeout") {
		a.Flags.Timeout = a.Config.Timeout
	}
	if !persistentFlagChanged(cmd, "read-only") {
		a.Flags.ReadOnly = a.Config.ReadOnly || config.ParseBool(a.Deps.Getenv("MONARCH_READONLY"))
	}
	if !persistentFlagChanged(cmd, "json") {
		a.Flags.JSONMode = config.ParseBool(a.Deps.Getenv("MONARCH_JSON"))
	}
	if !persistentFlagChanged(cmd, "pretty") {
		a.Flags.Pretty = config.ParseBool(a.Deps.Getenv("MONARCH_PRETTY"))
	}
	if !persistentFlagChanged(cmd, "events") {
		a.Flags.Events = config.ParseBool(a.Deps.Getenv("MONARCH_EVENTS"))
	}
	if !persistentFlagChanged(cmd, "dry-run") {
		a.Flags.DryRun = config.ParseBool(a.Deps.Getenv("MONARCH_DRY_RUN"))
	}
	if !persistentFlagChanged(cmd, "confirm") {
		a.Flags.Confirm = config.ParseBool(a.Deps.Getenv("MONARCH_CONFIRM"))
	}
	if !persistentFlagChanged(cmd, "verbose") {
		a.Flags.Verbose = config.ParseBool(a.Deps.Getenv("MONARCH_VERBOSE"))
	}
	a.Flags.RequestID = a.Deps.NewRequestID()
	syncLegacyGlobals(a.Flags)
}

func (a *App) buildVersion() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version number of monarch",
		Run: func(cmd *cobra.Command, _ []string) {
			if err := writeVersion(cmd.OutOrStdout(), a.Flags.Profile, a.Flags.JSONMode, a.Flags.Pretty, a.Flags.RequestID, 0); err != nil {
				a.Deps.Exit(1)
			}
		},
	}
}

func (a *App) loadService() (*monarch.Service, *auth.Session, error) {
	if a.ConfigErr != nil {
		return nil, nil, clierrors.New(clierrors.InternalError, "failed to load config", clierrors.CatInternal, false, a.ConfigErr)
	}
	if a.Config == nil {
		return nil, nil, clierrors.New(clierrors.InternalError, "configuration not initialized", clierrors.CatInternal, false, nil)
	}
	store := a.Deps.NewStore(a.Config.SessionPath)
	sess, err := store.Load()
	if err != nil {
		return nil, nil, clierrors.New(clierrors.AuthRequired, "not logged in", clierrors.CatAuth, false, err)
	}
	client := a.Deps.NewClient(a.Config.APIEndpoint, sess.Token, a.Config.Timeout)
	return a.Deps.NewService(client), sess, nil
}

func (a *App) handleError(renderer *output.Renderer, command string, err *clierrors.Error, start time.Time) {
	if err != nil && err.Code == clierrors.AuthSessionExpired {
		path := a.Deps.SessionPath()
		if sess, loadErr := a.Deps.NewStore(path).Load(); loadErr == nil {
			message := fmt.Sprintf("session token stored at %s expired or invalid; run `monarch auth login` again", path)
			if sess.Email != "" {
				message = fmt.Sprintf("session token for %s stored at %s expired or invalid; run `monarch auth login` again", sess.Email, path)
			}
			err = clierrors.New(err.Code, message, err.Category, err.Retryable, err.Err)
		}
	}
	env := output.NewErrorEnvelope(command, a.Flags.Profile, output.SchemaVersion, err, time.Since(start))
	env.Meta.RequestID = a.Flags.RequestID
	renderer.RenderError(env)
	a.Deps.Exit(err.ExitCode())
}
