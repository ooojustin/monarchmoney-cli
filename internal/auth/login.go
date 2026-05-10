package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/pquerna/otp/totp"
	"github.com/thedavidweng/monarchmoney-cli/internal/errors"
	"github.com/thedavidweng/monarchmoney-cli/internal/graphql"
)

const defaultLoginEndpoint = "https://api.monarch.com/auth/login/"

type Client struct {
	Endpoint   string
	HTTPClient *http.Client
}

func NewClient(endpoint string, httpClient *http.Client) *Client {
	if endpoint == "" {
		endpoint = defaultLoginEndpoint
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &Client{
		Endpoint:   endpoint,
		HTTPClient: httpClient,
	}
}

type loginRequest struct {
	Username      string `json:"username"`
	Password      string `json:"password"`
	SupportsMFA   bool   `json:"supports_mfa"`
	TrustedDevice bool   `json:"trusted_device"`
	TOTP          string `json:"totp,omitempty"`
}

type loginResponse struct {
	Token string `json:"token"`
}

// Authenticate logs in with the default auth client.
func Authenticate(email, password, mfaCode, mfaSecret string) (*Session, error) {
	return NewClient("", nil).Authenticate(email, password, mfaCode, mfaSecret)
}

// Authenticate logs in through Monarch's REST endpoint, not GraphQL.
// Monarch maps 401/403 without MFA to "MFA required" and with MFA to invalid credentials or code.
func (c *Client) Authenticate(email, password, mfaCode, mfaSecret string) (*Session, error) {
	if c == nil {
		c = NewClient("", nil)
	}

	if mfaSecret != "" {
		code, err := totp.GenerateCode(mfaSecret, time.Now())
		if err != nil {
			return nil, errors.New(errors.InternalError, "failed to generate MFA code", errors.CatInternal, false, err)
		}
		mfaCode = code
	}

	reqBody := loginRequest{
		Username:      email,
		Password:      password,
		SupportsMFA:   true,
		TrustedDevice: true,
		TOTP:          mfaCode,
	}
	body, _ := json.Marshal(reqBody)

	endpoint := c.Endpoint
	if endpoint == "" {
		endpoint = defaultLoginEndpoint
	}
	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(body))
	if err != nil {
		return nil, errors.New(errors.InternalError, "failed to create login request", errors.CatInternal, false, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Client-Platform", "web")
	req.Header.Set("User-Agent", graphql.UserAgent)

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, errors.New(errors.NetworkUnreachable, "failed to reach Monarch API", errors.CatNetwork, true, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 403 || resp.StatusCode == 401 {
		if mfaCode == "" && mfaSecret == "" {
			return nil, errors.New(errors.AuthMFARequired, "MFA code required", errors.CatAuth, false, nil)
		}
		return nil, errors.New(errors.AuthMFAInvalid, "invalid credentials or MFA code", errors.CatAuth, false, nil)
	}

	if resp.StatusCode != 200 {
		var apiErr struct {
			Detail    string `json:"detail"`
			ErrorCode string `json:"error_code"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&apiErr); err == nil && apiErr.Detail != "" {
			return nil, errors.New(errors.APIError, apiErr.Detail, errors.CatAPI, false, nil)
		}
		return nil, errors.New(errors.APIError, fmt.Sprintf("API returned status %d", resp.StatusCode), errors.CatAPI, false, nil)
	}

	var loginResp loginResponse
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		return nil, errors.New(errors.APISchemaChanged, "failed to parse login response", errors.CatAPI, false, err)
	}

	return &Session{
		Email:     email,
		Token:     loginResp.Token,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
}
