package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/thedavidweng/monarchmoney-cli/internal/testutil"
)

func TestTransactionsDuplicatesJSON(t *testing.T) {
	before := time.Now()
	var filters map[string]any
	h := newJSONCommandHarness(t, testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		var gqlReq struct {
			OperationName string         `json:"operationName"`
			Variables     map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(req.Body).Decode(&gqlReq); err != nil {
			t.Fatalf("Decode request error = %v", err)
		}
		if gqlReq.OperationName != "GetTransactionsList" {
			t.Fatalf("operation = %q, want GetTransactionsList", gqlReq.OperationName)
		}
		var ok bool
		filters, ok = gqlReq.Variables["filters"].(map[string]any)
		if !ok {
			t.Fatalf("filters = %#v, want object", gqlReq.Variables["filters"])
		}
		if gqlReq.Variables["offset"] != float64(0) || gqlReq.Variables["limit"] != float64(1000) {
			t.Fatalf("variables = %#v, want first 1000-item page", gqlReq.Variables)
		}
		return testutil.JSONResponse(`{"data":{"allTransactions":{"results":[{"id":"tx-1","date":"2026-07-01","amount":-42,"plaidName":"Store","account":{"id":"acc-1"}},{"id":"tx-2","date":"2026-07-01","amount":-42,"plaidName":"Store","account":{"id":"acc-1"}}],"totalCount":2}}}`), nil
	}))

	if err := h.execute("--json", "transactions", "duplicates"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	after := time.Now()
	if h.ExitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", h.ExitCode, h.Stdout.String())
	}
	if !matchesDuplicateWindow(filters, before) && !matchesDuplicateWindow(filters, after) {
		t.Fatalf("filters = %#v, want current-date through month-end window", filters)
	}
	for _, want := range []string{`"command":"transactions.duplicates"`, `"id":"tx-1"`, `"id":"tx-2"`, "uses legacy Monarch GraphQL root field: allTransactions"} {
		if !strings.Contains(h.Stdout.String(), want) {
			t.Fatalf("output missing %q = %q", want, h.Stdout.String())
		}
	}
}

func matchesDuplicateWindow(filters map[string]any, now time.Time) bool {
	start := now.Format("2006-01-02")
	end := time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
	return filters["startDate"] == start && filters["endDate"] == end
}

func TestTransactionsListAPIError(t *testing.T) {
	h := newJSONCommandHarness(t, testutil.RoundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       io.NopCloser(bytes.NewReader(nil)),
		}, nil
	}))

	if err := h.execute("--json", "transactions", "list"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	out := h.Stdout.String()
	if h.ExitCode == 0 || !strings.Contains(out, `"API_ERROR"`) {
		t.Fatalf("exitCode = %d; output=%q, want API_ERROR", h.ExitCode, out)
	}
}

func TestTransactionsShowRequiresID(t *testing.T) {
	h := newJSONCommandHarness(t, nil)
	if err := h.execute("--json", "transactions", "show"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if h.ExitCode != 2 || !strings.Contains(h.Stdout.String(), `"INVALID_ARGUMENTS"`) || !strings.Contains(h.Stdout.String(), `"command":"transactions.show"`) || !strings.Contains(h.Stdout.String(), "accepts 1 arg") {
		t.Fatalf("exitCode = %d; output=%q, want JSON argument error", h.ExitCode, h.Stdout.String())
	}
}

func TestTransactionsDeleteJSON(t *testing.T) {
	h := newJSONCommandHarness(t, testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		var gqlReq struct {
			OperationName string         `json:"operationName"`
			Variables     map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(req.Body).Decode(&gqlReq); err != nil {
			t.Fatalf("Decode request error = %v", err)
		}
		input, ok := gqlReq.Variables["input"].(map[string]any)
		if !ok {
			t.Fatalf("input = %#v, want object", gqlReq.Variables["input"])
		}
		if gqlReq.OperationName != "Common_DeleteTransactionMutation" || input["transactionId"] != "tx-1" {
			t.Fatalf("request = %#v, want Common_DeleteTransactionMutation tx-1", gqlReq)
		}
		return testutil.JSONResponse(`{"data":{"deleteTransaction":{"deleted":true}}}`), nil
	}))

	if err := h.execute("--json", "--confirm", "transactions", "delete", "tx-1"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	out := h.Stdout.String()
	if h.ExitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", h.ExitCode, out)
	}
	if !strings.Contains(out, `"command":"transactions.delete"`) || !strings.Contains(out, `"status":"deleted"`) {
		t.Fatalf("output = %q, want transaction delete status", out)
	}
}

