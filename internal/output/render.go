package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// Renderer handles writing data to the appropriate output streams.
type Renderer struct {
	Stdout io.Writer
	Stderr io.Writer
	JSON   bool
	Pretty bool
}

// NewRenderer returns a new Renderer.
func NewRenderer(stdout, stderr io.Writer, jsonMode, pretty bool) *Renderer {
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}
	return &Renderer{
		Stdout: stdout,
		Stderr: stderr,
		JSON:   jsonMode,
		Pretty: pretty,
	}
}

// RenderSuccess writes a successful result to stdout.
func (r *Renderer) RenderSuccess(env *Envelope) error {
	if r.JSON {
		var data []byte
		var err error
		if r.Pretty {
			data, err = json.MarshalIndent(env, "", "  ")
		} else {
			data, err = json.Marshal(env)
		}
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(r.Stdout, string(data))
		return err
	}

	// Non-JSON success rendering is command-specific; Renderer intentionally stays silent here.
	return nil
}

// RenderError writes an error to stdout (as JSON) or stderr (as text).
func (r *Renderer) RenderError(env *ErrorEnvelope) error {
	if r.JSON {
		var data []byte
		var err error
		if r.Pretty {
			data, err = json.MarshalIndent(env, "", "  ")
		} else {
			data, err = json.Marshal(env)
		}
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(r.Stdout, string(data))
		return err
	}

	_, err := fmt.Fprintf(r.Stderr, "Error: %s\n", env.Error.Message)
	return err
}

// PrintDiagnostic writes a diagnostic message to stderr.
func (r *Renderer) PrintDiagnostic(msg string) {
	_, _ = fmt.Fprintln(r.Stderr, msg)
}
