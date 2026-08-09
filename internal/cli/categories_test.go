package cli

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thedavidweng/monarchmoney-cli/internal/testutil"
)

func TestCategoriesGroupsJSON(t *testing.T) {
	h := newJSONCommandHarness(t, testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		var gqlReq struct {
			OperationName string `json:"operationName"`
		}
		if err := json.NewDecoder(req.Body).Decode(&gqlReq); err != nil {
			t.Fatalf("Decode request error = %v", err)
		}
		if gqlReq.OperationName != "ManageGetCategoryGroups" {
			t.Fatalf("operation = %q, want GetCategoryGroups", gqlReq.OperationName)
		}
		return testutil.JSONResponse(`{"data":{"categoryGroups":[
			{"id":"grp-1","name":"Food & Drink","type":"expense","categories":[{"id":"cat-1","name":"Dining"}]},
			{"id":"grp-2","name":"Income","type":"income","categories":[]}
		]}}`), nil
	}))

	if err := h.execute("--json", "categories", "groups"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	out := h.Stdout.String()
	if h.ExitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", h.ExitCode, out)
	}
	for _, want := range []string{`"command":"categories.groups"`, `"name":"Food`, `"type":"expense"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %s = %q", want, out)
		}
	}
}

func TestCategoriesRolloverMapsGraphQLNotFound(t *testing.T) {
	h := newJSONCommandHarness(t, testutil.RoundTripFunc(func(*http.Request) (*http.Response, error) {
		return testutil.JSONResponse(`{"data":{"category":null},"errors":[{"message":"Not found","path":["category"]}]}`), nil
	}))

	if err := h.execute("--json", "categories", "rollover", "missing"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if h.ExitCode != 8 || !strings.Contains(h.Stdout.String(), `"code":"RESOURCE_NOT_FOUND"`) {
		t.Fatalf("exitCode = %d; output=%q, want RESOURCE_NOT_FOUND", h.ExitCode, h.Stdout.String())
	}
}

func TestCategoriesDeleteJSON(t *testing.T) {
	h := newJSONCommandHarness(t, testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		var gqlReq struct {
			OperationName string         `json:"operationName"`
			Variables     map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(req.Body).Decode(&gqlReq); err != nil {
			t.Fatalf("Decode request error = %v", err)
		}
		if gqlReq.OperationName != "Web_DeleteCategory" || gqlReq.Variables["id"] != "cat-old" {
			t.Fatalf("request = %#v, want DeleteCategory cat-old", gqlReq)
		}
		return testutil.JSONResponse(`{"data":{"deleteCategory":{"ok":true}}}`), nil
	}))

	if err := h.execute("--json", "--confirm", "categories", "delete", "cat-old"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	out := h.Stdout.String()
	if h.ExitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", h.ExitCode, out)
	}
	if !strings.Contains(out, `"command":"categories.delete"`) || !strings.Contains(out, `"status":"deleted"`) {
		t.Fatalf("output = %q, want category delete status", out)
	}
}

func TestCategoriesDeleteManyValidation(t *testing.T) {
	t.Run("missing_file", func(t *testing.T) {
		h := newJSONCommandHarness(t, nil)
		if err := h.execute("--json", "--confirm", "categories", "delete-many"); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if h.ExitCode != 2 || !strings.Contains(h.Stdout.String(), `"INVALID_ARGUMENTS"`) || !strings.Contains(h.Stdout.String(), `required flag(s) \"file\" not set`) {
			t.Fatalf("exitCode = %d; output=%q, want JSON required-flag error", h.ExitCode, h.Stdout.String())
		}
	})

	t.Run("file_not_found", func(t *testing.T) {
		h := newJSONCommandHarness(t, nil)
		missing := filepath.Join(t.TempDir(), "nonexistent.txt")
		if err := h.execute("--json", "--confirm", "categories", "delete-many", "--file", missing); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		out := h.Stdout.String()
		if h.ExitCode == 0 || !strings.Contains(out, "failed to open file") {
			t.Fatalf("exitCode = %d; output=%q, want file-open failure", h.ExitCode, out)
		}
	})
}
