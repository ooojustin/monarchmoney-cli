package cli

import (
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/thedavidweng/monarchmoney-cli/internal/errors"
	"github.com/thedavidweng/monarchmoney-cli/internal/output"
	"github.com/thedavidweng/monarchmoney-cli/internal/version"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func persistentFlagChanged(cmd *cobra.Command, name string) bool {
	f := cmd.Root().PersistentFlags().Lookup(name)
	return f != nil && f.Changed
}

func Execute() {
	app := New(DefaultDeps())
	if err := app.Execute(); err != nil {
		exitCode := 1
		if e, ok := err.(*errors.Error); ok {
			exitCode = e.ExitCode()
		}
		fmt.Fprintln(app.Deps.Stderr, err)
		app.Deps.Exit(exitCode)
	}
}

type versionPayload struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
	BuiltBy string `json:"built_by"`
}

func writeVersion(out io.Writer, profileName string, jsonOut, prettyOut bool, requestID string, duration time.Duration) error {
	if jsonOut {
		renderer := output.NewRenderer(out, nil, true, prettyOut)
		env := output.NewEnvelope("version", profileName, output.SchemaVersion, requestID, versionPayload{
			Version: version.GetVersion(),
			Commit:  version.GetCommit(),
			Date:    version.GetDate(),
			BuiltBy: version.GetBuiltBy(),
		}, duration)
		renderer.RenderSuccess(env)
		return nil
	}

	fmt.Fprint(out, monarchBanner)
	fmt.Fprintln(out)
	_, err := fmt.Fprintf(out, "monarch version %s (commit: %s, date: %s, built by: %s)\n", version.GetVersion(), version.GetCommit(), version.GetDate(), version.GetBuiltBy())
	return err
}
