package doctor

import (
	"context"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/thedavidweng/monarchmoney-cli/internal/auth"
	"github.com/thedavidweng/monarchmoney-cli/internal/graphql"
	"github.com/thedavidweng/monarchmoney-cli/internal/version"
)

type Result struct {
	Version string       `json:"version"`
	OS      string       `json:"os"`
	Arch    string       `json:"arch"`
	Config  ConfigReport `json:"config"`
	Session Report       `json:"session"`
	Network Report       `json:"network"`
	Safety  Report       `json:"safety"`
}

type Report struct {
	Path          string `json:"path,omitempty"`
	Exists        bool   `json:"exists"`
	PermissionOK  bool   `json:"permission_ok,omitempty"`
	Authenticated bool   `json:"authenticated,omitempty"`
	APIReachable  bool   `json:"api_reachable,omitempty"`
}

type ConfigReport struct {
	Path   string `json:"path,omitempty"`
	Exists bool   `json:"exists"`
	Valid  bool   `json:"valid"`
}

type Options struct {
	Connect       bool
	ConfigPath    string
	ConfigError   error
	SessionPath   string
	APIEndpoint   string
	Timeout       time.Duration
	HTTPTransport http.RoundTripper
}

func Check(ctx context.Context, options Options) *Result {
	res := &Result{
		Version: version.GetVersion(),
		OS:      runtime.GOOS,
		Arch:    runtime.GOARCH,
	}

	_, err := os.Stat(options.ConfigPath)
	res.Config = ConfigReport{
		Path:   options.ConfigPath,
		Exists: !os.IsNotExist(err),
		Valid:  options.ConfigError == nil,
	}

	store := auth.NewStore(options.SessionPath)
	sess, err := store.Load()
	res.Session = Report{
		Path:   options.SessionPath,
		Exists: !os.IsNotExist(err),
	}

	if err == nil && sess != nil {
		res.Session.Authenticated = true
		info, err := os.Stat(options.SessionPath)
		if err == nil {
			res.Session.PermissionOK = checkFilePermission(info)
		}
	}

	if options.Connect && res.Session.Authenticated {
		client := graphql.NewClient(
			options.APIEndpoint,
			sess.Token,
			options.Timeout,
			graphql.WithHTTPTransport(options.HTTPTransport),
		)
		var identity any
		err := client.Do(ctx, &graphql.Request{
			OperationName: "GetIdentity",
			Query:         graphql.GetIdentityQuery,
		}, &identity)

		if err == nil {
			res.Network.APIReachable = true
		}
	}

	return res
}
