package cli

import "github.com/spf13/cobra"

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
//
// Only flags actually consumed by command bodies live here. Flags wired
// through cobra/viper but never read (e.g. --compact, --no-color) remain
// as package-level vars in root.go for now.
type Flags struct {
	JSONMode bool
	Pretty   bool
	Profile  string
	ReadOnly bool
	DryRun   bool
	Confirm  bool
	Events   bool
}

// New constructs an App with the given dependencies and registers all command
// groups against a fresh root command. Tests can call this with custom Deps.
//
// App construction touches process-global state in cobra and viper (flag
// registration via viper.BindPFlag, the cobra.OnInitialize hook). Production
// constructs exactly one App so this is uncontended; parallel tests construct
// one App per test, so we serialize the construction with globalRegistration
// to avoid concurrent map writes inside viper. Per-App state (App.Flags, the
// cobra command tree) is not touched by other Apps after New returns.
func New(deps Deps) *App {
	globalRegistration.Lock()
	defer globalRegistration.Unlock()

	a := &App{Deps: deps}
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
