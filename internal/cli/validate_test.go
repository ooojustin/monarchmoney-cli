package cli

import (
	"net/http"
	"strings"
	"testing"

	"github.com/thedavidweng/monarchmoney-cli/internal/testutil"
)

func noRequestHarness(t *testing.T) *appTestHarness {
	t.Helper()

	return newJSONCommandHarness(t, testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("validation must run before any request; got %s", req.URL)
		return nil, nil
	}))
}

func TestDateFlagsValidateBeforeRequests(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		command string
		message string
	}{
		{"spending inverted", []string{"--json", "cashflow", "spending", "--from", "2026-07-31", "--to", "2026-07-01"}, "cashflow.spending", "--from must not be after --to"},
		{"spending malformed", []string{"--json", "cashflow", "spending", "--from", "notadate"}, "cashflow.spending", "--from must use YYYY-MM-DD"},
		{"summary inverted", []string{"--json", "cashflow", "summary", "--from", "2026-07-31", "--to", "2026-07-01"}, "cashflow.summary", "--from must not be after --to"},
		{"list inverted", []string{"--json", "cashflow", "list", "--from", "2026-07-31", "--to", "2026-07-01"}, "cashflow.list", "--from must not be after --to"},
		{"transactions inverted", []string{"--json", "transactions", "list", "--from", "2026-07-31", "--to", "2026-07-01"}, "transactions.list", "--from must not be after --to"},
		{"transactions malformed", []string{"--json", "transactions", "list", "--from", "notadate"}, "transactions.list", "--from must use YYYY-MM-DD"},
		{"overview inverted", []string{"--json", "overview", "--from", "2026-07-31", "--to", "2026-07-01"}, "overview", "--from must not be after --to"},
		{"accounts history inverted", []string{"--json", "accounts", "history", "acc-1", "--from", "2026-07-31", "--to", "2026-07-01"}, "accounts.history", "--from must not be after --to"},
		{"networth malformed", []string{"--json", "networth", "--to", "notadate"}, "networth", "--to must use YYYY-MM-DD"},
		{"balance-at malformed", []string{"--json", "accounts", "balance-at", "--date", "notadate"}, "accounts.balance-at", "--date must use YYYY-MM-DD"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := noRequestHarness(t)
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
		})
	}
}

func TestDateRangeValidation(t *testing.T) {
	tests := []struct {
		name    string
		from    string
		to      string
		wantErr bool
	}{
		{"both empty", "", "", false},
		{"ordered", "2026-01-01", "2026-12-31", false},
		{"same day", "2026-01-01", "2026-01-01", false},
		{"only from", "2026-01-01", "", false},
		{"only to", "", "2026-01-01", false},
		{"inverted", "2026-12-31", "2026-01-01", true},
		{"malformed from", "nope", "2026-01-01", true},
		{"malformed to", "2026-01-01", "nope", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDateRange(tt.from, tt.to)
			if tt.wantErr != (err != nil) {
				t.Fatalf("validateDateRange(%q, %q) = %v, wantErr %v", tt.from, tt.to, err, tt.wantErr)
			}
		})
	}
}
