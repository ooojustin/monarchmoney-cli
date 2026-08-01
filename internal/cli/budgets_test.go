package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/thedavidweng/monarchmoney-cli/internal/testutil"
)

func TestBudgetsListAPIError(t *testing.T) {
	h := newJSONCommandHarness(t, testutil.RoundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       io.NopCloser(bytes.NewReader(nil)),
		}, nil
	}))

	if err := h.execute("--json", "budgets", "list", "--month", "2026-06"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	out := h.Stdout.String()
	if h.ExitCode == 0 || !strings.Contains(out, `"API_ERROR"`) {
		t.Fatalf("exitCode = %d; output=%q, want API_ERROR", h.ExitCode, out)
	}
}

func TestBudgetsListInvalidMonth(t *testing.T) {
	h := newJSONCommandHarness(t, nil)
	if err := h.execute("--json", "budgets", "list", "--month", "2026/06"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	out := h.Stdout.String()
	if h.ExitCode == 0 || !strings.Contains(out, "YYYY-MM") {
		t.Fatalf("exitCode = %d; output=%q, want month format guidance", h.ExitCode, out)
	}
}

func TestBudgetsResetJSON(t *testing.T) {
	h := newJSONCommandHarness(t, testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		var gqlReq struct {
			OperationName string         `json:"operationName"`
			Variables     map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(req.Body).Decode(&gqlReq); err != nil {
			t.Fatalf("Decode request error = %v", err)
		}
		if gqlReq.OperationName != "ResetBudget" {
			t.Fatalf("operation = %q, want ResetBudget", gqlReq.OperationName)
		}
		return testutil.JSONResponse(`{"data":{"resetBudget":{"ok":true}}}`), nil
	}))

	if err := h.execute("--json", "--confirm", "budgets", "reset", "--month", "2026-06"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	out := h.Stdout.String()
	if h.ExitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", h.ExitCode, out)
	}
	if !strings.Contains(out, `"command":"budgets.reset"`) || !strings.Contains(out, `"status":"budget reset"`) {
		t.Fatalf("output = %q, want budget reset status", out)
	}
}

func TestBudgetsResetValidation(t *testing.T) {
	t.Run("missing_month", func(t *testing.T) {
		h := newJSONCommandHarness(t, nil)
		err := h.execute("--json", "--confirm", "budgets", "reset")
		if err == nil || !strings.Contains(err.Error(), `required flag(s) "month" not set`) {
			t.Fatalf("Execute() error = %v, want required-month failure", err)
		}
		if h.ExitCode != 0 {
			t.Fatalf("exitCode = %d, want Cobra validation without handler exit", h.ExitCode)
		}
	})

	t.Run("invalid_month", func(t *testing.T) {
		h := newJSONCommandHarness(t, nil)
		if err := h.execute("--json", "--confirm", "budgets", "reset", "--month", "bad"); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		out := h.Stdout.String()
		if h.ExitCode == 0 || !strings.Contains(out, "YYYY-MM") {
			t.Fatalf("exitCode = %d; output=%q, want month format guidance", h.ExitCode, out)
		}
	})
}

func TestBudgetsFlexRolloverSetJSON(t *testing.T) {
	h := newJSONCommandHarness(t, testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		var gqlReq struct {
			OperationName string         `json:"operationName"`
			Variables     map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(req.Body).Decode(&gqlReq); err != nil {
			t.Fatalf("Decode request error = %v", err)
		}
		if gqlReq.OperationName != "UpdateFlexRolloverSettings" {
			t.Fatalf("operation = %q, want UpdateFlexRolloverSettings", gqlReq.OperationName)
		}
		return testutil.JSONResponse(`{"data":{"updateBudgetSettings":{"budgetRolloverPeriod":{"id":"period-1"}}}}`), nil
	}))

	if err := h.execute("--json", "--confirm", "budgets", "flex-rollover", "set", "--month", "2026-06-01", "--amount", "1000"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	out := h.Stdout.String()
	if h.ExitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", h.ExitCode, out)
	}
	if !strings.Contains(out, `"command":"budgets.flex-rollover.set"`) {
		t.Fatalf("output = %q, want flex-rollover command", out)
	}
}
