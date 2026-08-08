package cli

import (
	"bytes"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thedavidweng/monarchmoney-cli/internal/auth"
	"github.com/thedavidweng/monarchmoney-cli/internal/config"
	clierrors "github.com/thedavidweng/monarchmoney-cli/internal/errors"
	"github.com/thedavidweng/monarchmoney-cli/internal/testutil"
)

func newTestAuthApp(t *testing.T, sessionPath string, transport http.RoundTripper) (app *App, out, errOut *bytes.Buffer, exitCode *int) {
	t.Helper()

	out = &bytes.Buffer{}
	errOut = &bytes.Buffer{}
	exitCode = new(int)
	app = New(Deps{
		LoadConfig: func(string) (*config.Config, error) {
			cfg := config.Default()
			cfg.APIEndpoint = "https://example.invalid/graphql"
			cfg.SessionPath = sessionPath
			cfg.Timeout = time.Second
			return cfg, nil
		},
		Getenv:        func(string) string { return "" },
		NewRequestID:  func() string { return "request-id" },
		HTTPTransport: transport,
		Stdout:        out,
		Stderr:        errOut,
		Stdin:         strings.NewReader(""),
		ReadPassword: func() ([]byte, error) {
			return nil, stderrors.New("unexpected password prompt")
		},
		Exit: func(code int) { *exitCode = code },
	})
	return app, out, errOut, exitCode
}

type authGraphQLRequest struct {
	OperationName string `json:"operationName"`
}

func TestAppRootRegistersAuth(t *testing.T) {
	app, _ := newTestApp(t)
	for _, path := range [][]string{
		{"auth", "login"},
		{"auth", "status"},
		{"auth", "logout"},
		{"auth", "session", "path"},
	} {
		cmd, _, err := app.Root.Find(path)
		if err != nil || cmd == nil || cmd.Name() != path[len(path)-1] {
			t.Fatalf("Find(%v) = %#v, %v", path, cmd, err)
		}
	}
	authCommand, _, _ := app.Root.Find([]string{"auth"})
	if authCommand.GroupID != "utility" {
		t.Fatalf("auth GroupID = %q, want utility", authCommand.GroupID)
	}
	loginCommand, _, _ := app.Root.Find([]string{"auth", "login"})
	for _, flag := range []string{"email", "password", "mfa-code", "mfa-secret", "email-otp"} {
		if loginCommand.Flags().Lookup(flag) == nil {
			t.Fatalf("auth login missing --%s flag", flag)
		}
	}
}

func TestAppAuthLoginUsesFlagsWithoutPrompt(t *testing.T) {
	sessionPath := filepath.Join(t.TempDir(), "session.json")
	transport := testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "https://api.monarch.com/auth/login/" || req.Method != http.MethodPost {
			return nil, fmt.Errorf("login request = %s %s", req.Method, req.URL)
		}
		var body map[string]any
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			return nil, err
		}
		if body["username"] != "a@example.com" || body["password"] != "secret" {
			return nil, fmt.Errorf("login body = %#v", body)
		}
		return testutil.JSONResponse(`{"token":"token-123"}`), nil
	})
	app, out, errOut, exitCode := newTestAuthApp(t, sessionPath, transport)

	app.Root.SetArgs([]string{"auth", "login", "--email", "a@example.com", "--password", "secret"})
	if err := app.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if *exitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", *exitCode, out.String())
	}
	if errOut.Len() != 0 {
		t.Fatalf("stderr = %q, want no prompts", errOut.String())
	}
	if !strings.Contains(out.String(), "Successfully logged in as a@example.com.") || !strings.Contains(out.String(), sessionPath) {
		t.Fatalf("output = %q", out.String())
	}
	sess, err := auth.NewStore(sessionPath).Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if sess.Email != "a@example.com" || sess.Token != "token-123" || sess.Profile != "default" {
		t.Fatalf("session = %#v", sess)
	}
}

