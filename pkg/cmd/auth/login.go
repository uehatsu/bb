package auth

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/uehatsu/bb/internal/api"
	"github.com/uehatsu/bb/internal/bitbucket"
	"github.com/uehatsu/bb/internal/cmdutil"
	"github.com/uehatsu/bb/internal/config"
)

// LoginOptions holds flags for `bb auth login`.
type LoginOptions struct {
	WithToken bool
	Bearer    bool
	Email     string
	ExpiresIn string
	Web       bool

	// for tests
	newClient func(config.Credential) *api.Client
}

// NewCmdLogin returns the login command.
func NewCmdLogin(f *cmdutil.Factory) *cobra.Command {
	opts := &LoginOptions{}
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Log in to Bitbucket Cloud with an API token",
		Long: fmt.Sprintf(`Authenticate with Bitbucket Cloud.

Create an API token with scopes at:
  %s
Choose "Create API token with scopes", select the Bitbucket app, and grant the
scopes bb needs (see 'bb auth login --help' output below). Copy the token; it is
shown only once.

By default the token is read interactively together with your Atlassian
account email. The token is stored in %s/hosts.yml (mode 0600).

Use --with-token to read the token from standard input, e.g.
  echo "you@example.com:ATATT3x..." | bb auth login --with-token
  echo "ATATT3x..." | bb auth login --with-token --email you@example.com

Use --bearer for repository/project/workspace access tokens, which are sent as
Bearer tokens and do not need an email.

Recommended scopes:
%s`, TokenURL, config.Dir(), scopeTable()),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.Web {
				return errors.New("--web (OAuth) is not implemented yet; use an API token")
			}
			if !opts.WithToken && !f.IOStreams.CanPrompt() {
				return cmdutil.FlagErrorf("--with-token required when not running interactively")
			}
			return runLogin(cmd.Context(), f, opts)
		},
	}
	cmd.Flags().BoolVar(&opts.WithToken, "with-token", false, "Read token from standard input")
	cmd.Flags().BoolVar(&opts.Bearer, "bearer", false, "Token is a repository/project/workspace access token (Bearer)")
	cmd.Flags().StringVar(&opts.Email, "email", "", "Atlassian account email (for API tokens)")
	cmd.Flags().StringVar(&opts.ExpiresIn, "expires-in", "", "Token lifetime for expiry warnings, e.g. 90d, 1y (optional)")
	cmd.Flags().BoolVar(&opts.Web, "web", false, "Log in via OAuth in a browser (not yet supported)")
	return cmd
}

func scopeTable() string {
	var b strings.Builder
	for _, s := range RecommendedScopes {
		fmt.Fprintf(&b, "  %-30s %s\n", s.Scope, s.Purpose)
	}
	return b.String()
}

func runLogin(ctx context.Context, f *cmdutil.Factory, opts *LoginOptions) error {
	io := f.IOStreams
	cs := io.ColorScheme()
	cfg, err := f.Config()
	if err != nil {
		return err
	}

	cred := config.Credential{Method: config.AuthAPIToken, Email: strings.TrimSpace(opts.Email)}
	if opts.Bearer {
		cred.Method = config.AuthBearer
	}

	if opts.WithToken {
		email, token, err := readTokenFromStdin(io.In)
		if err != nil {
			return err
		}
		if email != "" {
			cred.Email = email
		}
		cred.Token = token
	} else {
		fmt.Fprintf(io.ErrOut, "%s Create an API token with scopes at %s\n", cs.Yellow("!"), TokenURL)
		if cred.Method == config.AuthAPIToken && cred.Email == "" {
			cred.Email, err = f.Prompter.Input("Atlassian account email", "you@example.com")
			if err != nil {
				return cmdutil.CancelError
			}
			cred.Email = strings.TrimSpace(cred.Email)
		}
		cred.Token, err = f.Prompter.Password("Paste your API token")
		if err != nil {
			return cmdutil.CancelError
		}
		cred.Token = strings.TrimSpace(cred.Token)
		if opts.ExpiresIn == "" {
			opts.ExpiresIn, _ = f.Prompter.Input("Token expiry (e.g. 90d, 1y; leave blank to skip)", "")
		}
	}

	if cred.Token == "" {
		return errors.New("token must not be empty")
	}
	if cred.Method == config.AuthAPIToken && cred.Email == "" {
		return errors.New("email is required for API tokens (use --email or --bearer)")
	}
	if opts.ExpiresIn != "" {
		d, err := parseLifetime(opts.ExpiresIn)
		if err != nil {
			return err
		}
		t := time.Now().Add(d)
		cred.ExpiresAt = &t
	}

	// Verify the credential.
	newClient := opts.newClient
	if newClient == nil {
		newClient = func(c config.Credential) *api.Client { return api.NewClient(api.NewAuthenticator(c)) }
	}
	client := newClient(cred)
	var user bitbucket.User
	if _, err := client.Do(ctx, api.Request{Path: "/user"}, &user); err != nil {
		var herr *api.HTTPError
		if errors.As(err, &herr) && (herr.StatusCode == 401 || herr.StatusCode == 403) {
			return fmt.Errorf("token verification failed (%s). Check the token, its scopes (needs read:user:bitbucket), and that the email matches your Atlassian account", herr.Error())
		}
		return fmt.Errorf("token verification failed: %w", err)
	}
	cred.User = user.Name()

	if err := cfg.Credentials().Set(config.DefaultHost, cred); err != nil {
		return fmt.Errorf("saving credentials: %w", err)
	}
	fmt.Fprintf(io.ErrOut, "%s Logged in to bitbucket.org as %s\n", cs.SuccessIcon(), cs.Bold(user.Name()))
	if cred.ExpiresAt != nil {
		fmt.Fprintf(io.ErrOut, "  Token expiry recorded as %s\n", cred.ExpiresAt.Format("2006-01-02"))
	}
	fmt.Fprintf(io.ErrOut, "  Run `bb auth setup-git` to use this token for git over HTTPS.\n")
	return nil
}

// readTokenFromStdin parses "email:token" or "token" from the first line.
func readTokenFromStdin(r io.Reader) (email, token string, err error) {
	line, err := bufio.NewReader(r).ReadString('\n')
	if err != nil && err != io.EOF {
		return "", "", err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return "", "", errors.New("no token provided on standard input")
	}
	if i := strings.Index(line, ":"); i > 0 && strings.Contains(line[:i], "@") {
		return line[:i], line[i+1:], nil
	}
	return "", line, nil
}

// parseLifetime accepts Go durations plus d/w/y suffixes.
func parseLifetime(s string) (time.Duration, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0, errors.New("empty lifetime")
	}
	unit := s[len(s)-1]
	mult := map[byte]time.Duration{'d': 24 * time.Hour, 'w': 7 * 24 * time.Hour, 'y': 365 * 24 * time.Hour}
	if m, ok := mult[unit]; ok {
		var n int
		if _, err := fmt.Sscanf(s[:len(s)-1], "%d", &n); err != nil || n <= 0 {
			return 0, fmt.Errorf("invalid lifetime %q (examples: 90d, 12w, 1y)", s)
		}
		return time.Duration(n) * m, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid lifetime %q (examples: 90d, 12w, 1y)", s)
	}
	return d, nil
}
