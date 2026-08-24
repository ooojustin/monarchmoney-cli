package monarch

import (
	"context"
	"net/http"
	"time"

	"github.com/thedavidweng/monarchmoney-cli/internal/graphql"
)

type graphQLClient interface {
	Do(ctx context.Context, reqBody *graphql.Request, result any) error
	DoMutation(ctx context.Context, reqBody *graphql.Request, result any) error
	TokenValue() string
	DeviceUUIDValue() string
}

type Service struct {
	Client                       graphQLClient
	HTTPClient                   *http.Client
	AttachmentHTTPClient         *http.Client
	BalanceHistoryUploadEndpoint string
	attachmentUploadHTTPClient   *http.Client
	receiptUploadHTTPClient      *http.Client
	balanceHistoryPollDelay      time.Duration
	balanceHistoryPollTimeout    time.Duration
}

type ServiceOption func(*Service)

func WithHTTPClient(client *http.Client) ServiceOption {
	return func(s *Service) {
		if client != nil {
			s.HTTPClient = client
			s.AttachmentHTTPClient = client
			s.attachmentUploadHTTPClient = client
			s.receiptUploadHTTPClient = client
		}
	}
}

func WithHTTPTransport(transport http.RoundTripper) ServiceOption {
	return func(s *Service) {
		if transport != nil {
			s.HTTPClient = &http.Client{Transport: transport}
			s.AttachmentHTTPClient = &http.Client{Timeout: 30 * time.Second, Transport: transport}
			s.attachmentUploadHTTPClient = &http.Client{Timeout: 60 * time.Second, Transport: transport}
			s.receiptUploadHTTPClient = &http.Client{Timeout: 120 * time.Second, Transport: transport}
		}
	}
}

func WithBalanceHistoryUploadEndpoint(endpoint string) ServiceOption {
	return func(s *Service) {
		if endpoint != "" {
			s.BalanceHistoryUploadEndpoint = endpoint
		}
	}
}

func NewService(client graphQLClient, opts ...ServiceOption) *Service {
	s := &Service{
		Client:                       client,
		HTTPClient:                   http.DefaultClient,
		AttachmentHTTPClient:         &http.Client{Timeout: 30 * time.Second},
		BalanceHistoryUploadEndpoint: "https://api.monarch.com/account-balance-history/upload/",
		attachmentUploadHTTPClient:   &http.Client{Timeout: 60 * time.Second},
		receiptUploadHTTPClient:      &http.Client{Timeout: 120 * time.Second},
		balanceHistoryPollDelay:      10 * time.Second,
		balanceHistoryPollTimeout:    300 * time.Second,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *Service) httpClient() *http.Client {
	if s.HTTPClient != nil {
		return s.HTTPClient
	}
	return http.DefaultClient
}

func (s *Service) attachmentHTTPClient() *http.Client {
	if s.AttachmentHTTPClient != nil {
		return s.AttachmentHTTPClient
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func (s *Service) attachmentUploadClient() *http.Client {
	if s.attachmentUploadHTTPClient != nil {
		return s.attachmentUploadHTTPClient
	}
	return &http.Client{Timeout: 60 * time.Second}
}

func (s *Service) receiptUploadClient() *http.Client {
	if s.receiptUploadHTTPClient != nil {
		return s.receiptUploadHTTPClient
	}
	return &http.Client{Timeout: 120 * time.Second}
}

func (s *Service) balanceHistoryUploadEndpoint() string {
	if s.BalanceHistoryUploadEndpoint != "" {
		return s.BalanceHistoryUploadEndpoint
	}
	return "https://api.monarch.com/account-balance-history/upload/"
}
