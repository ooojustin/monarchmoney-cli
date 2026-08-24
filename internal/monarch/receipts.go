package monarch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"

	"github.com/google/uuid"

	"github.com/thedavidweng/monarchmoney-cli/internal/errors"
	"github.com/thedavidweng/monarchmoney-cli/internal/graphql"
	"github.com/thedavidweng/monarchmoney-cli/queries"
)

var CreateBulkRetailSyncMutation = queries.Get("receipts/create_bulk_retail_sync.graphql")
var StartRetailSyncMutation = queries.Get("receipts/start_retail_sync.graphql")

const retailSyncFilesURL = "https://api.monarch.com/retail-sync/%s/files"

func createRetailSyncFormFile(w *multipart.Writer, field, filename, contentType string) (io.Writer, error) {
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", `form-data; name="`+escapeFormQuotes(field)+`"; filename="`+escapeFormQuotes(filename)+`"`)
	header.Set("Content-Type", contentType)
	return w.CreatePart(header)
}

type ReceiptSync struct {
	ID        string `json:"id"`
	Vendor    string `json:"vendor"`
	Status    string `json:"status"`
	StartedAt string `json:"started_at"`
	EndedAt   string `json:"ended_at"`
}

func (s *Service) UploadReceiptToInbox(ctx context.Context, path string) (*ReceiptSync, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.New(errors.InternalError, "failed to read receipt file", errors.CatInternal, false, err)
	}
	filename := filepath.Base(path)
	contentType := mime.TypeByExtension(filepath.Ext(filename))
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	var createResp struct {
		CreateBulkRetailSync struct {
			RetailSyncs []struct {
				ID     string `json:"id"`
				Vendor string `json:"vendor"`
				Status string `json:"status"`
			} `json:"retailSyncs"`
			Errors *struct {
				Message string `json:"message"`
			} `json:"errors"`
		} `json:"createBulkRetailSync"`
	}
	err = s.Client.DoMutation(ctx, &graphql.Request{
		OperationName: "Common_CreateBulkRetailSync",
		Query:         CreateBulkRetailSyncMutation,
		Variables:     map[string]any{"input": map[string]any{"count": 1}},
	}, &createResp)
	if err != nil {
		return nil, err
	}
	if len(createResp.CreateBulkRetailSync.RetailSyncs) == 0 || createResp.CreateBulkRetailSync.Errors != nil {
		msg := "failed to create retail sync session"
		if createResp.CreateBulkRetailSync.Errors != nil {
			msg = createResp.CreateBulkRetailSync.Errors.Message
		}
		return nil, errors.New(errors.APIError, msg, errors.CatAPI, false, nil)
	}
	syncID := createResp.CreateBulkRetailSync.RetailSyncs[0].ID

	if err := s.postReceiptFile(ctx, syncID, filename, contentType, content); err != nil {
		return nil, err
	}

	var startResp struct {
		StartRetailSync struct {
			RetailSync *struct {
				ID        string `json:"id"`
				Vendor    string `json:"vendor"`
				Status    string `json:"status"`
				StartedAt string `json:"startedAt"`
				EndedAt   string `json:"endedAt"`
			} `json:"retailSync"`
			Errors *struct {
				Message string `json:"message"`
			} `json:"errors"`
		} `json:"startRetailSync"`
	}
	err = s.Client.DoMutation(ctx, &graphql.Request{
		OperationName: "Common_StartRetailSync",
		Query:         StartRetailSyncMutation,
		Variables:     map[string]any{"syncId": syncID},
	}, &startResp)
	if err != nil {
		return nil, err
	}
	if startResp.StartRetailSync.RetailSync == nil || startResp.StartRetailSync.Errors != nil {
		msg := "failed to start retail sync"
		if startResp.StartRetailSync.Errors != nil {
			msg = startResp.StartRetailSync.Errors.Message
		}
		return nil, errors.New(errors.APIError, msg, errors.CatAPI, false, nil)
	}

	started := startResp.StartRetailSync.RetailSync
	return &ReceiptSync{
		ID:        started.ID,
		Vendor:    started.Vendor,
		Status:    started.Status,
		StartedAt: started.StartedAt,
		EndedAt:   started.EndedAt,
	}, nil
}

func (s *Service) postReceiptFile(ctx context.Context, syncID, filename, contentType string, content []byte) error {
	metadata, err := json.Marshal(map[string]string{
		"orderId":     uuid.NewString(),
		"vendor":      "user_import",
		"payloadType": "order",
		"contentType": contentType,
	})
	if err != nil {
		return errors.New(errors.InternalError, "failed to encode receipt metadata", errors.CatInternal, false, err)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("payloads_count", "1"); err != nil {
		return errors.New(errors.InternalError, "failed to write receipt form field", errors.CatInternal, false, err)
	}
	if err := writer.WriteField("metadata_0", string(metadata)); err != nil {
		return errors.New(errors.InternalError, "failed to write receipt form field", errors.CatInternal, false, err)
	}
	part, err := createRetailSyncFormFile(writer, "payload_0", filename, contentType)
	if err != nil {
		return err
	}
	if _, err := part.Write(content); err != nil {
		return errors.New(errors.InternalError, "failed to write receipt content", errors.CatInternal, false, err)
	}
	if err := writer.Close(); err != nil {
		return errors.New(errors.InternalError, "failed to finalize receipt upload body", errors.CatInternal, false, err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf(retailSyncFilesURL, syncID), body)
	if err != nil {
		return errors.New(errors.InternalError, "failed to create receipt upload request", errors.CatInternal, false, err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Client-Platform", "web")
	req.Header.Set("User-Agent", graphql.UserAgent())
	if token := s.Client.TokenValue(); token != "" {
		req.Header.Set("Authorization", "Token "+token)
	}
	if deviceUUID := s.Client.DeviceUUIDValue(); deviceUUID != "" {
		req.Header.Set("device-uuid", deviceUUID)
	}

	resp, err := s.receiptUploadClient().Do(req)
	if err != nil {
		return errors.New(errors.NetworkUnreachable, "failed to reach retail sync endpoint", errors.CatNetwork, true, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return errors.New(errors.APIError, fmt.Sprintf("receipt upload failed with status %d", resp.StatusCode), errors.CatAPI, false, nil)
	}
	return nil
}