func TestTransactionsExportJSON(t *testing.T) {
	h := newJSONCommandHarness(t, testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		var gqlReq struct {
			OperationName string `json:"operationName"`
		}
		if err := json.NewDecoder(req.Body).Decode(&gqlReq); err != nil {
			t.Fatalf("Decode request error = %v", err)
		}
		if gqlReq.OperationName != "GetTransactionsList" {
			t.Fatalf("operation = %q, want GetTransactionsList", gqlReq.OperationName)
		}
		return testutil.JSONResponse(`{"data":{"allTransactions":{"results":[{"id":"tx-1","date":"2026-05-08","amount":-20,"merchant":{"name":"Store"},"category":{"name":"Food"},"notes":"lunch","tags":[],"goal":{"id":"","name":""},"account":{"id":"acc-1","displayName":"Checking","order":0,"type":{"group":"depository"}},"ownedByUser":{"displayName":"Test User"}}],"totalCount":1}}}`), nil
	}))

	if err := h.execute("--json", "transactions", "export", "--format", "json"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	out := h.Stdout.String()
	if h.ExitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", h.ExitCode, out)
	}
	if !strings.Contains(out, `"command":"transactions.export"`) || !strings.Contains(out, "Store") {
		t.Fatalf("output = %q, want JSON export", out)
	}
}

func TestTransactionsExportJSONToFile(t *testing.T) {
	h := newJSONCommandHarness(t, testutil.RoundTripFunc(func(*http.Request) (*http.Response, error) {
		return testutil.JSONResponse(`{"data":{"allTransactions":{"results":[{"id":"tx-1","date":"2026-05-08","amount":-20,"merchant":{"name":"Store"},"category":{"name":"Food"},"account":{"id":"acc-1","type":{}}}],"totalCount":1}}}`), nil
	}))
	path := filepath.Join(t.TempDir(), "transactions.json")

	if err := h.execute("transactions", "export", "--format", "json", "--output", path); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if h.ExitCode != 0 || h.Stdout.Len() != 0 {
		t.Fatalf("exitCode = %d; stdout=%q", h.ExitCode, h.Stdout.String())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(data), `"command":"transactions.export"`) || !strings.Contains(string(data), "Store") {
		t.Fatalf("file = %q, want JSON export", data)
	}
}

func TestTransactionsExportRejectsInvalidFormat(t *testing.T) {
	h := newJSONCommandHarness(t, nil)
	if err := h.execute("--json", "transactions", "export", "--format", "yaml"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if h.ExitCode == 0 || !strings.Contains(h.Stdout.String(), `"INVALID_ARGUMENTS"`) {
		t.Fatalf("exitCode = %d; output=%q, want INVALID_ARGUMENTS", h.ExitCode, h.Stdout.String())
	}
}

func TestTransactionsExportJSONFormatsAPIErrors(t *testing.T) {
	h := newJSONCommandHarness(t, testutil.RoundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(bytes.NewReader(nil))}, nil
	}))
	if err := h.execute("transactions", "export", "--format", "json"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if h.ExitCode == 0 || !strings.Contains(h.Stdout.String(), `"API_ERROR"`) {
		t.Fatalf("exitCode = %d; output=%q, want JSON API_ERROR", h.ExitCode, h.Stdout.String())
	}
}

func TestTransactionsExportJSONReportsWriteFailure(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("requires /dev/full")
	}
	h := newJSONCommandHarness(t, testutil.RoundTripFunc(func(*http.Request) (*http.Response, error) {
		return testutil.JSONResponse(`{"data":{"allTransactions":{"results":[],"totalCount":0}}}`), nil
	}))
	if err := h.execute("transactions", "export", "--format", "json", "--output", "/dev/full"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if h.ExitCode == 0 || !strings.Contains(h.Stdout.String(), `"INTERNAL_ERROR"`) {
		t.Fatalf("exitCode = %d; output=%q, want INTERNAL_ERROR", h.ExitCode, h.Stdout.String())
	}
}

