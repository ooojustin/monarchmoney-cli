//go:build !windows

package doctor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/thedavidweng/monarchmoney-cli/internal/auth"
	"github.com/thedavidweng/monarchmoney-cli/internal/testutil"
)

func TestCheckWithoutLocalState(t *testing.T) {
	dir := t.TempDir()
	res := Check(context.Background(), &Options{
		ConfigPath:  filepath.Join(dir, "config.yaml"),
		SessionPath: filepath.Join(dir, "session.json"),
	})

	if res.Version == "" || res.OS == "" || res.Arch == "" {
		t.Fatalf("Check() returned incomplete identity: %#v", res)
	}
	if res.Config.Exists || !res.Config.Valid || res.Session.Exists || res.Session.Authenticated || res.Network.APIReachable {
		t.Fatalf("Check() returned unexpected state: %#v", res)
	}
}

func TestCheckWithSessionAndConnectivity(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	sessionPath := filepath.Join(dir, "session.json")
	apiEndpoint := "https://example.invalid/graphql"

	if err := os.WriteFile(configPath, []byte("profile: default\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() config error = %v", err)
	}

	sess := &auth.Session{Profile: "default", Token: "token-123", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := auth.NewStore(sessionPath).Save(sess); err != nil {
		t.Fatalf("Save() session error = %v", err)
	}
	deviceUUID, err := auth.LoadOrCreateDeviceUUID(sessionPath)
	if err != nil {
		t.Fatalf("LoadOrCreateDeviceUUID() error = %v", err)
	}
	if err := os.Chmod(sessionPath, 0o644); err != nil {
		t.Fatalf("Chmod() error = %v", err)
	}

	transport := testutil.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != apiEndpoint {
			t.Fatalf("request URL = %q, want %q", req.URL.String(), apiEndpoint)
		}
		if got := req.Header.Get("device-uuid"); got != deviceUUID {
			t.Fatalf("device-uuid = %q, want %q", got, deviceUUID)
		}
		body, _ := io.ReadAll(req.Body)
		if !bytes.Contains(body, []byte("GetIdentity")) {
			t.Fatalf("unexpected GraphQL request body: %s", string(body))
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewBufferString(`{"data":{"identity":{"id":"me"}}}`))}, nil
	})

	res := Check(context.Background(), &Options{
		Connect:       true,
		ConfigPath:    configPath,
		SessionPath:   sessionPath,
		APIEndpoint:   apiEndpoint,
		Timeout:       time.Second,
		HTTPTransport: transport,
	})
	if !res.Config.Exists || !res.Config.Valid || !res.Session.Exists || !res.Session.Authenticated || res.Session.PermissionOK || !res.Session.DeviceIDValid || !res.Network.APIReachable {
		t.Fatalf("Check() returned unexpected state: %#v", res)
	}
}

func TestCheckRejectsInvalidDeviceIdentity(t *testing.T) {
	sessionPath := filepath.Join(t.TempDir(), "session.json")
	if err := auth.NewStore(sessionPath).Save(&auth.Session{Token: "token-123"}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := os.WriteFile(auth.DevicePath(sessionPath), []byte("invalid\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	res := Check(context.Background(), &Options{
		Connect:     true,
		ConfigPath:  filepath.Join(t.TempDir(), "config.yaml"),
		SessionPath: sessionPath,
		APIEndpoint: "https://example.invalid/graphql",
		Timeout:     time.Second,
		HTTPTransport: testutil.RoundTripFunc(func(*http.Request) (*http.Response, error) {
			t.Fatal("invalid device identity must prevent connectivity request")
			return nil, nil
		}),
	})
	if !res.Session.Authenticated || res.Session.DeviceIDValid || res.Network.APIReachable {
		t.Fatalf("Check() returned unexpected state: %#v", res)
	}
	data, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if bytes.Count(data, []byte(`"device_id_valid"`)) != 1 {
		t.Fatalf("device_id_valid must appear only in session report: %s", data)
	}
}

func TestCheckReportsMalformedConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("profile: ["), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	res := Check(context.Background(), &Options{
		ConfigPath:  configPath,
		ConfigError: errors.New("parse config"),
		SessionPath: filepath.Join(dir, "session.json"),
	})

	if !res.Config.Exists || res.Config.Valid {
		t.Fatalf("Config = %#v, want existing and invalid", res.Config)
	}
	if res.Config.Path != configPath {
		t.Fatalf("Config.Path = %q, want %q", res.Config.Path, configPath)
	}
	data, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if bytes.Contains(data, []byte("parse config")) || !bytes.Contains(data, []byte(`"valid":false`)) {
		t.Fatalf("malformed config JSON = %s", data)
	}
}
