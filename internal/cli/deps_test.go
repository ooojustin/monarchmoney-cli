package cli

import (
	stderrors "errors"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/thedavidweng/monarchmoney-cli/internal/config"
	clierrors "github.com/thedavidweng/monarchmoney-cli/internal/errors"
	"github.com/thedavidweng/monarchmoney-cli/internal/graphql"
)

func TestWrapError(t *testing.T) {
	t.Run("plain error", func(t *testing.T) {
		plain := stderrors.New("something broke")
		wrapped := wrapError(plain, "operation failed")
		if wrapped.Code != clierrors.APIError || wrapped.Message != "operation failed" || wrapped.Err != plain {
			t.Fatalf("wrapError() = %#v", wrapped)
		}
	})

	t.Run("structured error passthrough", func(t *testing.T) {
		structured := clierrors.NewWithRetryAfter(clierrors.RateLimited, "rate limited", clierrors.CatAPI, true, 3*time.Second, nil)
		if wrapped := wrapError(structured, "ignored message"); wrapped != structured {
			t.Fatalf("wrapError() = %v, want same structured error", wrapped)
		}
		if structured.RetryAfterMS != 3000 {
			t.Fatalf("RetryAfterMS = %d, want 3000", structured.RetryAfterMS)
		}
	})

	t.Run("nil error", func(t *testing.T) {
		wrapped := wrapError(nil, "nil test")
		if wrapped.Code != clierrors.APIError || wrapped.Err != nil {
			t.Fatalf("wrapError(nil) = %#v", wrapped)
		}
	})
}

func TestAppLoadServiceComposesInjectedTransport(t *testing.T) {
	transport := &http.Transport{}
	sessionPath := filepath.Join(t.TempDir(), "session.json")
	saveTestSession(t, sessionPath)
	app := New(Deps{HTTPTransport: transport})
	app.Config = &config.Config{
		APIEndpoint: "https://example.com/graphql",
		SessionPath: sessionPath,
	}
	app.Flags.Timeout = time.Second

	service, err := app.loadService()
	if err != nil {
		t.Fatalf("loadService() error = %v", err)
	}
	client, ok := service.Client.(*graphql.Client)
	if !ok {
		t.Fatalf("service client type = %T", service.Client)
	}
	if client.HTTP.Transport != transport || service.HTTPClient.Transport != transport || service.AttachmentHTTPClient.Transport != transport {
		t.Fatal("loadService did not compose the injected transport across clients")
	}
}
