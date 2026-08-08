package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/thedavidweng/monarchmoney-cli/internal/audit"
	"github.com/thedavidweng/monarchmoney-cli/internal/errors"
	"github.com/thedavidweng/monarchmoney-cli/internal/output"
	"github.com/thedavidweng/monarchmoney-cli/internal/safety"
)

func (a *App) checkSafety(renderer *output.Renderer, command string, tier safety.OperationTier, start time.Time) bool {
	if err := safety.Check(tier, a.Flags.ReadOnly, a.Flags.DryRun, a.Flags.Confirm); err != nil {
		a.handleError(renderer, command, err, start)
		return false
	}
	return true
}

func (a *App) renderPlan(renderer *output.Renderer, command string, plan *safety.Plan, start time.Time) {
	if !renderer.JSON {
		fmt.Fprintln(renderer.Stdout, "Mutation Plan")
		for i, mutation := range plan.PlannedMutations {
			fmt.Fprintf(renderer.Stdout, "%d. %s", i+1, mutation.Operation)
			if mutation.ResourceID != "" {
				fmt.Fprintf(renderer.Stdout, " %s", mutation.ResourceID)
			}
			fmt.Fprintln(renderer.Stdout)
			if mutation.Before != nil {
				fmt.Fprintf(renderer.Stdout, "   Before: %s\n", planValue(mutation.Before))
			}
			if mutation.After != nil {
				fmt.Fprintf(renderer.Stdout, "   After: %s\n", planValue(mutation.After))
			}
		}
		return
	}
	env := output.NewEnvelope(command, a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, plan, time.Since(start))
	renderer.RenderSuccess(env)
}

func planValue(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprint(value)
	}
	return string(data)
}

func (a *App) mutate(renderer *output.Renderer, command, resourceID string, start time.Time, fn func() (any, error), failMsg string) (any, error) {
	result, err := fn()

	resultStr := "success"
	var errCode string
	if err != nil {
		resultStr = "failure"
		if e, ok := err.(*errors.Error); ok {
			errCode = string(e.Code)
		}
	}

	a.recordAudit(&audit.Record{
		Command:    command,
		ResourceID: resourceID,
		DryRun:     false,
		Confirmed:  a.Flags.Confirm,
		Profile:    a.Flags.Profile,
		Result:     resultStr,
		ErrorCode:  errCode,
	})

	if err != nil {
		a.handleError(renderer, command, wrapError(err, failMsg), start)
		return nil, err
	}

	return result, nil
}

func (a *App) recordAudit(record *audit.Record) {
	if a.Config == nil || !a.Config.AuditLog {
		return
	}
	_ = a.Deps.WriteAudit(record)
}
