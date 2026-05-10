package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/thedavidweng/monarchmoney-cli/internal/errors"
	"github.com/thedavidweng/monarchmoney-cli/internal/output"
)

func (a *App) buildAuthCommands(parent *cobra.Command) {
	authCmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage authentication and session",
	}

	authCmd.AddCommand(a.buildLogin())
	authCmd.AddCommand(a.buildStatus())
	authCmd.AddCommand(a.buildLogout())

	sessionCmd := &cobra.Command{
		Use:   "session",
		Short: "Manage local session",
	}
	sessionCmd.AddCommand(a.buildSessionPath())
	authCmd.AddCommand(sessionCmd)

	parent.AddCommand(authCmd)
}

func (a *App) buildLogin() *cobra.Command {
	var (
		email     string
		password  string
		mfaCode   string
		mfaSecret string
	)

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Log in to Monarch Money",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, a.Flags.JSONMode, a.Flags.Pretty)

			// Priority: Flags > Env Vars > Prompt. Read flag values from
			// the closure-captured locals (per-App, never shared) and fall
			// back to a.Deps.Viper which picks up MONARCH_* env vars via
			// AutomaticEnv configured in DefaultDeps.
			v := a.Deps.Viper
			if email == "" {
				email = v.GetString("email")
			}
			if password == "" {
				password = v.GetString("password")
			}
			if mfaCode == "" {
				mfaCode = v.GetString("mfa-code")
			}
			if mfaSecret == "" {
				mfaSecret = v.GetString("mfa-secret")
			}

			if email == "" {
				fmt.Fprint(a.Deps.Stdout, "Email: ")
				a.Deps.ScanInput(&email)
			}

			if password == "" {
				fmt.Fprint(a.Deps.Stdout, "Password: ")
				bytePassword, err := a.Deps.ReadPassword(int(os.Stdin.Fd()))
				fmt.Fprintln(a.Deps.Stdout)
				if err != nil {
					a.handleError(renderer, "login", errors.New(errors.InternalError, "failed to read password", errors.CatInternal, false, err), start)
					return
				}
				password = string(bytePassword)
			}

			sess, err := a.Deps.Authenticate(email, password, mfaCode, mfaSecret)

			// Handle MFA requirement if not already provided
			if err != nil {
				if e, ok := err.(*errors.Error); ok && e.Code == errors.AuthMFARequired && !a.Flags.JSONMode {
					fmt.Fprint(a.Deps.Stdout, "MFA Code: ")
					a.Deps.ScanInput(&mfaCode)
					sess, err = a.Deps.Authenticate(email, password, mfaCode, mfaSecret)
				}
			}

			if err != nil {
				var cliErr *errors.Error
				if e, ok := err.(*errors.Error); ok {
					cliErr = e
				} else {
					cliErr = errors.New(errors.InternalError, "authentication failed", errors.CatInternal, false, err)
				}
				a.handleError(renderer, "auth.login", cliErr, start)
				return
			}

			sess.Profile = a.Flags.Profile
			store := a.Deps.NewStore(a.Deps.SessionPath())
			if err := store.Save(sess); err != nil {
				a.handleError(renderer, "auth.login", errors.New(errors.InternalError, "failed to save session", errors.CatInternal, false, err), start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("auth.login", a.Flags.Profile, output.SchemaVersion, "", map[string]interface{}{
					"status":       "logged in",
					"email":        sess.Email,
					"profile":      sess.Profile,
					"created_at":   sess.CreatedAt,
					"updated_at":   sess.UpdatedAt,
					"session_path": a.Deps.SessionPath(),
				}, time.Since(start))
				renderer.RenderSuccess(env)
			} else {
				fmt.Fprintf(a.Deps.Stdout, "Successfully logged in as %s.\n", sess.Email)
				fmt.Fprintf(a.Deps.Stdout, "Logged in at: %s\n", sess.CreatedAt.Format(time.RFC3339))
				fmt.Fprintf(a.Deps.Stdout, "Session token saved to: %s\n", a.Deps.SessionPath())
			}
		},
	}

	cmd.Flags().StringVar(&email, "email", "", "email address")
	cmd.Flags().StringVar(&password, "password", "", "password")
	cmd.Flags().StringVar(&mfaCode, "mfa-code", "", "6-digit MFA code")
	cmd.Flags().StringVar(&mfaSecret, "mfa-secret", "", "TOTP secret key for automatic MFA")

	return cmd
}

