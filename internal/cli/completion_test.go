package cli

import (
	stderrors "errors"
	"strings"
	"testing"

	"github.com/thedavidweng/monarchmoney-cli/internal/config"
)

func TestAppRootRegistersCompletion(t *testing.T) {
	app, _ := newTestApp(t)
	command, _, err := app.Root.Find([]string{"completion"})
	if err != nil || command == nil || command.GroupID != "utility" {
		t.Fatalf("Find(completion) = %#v, %v", command, err)
	}
	for _, shell := range []string{"bash", "zsh", "fish", "powershell"} {
		found := false
		for _, valid := range command.ValidArgs {
			found = found || valid == shell
		}
		if !found {
			t.Fatalf("completion ValidArgs missing %q", shell)
		}
	}
	if err := command.Args(command, []string{"tcsh"}); err == nil {
		t.Fatal("completion tcsh should be rejected")
	}
}

func TestAppCompletionGeneratesScriptsFromFinalTree(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish", "powershell"} {
		t.Run(shell, func(t *testing.T) {
			app, out := newTestApp(t)
			app.Deps.LoadConfig = func(string) (*config.Config, error) {
				return config.Default(), stderrors.New("malformed config")
			}
			app.Root.SetArgs([]string{"--json", "completion", shell})
			if err := app.Execute(); err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if out.Len() == 0 {
				t.Fatal("completion produced no output")
			}
			if strings.HasPrefix(strings.TrimSpace(out.String()), "{") {
				t.Fatalf("completion was wrapped as JSON: %q", out.String())
			}
		})
	}
}
