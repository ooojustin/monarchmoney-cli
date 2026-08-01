package cli

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/thedavidweng/monarchmoney-cli/internal/testutil"
)

func TestRecurringListJSON(t *testing.T) {
	h := newJSONCommandHarness(t, testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		var gqlReq struct {
			OperationName string `json:"operationName"`
		}
		if err := json.NewDecoder(req.Body).Decode(&gqlReq); err != nil {
			t.Fatalf("Decode request error = %v", err)
		}
		if gqlReq.OperationName != "Web_GetUpcomingRecurringTransactionItems" {
			t.Fatalf("operation = %q, want Web_GetUpcomingRecurringTransactionItems", gqlReq.OperationName)
		}
		return testutil.JSONResponse(`{"data":{"recurringTransactionItems":[
			{"stream":{"id":"rec-1","frequency":"monthly","amount":15.99,"isApproximate":false,"merchant":{"id":"m-1","name":"Netflix","logoUrl":""}},"date":"2026-06-15","isPast":false,"transactionId":"","amount":15.99,"amountDiff":0,"category":{"id":"cat-1","name":"Entertainment"},"account":{"id":"acc-1","displayName":"Checking"}},
			{"stream":{"id":"rec-2","frequency":"weekly","amount":50,"isApproximate":false,"merchant":{"id":"m-2","name":"Gym","logoUrl":""}},"date":"2026-06-16","isPast":false,"transactionId":"","amount":50,"amountDiff":0,"category":{"id":"cat-2","name":"Health"},"account":{"id":"acc-1","displayName":"Checking"}}
		]}}`), nil
	}))

	if err := h.execute("--json", "recurring", "list"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	out := h.Stdout.String()
	if h.ExitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", h.ExitCode, out)
	}
	for _, want := range []string{`"command":"recurring.list"`, `"merchant":"Netflix"`, `"frequency":"monthly"`, `"amount":15.99`} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %s = %q", want, out)
		}
	}
}
