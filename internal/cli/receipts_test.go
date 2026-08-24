package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thedavidweng/monarchmoney-cli/internal/testutil"
)

func TestReceiptsUploadJSON(t *testing.T) {
	receiptPath := filepath.Join(t.TempDir(), "receipt.jpg")
	if err := os.WriteFile(receiptPath, []byte("jpg"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	h := newJSONCommandHarness(t, testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path == "/retail-sync/sync-1/files" {
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("ReadAll() error = %v", err)
			}
			if !strings.Contains(string(body), `filename="receipt.jpg"`) || !strings.Contains(string(body), `"vendor":"user_import"`) {
				t.Fatalf("retail sync body missing fields: %s", body)
			}
			return testutil.JSONResponse(`{}`), nil
		}
		var gqlReq struct {
			OperationName string `json:"operationName"`
		}
		if err := json.NewDecoder(req.Body).Decode(&gqlReq); err != nil {
			t.Fatalf("Decode request error = %v", err)
		}
		switch gqlReq.OperationName {
		case "Common_CreateBulkRetailSync":
			return testutil.JSONResponse(`{"data":{"createBulkRetailSync":{"retailSyncs":[{"id":"sync-1","vendor":"user_import","status":"created"}],"errors":null}}}`), nil
		case "Common_StartRetailSync":
			return testutil.JSONResponse(`{"data":{"startRetailSync":{"retailSync":{"id":"sync-1","vendor":"user_import","status":"started","startedAt":"2026-05-01T00:00:00Z"},"errors":null}}}`), nil
		default:
			t.Fatalf("operation = %q, want retail sync ops", gqlReq.OperationName)
			return nil, nil
		}
	}))

	if err := h.execute("--json", "--confirm", "receipts", "upload", receiptPath); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if h.ExitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", h.ExitCode, h.Stdout.String())
	}
	if !strings.Contains(h.Stdout.String(), `"command":"receipts.upload"`) || !strings.Contains(h.Stdout.String(), `"status":"started"`) {
		t.Fatalf("output = %q, want started receipt sync", h.Stdout.String())
	}
}

func TestReceiptsUploadDryRunDoesNotReadFileOrCallNetwork(t *testing.T) {
	h := newJSONCommandHarness(t, testutil.RoundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("dry-run made a network request")
		return nil, nil
	}))
	missing := filepath.Join(t.TempDir(), "missing.jpg")

	if err := h.execute("--json", "--dry-run", "receipts", "upload", missing); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if h.ExitCode != 0 || !strings.Contains(h.Stdout.String(), `"operation":"receipts.upload"`) || !strings.Contains(h.Stdout.String(), `"file":"`+missing+`"`) {
		t.Fatalf("exitCode = %d; output=%q, want mutation plan", h.ExitCode, h.Stdout.String())
	}
	if strings.Contains(h.Stdout.String(), `"resource_id"`) {
		t.Fatalf("output = %q, local file path must not be persisted as a resource ID", h.Stdout.String())
	}
}
