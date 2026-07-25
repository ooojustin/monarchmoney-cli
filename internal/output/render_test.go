package output

import (
	"bytes"
	"strings"
	"testing"
	"time"

	clierrors "github.com/thedavidweng/monarchmoney-cli/internal/errors"
)

func TestRenderer_RenderSuccess(t *testing.T) {
	tests := []struct {
		name      string
		jsonMode  bool
		pretty    bool
		env       *Envelope
		want      []string
		wantExact string
	}{
		{
			name:     "json",
			jsonMode: true,
			env:      NewEnvelope("test", "default", SchemaVersion, "req-123", map[string]string{"foo": "bar"}, 10*time.Millisecond),
			want:     []string{`"ok":true`, `"data":{"foo":"bar"}`, `"request_id":"req-123"`},
		},
		{
			name:     "pretty",
			jsonMode: true,
			pretty:   true,
			env:      NewEnvelope("test", "default", SchemaVersion, "", map[string]string{"foo": "bar"}, time.Second),
			want:     []string{"\n  \"ok\": true"},
		},
		{
			name:      "non-json is silent",
			jsonMode:  false,
			env:       NewEnvelope("test", "default", SchemaVersion, "", "value", 0),
			wantExact: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			NewRenderer(stdout, nil, tt.jsonMode, tt.pretty).RenderSuccess(tt.env)
			got := stdout.String()
			if !tt.jsonMode && got != tt.wantExact {
				t.Fatalf("output = %q, want %q", got, tt.wantExact)
			}
			for _, w := range tt.want {
				if !strings.Contains(got, w) {
					t.Fatalf("output = %q, want substring %q", got, w)
				}
			}
		})
	}
}

func TestRenderer_RenderError(t *testing.T) {
	tests := []struct {
		name       string
		jsonMode   bool
		pretty     bool
		wantStdout []string
		wantStderr []string
	}{
		{name: "json", jsonMode: true, wantStdout: []string{`"ok":false`, `"message":"boom"`}},
		{name: "pretty", jsonMode: true, pretty: true, wantStdout: []string{"\n  \"ok\": false"}},
		{name: "text", jsonMode: false, wantStderr: []string{"Error: boom"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
			r := NewRenderer(stdout, stderr, tt.jsonMode, tt.pretty)
			r.RenderError(NewErrorEnvelope("test", "default", SchemaVersion, &clierrors.Error{Message: "boom"}, 0))
			for _, w := range tt.wantStdout {
				if !strings.Contains(stdout.String(), w) {
					t.Fatalf("stdout = %q, want substring %q", stdout.String(), w)
				}
			}
			for _, w := range tt.wantStderr {
				if !strings.Contains(stderr.String(), w) {
					t.Fatalf("stderr = %q, want substring %q", stderr.String(), w)
				}
			}
		})
	}
}

func TestRenderer_PrintDiagnostic(t *testing.T) {
	stderr := &bytes.Buffer{}
	NewRenderer(nil, stderr, false, false).PrintDiagnostic("hello")
	if !strings.Contains(stderr.String(), "hello") {
		t.Fatalf("stderr = %q, want substring %q", stderr.String(), "hello")
	}
}

func TestNewRendererDefaults(t *testing.T) {
	r := NewRenderer(nil, nil, false, false)
	if r.Stdout == nil || r.Stderr == nil {
		t.Fatalf("NewRenderer defaults: Stdout=%v Stderr=%v, want non-nil", r.Stdout, r.Stderr)
	}
}
