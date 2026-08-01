package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thedavidweng/monarchmoney-cli/internal/testutil"
)

func TestSubscription(t *testing.T) {
	t.Run("show", testSubscriptionShowJSON)
	t.Run("show_api_error", testSubscriptionShowAPIError)
}

func testSubscriptionShowJSON(t *testing.T) {
	dir := t.TempDir()
	sessionPath := filepath.Join(dir, "session.json")
	saveTestSession(t, sessionPath)

	h := newAppTestHarness(t, func(deps *Deps) {
		deps.LoadConfig = testConfigLoader(sessionPath, "")
		deps.HTTPTransport = testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
			var gqlReq struct {
				OperationName string `json:"operationName"`
			}
			if err := json.NewDecoder(req.Body).Decode(&gqlReq); err != nil {
				t.Fatalf("Decode request error = %v", err)
			}
			if gqlReq.OperationName != "GetSubscriptionDetails" {
				t.Fatalf("operation = %q, want GetSubscriptionDetails", gqlReq.OperationName)
			}
			return testutil.JSONResponse(`{"data":{"subscription":{"id":"sub-1","paymentSource":"Visa ending 1234","referralCode":"ABC123","isOnFreeTrial":false,"hasPremiumEntitlement":true}}}`), nil
		})
	})

	if err := h.execute("--json", "subscription", "show"); err != nil {
		t.Fatalf("execute() error = %v", err)
	}
	if h.ExitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", h.ExitCode, h.Stdout.String())
	}
	if got := h.Stdout.String(); !strings.Contains(got, `"command":"subscription.show"`) ||
		!strings.Contains(got, `"payment_source":"Visa ending 1234"`) ||
		!strings.Contains(got, `"referral_code":"ABC123"`) ||
		!strings.Contains(got, `"has_premium_entitlement":true`) ||
		!strings.Contains(got, "uses legacy Monarch GraphQL root field") {
		t.Fatalf("output = %q, want subscription JSON and legacy warning", got)
	}
}

func testSubscriptionShowAPIError(t *testing.T) {
	dir := t.TempDir()
	sessionPath := filepath.Join(dir, "session.json")
	saveTestSession(t, sessionPath)

	h := newAppTestHarness(t, func(deps *Deps) {
		deps.LoadConfig = testConfigLoader(sessionPath, "")
		deps.HTTPTransport = testutil.RoundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(bytes.NewReader(nil)),
			}, nil
		})
	})

	if err := h.execute("--json", "subscription", "show"); err != nil {
		t.Fatalf("execute() error = %v", err)
	}
	if h.ExitCode == 0 {
		t.Fatalf("exitCode = 0, want API failure; output=%q", h.Stdout.String())
	}
	if got := h.Stdout.String(); !strings.Contains(got, `"API_ERROR"`) {
		t.Fatalf("output = %q, want API_ERROR", got)
	}
}
