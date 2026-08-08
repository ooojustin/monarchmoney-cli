package cli

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestAppParentCommandsRejectStrayArguments(t *testing.T) {
	app, _ := newTestApp(t)

	var walk func(*cobra.Command)
	walk = func(parent *cobra.Command) {
		for _, cmd := range parent.Commands() {
			walk(cmd)
			if !cmd.HasSubCommands() {
				continue
			}
			if cmd.RunE == nil && cmd.Args == nil {
				t.Errorf("%q has subcommands but accepts stray arguments", cmd.CommandPath())
			}
		}
	}
	walk(app.Root)
}

func TestParentCommandsRequireSubcommandJSON(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		command string
		message string
	}{
		{"bare top level", []string{"--json", "accounts"}, "accounts", "requires a subcommand"},
		{"unknown subcommand", []string{"--json", "accounts", "bogus"}, "accounts", `unknown command \"bogus\" for \"monarch accounts\"`},
		{"bare nested", []string{"--json", "budgets", "flexible"}, "budgets.flexible", "requires a subcommand"},
		{"bare inline nested", []string{"--json", "auth", "session"}, "auth.session", "requires a subcommand"},
		{"runnable parent stray arg", []string{"--json", "categories", "groups", "extra"}, "categories.groups", `unknown command \"extra\" for \"monarch categories groups\"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newAppTestHarness(t, nil)
			if err := h.execute(tt.args...); err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			out := h.Stdout.String()
			if h.ExitCode != 2 {
				t.Fatalf("exitCode = %d, want 2; output=%q", h.ExitCode, out)
			}
			if !strings.Contains(out, `"INVALID_ARGUMENTS"`) {
				t.Fatalf("output=%q, want INVALID_ARGUMENTS", out)
			}
			if !strings.Contains(out, `"command":"`+tt.command+`"`) {
				t.Fatalf("output=%q, want command %q", out, tt.command)
			}
			if !strings.Contains(out, tt.message) {
				t.Fatalf("output=%q, want message %q", out, tt.message)
			}
			if strings.Contains(out, "Available Commands") {
				t.Fatalf("output=%q, want no help text in JSON mode", out)
			}
		})
	}
}

func TestParentCommandRendersHelpWithoutJSON(t *testing.T) {
	h := newAppTestHarness(t, nil)
	if err := h.execute("accounts"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if h.ExitCode != 2 {
		t.Fatalf("exitCode = %d, want 2", h.ExitCode)
	}
	if !strings.Contains(h.Stdout.String(), "Available Commands") {
		t.Fatalf("stdout=%q, want help text", h.Stdout.String())
	}
	if !strings.Contains(h.Stderr.String(), "requires a subcommand") {
		t.Fatalf("stderr=%q, want subcommand error", h.Stderr.String())
	}
}

func TestParentCommandHelpFlagSucceeds(t *testing.T) {
	h := newAppTestHarness(t, nil)
	if err := h.execute("accounts", "--help"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if h.ExitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", h.ExitCode)
	}
	if !strings.Contains(h.Stdout.String(), "Available Commands") {
		t.Fatalf("stdout=%q, want help text", h.Stdout.String())
	}
}
