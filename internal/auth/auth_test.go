package auth

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	clierrors "github.com/thedavidweng/monarchmoney-cli/internal/errors"
	"github.com/thedavidweng/monarchmoney-cli/internal/testutil"
)

func TestStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(filepath.Join(dir, "session", "session.json"))
	now := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	sess := &Session{
		Profile:   "default",
		Email:     "a@example.com",
		CreatedAt: now,
		UpdatedAt: now,
		Token:     "token-123",
	}

	if err := store.Save(sess); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	assertFilePerms(t, store.Path, 0o600)

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.Token != sess.Token {
		t.Errorf("Token = %q, want %q", loaded.Token, sess.Token)
	}
	if loaded.Profile != sess.Profile {
		t.Errorf("Profile = %q, want %q", loaded.Profile, sess.Profile)
	}
	if loaded.Email != sess.Email {
		t.Errorf("Email = %q, want %q", loaded.Email, sess.Email)
	}
	if !loaded.CreatedAt.Equal(sess.CreatedAt) {
		t.Errorf("CreatedAt = %v, want %v", loaded.CreatedAt, sess.CreatedAt)
	}
	if !loaded.UpdatedAt.Equal(sess.UpdatedAt) {
		t.Errorf("UpdatedAt = %v, want %v", loaded.UpdatedAt, sess.UpdatedAt)
	}

	assertNoForbiddenFields(t, store.Path)

	if err := store.Delete(); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := os.Stat(store.Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Stat() error = %v, want os.ErrNotExist", err)
	}
}

func assertFilePerms(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	if runtime.GOOS == "windows" {
		return
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Errorf("perms = %v, want %v", got, want)
	}
}

func assertNoForbiddenFields(t *testing.T, path string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	for _, forbidden := range []string{"user_id", "household_id", "expires_at", "cookies"} {
		if strings.Contains(string(raw), forbidden) {
			t.Errorf("session file contains forbidden field %q", forbidden)
		}
	}
}

func TestStoreSaveReplacesExistingSession(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(filepath.Join(dir, "session.json"))

	first := &Session{Profile: "default", Email: "old@example.com", Token: "old-token"}
	if err := store.Save(first); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	second := &Session{Profile: "default", Email: "new@example.com", Token: "new-token"}
	if err := store.Save(second); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.Email != second.Email || loaded.Token != second.Token {
		t.Fatalf("Load() = %#v, want %#v", loaded, second)
	}
}

func TestStoreSaveRestrictsExistingSessionPermissions(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "session.json"))
	if err := os.WriteFile(store.Path, []byte("{}"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := store.Save(&Session{Profile: "default"}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	assertFilePerms(t, store.Path, 0o600)
}

func TestStoreDeleteMissing(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "missing.json"))
	if err := store.Delete(); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Delete() error = %v, want os.ErrNotExist", err)
	}
}

func TestStoreSaveReturnsMkdirError(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocked")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	store := NewStore(filepath.Join(blocker, "session.json"))
	if err := store.Save(&Session{Profile: "default"}); err == nil {
		t.Fatal("Save() error = nil, want failure")
	}
}

func TestStoreLoadReturnsDecodeError(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "session.json"))
	if err := os.WriteFile(store.Path, []byte("not-json"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := store.Load(); err == nil {
		t.Fatal("Load() error = nil, want failure")
	}
}

func TestStoreLoadReturnsMissingFileError(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "missing.json"))
	if _, err := store.Load(); err == nil {
		t.Fatal("Load() error = nil, want failure")
	}
}

func TestStoreSaveReturnsWriteError(t *testing.T) {
	store := NewStore(t.TempDir())
	if err := store.Save(&Session{Profile: "default"}); err == nil {
		t.Fatal("Save() error = nil, want failure")
	}
}

func TestAuthenticate(t *testing.T) {
	testAuthenticateInputValidation(t)
	testAuthenticateFailureResponses(t)
	testAuthenticateSuccessResponses(t)
}

func mustErrContains(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %v, want containing %q", err, want)
	}
}

