package cli

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

// TestAnalyzeAnomaliesRequiresAuth verifies the analyze command surface goes
// through Deps.LoadService and reports AUTH_REQUIRED when the session is
// missing, exercising the App+Deps flow end-to-end.
func TestAnalyzeAnomaliesRequiresAuth(t *testing.T) {
	t.Parallel()
	sessionPath := filepath.Join(t.TempDir(), "missing.json")
	app, buf, exitCode := newTestApp(t, sessionPath)

	cmd, _, err := app.Root.Find([]string{"analyze", "anomalies"})
	if err != nil {
		t.Fatalf("Find analyze anomalies = %v", err)
	}
	_ = cmd.Flags().Set("month", "2026-05")
	cmd.SetContext(context.Background())
	cmd.Run(cmd, nil)

	if *exitCode != 3 {
		t.Fatalf("exitCode = %d, want 3 (AUTH_REQUIRED); output=%q", *exitCode, buf.String())
	}
	if !strings.Contains(buf.String(), "AUTH_REQUIRED") {
		t.Fatalf("output = %q", buf.String())
	}
}

// TestAnalyzeMerchantsRejectsUnsupportedCompare verifies validation errors
// surface before any session load attempt.
func TestAnalyzeMerchantsRejectsUnsupportedCompare(t *testing.T) {
	t.Parallel()
	sessionPath := filepath.Join(t.TempDir(), "session.json")
	app, buf, exitCode := newTestApp(t, sessionPath)

	cmd, _, err := app.Root.Find([]string{"analyze", "merchants"})
	if err != nil {
		t.Fatalf("Find analyze merchants = %v", err)
	}
	_ = cmd.Flags().Set("compare", "quarter")
	cmd.SetContext(context.Background())
	cmd.Run(cmd, nil)

	if *exitCode == 0 {
		t.Fatalf("exitCode = 0, want validation failure; output=%q", buf.String())
	}
	if !strings.Contains(buf.String(), "previous-month") {
		t.Fatalf("output = %q, want supported-compare guidance", buf.String())
	}
}
