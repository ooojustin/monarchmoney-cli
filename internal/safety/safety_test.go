package safety

import (
	"strings"
	"testing"
)

func TestCheck(t *testing.T) {
	tests := []struct {
		name      string
		tier      OperationTier
		readOnly  bool
		dryRun    bool
		confirmed bool
		wantErr   string
	}{
		{
			name:     "Read allowed in read-only",
			tier:     TierRead,
			readOnly: true,
			wantErr:  "",
		},
		{
			name:     "Mutation blocked in read-only",
			tier:     TierMutation,
			readOnly: true,
			wantErr:  "remote writes are blocked in read-only mode",
		},
		{
			name:     "Mutation blocked in read-only takes precedence over dry-run",
			tier:     TierMutation,
			readOnly: true,
			dryRun:   true,
			wantErr:  "remote writes are blocked in read-only mode",
		},
		{
			name:      "Mutation requires confirm",
			tier:      TierMutation,
			readOnly:  false,
			dryRun:    false,
			confirmed: false,
			wantErr:   "requires --confirm",
		},
		{
			name:      "Mutation allowed with confirm",
			tier:      TierMutation,
			readOnly:  false,
			dryRun:    false,
			confirmed: true,
			wantErr:   "",
		},
		{
			name:      "Mutation allowed with dry-run",
			tier:      TierMutation,
			readOnly:  false,
			dryRun:    true,
			confirmed: false,
			wantErr:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Check(tt.tier, tt.readOnly, tt.dryRun, tt.confirmed)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Check() error = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Check() error = nil, want containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Check() error = %q, want containing %q", err.Error(), tt.wantErr)
			}
		})
	}
}
