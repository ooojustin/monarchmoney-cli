package cli

import (
	"time"

	"github.com/thedavidweng/monarchmoney-cli/internal/output"
)

func (a *App) envelopeWithWarnings(command string, data any, start time.Time, warnings ...string) *output.Envelope {
	env := output.NewEnvelope(command, a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, data, time.Since(start))
	if len(warnings) > 0 {
		env.Meta.Warnings = append([]string(nil), warnings...)
	}
	return env
}
