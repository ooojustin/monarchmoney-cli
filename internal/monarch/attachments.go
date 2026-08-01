package monarch

import (
	"context"
	"io"
	"net/http"

	"github.com/thedavidweng/monarchmoney-cli/internal/errors"
	"github.com/thedavidweng/monarchmoney-cli/internal/graphql"
)

type Attachment struct {
	ID        string `json:"id"`
	Filename  string `json:"filename"`
	Extension string `json:"extension"`
	URL       string `json:"url"`
	SizeBytes int    `json:"size_bytes"`
}

func (s *Service) ListTransactionAttachments(ctx context.Context, txID string) ([]Attachment, error) {
	var resp struct {
		GetTransaction struct {
			Attachments []struct {
				ID               string `json:"id"`
				Extension        string `json:"extension"`
				Filename         string `json:"filename"`
				OriginalAssetUrl string `json:"originalAssetUrl"`
				SizeBytes        int    `json:"sizeBytes"`
			} `json:"attachments"`
		} `json:"getTransaction"`
	}

	err := s.Client.Do(ctx, &graphql.Request{
		OperationName: "GetTransactionDrawer",
		Query:         GetTransactionQuery,
		Variables:     map[string]any{"id": txID},
	}, &resp)
	if err != nil {
		return nil, err
	}

	attachments := make([]Attachment, len(resp.GetTransaction.Attachments))
	for i, a := range resp.GetTransaction.Attachments {
		attachments[i] = Attachment{
			ID:        a.ID,
			Filename:  a.Filename,
			Extension: a.Extension,
			URL:       a.OriginalAssetUrl,
			SizeBytes: a.SizeBytes,
		}
	}
	return attachments, nil
}

func (s *Service) DownloadAttachment(ctx context.Context, url string, w io.Writer) error {
	// Attachment assets are served from Monarch's file URL and are fetched directly.
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return errors.New(errors.InternalError, "failed to create download request", errors.CatInternal, false, err)
	}

	resp, err := s.attachmentHTTPClient().Do(req)
	if err != nil {
		return errors.New(errors.NetworkUnreachable, "failed to reach attachment URL", errors.CatNetwork, true, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return errors.New(errors.APIError, "failed to download attachment", errors.CatAPI, false, nil)
	}

	_, err = io.Copy(w, resp.Body)
	return err
}

func (s *Service) UploadAttachment(ctx context.Context, txID, path string) error {
	return errors.New(errors.FEATURE_UNAVAILABLE, "transaction attachment upload is unavailable in the current Monarch API", errors.CatAPI, false, nil)
}
