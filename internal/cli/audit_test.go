package cli

import (
	"bytes"
	stderrors "errors"
	"io"
	"strings"
	"testing"

	"github.com/thedavidweng/monarchmoney-cli/internal/audit"
	"github.com/thedavidweng/monarchmoney-cli/internal/config"
)

func newTestAuditApp(t *testing.T, cleanup func(int) (int, error), write func(*audit.Record) error) (*App, *bytes.Buffer, *int) {
	t.Helper()

	var out bytes.Buffer
	exitCode := 0
	app := New(Deps{
		LoadConfig: func(string) (*config.Config, error) {
			return config.Default(), nil
		},
		Getenv:       func(string) string { return "" },
		NewRequestID: func() string { return "request-id" },
		CleanupAudit: cleanup,
		WriteAudit:   write,
		Stdout:       &out,
		Stderr:       io.Discard,
		Exit:         func(code int) { exitCode = code },
	})
	return app, &out, &exitCode
}

func TestAppRootRegistersAudit(t *testing.T) {
	app, _ := newTestApp(t)
	auditCommand, _, err := app.Root.Find([]string{"audit"})
	if err != nil || auditCommand == nil || auditCommand.GroupID != "utility" {
		t.Fatalf("Find(audit) = %#v, %v", auditCommand, err)
	}
	cleanupCommand, _, err := app.Root.Find([]string{"audit", "cleanup"})
	if err != nil || cleanupCommand == nil {
		t.Fatalf("Find(audit cleanup) = %#v, %v", cleanupCommand, err)
	}
	flag := cleanupCommand.Flags().Lookup("older-than")
	if flag == nil || flag.DefValue != "30" {
		t.Fatalf("--older-than flag = %#v", flag)
	}
}

func TestAppAuditCleanupUsesInjectedDependency(t *testing.T) {
	gotDays := 0
	app, out, exitCode := newTestAuditApp(t, func(days int) (int, error) {
		gotDays = days
		return 3, nil
	}, nil)
	app.Deps.LoadConfig = func(string) (*config.Config, error) {
		return config.Default(), stderrors.New("malformed config")
	}

	app.Root.SetArgs([]string{"--json", "audit", "cleanup", "--older-than", "45"})
	if err := app.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if *exitCode != 0 || gotDays != 45 {
		t.Fatalf("exitCode=%d cleanup days=%d", *exitCode, gotDays)
	}
	if got := out.String(); !strings.Contains(got, `"removed":3`) || !strings.Contains(got, `"older_than_days":45`) || !strings.Contains(got, `"request_id":"request-id"`) {
		t.Fatalf("output = %q", got)
	}
}

func TestAppAuditCleanupValidatesBeforeDependency(t *testing.T) {
	called := false
	app, out, exitCode := newTestAuditApp(t, func(int) (int, error) {
		called = true
		return 0, nil
	}, nil)
	app.Root.SetArgs([]string{"--json", "audit", "cleanup", "--older-than", "0"})
	if err := app.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if called || *exitCode != 2 || !strings.Contains(out.String(), "INVALID_ARGUMENTS") {
		t.Fatalf("called=%v exitCode=%d output=%q", called, *exitCode, out.String())
	}
}

func TestAppRecordAuditHonorsConfig(t *testing.T) {
	writes := 0
	app, _, _ := newTestAuditApp(t, nil, func(*audit.Record) error {
		writes++
		return stderrors.New("best-effort failure")
	})
	app.Config = config.Default()

	app.recordAudit(&audit.Record{Command: "test"})
	if writes != 1 {
		t.Fatalf("writes = %d, want 1", writes)
	}
	app.Config.AuditLog = false
	app.recordAudit(&audit.Record{Command: "test"})
	if writes != 1 {
		t.Fatalf("writes = %d after disabled audit, want 1", writes)
	}
}
