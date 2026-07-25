package cli

import (
	"context"
	"time"

	"github.com/spf13/cobra"

	"github.com/thedavidweng/monarchmoney-cli/internal/audit"
	"github.com/thedavidweng/monarchmoney-cli/internal/auth"
	"github.com/thedavidweng/monarchmoney-cli/internal/config"
	"github.com/thedavidweng/monarchmoney-cli/internal/errors"
	"github.com/thedavidweng/monarchmoney-cli/internal/graphql"
	"github.com/thedavidweng/monarchmoney-cli/internal/monarch"
	"github.com/thedavidweng/monarchmoney-cli/internal/output"
	"github.com/thedavidweng/monarchmoney-cli/internal/safety"
)

// CommandDeps bundles the dependencies every command handler needs.
type CommandDeps struct {
	Start    time.Time
	Renderer *output.Renderer
	Service  *monarch.Service
}

// newDeps constructs a CommandDeps by loading the session, creating the GraphQL
// client, and building the monarch.Service. On failure it renders the error and
// returns ok=false; callers should return immediately.
func newDeps(renderer *output.Renderer, command string, start time.Time) (CommandDeps, bool) {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		handleError(renderer, command, errors.New(errors.InternalError, "failed to load config", errors.CatInternal, false, err), start)
		return CommandDeps{}, false
	}

	store := auth.NewStore(defaultSessionPath())
	sess, err := store.Load()
	if err != nil {
		handleError(renderer, command, errors.New(errors.AuthRequired, "not logged in", errors.CatAuth, false, err), start)
		return CommandDeps{}, false
	}

	client := graphql.NewClient(cfg.APIEndpoint, sess.Token, cfg.Timeout)
	return CommandDeps{
		Start:    start,
		Renderer: renderer,
		Service:  monarch.NewService(client),
	}, true
}

// run executes a read command on the session-backed service, then emits the JSON
// envelope or hands the typed result to human. Errors render via handleError.
func run[T any](ctx context.Context, command, failMsg string, fn func(context.Context, *monarch.Service) (T, error), human func(T)) {
	runRead(ctx, command, failMsg, nil, fn, human)
}

// runWarn is run with static warnings attached to the envelope meta.
func runWarn[T any](ctx context.Context, command, failMsg string, warnings []string, fn func(context.Context, *monarch.Service) (T, error), human func(T)) {
	runRead(ctx, command, failMsg, warnings, fn, human)
}

func runRead[T any](ctx context.Context, command, failMsg string, warnings []string, fn func(context.Context, *monarch.Service) (T, error), human func(T)) {
	start := time.Now()
	renderer := output.NewRenderer(nil, nil, jsonMode, pretty)

	deps, ok := newDeps(renderer, command, start)
	if !ok {
		return
	}

	data, err := fn(ctx, deps.Service)
	if err != nil {
		handleError(renderer, command, wrapError(err, failMsg), start)
		return
	}

	if jsonMode {
		env := output.NewEnvelope(command, profile, output.SchemaVersion, requestID, data, time.Since(start))
		if len(warnings) > 0 {
			env.Meta.Warnings = append([]string(nil), warnings...)
		}
		renderer.RenderSuccess(env)
		return
	}
	human(data)
}

// mutation is a prepared write: audit resource id, the dry-run plan "after"
// payload, the remote write returning its envelope data, and the human line.
type mutation struct {
	resourceID string
	planAfter  any
	do         func(context.Context, *monarch.Service) (any, error)
	human      func()
}

// runMutation owns the write pipeline: safety gate, dry-run plan, audit, envelope.
// prepare runs after the safety check, reports validation errors, and builds the
// plan and write closure. do runs only for a confirmed (non-dry-run) write.
func runMutation(cmd *cobra.Command, command, failMsg string, tier safety.OperationTier, prepare func() (mutation, *errors.Error)) {
	start := time.Now()
	renderer := output.NewRenderer(nil, nil, jsonMode, pretty)

	if err := safety.Check(tier, readOnly, dryRun, confirm); err != nil {
		handleError(renderer, command, err, start)
		return
	}

	m, verr := prepare()
	if verr != nil {
		handleError(renderer, command, verr, start)
		return
	}

	if dryRun {
		plan := safety.NewPlan()
		plan.Add(command, m.resourceID, nil, m.planAfter)
		renderer.RenderSuccess(output.NewEnvelope(command, profile, output.SchemaVersion, requestID, plan, time.Since(start)))
		return
	}

	deps, ok := newDeps(renderer, command, start)
	if !ok {
		return
	}

	data, err := deps.Mutate(command, m.resourceID, func() (any, error) {
		return m.do(cmd.Context(), deps.Service)
	}, failMsg)
	if err != nil {
		return
	}

	if jsonMode {
		renderer.RenderSuccess(output.NewEnvelope(command, profile, output.SchemaVersion, requestID, data, time.Since(start)))
		return
	}
	m.human()
}

// wrapError converts a generic error into a structured *errors.Error.
// If err is already an *errors.Error, it is returned as-is.
func wrapError(err error, message string) *errors.Error {
	if e, ok := err.(*errors.Error); ok {
		return e
	}
	return errors.New(errors.APIError, message, errors.CatAPI, false, err)
}

// Mutate executes a write operation with audit logging. It runs fn, logs the
// result to the audit log, and either returns the result or renders the error.
// Callers handle success output (JSON envelope or human-readable text).
// Returns nil if the mutation failed (error already rendered).
func (d CommandDeps) Mutate(command, resourceID string, fn func() (any, error), failMsg string) (any, error) {
	result, err := fn()

	resultStr := "success"
	var errCode string
	if err != nil {
		resultStr = "failure"
		if e, ok := err.(*errors.Error); ok {
			errCode = string(e.Code)
		}
	}

	audit.NewLogger().Log(&audit.Record{ //nolint:errcheck // best-effort audit
		Command:    command,
		ResourceID: resourceID,
		DryRun:     false,
		Confirmed:  confirm,
		Profile:    profile,
		Result:     resultStr,
		ErrorCode:  errCode,
	})

	if err != nil {
		handleError(d.Renderer, command, wrapError(err, failMsg), d.Start)
		return nil, err
	}

	return result, nil
}
