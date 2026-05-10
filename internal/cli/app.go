package cli

import (
	"net/http"
	"time"

	"github.com/spf13/cobra"
	"github.com/thedavidweng/monarchmoney-cli/internal/auth"
	"github.com/thedavidweng/monarchmoney-cli/internal/config"
	"github.com/thedavidweng/monarchmoney-cli/internal/graphql"
	"github.com/thedavidweng/monarchmoney-cli/internal/monarch"
)

// App owns the root cobra command, the dependencies that command handlers
// reach for at runtime, and the parsed values of root persistent flags.
// Tests construct an App with custom Deps and Flags to drive behavior;
// production wires DefaultDeps via cmd/monarch/main.go.
type App struct {
	Deps  Deps
	Flags Flags
	Root  *cobra.Command
}

// Flags holds the parsed values of root persistent flags that command
// bodies read at runtime. Cobra binds &a.Flags.X to each flag and
// PersistentPreRun copies viper-resolved values back into these fields.
type Flags struct {
	JSONMode bool
	Pretty   bool
	Profile  string
	ReadOnly bool
	DryRun   bool
	Confirm  bool
}

// New constructs an App with the given dependencies and registers all command
// groups against a fresh root command. Tests can call this with custom Deps.
//
// Default accessor closures are installed for any nil function fields so they
// can capture &a.Deps and observe later mutations to app.Deps.Viper or
// app.Deps.HTTPTransport. Construction is lock-free: each App owns its own
// *viper.Viper and cobra command tree, so parallel test construction is safe.
func New(deps Deps) *App {
	a := &App{Deps: deps}

	if a.Deps.ConfigPath == nil {
		a.Deps.ConfigPath = func() string {
			if path := a.Deps.Viper.GetString("config"); path != "" {
				return path
			}
			return config.DefaultConfigPath()
		}
	}
	if a.Deps.SessionPath == nil {
		a.Deps.SessionPath = config.DefaultSessionPath
	}
	if a.Deps.APIEndpoint == nil {
		a.Deps.APIEndpoint = func() string {
			if endpoint := a.Deps.Viper.GetString("api_endpoint"); endpoint != "" {
				return endpoint
			}
			return "https://api.monarch.com/graphql"
		}
	}
	if a.Deps.Timeout == nil {
		a.Deps.Timeout = func() time.Duration {
			if t := a.Deps.Viper.GetDuration("timeout"); t > 0 {
				return t
			}
			return 30 * time.Second
		}
	}
	if a.Deps.Authenticate == nil {
		a.Deps.Authenticate = func(email, password, mfaCode, mfaSecret string) (*auth.Session, error) {
			client := &http.Client{Timeout: 10 * time.Second, Transport: a.Deps.HTTPTransport}
			return auth.NewClient("", client).Authenticate(email, password, mfaCode, mfaSecret)
		}
	}
	if a.Deps.NewClient == nil {
		a.Deps.NewClient = func(endpoint, token string, timeout time.Duration) GraphQLClient {
			return graphql.NewClient(endpoint, token, timeout, a.Deps.HTTPTransport)
		}
	}
	if a.Deps.NewService == nil {
		a.Deps.NewService = func(client GraphQLClient) *monarch.Service {
			return monarch.NewService(client, monarch.WithHTTPTransport(a.Deps.HTTPTransport))
		}
	}
	if a.Deps.NewAuditLogger == nil {
		a.Deps.NewAuditLogger = func() AuditLogger {
			return defaultAuditLogger(a.Deps.Viper)
		}
	}

	a.Root = a.buildRoot()

	a.buildAuthCommands(a.Root)
	a.buildAccountsCommands(a.Root)
	a.buildTransactionsCommands(a.Root)
	a.buildBudgetsCommands(a.Root)
	a.buildCashflowCommands(a.Root)
	a.buildCategoriesCommands(a.Root)
	a.buildCreditCommands(a.Root)
	a.buildInstitutionsCommands(a.Root)
	a.buildRecurringCommands(a.Root)
	a.buildRulesCommands(a.Root)
	a.buildSubscriptionCommands(a.Root)
	a.buildTagsCommands(a.Root)
	a.buildCacheCommands(a.Root)
	a.buildAnalyzeCommands(a.Root)
	a.buildDoctorCommand(a.Root)

	return a
}

// Execute runs the root command and returns its error.
func (a *App) Execute() error {
	return a.Root.Execute()
}