func TestTransactionsTagsClearJSON(t *testing.T) {
	h := newJSONCommandHarness(t, testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		var gqlReq struct {
			OperationName string         `json:"operationName"`
			Variables     map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(req.Body).Decode(&gqlReq); err != nil {
			t.Fatalf("Decode request error = %v", err)
		}
		if gqlReq.OperationName != "Web_SetTransactionTags" {
			t.Fatalf("operation = %q, want Web_SetTransactionTags", gqlReq.OperationName)
		}
		input, ok := gqlReq.Variables["input"].(map[string]any)
		if !ok {
			t.Fatalf("input = %#v, want object", gqlReq.Variables["input"])
		}
		tagIDs, ok := input["tagIds"].([]any)
		if !ok || len(tagIDs) != 0 {
			t.Fatalf("tagIds = %#v, want empty list", input["tagIds"])
		}
		return testutil.JSONResponse(`{"data":{"setTransactionTags":{}}}`), nil
	}))

	if err := h.execute("--json", "--confirm", "transactions", "tags", "clear", "tx-1"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	out := h.Stdout.String()
	if h.ExitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", h.ExitCode, out)
	}
	if !strings.Contains(out, `"command":"transactions.tags.clear"`) || !strings.Contains(out, `"status":"tags cleared"`) {
		t.Fatalf("output = %q, want cleared tags status", out)
	}
}

func TestTransactionsTagsAddJSON(t *testing.T) {
	callCount := 0
	h := newJSONCommandHarness(t, testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		var gqlReq struct {
			OperationName string         `json:"operationName"`
			Variables     map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(req.Body).Decode(&gqlReq); err != nil {
			t.Fatalf("Decode request error = %v", err)
		}
		callCount++
		if callCount == 1 {
			if gqlReq.OperationName != "GetTransactionDrawer" {
				t.Fatalf("call 1 operation = %q, want GetTransaction", gqlReq.OperationName)
			}
			return testutil.JSONResponse(`{"data":{"getTransaction":{"id":"tx-1","date":"2026-05-08","amount":-20,"merchant":{"name":"Store"},"category":{"name":"Food"},"notes":"lunch","account":{"id":"acc-1","displayName":"Checking"},"tags":[{"id":"tag-old","name":"existing","color":"#ff0000"}]}}}`), nil
		}
		if gqlReq.OperationName != "Web_SetTransactionTags" {
			t.Fatalf("call 2 operation = %q, want Web_SetTransactionTags", gqlReq.OperationName)
		}
		input, ok := gqlReq.Variables["input"].(map[string]any)
		if !ok {
			t.Fatalf("input = %#v, want object", gqlReq.Variables["input"])
		}
		tagIDs, ok := input["tagIds"].([]any)
		if !ok {
			t.Fatalf("tagIds = %#v, want array", input["tagIds"])
		}
		if len(tagIDs) != 2 || tagIDs[0] != "tag-old" || tagIDs[1] != "tag-new" {
			t.Fatalf("tagIds = %#v, want existing and new tags", tagIDs)
		}
		return testutil.JSONResponse(`{"data":{"setTransactionTags":{}}}`), nil
	}))

	if err := h.execute("--json", "--confirm", "transactions", "tags", "add", "tx-1", "--tag", "tag-new"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	out := h.Stdout.String()
	if h.ExitCode != 0 || callCount != 2 {
		t.Fatalf("exitCode = %d; calls=%d; output=%q", h.ExitCode, callCount, out)
	}
	if !strings.Contains(out, `"command":"transactions.tags.add"`) || !strings.Contains(out, `"status":"tags added"`) {
		t.Fatalf("output = %q, want tags-added status", out)
	}
}

func TestTransactionsSplitsJSON(t *testing.T) {
	h := newJSONCommandHarness(t, testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		var gqlReq struct {
			OperationName string `json:"operationName"`
		}
		if err := json.NewDecoder(req.Body).Decode(&gqlReq); err != nil {
			t.Fatalf("Decode request error = %v", err)
		}
		if gqlReq.OperationName != "TransactionSplitQuery" {
			t.Fatalf("operation = %q, want TransactionSplitQuery", gqlReq.OperationName)
		}
		return testutil.JSONResponse(`{"data":{"getTransaction":{"id":"tx-1","amount":-60,"splitTransactions":[{"id":"split-1","amount":-20,"notes":"part1","merchant":{"name":"Store"},"category":{"name":"Food"}},{"id":"split-2","amount":-40,"notes":"part2","merchant":{"name":"Store"},"category":{"name":"Drinks"}}]}}}`), nil
	}))

	if err := h.execute("--json", "transactions", "splits", "tx-1"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	out := h.Stdout.String()
	if h.ExitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", h.ExitCode, out)
	}
	if !strings.Contains(out, `"command":"transactions.splits"`) || !strings.Contains(out, `"category":"Food"`) {
		t.Fatalf("output = %q, want split transaction", out)
	}
}