func TestAppAuthLoginJSONUsesEnvironmentAndRequestID(t *testing.T) {
	sessionPath := filepath.Join(t.TempDir(), "session.json")
	transport := testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		var body map[string]any
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			return nil, err
		}
		if body["username"] != "env@example.com" || body["password"] != "env-secret" {
			return nil, fmt.Errorf("login body = %#v", body)
		}
		return testutil.JSONResponse(`{"token":"token-123"}`), nil
	})
	app, out, errOut, exitCode := newTestAuthApp(t, sessionPath, transport)
	app.Deps.Getenv = func(key string) string {
		return map[string]string{
			"MONARCH_EMAIL":    "env@example.com",
			"MONARCH_PASSWORD": "env-secret",
		}[key]
	}

	app.Root.SetArgs([]string{"--json", "auth", "login"})
	if err := app.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if *exitCode != 0 || errOut.Len() != 0 {
		t.Fatalf("exitCode=%d stderr=%q", *exitCode, errOut.String())
	}
	var env struct {
		Data struct {
			Status      string `json:"status"`
			Email       string `json:"email"`
			SessionPath string `json:"session_path"`
		} `json:"data"`
		Meta struct {
			Command   string `json:"command"`
			RequestID string `json:"request_id"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &env); err != nil {
		t.Fatalf("json.Unmarshal() error = %v; output=%q", err, out.String())
	}
	if env.Data.Status != "logged in" || env.Data.Email != "env@example.com" || env.Data.SessionPath != sessionPath {
		t.Fatalf("data = %#v", env.Data)
	}
	if env.Meta.Command != "auth.login" || env.Meta.RequestID != "request-id" {
		t.Fatalf("metadata = %#v", env.Meta)
	}
}

func TestAppAuthSessionPathJSON(t *testing.T) {
	sessionPath := filepath.Join(t.TempDir(), "session.json")
	app, out, errOut, exitCode := newTestAuthApp(t, sessionPath, nil)
	app.Root.SetArgs([]string{"--json", "auth", "session", "path"})

	if err := app.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if *exitCode != 0 || errOut.Len() != 0 {
		t.Fatalf("exitCode=%d stderr=%q", *exitCode, errOut.String())
	}
	var env struct {
		Data struct {
			Path string `json:"path"`
		} `json:"data"`
		Meta struct {
			Command string `json:"command"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &env); err != nil {
		t.Fatalf("json.Unmarshal() error = %v; output=%q", err, out.String())
	}
	if env.Data.Path != sessionPath || env.Meta.Command != "auth.session.path" {
		t.Fatalf("envelope = %#v", env)
	}
}

func TestAppAuthLoginPromptsForMFA(t *testing.T) {
	sessionPath := filepath.Join(t.TempDir(), "session.json")
	requestCount := 0
	transport := testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		var body map[string]any
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			return nil, err
		}
		if requestCount == 1 {
			return &http.Response{StatusCode: http.StatusForbidden, Body: io.NopCloser(strings.NewReader(`{"error_code":"TOTP_REQUIRED"}`))}, nil
		}
		if body["totp"] != "123456" {
			return nil, fmt.Errorf("MFA login body = %#v", body)
		}
		return testutil.JSONResponse(`{"token":"token-123"}`), nil
	})
	app, _, errOut, exitCode := newTestAuthApp(t, sessionPath, transport)
	app.Deps.Stdin = strings.NewReader("123456\n")
	app.Root.SetIn(app.Deps.Stdin)

	app.Root.SetArgs([]string{"auth", "login", "--email", "a@example.com", "--password", "secret"})
	if err := app.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if *exitCode != 0 || requestCount != 2 {
		t.Fatalf("exitCode=%d requests=%d", *exitCode, requestCount)
	}
	if !strings.Contains(errOut.String(), "MFA Code:") {
		t.Fatalf("stderr = %q, want MFA prompt", errOut.String())
	}
}

func TestAppAuthLoginPromptsForEmailOTP(t *testing.T) {
	sessionPath := filepath.Join(t.TempDir(), "session.json")
	requestCount := 0
	deviceUUID := ""
	transport := testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		if requestCount == 1 {
			deviceUUID = req.Header.Get("device-uuid")
			return &http.Response{StatusCode: http.StatusForbidden, Body: io.NopCloser(strings.NewReader(`{"detail":"code sent","error_code":"EMAIL_OTP_REQUIRED"}`))}, nil
		}
		var body map[string]any
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			return nil, err
		}
		if body["email_otp"] != "654321" {
			return nil, fmt.Errorf("email OTP login body = %#v", body)
		}
		if got := req.Header.Get("device-uuid"); got == "" || got != deviceUUID {
			return nil, fmt.Errorf("device-uuid = %q, want %q", got, deviceUUID)
		}
		return testutil.JSONResponse(`{"token":"token-123"}`), nil
	})
	app, _, errOut, exitCode := newTestAuthApp(t, sessionPath, transport)
	app.Deps.Stdin = strings.NewReader("654321\n")
	app.Root.SetIn(app.Deps.Stdin)

	app.Root.SetArgs([]string{"auth", "login", "--email", "a@example.com", "--password", "secret"})
	if err := app.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if *exitCode != 0 || requestCount != 2 {
		t.Fatalf("exitCode=%d requests=%d", *exitCode, requestCount)
	}
	if !strings.Contains(errOut.String(), "Email Code:") {
		t.Fatalf("stderr = %q, want email code prompt", errOut.String())
	}
	_, err := auth.NewStore(sessionPath).Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	storedDeviceUUID, err := auth.LoadDeviceUUID(sessionPath)
	if err != nil || storedDeviceUUID != deviceUUID {
		t.Fatalf("LoadDeviceUUID() = %q, %v; want %q", storedDeviceUUID, err, deviceUUID)
	}
}

