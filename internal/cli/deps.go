package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/spf13/viper"
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

// identityResult is the trimmed payload returned by FetchIdentity.
type identityResult struct {
	Email string
}

// Deps holds every dependency a CLI command might need to replace in tests.
// Production code constructs Deps via DefaultDeps; tests construct Deps with
// custom fields to inject behavior.
type Deps struct {
	// Configuration accessors evaluated at command time so cobra's
	// PersistentPreRun (which populates viper from flags) can run first.
	SessionPath func() string
	APIEndpoint func() string
	Timeout     func() time.Duration

	// HTTPTransport, if non-nil, is used for all outgoing GraphQL traffic.
	// Tests inject a fake RoundTripper here to avoid touching the process-
	// global http.DefaultTransport. Production leaves this nil.
	HTTPTransport http.RoundTripper

	// Auth and API factories.
	NewStore     func(path string) *auth.Store
	Authenticate func(email, password, mfaCode, mfaSecret string) (*auth.Session, error)
	NewClient    func(endpoint, token string, timeout time.Duration) GraphQLClient
	NewService   func(client GraphQLClient) *monarch.Service

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

// DefaultDeps returns Deps wired to real implementations.
//
// NewClient is intentionally left nil here; App.New installs the default
// NewClient closure with a pointer back to App.Deps so that tests which
// mutate app.Deps.HTTPTransport after construction still take effect when
// commands later call a.Deps.NewClient.
func DefaultDeps() Deps {
	return Deps{
		SessionPath: config.DefaultSessionPath,
		APIEndpoint: defaultAPIEndpoint,
		Timeout:     defaultTimeout,

		NewStore:     auth.NewStore,
		Authenticate: auth.Authenticate,
		NewService: func(client GraphQLClient) *monarch.Service {
			return monarch.NewService(client)
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

// defaultAPIEndpoint reads the GraphQL endpoint from viper, with the default
// set in internal/config. Allows MONARCH_API_ENDPOINT to take effect.
//
// Held under globalRegistration because viper has no internal locking and a
// concurrent App construction (which calls viper.BindPFlag) would race with
// this read.
func defaultAPIEndpoint() string {
	globalRegistration.Lock()
	endpoint := viper.GetString("api_endpoint")
	globalRegistration.Unlock()
	if endpoint != "" {
		return endpoint
	}
	return "https://api.monarch.com/graphql"
}

// defaultTimeout reads the request timeout from viper. Falls back to 30s.
// Locked for the same reason as defaultAPIEndpoint.
func defaultTimeout() time.Duration {
	globalRegistration.Lock()
	t := viper.GetDuration("timeout")
	globalRegistration.Unlock()
	if t > 0 {
		return t
	}
	return 30 * time.Second
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
