package cli

import (
	"net/http"
	"strings"
	"testing"
	"time"

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
		{"overview lone to", []string{"--json", "overview", "--to", "2020-01-01"}, "overview", "--from must not be after --to"},
		{"overview future from", []string{"--json", "overview", "--from", "2999-01-01"}, "overview", "--from must not be after --to"},
		{"cashflow spending lone to", []string{"--json", "cashflow", "spending", "--to", "2020-01-01"}, "cashflow.spending", "--from must not be after --to"},
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

func TestNumericFlagsValidateBeforeRequests(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		command string
		message string
	}{
		{"list negative limit", []string{"--json", "transactions", "list", "--limit", "-5"}, "transactions.list", "--limit must be greater than zero"},
		{"list zero limit", []string{"--json", "transactions", "list", "--limit", "0"}, "transactions.list", "--limit must be greater than zero"},
		{"list negative offset", []string{"--json", "transactions", "list", "--offset", "-1"}, "transactions.list", "--offset must not be negative"},
		{"search negative limit", []string{"--json", "transactions", "search", "coffee", "--limit", "-5"}, "transactions.search", "--limit must be greater than zero"},
		{"export zero limit", []string{"--json", "transactions", "export", "--limit", "0"}, "transactions.export", "--limit must be greater than zero"},
		{"merchants zero limit", []string{"--json", "analyze", "merchants", "--limit", "0"}, "analyze.merchants", "--limit must be greater than zero"},
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
			if !strings.Contains(out, `"INVALID_ARGUMENTS"`) || !strings.Contains(out, tt.message) {
				t.Fatalf("output=%q, want %q", out, tt.message)
			}
			if !strings.Contains(out, `"command":"`+tt.command+`"`) {
				t.Fatalf("output=%q, want command %q", out, tt.command)
			}
		})
	}
}

func TestMonthFlagRejectsMalformedValues(t *testing.T) {
	for _, args := range [][]string{
		{"--json", "budgets", "list", "--month", "abc-def"},
		{"--json", "budgets", "show", "cat-1", "--month", "2026-13"},
		{"--json", "goals", "budgets", "--month", "abc-def"},
	} {
		t.Run(strings.Join(args[1:3], "."), func(t *testing.T) {
			h := noRequestHarness(t)
			if err := h.execute(args...); err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			out := h.Stdout.String()
			if h.ExitCode != 2 || !strings.Contains(out, "--month must use YYYY-MM") {
				t.Fatalf("exitCode = %d, output=%q, want month validation error", h.ExitCode, out)
			}
		})
	}
}

func TestResolveDateRange(t *testing.T) {
	// 21:13 EDT on the 8th is 01:13 UTC on the 9th: a UTC clock would end the
	// default window a day later than the day the caller is living in.
	now := time.Date(2026, 8, 8, 21, 13, 0, 0, time.FixedZone("EDT", -4*60*60))

	tests := []struct {
		name     string
		from     string
		to       string
		wantFrom string
		wantTo   string
	}{
		{"both empty", "", "", "2026-08-01", "2026-08-08"},
		{"lone from", "2020-01-01", "", "2020-01-01", "2026-08-08"},
		{"lone to", "", "2026-01-31", "2026-08-01", "2026-01-31"},
		{"both supplied", "2026-01-01", "2026-01-31", "2026-01-01", "2026-01-31"},
		{"malformed passes through to validation", "notadate", "", "notadate", "2026-08-08"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotFrom, gotTo := resolveDateRange(tt.from, tt.to, now)
			if gotFrom != tt.wantFrom || gotTo != tt.wantTo {
				t.Fatalf("resolveDateRange(%q, %q) = (%q, %q), want (%q, %q)", tt.from, tt.to, gotFrom, gotTo, tt.wantFrom, tt.wantTo)
			}
		})
	}
}