func TestAppAuthLoginRejectsEmptyChallengeInput(t *testing.T) {
	for _, test := range []struct {
		name      string
		response  string
		input     string
		wantError string
	}{
		{"blank email OTP", `{"error_code":"EMAIL_OTP_REQUIRED"}`, "\n", "email code is required"},
		{"email OTP EOF", `{"error_code":"EMAIL_OTP_REQUIRED"}`, "", "email code is required"},
		{"blank MFA", `{"error_code":"TOTP_REQUIRED"}`, "   \n", "MFA code is required"},
		{"MFA EOF", `{"error_code":"TOTP_REQUIRED"}`, "", "MFA code is required"},
	} {
		t.Run(test.name, func(t *testing.T) {
			requestCount := 0
			transport := testutil.RoundTripFunc(func(*http.Request) (*http.Response, error) {
				requestCount++
				return &http.Response{StatusCode: http.StatusForbidden, Body: io.NopCloser(strings.NewReader(test.response))}, nil
			})
			app, _, errOut, exitCode := newTestAuthApp(t, filepath.Join(t.TempDir(), "session.json"), transport)
			app.Deps.Stdin = strings.NewReader(test.input)
			app.Root.SetIn(app.Deps.Stdin)
			app.Root.SetArgs([]string{"auth", "login", "--email", "a@example.com", "--password", "secret"})

			if err := app.Execute(); err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if *exitCode != 2 || requestCount != 1 || !strings.Contains(errOut.String(), test.wantError) {
				t.Fatalf("exitCode=%d requests=%d stderr=%q", *exitCode, requestCount, errOut.String())
			}
		})
	}
}

func TestAppAuthLoginJSONReturnsEmailOTPChallenge(t *testing.T) {
	sessionPath := filepath.Join(t.TempDir(), "session.json")
	requestCount := 0
	deviceUUID := ""
	transport := testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		deviceUUID = req.Header.Get("device-uuid")
		return &http.Response{StatusCode: http.StatusForbidden, Body: io.NopCloser(strings.NewReader(`{"error_code":"EMAIL_OTP_REQUIRED"}`))}, nil
	})
	app, out, errOut, exitCode := newTestAuthApp(t, sessionPath, transport)
	app.Root.SetArgs([]string{"--json", "auth", "login", "--email", "a@example.com", "--password", "secret"})

	if err := app.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if *exitCode != 3 || requestCount != 1 || errOut.Len() != 0 {
		t.Fatalf("exitCode=%d requests=%d stderr=%q", *exitCode, requestCount, errOut.String())
	}
	if !strings.Contains(out.String(), `"code":"AUTH_EMAIL_OTP_REQUIRED"`) {
		t.Fatalf("stdout = %q", out.String())
	}
	storedDeviceUUID, err := auth.LoadDeviceUUID(sessionPath)
	if err != nil || storedDeviceUUID != deviceUUID {
		t.Fatalf("LoadDeviceUUID() = %q, %v; want %q", storedDeviceUUID, err, deviceUUID)
	}

	transport = testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if got := req.Header.Get("device-uuid"); got != deviceUUID {
			return nil, fmt.Errorf("device-uuid = %q, want %q", got, deviceUUID)
		}
		var body map[string]any
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			return nil, err
		}
		if body["email_otp"] != "654321" {
			return nil, fmt.Errorf("email OTP login body = %#v", body)
		}
		return testutil.JSONResponse(`{"token":"token-123"}`), nil
	})
	app, out, errOut, exitCode = newTestAuthApp(t, sessionPath, transport)
	app.Deps.Getenv = func(key string) string {
		return map[string]string{
			"MONARCH_EMAIL":     "a@example.com",
			"MONARCH_PASSWORD":  "secret",
			"MONARCH_EMAIL_OTP": "654321",
		}[key]
	}
	app.Root.SetArgs([]string{"--json", "auth", "login"})
	if err := app.Execute(); err != nil {
		t.Fatalf("continuation Execute() error = %v", err)
	}
	if *exitCode != 0 || errOut.Len() != 0 || !strings.Contains(out.String(), `"status":"logged in"`) {
		t.Fatalf("exitCode=%d stderr=%q stdout=%q", *exitCode, errOut.String(), out.String())
	}
}

