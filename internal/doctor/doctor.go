package doctor

import (
	"context"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/thedavidweng/monarchmoney-cli/internal/auth"
	"github.com/thedavidweng/monarchmoney-cli/internal/config"
	"github.com/thedavidweng/monarchmoney-cli/internal/graphql"
	"github.com/thedavidweng/monarchmoney-cli/internal/version"
)

// Result represents the output of the doctor command.
type Result struct {
	Version string `json:"version"`
	OS      string `json:"os"`
	Arch    string `json:"arch"`
	Config  Report `json:"config"`
	Session Report `json:"session"`
	Network Report `json:"network"`
	Safety  Report `json:"safety"`
}

// Report represents a specific component's check report.
type Report struct {
	Path          string `json:"path,omitempty"`
	Exists        bool   `json:"exists"`
	Valid         bool   `json:"valid,omitempty"`
	PermissionOK  bool   `json:"permission_ok,omitempty"`
	Authenticated bool   `json:"authenticated,omitempty"`
	APIReachable  bool   `json:"api_reachable,omitempty"`
}

type Options struct {
	ConfigPath      string
	SessionPath     string
	GraphQLEndpoint string
	Timeout         time.Duration
	HTTPTransport   http.RoundTripper
}

// Check performs local system and configuration checks.
func Check(ctx context.Context, connect bool, options ...Options) *Result {
	opts := doctorOptions(options)
	res := &Result{
		Version: version.Version,
		OS:      runtime.GOOS,
		Arch:    runtime.GOARCH,
	}

	// Config check
	cfgPath := opts.ConfigPath
	_, err := os.Stat(cfgPath)
	res.Config = Report{
		Path:   cfgPath,
		Exists: !os.IsNotExist(err),
	}

	// Session check
	sessPath := opts.SessionPath
	store := auth.NewStore(sessPath)
	sess, err := store.Load()
	res.Session = Report{
		Path:   sessPath,
		Exists: !os.IsNotExist(err),
	}

	if err == nil && sess != nil {
		res.Session.Authenticated = true
		info, err := os.Stat(sessPath)
		if err == nil {
			res.Session.PermissionOK = (info.Mode()&0777 == 0600)
		}
	}

	if connect && res.Session.Authenticated {
		client := graphql.NewClient(opts.GraphQLEndpoint, sess.Token, opts.Timeout, opts.HTTPTransport)
		var identity interface{}
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

func doctorOptions(options []Options) Options {
	opts := Options{
		ConfigPath:      config.DefaultConfigPath(),
		SessionPath:     config.DefaultSessionPath(),
		GraphQLEndpoint: config.GraphQLEndpoint(config.DefaultAPIBaseURL),
		Timeout:         10 * time.Second,
	}
	for _, override := range options {
		if override.ConfigPath != "" {
			opts.ConfigPath = override.ConfigPath
		}
		if override.SessionPath != "" {
			opts.SessionPath = override.SessionPath
		}
		if override.GraphQLEndpoint != "" {
			opts.GraphQLEndpoint = override.GraphQLEndpoint
		}
		if override.Timeout > 0 {
			opts.Timeout = override.Timeout
		}
		if override.HTTPTransport != nil {
			opts.HTTPTransport = override.HTTPTransport
		}
	}
	return opts
}
