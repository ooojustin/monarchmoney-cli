package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
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
	EmailOTP  string
}

type Client struct {
	http       *http.Client
	deviceUUID string
}

func NewClient(transport http.RoundTripper, deviceUUID string) *Client {
	if deviceUUID == "" {
		deviceUUID = uuid.NewString()
	}
	return &Client{
		http: &http.Client{
			Timeout:       10 * time.Second,
			Transport:     transport,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
		},
		deviceUUID: deviceUUID,
	}
}

type loginRequest struct {
	Username         string `json:"username"`
	Password         string `json:"password"`
	SupportsMFA      bool   `json:"supports_mfa"`
	SupportsEmailOTP bool   `json:"supports_email_otp"`
	TrustedDevice    bool   `json:"trusted_device"`
	TOTP             string `json:"totp,omitempty"`
	EmailOTP         string `json:"email_otp,omitempty"`
}

type loginResponse struct {
	Token     string `json:"token"`
	Detail    string `json:"detail"`
	ErrorCode string `json:"error_code"`
}

func Authenticate(email, password, mfaCode, mfaSecret string) (*Session, error) {
	return NewClient(nil, "").Authenticate(context.Background(), &Credentials{
		Email:     email,
		Password:  password,
		MFACode:   mfaCode,
		MFASecret: mfaSecret,
	})
}

func (c *Client) Authenticate(ctx context.Context, credentials *Credentials) (*Session, error) {
	if credentials.MFASecret != "" {
		code, err := totp.GenerateCode(credentials.MFASecret, time.Now())
		if err != nil {
			return nil, errors.New(errors.InternalError, "failed to generate MFA code", errors.CatInternal, false, err)
		}
		credentials.MFACode = code
	}

	reqBody := loginRequest{
		Username:         credentials.Email,
		Password:         credentials.Password,
		SupportsMFA:      true,
		SupportsEmailOTP: true,
		TrustedDevice:    true,
		TOTP:             credentials.MFACode,
		EmailOTP:         credentials.EmailOTP,
	}
	body, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, loginEndpoint, bytes.NewReader(body))
	if err != nil {
		return nil, errors.New(errors.InternalError, "failed to create login request", errors.CatInternal, false, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Client-Platform", "web")
	req.Header.Set("Origin", "https://app.monarch.com")
	req.Header.Set("Referer", "https://app.monarch.com/")
	req.Header.Set("device-uuid", c.deviceUUID)
	req.Header.Set("User-Agent", graphql.UserAgent())

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, errors.New(errors.NetworkUnreachable, "failed to reach Monarch API", errors.CatNetwork, true, err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, maxLoginResponseSize+1))
	if err != nil {
		return nil, errors.New(errors.InternalError, "failed to read login response", errors.CatInternal, false, err)
	}
	if int64(len(responseBody)) > maxLoginResponseSize {
		return nil, errors.New(errors.APISchemaChanged, "login response exceeds size limit", errors.CatAPI, false, nil)
	}

	var loginResp loginResponse
	decodeErr := json.Unmarshal(responseBody, &loginResp)
	if resp.StatusCode != http.StatusOK {
		return nil, classifyLoginFailure(resp.StatusCode, credentials, loginResp)
	}
	if decodeErr != nil {
		return nil, errors.New(errors.APISchemaChanged, "failed to parse login response", errors.CatAPI, false, decodeErr)
	}
	if loginResp.Token == "" {
		return nil, errors.New(errors.APISchemaChanged, "login response did not include a token", errors.CatAPI, false, nil)
	}

	return &Session{
		Email:     credentials.Email,
		Token:     loginResp.Token,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
}

func classifyLoginFailure(status int, credentials *Credentials, response loginResponse) error {
	message := strings.TrimSpace(response.Detail)
	switch response.ErrorCode {
	case "EMAIL_OTP_REQUIRED":
		if credentials.EmailOTP != "" {
			return errors.New(errors.AuthEmailOTPInvalid, firstMessage(message, "invalid or expired email code"), errors.CatAuth, false, nil)
		}
		return errors.New(errors.AuthEmailOTPRequired, "email verification code required", errors.CatAuth, false, nil)
	case "MFA_REQUIRED", "TOTP_REQUIRED":
		if credentials.MFACode != "" || credentials.MFASecret != "" {
			return errors.New(errors.AuthMFAInvalid, firstMessage(message, "invalid MFA code"), errors.CatAuth, false, nil)
		}
		return errors.New(errors.AuthMFARequired, "MFA code required", errors.CatAuth, false, nil)
	case "INVALID_CREDENTIALS":
		return errors.New(errors.AuthLoginFailed, "invalid email or password", errors.CatAuth, false, nil)
	}
	if message == "" && response.ErrorCode != "" {
		message = response.ErrorCode
	}

	if status == http.StatusUnauthorized || status == http.StatusForbidden || status == http.StatusNotFound {
		return errors.New(errors.AuthLoginFailed, firstMessage(message, "authentication rejected"), errors.CatAuth, false, nil)
	}

	return errors.New(errors.APIError, firstMessage(message, fmt.Sprintf("API returned status %d", status)), errors.CatAPI, false, nil)
}

func firstMessage(message, fallback string) string {
	if message != "" {
		return message
	}
	return fallback
}
