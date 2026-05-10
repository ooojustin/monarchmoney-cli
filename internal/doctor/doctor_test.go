package doctor

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/thedavidweng/monarchmoney-cli/internal/auth"
)

type doctorRoundTripper func(*http.Request) (*http.Response, error)

func (f doctorRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestCheckWithoutLocalState(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	res := Check(context.Background(), false, Options{
		ConfigPath:  filepath.Join(dir, "config.yaml"),
		SessionPath: filepath.Join(dir, "session.json"),
	})

	if res.Version == "" || res.OS == "" || res.Arch == "" {
		t.Fatalf("Check() returned incomplete identity: %#v", res)
	}
	if res.Config.Exists || res.Session.Exists || res.Session.Authenticated || res.Network.APIReachable {
		t.Fatalf("Check() returned unexpected state: %#v", res)
	}
}

func TestCheckWithSessionAndConnectivity(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	sessionPath := filepath.Join(dir, "session.json")

	if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(configPath, []byte("profile: default\n"), 0600); err != nil {
		t.Fatalf("WriteFile() config error = %v", err)
	}

	sess := &auth.Session{Profile: "default", Token: "token-123", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := auth.NewStore(sessionPath).Save(sess); err != nil {
		t.Fatalf("Save() session error = %v", err)
	}
	if err := os.Chmod(sessionPath, 0644); err != nil {
		t.Fatalf("Chmod() error = %v", err)
	}

	transport := doctorRoundTripper(func(req *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(req.Body)
		if !bytes.Contains(body, []byte("GetIdentity")) {
			t.Fatalf("unexpected GraphQL request body: %s", string(body))
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewBufferString(`{"data":{"identity":{"id":"me"}}}`))}, nil
	})

	res := Check(context.Background(), true, Options{
		ConfigPath:    configPath,
		SessionPath:   sessionPath,
		HTTPTransport: transport,
	})
	if !res.Config.Exists || !res.Session.Exists || !res.Session.Authenticated || res.Session.PermissionOK || !res.Network.APIReachable {
		t.Fatalf("Check() returned unexpected state: %#v", res)
	}
}
