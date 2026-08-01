package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/thedavidweng/monarchmoney-cli/internal/auth"
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

type identityResult struct {
	Email string
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
				fmt.Fprint(cmd.ErrOrStderr(), "Email: ")
				if _, err := fmt.Fscan(cmd.InOrStdin(), &resolvedEmail); err != nil {
					a.handleError(renderer, "auth.login", errors.New(errors.InvalidArguments, "failed to read email", errors.CatValidation, false, err), start)
					return
				}
			}

			if resolvedPassword == "" {
				fmt.Fprint(cmd.ErrOrStderr(), "Password: ")
				passwordBytes, err := a.Deps.ReadPassword()
				fmt.Fprintln(cmd.ErrOrStderr())
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
				fmt.Fprint(cmd.ErrOrStderr(), "MFA Code: ")
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
				renderer.RenderSuccess(env)
				return
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully logged in as %s.\n", sess.Email)
			fmt.Fprintf(cmd.OutOrStdout(), "Logged in at: %s\n", sess.CreatedAt.Format(time.RFC3339))
			fmt.Fprintf(cmd.OutOrStdout(), "Session token saved to: %s\n", a.sessionPath())
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
				renderer.RenderSuccess(env)
				return
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Authenticated: yes")
			fmt.Fprintf(cmd.OutOrStdout(), "Email: %s\n", displayEmail)
			fmt.Fprintf(cmd.OutOrStdout(), "Profile: %s\n", sess.Profile)
			fmt.Fprintf(cmd.OutOrStdout(), "Logged in at: %s\n", sess.CreatedAt.Format(time.RFC3339))
			fmt.Fprintln(cmd.OutOrStdout(), "Session valid: yes")
			fmt.Fprintf(cmd.OutOrStdout(), "Session path: %s\n", a.sessionPath())
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
				renderer.RenderSuccess(env)
				return
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Successfully logged out.")
		},
	}

	sessionCommand := &cobra.Command{Use: "session", Short: "Manage local session"}
	sessionPathCommand := &cobra.Command{
		Use:   "path",
		Short: "Print the path to the session file",
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Fprintln(cmd.OutOrStdout(), a.sessionPath())
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
