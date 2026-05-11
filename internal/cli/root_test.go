package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thedavidweng/monarchmoney-cli/internal/graphql"
)

type stubGraphQLClient struct{}

func (stubGraphQLClient) Do(context.Context, *graphql.Request, any) error {
	return nil
}

func (stubGraphQLClient) TokenValue() string {
	return ""
}

func TestWriteVersion(t *testing.T) {
	t.Parallel()
	t.Run("plain text", func(t *testing.T) {
		var buf bytes.Buffer
		if err := writeVersion(&buf, "default", false, false, time.Second); err != nil {
			t.Fatalf("writeVersion() error = %v", err)
		}
		got := buf.String()
		if !strings.Contains(got, "monarch version ") {
			t.Fatalf("writeVersion() = %q", got)
		}
	})

	t.Run("compact json", func(t *testing.T) {
		var buf bytes.Buffer
		if err := writeVersion(&buf, "default", true, false, time.Second); err != nil {
			t.Fatalf("writeVersion() error = %v", err)
		}
		var got struct {
			OK   bool `json:"ok"`
			Data struct {
				Version string `json:"version"`
				Commit  string `json:"commit"`
				Date    string `json:"date"`
			} `json:"data"`
			Meta struct {
				Command  string   `json:"command"`
				Profile  string   `json:"profile"`
				Warnings []string `json:"warnings"`
			} `json:"meta"`
		}
		if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &got); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		if !got.OK || got.Data.Version == "" || got.Meta.Command != "version" || got.Meta.Profile != "default" {
			t.Fatalf("writeVersion() = %#v", got)
		}
	})

	t.Run("pretty json", func(t *testing.T) {
		var buf bytes.Buffer
		if err := writeVersion(&buf, "default", true, true, time.Second); err != nil {
			t.Fatalf("writeVersion() error = %v", err)
		}
		got := buf.String()
		if !strings.Contains(got, "\n  \"ok\"") {
			t.Fatalf("writeVersion() = %q", got)
		}
		var decoded map[string]interface{}
		if err := json.Unmarshal([]byte(got), &decoded); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
	})
}

func TestEnvelopeWithWarnings(t *testing.T) {
	t.Parallel()
	a := &App{Flags: Flags{Profile: "default"}}
	env := a.envelopeWithWarnings("transactions.list", map[string]string{"status": "ok"}, time.Now(), "uses legacy Monarch GraphQL root field: allTransactions")
	if len(env.Meta.Warnings) != 1 || env.Meta.Warnings[0] == "" {
		t.Fatalf("envelopeWithWarnings() = %#v", env.Meta.Warnings)
	}
	data, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if !strings.Contains(string(data), `"warnings":["uses legacy Monarch GraphQL root field: allTransactions"]`) {
		t.Fatalf("envelopeWithWarnings() = %s", string(data))
	}
}

func TestDepsConfigPathReadsViper(t *testing.T) {
	t.Parallel()

	deps := DefaultDeps()
	app := New(deps)
	app.Deps.Viper.Set("config", "/tmp/custom-config.yaml")

	if got, want := app.Deps.ConfigPath(), "/tmp/custom-config.yaml"; got != want {
		t.Fatalf("ConfigPath() = %q, want %q", got, want)
	}
}

