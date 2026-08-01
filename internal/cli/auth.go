package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/thedavidweng/monarchmoney-cli/internal/auth"
	"github.com/thedavidweng/monarchmoney-cli/internal/config"
	"github.com/thedavidweng/monarchmoney-cli/internal/errors"
	"github.com/thedavidweng/monarchmoney-cli/internal/graphql"
	"github.com/thedavidweng/monarchmoney-cli/internal/output"
)

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

var (
	email     string
	password  string
	mfaCode   string
	mfaSecret string
)

var readPassword = term.ReadPassword
var scanInput = fmt.Scanln
var authenticateSession = auth.Authenticate
var newSessionStore = auth.NewStore

// defaultSessionPath is the injectable session path function. All command handlers
// MUST use defaultSessionPath() instead of config.DefaultSessionPath() directly,
// otherwise tests will fail in CI where no real session file exists.
var defaultSessionPath = config.DefaultSessionPath
var exitFunc = os.Exit

type identityResult struct {
	Email string
}

var fetchIdentity = func(ctx context.Context, token string) (*identityResult, error) {
	client := graphql.NewClient("https://api.monarch.com/graphql", token, timeout)
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

var authCmd = &cobra.Command{
	Use:     "auth",
	Short:   "Manage authentication and session",
	GroupID: "utility",
	Example: "  monarch auth login\n  monarch auth status\n  monarch auth logout",
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in to Monarch Money",
	Run: func(cmd *cobra.Command, args []string) {
		start := time.Now()
		renderer := output.NewRenderer(nil, nil, jsonMode, pretty)

		email := firstNonEmpty(email, os.Getenv("MONARCH_EMAIL"))
		password := firstNonEmpty(password, os.Getenv("MONARCH_PASSWORD"))
		mfaCode := firstNonEmpty(mfaCode, os.Getenv("MONARCH_MFA_CODE"))
		mfaSecret := firstNonEmpty(mfaSecret, os.Getenv("MONARCH_MFA_SECRET"))

		if email == "" {
			fmt.Print("Email: ")
			scanInput(&email) //nolint:errcheck // interactive input
		}

		if password == "" {
			fmt.Print("Password: ")
			bytePassword, err := readPassword(int(os.Stdin.Fd()))
			fmt.Println()
			if err != nil {
				handleError(renderer, "login", errors.New(errors.InternalError, "failed to read password", errors.CatInternal, false, err), start)
				return
			}
			password = string(bytePassword)
		}

		sess, err := authenticateSession(email, password, mfaCode, mfaSecret)

		if err != nil {
			if e, ok := err.(*errors.Error); ok && e.Code == errors.AuthMFARequired && !jsonMode {
				fmt.Print("MFA Code: ")
				scanInput(&mfaCode) //nolint:errcheck // interactive input
				sess, err = authenticateSession(email, password, mfaCode, mfaSecret)
			}
		}

		if err != nil {
			var cliErr *errors.Error
			if e, ok := err.(*errors.Error); ok {
				cliErr = e
			} else {
				cliErr = errors.New(errors.InternalError, "authentication failed", errors.CatInternal, false, err)
			}
			handleError(renderer, "auth.login", cliErr, start)
			return
		}

		sess.Profile = profile
		store := newSessionStore(defaultSessionPath())
		if err := store.Save(sess); err != nil {
			handleError(renderer, "auth.login", errors.New(errors.InternalError, "failed to save session", errors.CatInternal, false, err), start)
			return
		}

		if jsonMode {
			env := output.NewEnvelope("auth.login", profile, output.SchemaVersion, requestID, map[string]any{
				"status":       "logged in",
				"email":        sess.Email,
				"profile":      sess.Profile,
				"created_at":   sess.CreatedAt,
				"updated_at":   sess.UpdatedAt,
				"session_path": defaultSessionPath(),
			}, time.Since(start))
			renderer.RenderSuccess(env)
		} else {
			fmt.Printf("Successfully logged in as %s.\n", sess.Email)
			fmt.Printf("Logged in at: %s\n", sess.CreatedAt.Format(time.RFC3339))
			fmt.Printf("Session token saved to: %s\n", defaultSessionPath())
		}
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check authentication status",
	Run: func(cmd *cobra.Command, args []string) {
		start := time.Now()
		renderer := output.NewRenderer(nil, nil, jsonMode, pretty)

		store := newSessionStore(defaultSessionPath())
		sess, err := store.Load()
		if err != nil {
			handleError(renderer, "auth.status", errors.New(errors.AuthRequired, "not logged in", errors.CatAuth, false, err), start)
			return
		}

		identity, err := fetchIdentity(cmd.Context(), sess.Token)
		if err != nil {
			cliErr, ok := err.(*errors.Error)
			if !ok {
				cliErr = errors.New(errors.InternalError, "failed to verify session", errors.CatInternal, false, err)
			}
			handleError(renderer, "auth.status", cliErr, start)
			return
		}

		displayEmail := sess.Email
		if displayEmail == "" && identity != nil && identity.Email != "" {
			displayEmail = identity.Email
		}

		data := map[string]any{
			"authenticated": true,
			"session_valid": true,
			"email":         displayEmail,
			"profile":       sess.Profile,
			"created_at":    sess.CreatedAt,
			"updated_at":    sess.UpdatedAt,
			"session_path":  defaultSessionPath(),
		}

		if jsonMode {
			env := output.NewEnvelope("auth.status", profile, output.SchemaVersion, requestID, data, time.Since(start))
			renderer.RenderSuccess(env)
		} else {
			fmt.Printf("Authenticated: yes\n")
			fmt.Printf("Email: %s\n", displayEmail)
			fmt.Printf("Profile: %s\n", sess.Profile)
			fmt.Printf("Logged in at: %s\n", sess.CreatedAt.Format(time.RFC3339))
			fmt.Printf("Session valid: yes\n")
			fmt.Printf("Session path: %s\n", defaultSessionPath())
		}
	},
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Log out and remove local session",
	Run: func(cmd *cobra.Command, args []string) {
		start := time.Now()
		renderer := output.NewRenderer(nil, nil, jsonMode, pretty)

		store := newSessionStore(defaultSessionPath())
		if err := store.Delete(); err != nil && !os.IsNotExist(err) {
			handleError(renderer, "auth.logout", errors.New(errors.InternalError, "failed to delete session", errors.CatInternal, false, err), start)
			return
		}

		if jsonMode {
			env := output.NewEnvelope("auth.logout", profile, output.SchemaVersion, requestID, map[string]string{"status": "logged out"}, time.Since(start))
			renderer.RenderSuccess(env)
		} else {
			fmt.Println("Successfully logged out.")
		}
	},
}

var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Manage local session",
}

var sessionPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print the path to the session file",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(defaultSessionPath())
	},
}

