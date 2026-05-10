package cli

import (
	"fmt"
	"io"
	"time"

	"github.com/thedavidweng/monarchmoney-cli/internal/errors"
	"github.com/thedavidweng/monarchmoney-cli/internal/output"
)

func (a *App) renderSuccess(r *output.Renderer, env *output.Envelope, start time.Time) {
	if err := r.RenderSuccess(env); err != nil {
		a.handleError(r, env.Meta.Command, errors.New(errors.InternalError, "failed to render output", errors.CatInternal, false, err), start)
	}
}

func writeText(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}

func printText(w io.Writer, args ...any) {
	_, _ = fmt.Fprint(w, args...)
}

func printlnText(w io.Writer, args ...any) {
	_, _ = fmt.Fprintln(w, args...)
}