func mustErrorCode(t *testing.T, err error, want clierrors.Code) {
	t.Helper()
	cliErr, ok := err.(*clierrors.Error)
	if !ok || cliErr.Code != want {
		t.Fatalf("error = %#v, want code %s", err, want)
	}
}

func testAuthenticateInputValidation(t *testing.T) {
	t.Helper()

	t.Run("invalid mfa secret", func(t *testing.T) {
		client := NewClient(testutil.RoundTripFunc(func(*http.Request) (*http.Response, error) {
			t.Fatal("invalid MFA secret should fail before the request")
			return nil, nil
		}), "")
		_, err := client.Authenticate(context.Background(), &Credentials{Email: "a@example.com", Password: "password", MFASecret: "not-base32"})
		mustErrContains(t, err, "failed to generate MFA code")
	})
}

func testAuthenticateFailureResponses(t *testing.T) {
	t.Helper()

	t.Run("network unreachable", func(t *testing.T) {
		client := NewClient(testutil.RoundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("network down")
		}), "")
		_, err := client.Authenticate(context.Background(), &Credentials{Email: "a@example.com", Password: "password"})
		mustErrContains(t, err, "failed to reach Monarch API")
	})

	t.Run("email OTP required", func(t *testing.T) {
		client := NewClient(testutil.RoundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 403, Body: io.NopCloser(bytes.NewBufferString(`{"detail":"code sent","error_code":"EMAIL_OTP_REQUIRED"}`))}, nil
		}), "")
		_, err := client.Authenticate(context.Background(), &Credentials{Email: "a@example.com", Password: "password"})
		mustErrorCode(t, err, clierrors.AuthEmailOTPRequired)
		mustErrContains(t, err, "email verification code required")
	})

	t.Run("email OTP invalid", func(t *testing.T) {
		client := NewClient(testutil.RoundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 403, Body: io.NopCloser(bytes.NewBufferString(`{"detail":"code expired","error_code":"EMAIL_OTP_REQUIRED"}`))}, nil
		}), "")
		_, err := client.Authenticate(context.Background(), &Credentials{Email: "a@example.com", Password: "password", EmailOTP: "123456"})
		mustErrorCode(t, err, clierrors.AuthEmailOTPInvalid)
		mustErrContains(t, err, "code expired")
	})

	t.Run("mfa required", func(t *testing.T) {
		client := NewClient(testutil.RoundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 403, Body: io.NopCloser(bytes.NewBufferString(`{"error_code":"TOTP_REQUIRED"}`))}, nil
		}), "")
		_, err := client.Authenticate(context.Background(), &Credentials{Email: "a@example.com", Password: "password"})
		mustErrorCode(t, err, clierrors.AuthMFARequired)
	})

	t.Run("mfa invalid", func(t *testing.T) {
		client := NewClient(testutil.RoundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 403, Body: io.NopCloser(bytes.NewBufferString(`{"error_code":"TOTP_REQUIRED"}`))}, nil
		}), "")
		_, err := client.Authenticate(context.Background(), &Credentials{Email: "a@example.com", Password: "password", MFACode: "123456"})
		mustErrorCode(t, err, clierrors.AuthMFAInvalid)
	})

	t.Run("unknown denial with mfa", func(t *testing.T) {
		client := NewClient(testutil.RoundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 401, Body: io.NopCloser(bytes.NewBufferString(""))}, nil
		}), "")
		_, err := client.Authenticate(context.Background(), &Credentials{Email: "a@example.com", Password: "password", MFACode: "123456"})
		mustErrorCode(t, err, clierrors.AuthLoginFailed)
	})

	t.Run("invalid credentials", func(t *testing.T) {
		client := NewClient(testutil.RoundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 401, Body: io.NopCloser(bytes.NewBufferString(`{"detail":"invalid login"}`))}, nil
		}), "")
		_, err := client.Authenticate(context.Background(), &Credentials{Email: "a@example.com", Password: "password"})
		mustErrorCode(t, err, clierrors.AuthLoginFailed)
		mustErrContains(t, err, "invalid login")
	})

	t.Run("invalid credentials code", func(t *testing.T) {
		client := NewClient(testutil.RoundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 404, Body: io.NopCloser(bytes.NewBufferString(`{"error_code":"INVALID_CREDENTIALS"}`))}, nil
		}), "")
		_, err := client.Authenticate(context.Background(), &Credentials{Email: "a@example.com", Password: "password"})
		mustErrorCode(t, err, clierrors.AuthLoginFailed)
		mustErrContains(t, err, "invalid email or password")
	})

	t.Run("api error", func(t *testing.T) {
		client := NewClient(testutil.RoundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 500, Body: io.NopCloser(bytes.NewBufferString(""))}, nil
		}), "")
		_, err := client.Authenticate(context.Background(), &Credentials{Email: "a@example.com", Password: "password", MFACode: "123456"})
		mustErrContains(t, err, "API returned status 500")
	})

	t.Run("schema changed", func(t *testing.T) {
		client := NewClient(testutil.RoundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewBufferString("not-json"))}, nil
		}), "")
		_, err := client.Authenticate(context.Background(), &Credentials{Email: "a@example.com", Password: "password", MFACode: "123456"})
		mustErrContains(t, err, "failed to parse login response")
	})

	t.Run("missing token", func(t *testing.T) {
		client := NewClient(testutil.RoundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewBufferString(`{}`))}, nil
		}), "")
		_, err := client.Authenticate(context.Background(), &Credentials{Email: "a@example.com", Password: "password"})
		mustErrContains(t, err, "did not include a token")
	})
}