func TestTransactionsAttachmentsListJSON(t *testing.T) {
	h := newJSONCommandHarness(t, testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		var gqlReq struct {
			OperationName string `json:"operationName"`
		}
		if err := json.NewDecoder(req.Body).Decode(&gqlReq); err != nil {
			t.Fatalf("Decode request error = %v", err)
		}
		if gqlReq.OperationName != "GetTransactionDrawer" {
			t.Fatalf("operation = %q, want GetTransaction", gqlReq.OperationName)
		}
		return testutil.JSONResponse(`{"data":{"getTransaction":{"attachments":[{"id":"att-1","extension":"pdf","filename":"receipt.pdf","originalAssetUrl":"https://example.com/receipt.pdf","sizeBytes":1024}]}}}`), nil
	}))

	if err := h.execute("--json", "transactions", "attachments", "list", "tx-1"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	out := h.Stdout.String()
	if h.ExitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", h.ExitCode, out)
	}
	if !strings.Contains(out, `"command":"transactions.attachments.list"`) || !strings.Contains(out, "receipt.pdf") {
		t.Fatalf("output = %q, want attachment filename", out)
	}
}

func TestTransactionsAttachmentsUploadJSON(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "receipt.pdf")
	if err := os.WriteFile(tmpFile, []byte("pdf"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	h := newJSONCommandHarness(t, testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host == "api.cloudinary.com" {
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("ReadAll() error = %v", err)
			}
			if !strings.Contains(string(body), `filename="receipt.pdf"`) || !strings.Contains(string(body), `name="upload_preset"`) {
				t.Fatalf("cloudinary body missing fields: %s", body)
			}
			return testutil.JSONResponse(`{"public_id":"pub-1","format":"pdf","bytes":3}`), nil
		}
		var gqlReq struct {
			OperationName string `json:"operationName"`
		}
		if err := json.NewDecoder(req.Body).Decode(&gqlReq); err != nil {
			t.Fatalf("Decode request error = %v", err)
		}
		switch gqlReq.OperationName {
		case "Common_GetTransactionAttachmentUploadInfo":
			return testutil.JSONResponse(`{"data":{"getTransactionAttachmentUploadInfo":{"info":{"requestParams":{"timestamp":1700000000,"folder":"monarch","signature":"sig-1","api_key":"key-1","upload_preset":"preset-1"}}}}}`), nil
		case "Common_AddTransactionAttachment":
			return testutil.JSONResponse(`{"data":{"addTransactionAttachment":{"errors":null}}}`), nil
		default:
			t.Fatalf("operation = %q, want attachment upload ops", gqlReq.OperationName)
			return nil, nil
		}
	}))

	if err := h.execute("--json", "--confirm", "transactions", "attachments", "upload", "tx-1", tmpFile); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if h.ExitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", h.ExitCode, h.Stdout.String())
	}
	if !strings.Contains(h.Stdout.String(), `"command":"transactions.attachments.upload"`) || !strings.Contains(h.Stdout.String(), `"status":"uploaded"`) {
		t.Fatalf("output = %q, want upload status", h.Stdout.String())
	}
}

func TestTransactionsSearchJSON(t *testing.T) {
	h := newJSONCommandHarness(t, testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		var gqlReq struct {
			OperationName string         `json:"operationName"`
			Variables     map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(req.Body).Decode(&gqlReq); err != nil {
			t.Fatalf("Decode request error = %v", err)
		}
		if gqlReq.OperationName != "GetTransactionsList" {
			t.Fatalf("operation = %q, want GetTransactionsList", gqlReq.OperationName)
		}
		filters, ok := gqlReq.Variables["filters"].(map[string]any)
		if !ok {
			t.Fatalf("filters = %#v, want object", gqlReq.Variables["filters"])
		}
		if filters["search"] != "Amazon" {
			t.Fatalf("search = %v, want Amazon", filters["search"])
		}
		return testutil.JSONResponse(`{"data":{"allTransactions":{"results":[{"id":"tx-1","date":"2026-05-08","amount":-50,"merchant":{"name":"Amazon"},"category":{"name":"Shopping"},"notes":"order","tags":[],"goal":{"id":"","name":""},"account":{"id":"acc-1","displayName":"Checking","order":0,"type":{"group":"depository"}},"ownedByUser":{"displayName":"Test User"}}],"totalCount":1}}}`), nil
	}))

	if err := h.execute("--json", "transactions", "search", "Amazon"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	out := h.Stdout.String()
	if h.ExitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", h.ExitCode, out)
	}
	if !strings.Contains(out, `"command":"transactions.search"`) || !strings.Contains(out, `"total":1`) {
		t.Fatalf("output = %q, want search result", out)
	}
}
