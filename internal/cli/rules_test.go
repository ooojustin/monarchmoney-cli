package cli

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/thedavidweng/monarchmoney-cli/internal/testutil"
)

func TestRulesUpdateJSON(t *testing.T) {
	h := newJSONCommandHarness(t, testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		var gqlReq struct {
			OperationName string         `json:"operationName"`
			Variables     map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(req.Body).Decode(&gqlReq); err != nil {
			t.Fatalf("Decode request error = %v", err)
		}
		if gqlReq.OperationName != "Common_UpdateTransactionRuleMutationV2" {
			t.Fatalf("operation = %q, want Common_UpdateTransactionRuleMutationV2", gqlReq.OperationName)
		}
		input, ok := gqlReq.Variables["input"].(map[string]any)
		if !ok {
			t.Fatalf("input = %#v, want object", gqlReq.Variables["input"])
		}
		if input["id"] != "rule-1" || input["setCategoryAction"] != "cat-transport" {
			t.Fatalf("input = %#v, want rule-1 and cat-transport", input)
		}
		criteria, ok := input["merchantNameCriteria"].([]any)
		if !ok || len(criteria) != 1 {
			t.Fatalf("merchant criteria = %#v, want one criterion", input["merchantNameCriteria"])
		}
		criterion, ok := criteria[0].(map[string]any)
		if !ok || criterion["value"] != "Lyft" {
			t.Fatalf("merchant criteria = %#v, want Lyft", criteria)
		}
		return testutil.JSONResponse(`{"data":{"updateTransactionRuleV2":{}}}`), nil
	}))

	if err := h.execute("--json", "--confirm", "rules", "update", "rule-1", "--merchant-operator", "contains", "--merchant-value", "Lyft", "--set-category-id", "cat-transport"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	out := h.Stdout.String()
	if h.ExitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", h.ExitCode, out)
	}
	if !strings.Contains(out, `"command":"rules.update"`) || !strings.Contains(out, `"status":"updated"`) {
		t.Fatalf("output = %q, want rule update status", out)
	}
}

func TestRulesDeleteJSON(t *testing.T) {
	h := newJSONCommandHarness(t, testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		var gqlReq struct {
			OperationName string         `json:"operationName"`
			Variables     map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(req.Body).Decode(&gqlReq); err != nil {
			t.Fatalf("Decode request error = %v", err)
		}
		if gqlReq.OperationName != "Common_DeleteTransactionRule" || gqlReq.Variables["id"] != "rule-old" {
			t.Fatalf("request = %#v, want Common_DeleteTransactionRule rule-old", gqlReq)
		}
		return testutil.JSONResponse(`{"data":{"deleteTransactionRule":{"deleted":true,"errors":null}}}`), nil
	}))

	if err := h.execute("--json", "--confirm", "rules", "delete", "rule-old"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	out := h.Stdout.String()
	if h.ExitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", h.ExitCode, out)
	}
	if !strings.Contains(out, `"command":"rules.delete"`) || !strings.Contains(out, `"status":"deleted"`) {
		t.Fatalf("output = %q, want rule delete status", out)
	}
}