func testAuthenticateSuccessResponses(t *testing.T) {
	t.Helper()

	t.Run("success", func(t *testing.T) {
		const deviceUUID = "7ebee331-91b5-4a70-8c6f-b28818f1a8cf"
		client := NewClient(testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
			body, _ := io.ReadAll(req.Body)
			for _, want := range []string{`"username":"a@example.com"`, `"supports_email_otp":true`} {
				if !strings.Contains(string(body), want) {
					t.Errorf("request body = %q, want %s", string(body), want)
				}
			}
			if got := req.Header.Get("device-uuid"); got != deviceUUID {
				t.Errorf("device-uuid = %q, want %q", got, deviceUUID)
			}
			if req.Header.Get("Origin") == "" || req.Header.Get("Referer") == "" || req.Header.Get("Accept") != "application/json" {
				t.Errorf("login headers = %#v", req.Header)
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewBufferString(`{"token":"token-123"}`))}, nil
		}), deviceUUID)
		sess, err := client.Authenticate(context.Background(), &Credentials{Email: "a@example.com", Password: "password", MFACode: "123456"})
		if err != nil {
			t.Fatalf("Authenticate() error = %v", err)
		}
		if sess == nil {
			t.Fatal("Authenticate() session = nil")
		}
		if sess.Email != "a@example.com" {
			t.Errorf("Email = %q, want %q", sess.Email, "a@example.com")
		}
		if sess.Token != "token-123" {
			t.Errorf("Token = %q, want %q", sess.Token, "token-123")
		}
		if sess.CreatedAt.IsZero() || sess.UpdatedAt.IsZero() {
			t.Error("CreatedAt/UpdatedAt should not be zero")
		}
	})

	t.Run("success with mfa secret", func(t *testing.T) {
		client := NewClient(testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
			body, _ := io.ReadAll(req.Body)
			if !strings.Contains(string(body), `"totp"`) {
				t.Errorf("request body = %q, want totp", string(body))
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewBufferString(`{"token":"token-456"}`))}, nil
		}), "")
		sess, err := client.Authenticate(context.Background(), &Credentials{Email: "a@example.com", Password: "password", MFASecret: "JBSWY3DPEHPK3PXP"})
		if err != nil {
			t.Fatalf("Authenticate() error = %v", err)
		}
		if sess == nil {
			t.Fatal("Authenticate() session = nil")
		}
		if sess.Token != "token-456" {
			t.Errorf("Token = %q, want %q", sess.Token, "token-456")
		}
	})
}
