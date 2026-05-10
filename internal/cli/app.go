package cli

import "github.com/spf13/cobra"

// App owns the root cobra command and the dependencies that command handlers
// reach for at runtime. Tests construct an App with custom Deps to inject
// behavior; production wires DefaultDeps via cmd/monarch/main.go.
type App struct {
	Deps Deps
	Root *cobra.Command
}

// New constructs an App with the given dependencies. In Commit 1 it reuses
// the existing package-level RootCmd so the binary still works while the
// command-by-command migration happens in Commit 2.
func New(deps Deps) *App {
	return &App{
		Deps: deps,
		Root: RootCmd,
	}
}

// Execute runs the root command. Replaces the package-level Execute() once
// the migration completes.
func (a *App) Execute() error {
	return a.Root.Execute()
}
