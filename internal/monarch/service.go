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
}

type ServiceOption func(*Service)

func WithHTTPClient(client *http.Client) ServiceOption {
	return func(s *Service) {
		if client != nil {
			s.HTTPClient = client
			s.AttachmentHTTPClient = client
		}
	}
}

func WithHTTPTransport(transport http.RoundTripper) ServiceOption {
	return func(s *Service) {
		if transport != nil {
			s.HTTPClient = &http.Client{Transport: transport}
			s.AttachmentHTTPClient = &http.Client{Timeout: 30 * time.Second, Transport: transport}
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

func (s *Service) balanceHistoryUploadEndpoint() string {
	if s.BalanceHistoryUploadEndpoint != "" {
		return s.BalanceHistoryUploadEndpoint
	}
	return "https://api.monarch.com/account-balance-history/upload/"
}
