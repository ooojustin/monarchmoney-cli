package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/pquerna/otp/totp"

	"github.com/thedavidweng/monarchmoney-cli/internal/errors"
	"github.com/thedavidweng/monarchmoney-cli/internal/graphql"
)

const loginEndpoint = "https://api.monarch.com/auth/login/"
const maxLoginResponseSize = int64(1 << 20)

type Credentials struct {
	Email     string
	Password  string
	MFACode   string
	MFASecret string
}

type Client struct {
	http *http.Client
}

func NewClient(transport http.RoundTripper) *Client {
	return &Client{http: &http.Client{
		Timeout:       10 * time.Second,
		Transport:     transport,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
	}}
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

// Authenticate logs in through Monarch's REST endpoint, not GraphQL.
// Monarch maps 401/403 without MFA to "MFA required" and with MFA to invalid credentials or code.
func Authenticate(email, password, mfaCode, mfaSecret string) (*Session, error) {
	return NewClient(nil).Authenticate(context.Background(), Credentials{
		Email:     email,
		Password:  password,
		MFACode:   mfaCode,
		MFASecret: mfaSecret,
	})
}

func (c *Client) Authenticate(ctx context.Context, credentials Credentials) (*Session, error) {
	if credentials.MFASecret != "" {
		code, err := totp.GenerateCode(credentials.MFASecret, time.Now())
		if err != nil {
			return nil, errors.New(errors.InternalError, "failed to generate MFA code", errors.CatInternal, false, err)
		}
		credentials.MFACode = code
	}

	reqBody := loginRequest{
		Username:      credentials.Email,
		Password:      credentials.Password,
		SupportsMFA:   true,
		TrustedDevice: true,
		TOTP:          credentials.MFACode,
	}
	body, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, loginEndpoint, bytes.NewReader(body))
	if err != nil {
		return nil, errors.New(errors.InternalError, "failed to create login request", errors.CatInternal, false, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Client-Platform", "web")
	req.Header.Set("User-Agent", graphql.UserAgent())

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, errors.New(errors.NetworkUnreachable, "failed to reach Monarch API", errors.CatNetwork, true, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 403 || resp.StatusCode == 401 {
		if credentials.MFACode == "" && credentials.MFASecret == "" {
			return nil, errors.New(errors.AuthMFARequired, "MFA code required", errors.CatAuth, false, nil)
		}
		return nil, errors.New(errors.AuthMFAInvalid, "invalid credentials or MFA code", errors.CatAuth, false, nil)
	}

	if resp.StatusCode != 200 {
		var apiErr struct {
			Detail    string `json:"detail"`
			ErrorCode string `json:"error_code"`
		}
		if err := json.NewDecoder(io.LimitReader(resp.Body, maxLoginResponseSize)).Decode(&apiErr); err == nil && apiErr.Detail != "" {
			return nil, errors.New(errors.APIError, apiErr.Detail, errors.CatAPI, false, nil)
		}
		return nil, errors.New(errors.APIError, fmt.Sprintf("API returned status %d", resp.StatusCode), errors.CatAPI, false, nil)
	}

	var loginResp loginResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxLoginResponseSize)).Decode(&loginResp); err != nil {
		return nil, errors.New(errors.APISchemaChanged, "failed to parse login response", errors.CatAPI, false, err)
	}

	return &Session{
		Email:     credentials.Email,
		Token:     loginResp.Token,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
}