func TestRootLoadsConfigFlagIntoAppViper(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	configBody := strings.Join([]string{
		"audit_log: false",
		"profile: custom",
		"api_base_url: https://api.example/",
		"read_only: true",
		"session_path: " + filepath.Join(dir, "configured-session.json"),
		"",
	}, "\n")
	if err := os.WriteFile(configPath, []byte(configBody), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	app := New(DefaultDeps())
	var out bytes.Buffer
	app.Root.SetOut(&out)
	app.Root.SetErr(&out)
	app.Root.SetArgs([]string{"--config", configPath, "version"})

	if err := app.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got := app.Deps.ConfigPath(); got != configPath {
		t.Fatalf("ConfigPath() = %q, want %q", got, configPath)
	}
	if app.Deps.Viper.GetBool("audit_log") {
		t.Fatal("audit_log = true, want false from config file")
	}
	if got := app.Flags.Profile; got != "custom" {
		t.Fatalf("Profile = %q, want custom", got)
	}
	if got, want := app.Deps.APIBaseURL(), "https://api.example/"; got != want {
		t.Fatalf("APIBaseURL() = %q, want %q", got, want)
	}
	if got, want := app.Deps.GraphQLEndpoint(), "https://api.example/graphql"; got != want {
		t.Fatalf("GraphQLEndpoint() = %q, want %q", got, want)
	}
	if !app.Flags.ReadOnly {
		t.Fatal("ReadOnly = false, want true from config file")
	}
	if got, want := app.Deps.SessionPath(), filepath.Join(dir, "configured-session.json"); got != want {
		t.Fatalf("SessionPath() = %q, want %q", got, want)
	}
}

func TestReadOnlyConfigBlocksConfirmedMutationBeforeAuth(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("read_only: true\n"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	var out bytes.Buffer
	exitCode := 0
	deps := DefaultDeps()
	deps.Stdout = &out
	deps.Stderr = &out
	deps.Exit = func(code int) { exitCode = code }

	app := New(deps)
	app.Root.SetArgs([]string{"--config", configPath, "--json", "accounts", "refresh", "--confirm"})

	if err := app.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if exitCode == 0 {
		t.Fatalf("exitCode = 0, want read-only failure; output=%q", out.String())
	}
	if !strings.Contains(out.String(), "READ_ONLY_VIOLATION") {
		t.Fatalf("output = %q, want READ_ONLY_VIOLATION", out.String())
	}
	if strings.Contains(out.String(), "AUTH_REQUIRED") {
		t.Fatalf("output = %q, read-only should fail before auth load", out.String())
	}
}

func TestConfigSessionPathCommandUsesConfiguredPath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	sessionPath := filepath.Join(dir, "custom-session.json")
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("session_path: "+sessionPath+"\n"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	var out bytes.Buffer
	deps := DefaultDeps()
	deps.Stdout = &out
	deps.Stderr = &out

	app := New(deps)
	app.Root.SetArgs([]string{"--config", configPath, "auth", "session", "path"})

	if err := app.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got := strings.TrimSpace(out.String()); got != sessionPath {
		t.Fatalf("auth session path = %q, want %q", got, sessionPath)
	}
}

func TestRootUsesDepsStdoutForTextVersion(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	deps := DefaultDeps()
	deps.Stdout = &out
	deps.Stderr = &out

	app := New(deps)
	app.Root.SetArgs([]string{"version"})

	if err := app.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out.String(), "monarch version ") {
		t.Fatalf("version output = %q", out.String())
	}
}

func TestDefaultNewServiceUsesDerivedUploadEndpoint(t *testing.T) {
	t.Parallel()

	app := New(DefaultDeps())
	app.Deps.Viper.Set("api_base_url", "https://api.example/")

	svc := app.Deps.NewService(stubGraphQLClient{})
	if got, want := svc.BalanceHistoryUploadEndpoint, "https://api.example/account-balance-history/upload/"; got != want {
		t.Fatalf("BalanceHistoryUploadEndpoint = %q, want %q", got, want)
	}
}

func TestDefaultAuditLoggerHonorsConfig(t *testing.T) {
	t.Parallel()

	deps := DefaultDeps()
	app := New(deps)

	app.Deps.Viper.Set("audit_log", false)
	if _, ok := app.Deps.NewAuditLogger().(disabledAuditLogger); !ok {
		t.Fatalf("NewAuditLogger() returned %T, want disabledAuditLogger", app.Deps.NewAuditLogger())
	}

	app.Deps.Viper.Set("audit_log", true)
	if _, ok := app.Deps.NewAuditLogger().(disabledAuditLogger); ok {
		t.Fatal("NewAuditLogger() returned disabledAuditLogger, want real logger")
	}
}
