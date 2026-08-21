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

func TestReceipts(t *testing.T) {
	t.Run("upload", testReceiptsUpload)
}

func testReceiptsUpload(t *testing.T) {
	dir := t.TempDir()
	sessionPath := filepath.Join(dir, "session.json")
	exitCode := withWriteCommandTestDefaults(t, sessionPath, receiptsUploadCmd)
	saveTestSession(t, sessionPath)

	receiptPath := filepath.Join(dir, "receipt.jpg")
	if err := os.WriteFile(receiptPath, []byte("jpg"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	http.DefaultTransport = testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
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
	})

	out := captureStdout(t, func() {
		receiptsUploadCmd.Run(receiptsUploadCmd, []string{receiptPath})
	})

	if *exitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", *exitCode, out)
	}
	if !strings.Contains(out, `"command":"receipts.upload"`) {
		t.Fatalf("output missing command = %q", out)
	}
	if !strings.Contains(out, `"status":"started"`) {
		t.Fatalf("output missing status = %q", out)
	}
}
