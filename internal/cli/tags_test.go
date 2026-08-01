package cli

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/thedavidweng/monarchmoney-cli/internal/testutil"
)

func TestTagsListJSON(t *testing.T) {
	h := newJSONCommandHarness(t, testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		var gqlReq struct {
			OperationName string `json:"operationName"`
		}
		if err := json.NewDecoder(req.Body).Decode(&gqlReq); err != nil {
			t.Fatalf("Decode request error = %v", err)
		}
		if gqlReq.OperationName != "GetHouseholdTransactionTags" {
			t.Fatalf("operation = %q, want GetTags", gqlReq.OperationName)
		}
		return testutil.JSONResponse(`{"data":{"householdTransactionTags":[
			{"id":"tag-1","name":"reimbursable","color":"#ff0000"},
			{"id":"tag-2","name":"tax-deductible","color":"#00ff00"}
		]}}`), nil
	}))

	if err := h.execute("--json", "tags", "list"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	out := h.Stdout.String()
	if h.ExitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", h.ExitCode, out)
	}
	for _, want := range []string{`"command":"tags.list"`, `"name":"reimbursable"`, `"color":"#ff0000"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %s = %q", want, out)
		}
	}
}