func init() {
	loginCmd.Flags().StringVar(&email, "email", "", "email address")
	loginCmd.Flags().StringVar(&password, "password", "", "password")
	loginCmd.Flags().StringVar(&mfaCode, "mfa-code", "", "6-digit MFA code")
	loginCmd.Flags().StringVar(&mfaSecret, "mfa-secret", "", "TOTP secret key for automatic MFA")

	sessionCmd.AddCommand(sessionPathCmd)
	authCmd.AddCommand(loginCmd)
	authCmd.AddCommand(statusCmd)
	authCmd.AddCommand(logoutCmd)
	authCmd.AddCommand(sessionCmd)
	RootCmd.AddCommand(authCmd)
}

func (a *App) buildAuthCommand() *cobra.Command {
	var loginEmail string
	var loginPassword string
	var loginMFACode string
	var loginMFASecret string

	authCommand := &cobra.Command{
		Use:     "auth",
		Short:   "Manage authentication and session",
		GroupID: "utility",
		Example: "  monarch auth login\n  monarch auth status\n  monarch auth logout",
	}

	loginCommand := &cobra.Command{
		Use:   "login",
		Short: "Log in to Monarch Money",
		Run: func(cmd *cobra.Command, _ []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)
			if a.ConfigErr != nil {
				a.handleError(renderer, "auth.login", errors.New(errors.InternalError, "failed to load config", errors.CatInternal, false, a.ConfigErr), start)
				return
			}

			resolvedEmail := firstNonEmpty(loginEmail, a.Deps.Getenv("MONARCH_EMAIL"))
			resolvedPassword := firstNonEmpty(loginPassword, a.Deps.Getenv("MONARCH_PASSWORD"))
			resolvedMFACode := firstNonEmpty(loginMFACode, a.Deps.Getenv("MONARCH_MFA_CODE"))
			resolvedMFASecret := firstNonEmpty(loginMFASecret, a.Deps.Getenv("MONARCH_MFA_SECRET"))

			if resolvedEmail == "" {
				fmt.Fprint(cmd.ErrOrStderr(), "Email: ") //nolint:errcheck // best-effort prompt
				if _, err := fmt.Fscan(cmd.InOrStdin(), &resolvedEmail); err != nil {
					a.handleError(renderer, "auth.login", errors.New(errors.InvalidArguments, "failed to read email", errors.CatValidation, false, err), start)
					return
				}
			}

			if resolvedPassword == "" {
				fmt.Fprint(cmd.ErrOrStderr(), "Password: ") //nolint:errcheck // best-effort prompt
				passwordBytes, err := a.Deps.ReadPassword()
				fmt.Fprintln(cmd.ErrOrStderr()) //nolint:errcheck // prompt newline
				if err != nil {
					a.handleError(renderer, "auth.login", errors.New(errors.InternalError, "failed to read password", errors.CatInternal, false, err), start)
					return
				}
				resolvedPassword = string(passwordBytes)
			}

			client := auth.NewClient(a.Deps.HTTPTransport)
			credentials := auth.Credentials{
				Email:     resolvedEmail,
				Password:  resolvedPassword,
				MFACode:   resolvedMFACode,
				MFASecret: resolvedMFASecret,
			}
			sess, err := client.Authenticate(cmd.Context(), credentials)
			if cliErr, ok := err.(*errors.Error); ok && cliErr.Code == errors.AuthMFARequired && !a.Flags.JSONMode {
				fmt.Fprint(cmd.ErrOrStderr(), "MFA Code: ") //nolint:errcheck // best-effort prompt
				if _, scanErr := fmt.Fscan(cmd.InOrStdin(), &credentials.MFACode); scanErr != nil {
					a.handleError(renderer, "auth.login", errors.New(errors.InvalidArguments, "failed to read MFA code", errors.CatValidation, false, scanErr), start)
					return
				}
				sess, err = client.Authenticate(cmd.Context(), credentials)
			}
			if err != nil {
				cliErr, ok := err.(*errors.Error)
				if !ok {
					cliErr = errors.New(errors.InternalError, "authentication failed", errors.CatInternal, false, err)
				}
				a.handleError(renderer, "auth.login", cliErr, start)
				return
			}

			sess.Profile = a.Flags.Profile
			if err := auth.NewStore(a.sessionPath()).Save(sess); err != nil {
				a.handleError(renderer, "auth.login", errors.New(errors.InternalError, "failed to save session", errors.CatInternal, false, err), start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("auth.login", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, map[string]any{
					"status":       "logged in",
					"email":        sess.Email,
					"profile":      sess.Profile,
					"created_at":   sess.CreatedAt,
					"updated_at":   sess.UpdatedAt,
					"session_path": a.sessionPath(),
				}, time.Since(start))
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully logged in as %s.\n", sess.Email)             //nolint:errcheck // best-effort output
			fmt.Fprintf(cmd.OutOrStdout(), "Logged in at: %s\n", sess.CreatedAt.Format(time.RFC3339)) //nolint:errcheck // best-effort output
			fmt.Fprintf(cmd.OutOrStdout(), "Session token saved to: %s\n", a.sessionPath())           //nolint:errcheck // best-effort output
		},
	}
	loginCommand.Flags().StringVar(&loginEmail, "email", "", "email address")
	loginCommand.Flags().StringVar(&loginPassword, "password", "", "password")
	loginCommand.Flags().StringVar(&loginMFACode, "mfa-code", "", "6-digit MFA code")
	loginCommand.Flags().StringVar(&loginMFASecret, "mfa-secret", "", "TOTP secret key for automatic MFA")

	statusCommand := &cobra.Command{
		Use:   "status",
		Short: "Check authentication status",
		Run: func(cmd *cobra.Command, _ []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)
			sess, err := auth.NewStore(a.sessionPath()).Load()
			if err != nil {
				a.handleError(renderer, "auth.status", errors.New(errors.AuthRequired, "not logged in", errors.CatAuth, false, err), start)
				return
			}

			identity, err := a.fetchIdentity(cmd.Context(), sess.Token)
			if err != nil {
				cliErr, ok := err.(*errors.Error)
				if !ok {
					cliErr = errors.New(errors.InternalError, "failed to verify session", errors.CatInternal, false, err)
				}
				a.handleError(renderer, "auth.status", cliErr, start)
				return
			}

			displayEmail := sess.Email
			if displayEmail == "" && identity != nil && identity.Email != "" {
				displayEmail = identity.Email
			}
			data := map[string]any{
				"authenticated": true,
				"session_valid": true,
				"email":         displayEmail,
				"profile":       sess.Profile,
				"created_at":    sess.CreatedAt,
				"updated_at":    sess.UpdatedAt,
				"session_path":  a.sessionPath(),
			}
			if a.Flags.JSONMode {
				env := output.NewEnvelope("auth.status", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, data, time.Since(start))
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Authenticated: yes")                                     //nolint:errcheck // best-effort output
			fmt.Fprintf(cmd.OutOrStdout(), "Email: %s\n", displayEmail)                               //nolint:errcheck // best-effort output
			fmt.Fprintf(cmd.OutOrStdout(), "Profile: %s\n", sess.Profile)                             //nolint:errcheck // best-effort output
			fmt.Fprintf(cmd.OutOrStdout(), "Logged in at: %s\n", sess.CreatedAt.Format(time.RFC3339)) //nolint:errcheck // best-effort output
			fmt.Fprintln(cmd.OutOrStdout(), "Session valid: yes")                                     //nolint:errcheck // best-effort output
			fmt.Fprintf(cmd.OutOrStdout(), "Session path: %s\n", a.sessionPath())                     //nolint:errcheck // best-effort output
		},
	}

	logoutCommand := &cobra.Command{
		Use:   "logout",
		Short: "Log out and remove local session",
		Run: func(cmd *cobra.Command, _ []string) {
			start := time.Now()
			renderer := output.NewRenderer(cmd.OutOrStdout(), cmd.ErrOrStderr(), a.Flags.JSONMode, a.Flags.Pretty)
			if err := auth.NewStore(a.sessionPath()).Delete(); err != nil && !os.IsNotExist(err) {
				a.handleError(renderer, "auth.logout", errors.New(errors.InternalError, "failed to delete session", errors.CatInternal, false, err), start)
				return
			}
			if a.Flags.JSONMode {
				env := output.NewEnvelope("auth.logout", a.Flags.Profile, output.SchemaVersion, a.Flags.RequestID, map[string]string{"status": "logged out"}, time.Since(start))
				renderer.RenderSuccess(env) //nolint:errcheck // best-effort render
				return
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Successfully logged out.") //nolint:errcheck // best-effort output
		},
	}

	sessionCommand := &cobra.Command{Use: "session", Short: "Manage local session"}
	sessionPathCommand := &cobra.Command{
		Use:   "path",
		Short: "Print the path to the session file",
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Fprintln(cmd.OutOrStdout(), a.sessionPath()) //nolint:errcheck // best-effort output
		},
	}
	sessionCommand.AddCommand(sessionPathCommand)
	authCommand.AddCommand(loginCommand, statusCommand, logoutCommand, sessionCommand)
	return authCommand
}

func (a *App) fetchIdentity(ctx context.Context, token string) (*identityResult, error) {
	if a.ConfigErr != nil {
		return nil, errors.New(errors.InternalError, "failed to load config", errors.CatInternal, false, a.ConfigErr)
	}
	if a.Config == nil {
		return nil, errors.New(errors.InternalError, "configuration not initialized", errors.CatInternal, false, nil)
	}

	client := graphql.NewClient(a.Config.APIEndpoint, token, a.Flags.Timeout, graphql.WithHTTPTransport(a.Deps.HTTPTransport))
	var response struct {
		Me struct {
			Email string `json:"email"`
		} `json:"me"`
	}
	if err := client.Do(ctx, &graphql.Request{
		OperationName: "GetIdentity",
		Query:         graphql.GetIdentityQuery,
	}, &response); err != nil {
		return nil, err
	}
	return &identityResult{Email: response.Me.Email}, nil
}

func handleError(r *output.Renderer, command string, err *errors.Error, start time.Time) {
	if err != nil && err.Code == errors.AuthSessionExpired {
		store := newSessionStore(defaultSessionPath())
		if sess, loadErr := store.Load(); loadErr == nil {
			if sess.Email != "" {
				err = errors.New(err.Code, fmt.Sprintf("session token for %s stored at %s expired or invalid; run `monarch auth login` again", sess.Email, defaultSessionPath()), err.Category, err.Retryable, err.Err)
			} else {
				err = errors.New(err.Code, fmt.Sprintf("session token stored at %s expired or invalid; run `monarch auth login` again", defaultSessionPath()), err.Category, err.Retryable, err.Err)
			}
		}
	}

	env := output.NewErrorEnvelope(command, profile, output.SchemaVersion, err, time.Since(start))
	env.Meta.RequestID = requestID
	r.RenderError(env)
	exitFunc(err.ExitCode())
}
