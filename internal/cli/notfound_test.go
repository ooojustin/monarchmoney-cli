package cli

import (
	"net/http"
	"strings"
	"testing"

	"github.com/thedavidweng/monarchmoney-cli/internal/testutil"
)

func nullPayloadHarness(t *testing.T, payload string) *appTestHarness {
	t.Helper()

	return newJSONCommandHarness(t, testutil.RoundTripFunc(func(*http.Request) (*http.Response, error) {
		return testutil.JSONResponse(payload), nil
	}))
}

func TestLookupCommandsReportNotFound(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		args    []string
		command string
	}{
		{"accounts show", `{"data":{"account":null}}`, []string{"--json", "accounts", "show", "missing"}, "accounts.show"},
		{"transactions show", `{"data":{"getTransaction":null}}`, []string{"--json", "transactions", "show", "missing"}, "transactions.show"},
		{"categories rollover", `{"data":{"category":null}}`, []string{"--json", "categories", "rollover", "missing"}, "categories.rollover"},
		{"budgets show", `{"data":{"budgetData":{"monthlyAmountsByCategory":[]}}}`, []string{"--json", "budgets", "show", "missing", "--month", "2026-01"}, "budgets.show"},
		{"transactions tags add", `{"data":{"getTransaction":null}}`, []string{"--json", "transactions", "tags", "add", "missing", "--tag", "t1", "--confirm"}, "transactions.tags.add"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := nullPayloadHarness(t, tt.payload)
			if err := h.execute(tt.args...); err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			out := h.Stdout.String()
			if h.ExitCode != 8 {
				t.Fatalf("exitCode = %d, want 8; output=%q", h.ExitCode, out)
			}
			if !strings.Contains(out, `"RESOURCE_NOT_FOUND"`) {
				t.Fatalf("output=%q, want RESOURCE_NOT_FOUND", out)
			}
			if !strings.Contains(out, `"command":"`+tt.command+`"`) {
				t.Fatalf("output=%q, want command %q", out, tt.command)
			}
		})
	}
}

func TestBudgetsShowNotFoundWithoutJSON(t *testing.T) {
	h := nullPayloadHarness(t, `{"data":{"budgetData":{"monthlyAmountsByCategory":[]}}}`)
	if err := h.execute("budgets", "show", "missing", "--month", "2026-01"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if h.ExitCode != 8 {
		t.Fatalf("exitCode = %d, want 8; stderr=%q", h.ExitCode, h.Stderr.String())
	}
	if !strings.Contains(h.Stderr.String(), "not found") {
		t.Fatalf("stderr=%q, want not found", h.Stderr.String())
	}
}