func TestAppAuthStatusUsesConfiguredSessionAndIdentity(t *testing.T) {
	sessionPath := filepath.Join(t.TempDir(), "session.json")
	if err := auth.NewStore(sessionPath).Save(&auth.Session{Profile: "default", Token: "token-123", CreatedAt: time.Now(), UpdatedAt: time.Now()}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	transport := testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Header.Get("Authorization") != "Token token-123" {
			return nil, fmt.Errorf("Authorization = %q", req.Header.Get("Authorization"))
		}
		var gqlReq authGraphQLRequest
		if err := json.NewDecoder(req.Body).Decode(&gqlReq); err != nil {
			return nil, err
		}
		if gqlReq.OperationName != "GetIdentity" {
			return nil, fmt.Errorf("operation = %q", gqlReq.OperationName)
		}
		return testutil.JSONResponse(`{"data":{"me":{"email":"fallback@example.com"}}}`), nil
	})
	app, out, _, exitCode := newTestAuthApp(t, sessionPath, transport)
	app.Root.SetArgs([]string{"--json", "auth", "status"})
	if err := app.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if *exitCode != 0 {
		t.Fatalf("exitCode = %d; output=%q", *exitCode, out.String())
	}
	if got := out.String(); !strings.Contains(got, `"email":"fallback@example.com"`) || !strings.Contains(got, `"session_valid":true`) || !strings.Contains(got, `"request_id":"request-id"`) {
		t.Fatalf("output = %q", got)
	}
}

func TestAppAuthStatusErrors(t *testing.T) {
	t.Run("missing session", func(t *testing.T) {
		sessionPath := filepath.Join(t.TempDir(), "missing.json")
		app, out, _, exitCode := newTestAuthApp(t, sessionPath, testutil.RoundTripFunc(func(*http.Request) (*http.Response, error) {
			t.Fatal("missing session should fail before a request")
			return nil, nil
		}))
		app.Root.SetArgs([]string{"--json", "auth", "status"})
		if err := app.Execute(); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if *exitCode != 3 || !strings.Contains(out.String(), string(clierrors.AuthRequired)) {
			t.Fatalf("exitCode=%d output=%q", *exitCode, out.String())
		}
	})

	t.Run("expired session", func(t *testing.T) {
		sessionPath := filepath.Join(t.TempDir(), "session.json")
		if err := auth.NewStore(sessionPath).Save(&auth.Session{Email: "a@example.com", Token: "expired"}); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
		app, out, _, exitCode := newTestAuthApp(t, sessionPath, testutil.RoundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusUnauthorized, Body: io.NopCloser(strings.NewReader(""))}, nil
		}))
		app.Root.SetArgs([]string{"--json", "auth", "status"})
		if err := app.Execute(); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if *exitCode != 3 || !strings.Contains(out.String(), string(clierrors.AuthSessionExpired)) || !strings.Contains(out.String(), "a@example.com") || !strings.Contains(out.String(), sessionPath) {
			t.Fatalf("exitCode=%d output=%q", *exitCode, out.String())
		}
	})
}

func TestAppAuthLocalCommandsUseConfiguredPathDespiteConfigError(t *testing.T) {
	sessionPath := filepath.Join(t.TempDir(), "session.json")
	if err := auth.NewStore(sessionPath).Save(&auth.Session{Token: "token"}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	deviceUUID, err := auth.LoadOrCreateDeviceUUID(sessionPath)
	if err != nil {
		t.Fatalf("LoadOrCreateDeviceUUID() error = %v", err)
	}
	app, out, _, _ := newTestAuthApp(t, sessionPath, nil)
	app.Deps.LoadConfig = func(string) (*config.Config, error) {
		cfg := config.Default()
		cfg.SessionPath = sessionPath
		return cfg, stderrors.New("malformed config")
	}
	app.Root.SetArgs([]string{"auth", "session", "path"})
	if err := app.Execute(); err != nil {
		t.Fatalf("Execute(path) error = %v", err)
	}
	if strings.TrimSpace(out.String()) != sessionPath {
		t.Fatalf("path output = %q, want %q", out.String(), sessionPath)
	}

	app, out, _, exitCode := newTestAuthApp(t, sessionPath, nil)
	app.Deps.LoadConfig = func(string) (*config.Config, error) {
		cfg := config.Default()
		cfg.SessionPath = sessionPath
		return cfg, stderrors.New("malformed config")
	}
	app.Root.SetArgs([]string{"--json", "auth", "logout"})
	if err := app.Execute(); err != nil {
		t.Fatalf("Execute(logout) error = %v", err)
	}
	if *exitCode != 0 || !strings.Contains(out.String(), `"status":"logged out"`) {
		t.Fatalf("exitCode=%d output=%q", *exitCode, out.String())
	}
	if _, err := os.Stat(sessionPath); !os.IsNotExist(err) {
		t.Fatalf("session stat error = %v, want missing", err)
	}
	if got, err := auth.LoadDeviceUUID(sessionPath); err != nil || got != deviceUUID {
		t.Fatalf("LoadDeviceUUID() = %q, %v; want %q", got, err, deviceUUID)
	}
}