func (a *App) buildStatus() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check authentication status",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, a.Flags.JSONMode, a.Flags.Pretty)

			store := a.Deps.NewStore(a.Deps.SessionPath())
			sess, err := store.Load()
			if err != nil {
				a.handleError(renderer, "auth.status", errors.New(errors.AuthRequired, "not logged in", errors.CatAuth, false, err), start)
				return
			}

			identity, err := a.Deps.FetchIdentity(cmd.Context(), sess.Token)
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

			data := map[string]interface{}{
				"authenticated": true,
				"session_valid": true,
				"email":         displayEmail,
				"profile":       sess.Profile,
				"created_at":    sess.CreatedAt,
				"updated_at":    sess.UpdatedAt,
				"session_path":  a.Deps.SessionPath(),
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("auth.status", a.Flags.Profile, output.SchemaVersion, "", data, time.Since(start))
				renderer.RenderSuccess(env)
			} else {
				fmt.Fprintf(a.Deps.Stdout, "Authenticated: yes\n")
				fmt.Fprintf(a.Deps.Stdout, "Email: %s\n", displayEmail)
				fmt.Fprintf(a.Deps.Stdout, "Profile: %s\n", sess.Profile)
				fmt.Fprintf(a.Deps.Stdout, "Logged in at: %s\n", sess.CreatedAt.Format(time.RFC3339))
				fmt.Fprintf(a.Deps.Stdout, "Session valid: yes\n")
				fmt.Fprintf(a.Deps.Stdout, "Session path: %s\n", a.Deps.SessionPath())
			}
		},
	}
}

func (a *App) buildLogout() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Log out and remove local session",
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			renderer := output.NewRenderer(a.Deps.Stdout, a.Deps.Stderr, a.Flags.JSONMode, a.Flags.Pretty)

			store := a.Deps.NewStore(a.Deps.SessionPath())
			if err := store.Delete(); err != nil && !os.IsNotExist(err) {
				a.handleError(renderer, "auth.logout", errors.New(errors.InternalError, "failed to delete session", errors.CatInternal, false, err), start)
				return
			}

			if a.Flags.JSONMode {
				env := output.NewEnvelope("auth.logout", a.Flags.Profile, output.SchemaVersion, "", map[string]string{"status": "logged out"}, time.Since(start))
				renderer.RenderSuccess(env)
			} else {
				fmt.Fprintln(a.Deps.Stdout, "Successfully logged out.")
			}
		},
	}
}

func (a *App) buildSessionPath() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Print the path to the session file",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintln(a.Deps.Stdout, a.Deps.SessionPath())
		},
	}
}

// handleError formats and renders a structured error, then exits with the
// error's recommended exit code. For AUTH_SESSION_EXPIRED, it enriches the
// message with the email and session path from the on-disk session, when
// available, so users know which session expired.
func (a *App) handleError(r *output.Renderer, command string, err *errors.Error, start time.Time) {
	if err != nil && err.Code == errors.AuthSessionExpired {
		store := a.Deps.NewStore(a.Deps.SessionPath())
		if sess, loadErr := store.Load(); loadErr == nil {
			if sess.Email != "" {
				err = errors.New(err.Code, fmt.Sprintf("session token for %s stored at %s expired or invalid; run `monarch auth login` again", sess.Email, a.Deps.SessionPath()), err.Category, err.Retryable, err.Err)
			} else {
				err = errors.New(err.Code, fmt.Sprintf("session token stored at %s expired or invalid; run `monarch auth login` again", a.Deps.SessionPath()), err.Category, err.Retryable, err.Err)
			}
		}
	}

	env := output.NewErrorEnvelope(command, a.Flags.Profile, output.SchemaVersion, err, time.Since(start))
	r.RenderError(env)
	a.Deps.Exit(err.ExitCode())
}
