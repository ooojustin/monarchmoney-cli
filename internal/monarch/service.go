package monarch

import (
	"context"
	"net/http"
	"time"

	"github.com/thedavidweng/monarchmoney-cli/internal/config"
	"github.com/thedavidweng/monarchmoney-cli/internal/graphql"
)

type graphQLClient interface {
	Do(ctx context.Context, reqBody *graphql.Request, result interface{}) error
	TokenValue() string
}

// Service provides access to Monarch Money data.
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

// NewService returns a new Service.
func NewService(client graphQLClient, opts ...ServiceOption) *Service {
	s := &Service{
		Client:                       client,
		HTTPClient:                   &http.Client{},
		AttachmentHTTPClient:         &http.Client{Timeout: 30 * time.Second},
		BalanceHistoryUploadEndpoint: config.AccountBalanceHistoryUploadEndpoint(config.DefaultAPIBaseURL),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *Service) balanceHistoryUploadEndpoint() string {
	if s.BalanceHistoryUploadEndpoint != "" {
		return s.BalanceHistoryUploadEndpoint
	}
	return config.AccountBalanceHistoryUploadEndpoint(config.DefaultAPIBaseURL)
}

func (s *Service) httpClient() *http.Client {
	if s.HTTPClient != nil {
		return s.HTTPClient
	}
	return &http.Client{}
}

func (s *Service) attachmentHTTPClient() *http.Client {
	if s.AttachmentHTTPClient != nil {
		return s.AttachmentHTTPClient
	}
	return &http.Client{Timeout: 30 * time.Second}
}
