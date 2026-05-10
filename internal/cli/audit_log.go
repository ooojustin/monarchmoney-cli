package cli

import (
	"fmt"

	"github.com/thedavidweng/monarchmoney-cli/internal/audit"
)

func (a *App) logAudit(logger AuditLogger, record *audit.Record) {
	if err := logger.Log(record); err != nil {
		_, _ = fmt.Fprintf(a.Deps.Stderr, "Warning: failed to write audit log: %v\n", err)
	}
}
