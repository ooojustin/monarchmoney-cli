package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
	"github.com/thedavidweng/monarchmoney-cli/internal/audit"
	"github.com/thedavidweng/monarchmoney-cli/internal/auth"
	"github.com/thedavidweng/monarchmoney-cli/internal/config"
	clierrors "github.com/thedavidweng/monarchmoney-cli/internal/errors"
	"github.com/thedavidweng/monarchmoney-cli/internal/graphql"
	"github.com/thedavidweng/monarchmoney-cli/internal/monarch"
	"golang.org/x/term"
)

// GraphQLClient is the surface that monarch.NewService consumes. Defining it
// here lets tests inject a mock client without constructing a real
// *graphql.Client. *graphql.Client satisfies this interface.
type GraphQLClient interface {
	Do(ctx context.Context, req *graphql.Request, result any) error
	TokenValue() string
}

type AuditLogger interface {
	Log(*audit.Record) error
}

// identityResult is the trimmed payload returned by FetchIdentity.
type identityResult struct {
	Email string
}

// Deps holds every dependency a CLI command might need to replace in tests.
// Production code constructs Deps via DefaultDeps; tests construct Deps with
// custom fields to inject behavior.
type Deps struct {
	// Viper is the configuration store this App reads and writes. Each App
	// owns its own *viper.Viper so parallel tests don't fight over the
	// process-global instance.
	Viper *viper.Viper

	// HTTPTransport, if non-nil, is used for all outgoing GraphQL traffic.
	// Tests inject a fake RoundTripper here to avoid touching the process-
	// global http.DefaultTransport. Production leaves this nil.
	HTTPTransport http.RoundTripper

	// Configuration accessors evaluated at command time. Defaults are
	// installed by App.New if left nil so that test mutations to Viper
	// after construction (e.g. viper.Set("api_endpoint", ...)) take effect.
	SessionPath func() string
	APIEndpoint func() string
	Timeout     func() time.Duration

	// Auth and API factories.
	NewStore       func(path string) *auth.Store
	Authenticate   func(email, password, mfaCode, mfaSecret string) (*auth.Session, error)
	NewClient      func(endpoint, token string, timeout time.Duration) GraphQLClient
	NewService     func(client GraphQLClient) *monarch.Service
	NewAuditLogger func() AuditLogger

	// IO and process control.
	Stdout       io.Writer
	Stderr       io.Writer
	Stdin        io.Reader
	ReadPassword func(fd int) ([]byte, error)
	ScanInput    func(args ...any) (int, error)
	Exit         func(code int)

	// Standard library swappables (rarely overridden but historically present).
	JSONUnmarshal func([]byte, any) error
}

// DefaultDeps returns Deps wired to real implementations against a fresh
// *viper.Viper with application defaults and env-var bindings installed.
//
// SessionPath/APIEndpoint/Timeout/NewClient are intentionally left nil; App.New
// fills them in with closures that capture &a.Deps so that test code which
// mutates app.Deps.Viper or app.Deps.HTTPTransport after construction still
// takes effect when commands later invoke these accessors.
func DefaultDeps() Deps {
	v := viper.New()
	v.SetEnvPrefix("MONARCH")
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	v.AutomaticEnv()
	config.SetDefaults(v)

	// Legacy env var aliases that don't follow the prefix-replacer convention.
	v.BindEnv("read-only", "MONARCH_READONLY")
	v.BindEnv("profile", "MONARCH_PROFILE")
	v.BindEnv("timeout", "MONARCH_TIMEOUT")
	v.BindEnv("config", "MONARCH_CONFIG")

	return Deps{
		Viper: v,

		NewStore: auth.NewStore,
		NewAuditLogger: func() AuditLogger {
			return audit.NewLogger(config.DefaultAuditDir())
		},

		Stdout:       os.Stdout,
		Stderr:       os.Stderr,
		Stdin:        os.Stdin,
		ReadPassword: term.ReadPassword,
		ScanInput:    fmt.Scanln,
		Exit:         os.Exit,

		JSONUnmarshal: json.Unmarshal,
	}
}

// LoadService runs the load-session-then-build-service boilerplate that ~50
// commands repeat. Returns the service, the session (some commands need the
// email or token directly), and a wrapped AUTH_REQUIRED error if the session
// can't be loaded.
func (d Deps) LoadService() (*monarch.Service, *auth.Session, error) {
	sess, err := d.NewStore(d.SessionPath()).Load()
	if err != nil {
		return nil, nil, clierrors.New(
			clierrors.AuthRequired,
			"not logged in",
			clierrors.CatAuth,
			false,
			err,
		)
	}
	client := d.NewClient(d.APIEndpoint(), sess.Token, d.Timeout())
	return d.NewService(client), sess, nil
}

// FetchIdentity verifies a session token by issuing the GetIdentity query.
// Tests typically inject a mock via Deps.NewClient rather than overriding
// this method directly.
func (d Deps) FetchIdentity(ctx context.Context, token string) (*identityResult, error) {
	client := d.NewClient(d.APIEndpoint(), token, d.Timeout())
	var resp struct {
		Me struct {
			Email string `json:"email"`
		} `json:"me"`
	}
	if err := client.Do(ctx, &graphql.Request{
		OperationName: "GetIdentity",
		Query:         graphql.GetIdentityQuery,
	}, &resp); err != nil {
		return nil, err
	}
	return &identityResult{Email: resp.Me.Email}, nil
}
