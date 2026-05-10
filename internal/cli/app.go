package cli

import "github.com/spf13/cobra"

// App owns the root cobra command and the dependencies that command handlers
// reach for at runtime. Tests construct an App with custom Deps to inject
// behavior; production wires DefaultDeps via cmd/monarch/main.go.
type App struct {
	Deps Deps
	Root *cobra.Command
}

// New constructs an App with the given dependencies and registers all command
// groups against a fresh root command. Tests can call this with custom Deps.
func New(deps Deps) *App {
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
