package commands

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/justinallenmarsh/tangotube-cli/internal/api"
	"github.com/justinallenmarsh/tangotube-cli/internal/auth"
	"github.com/justinallenmarsh/tangotube-cli/internal/output"
)

func newAuth(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Sign tt in, see who it is signed in as, or sign it out.",
		Long: `Sign tt in, see who it is signed in as, or sign it out.

Searching needs no account. Making and deleting practice clips does: tt sends a
personal access token as "Authorization: Bearer tt_live_…".

tt looks for a token in this order: --token, TANGOTUBE_TOKEN, the OS keyring,
then ~/.config/tangotube/token. Set TANGOTUBE_NO_KEYRING=1 to keep it in the
file on a headless box.`,
		Args: cobra.NoArgs,
	}
	cmd.AddCommand(newAuthLogin(a), newAuthStatus(a), newAuthLogout(a))
	return cmd
}

func newAuthLogin(a *App) *cobra.Command {
	var device, withToken, readOnly, admin bool
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Sign tt in to your TangoTube account.",
		Long: `Sign tt in to your TangoTube account.

By default tt opens tangotube.tv in your browser, you approve, and a token
named "tt on <this machine>" comes back to the terminal.

With --device, tt prints a short code instead. Type it at tangotube.tv/device
on any signed-in browser — useful when tt runs over SSH, in a container, or in
a cloud IDE.

With --admin, an operator asks for an admin token: everything under tt admin.
Only accounts that operate TangoTube can approve one, and it lapses after 90
days, when tt auth login --admin asks again.

With --with-token, tt reads a token you made in Settings → Tokens from stdin.
Agents and CI can skip login entirely and set TANGOTUBE_TOKEN.`,
		Example: `  tt auth login
  tt auth login --device
  tt auth login --admin --device
  pbpaste | tt auth login --with-token`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if admin && readOnly {
				return a.Fail(Usage("--admin and --read-only ask for opposite things", "tt auth login --admin"))
			}
			scope := "write"
			if readOnly {
				scope = "read"
			}
			if admin {
				scope = "admin"
			}
			var token string
			var err error
			switch {
			case withToken || a.Flags.Token != "":
				token, err = a.readToken()
			case device:
				token, err = a.deviceLogin(cmd.Context(), scope)
			default:
				if !a.Interactive() {
					return a.Fail(api.Fail(output.CodeAuth,
						"tt will not open a browser without a person at the terminal",
						"set TANGOTUBE_TOKEN, or run tt auth login --device"))
				}
				token, err = a.browserLogin(cmd.Context(), scope)
			}
			if err != nil {
				return a.Fail(err)
			}
			return a.saveAndGreet(cmd.Context(), token)
		},
	}
	f := cmd.Flags()
	f.BoolVar(&device, "device", false, "Sign in with a code typed on another device.")
	f.BoolVar(&withToken, "with-token", false, "Read a token from stdin.")
	f.BoolVar(&readOnly, "read-only", false, "Ask for a token that can read but not make clips.")
	f.BoolVar(&admin, "admin", false, "Ask for an admin token (operators only; lasts 90 days).")
	return cmd
}

func (a *App) readToken() (string, error) {
	if a.Flags.Token != "" {
		return a.Flags.Token, nil
	}
	line, _ := bufio.NewReader(a.In).ReadString('\n')
	token := strings.TrimSpace(line)
	if token == "" {
		return "", Usage("No token came in on stdin", "tt auth login --with-token < token.txt")
	}
	return token, nil
}

func (a *App) oauth(scope string) *auth.OAuth {
	host, _ := os.Hostname()
	if host == "" {
		host = "this machine"
	}
	name := "tt on " + host
	if scope == "admin" {
		name = "tt admin on " + host
	}
	return &auth.OAuth{BaseURL: a.BaseURL(), Scope: scope, TokenName: name}
}

func (a *App) browserLogin(parent context.Context, scope string) (string, error) {
	c, cancel := context.WithTimeout(parent, 5*time.Minute)
	defer cancel()
	p := a.Printer()
	tr, err := a.oauth(scope).Loopback(c, func(authorize string) {
		fmt.Fprintln(a.Err, p.Style.Text("Opening "+hostOf(a.BaseURL())+" to approve tt…"))
		if a.Browser(authorize) != nil {
			fmt.Fprintln(a.Err, "Open this in a browser: "+authorize)
		} else {
			fmt.Fprintln(a.Err, p.Style.Dim("If nothing opened: "+authorize))
		}
	})
	if err != nil {
		return "", api.Fail(output.CodeAuth, "Login did not finish: "+err.Error(), "tt auth login --device")
	}
	return tr.AccessToken, nil
}

func (a *App) deviceLogin(parent context.Context, scope string) (string, error) {
	o := a.oauth(scope)
	dc, err := o.StartDevice(parent)
	if err != nil {
		return "", api.Fail(output.CodeAuth, err.Error(), "tt doctor")
	}
	p := a.Printer()
	where := dc.VerificationURI
	if where == "" {
		where = a.BaseURL() + "/device"
	}
	// The code is for a person, so it goes to stderr: stdout stays the envelope.
	// An agent gets it as one JSON line it can hand to its person.
	if !a.Interactive() {
		line, _ := json.Marshal(map[string]any{
			"user_code": dc.UserCode, "verification_uri": where,
			"verification_uri_complete": dc.VerificationURIComplete, "expires_in": dc.ExpiresIn,
		})
		fmt.Fprintln(a.Err, string(line))
	}
	fmt.Fprintln(a.Err)
	fmt.Fprintln(a.Err, "  "+p.Style.Text("On any signed-in browser, go to ")+p.Style.Gold(where))
	fmt.Fprintln(a.Err, "  "+p.Style.Text("and enter ")+p.Style.Mark(dc.UserCode))
	fmt.Fprintln(a.Err)
	fmt.Fprintln(a.Err, "  "+p.Style.Dim("Waiting for approval…"))
	tr, err := o.PollDevice(parent, dc)
	if err != nil {
		return "", api.Fail(output.CodeAuth, "Login did not finish: "+err.Error(), "tt auth login --device")
	}
	return tr.AccessToken, nil
}

type meData struct {
	User struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	} `json:"user"`
	Token struct {
		Name       string   `json:"name"`
		Scopes     []string `json:"scopes"`
		MintedBy   string   `json:"minted_by"`
		CreatedAt  string   `json:"created_at"`
		LastUsedAt string   `json:"last_used_at"`
		ExpiresAt  string   `json:"expires_at,omitempty"`
		Admin      bool     `json:"admin"`
	} `json:"token"`
}

func (a *App) saveAndGreet(c context.Context, token string) error {
	client := a.Client()
	client.Token = token
	env, err := client.Do(c, "GET", "/api/v1/me", nil, nil, true)
	if err != nil {
		return a.Fail(err)
	}
	store := a.Store()
	where, err := store.Save(token)
	if err != nil {
		return a.Fail(Usage("Could not keep the token: "+err.Error(), "TANGOTUBE_NO_KEYRING=1 tt auth login"))
	}
	me := decode[meData](env.Data)
	data := map[string]any{"user": me.User, "token": me.Token, "stored_in": store.Describe(where)}
	summary := fmt.Sprintf("Signed in as %s", me.User.Name)
	next := `tt search "sacada" --technique sacada`
	if me.Token.Admin {
		summary += " · admin"
		next = "tt admin dashboard"
	}
	return a.Printer().Success(output.NewEnvelope(data, summary, "tt auth status", next), func(p *output.Printer, _ json.RawMessage) {
		p.Field("scopes", strings.Join(me.Token.Scopes, " "))
		if me.Token.ExpiresAt != "" {
			p.Field("expires", expiry(me.Token.ExpiresAt, time.Now()))
		}
		p.Field("stored in", store.Describe(where))
	})
}

func newAuthStatus(a *App) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show who tt is signed in as, and where the token lives.",
		Long:  `Show who tt is signed in as, the token's scopes, and where it is kept. Never the token itself.`,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			store := a.Store()
			token, source := store.Resolve(a.Flags.Token)
			if token == "" {
				return a.Fail(api.Fail(output.CodeAuth, "tt is not signed in. Search still works without an account.", "tt auth login"))
			}
			client := a.Client()
			client.Token = token
			env, err := client.Do(ctx(cmd), "GET", "/api/v1/me", nil, nil, true)
			if err != nil {
				return a.Fail(err)
			}
			me := decode[meData](env.Data)
			data := map[string]any{"user": me.User, "token": me.Token, "stored_in": store.Describe(source), "api_url": a.BaseURL()}
			summary := "Signed in as " + me.User.Name
			if me.Token.Admin {
				summary += " · admin"
			}
			return a.Printer().Success(output.NewEnvelope(data, summary), func(p *output.Printer, _ json.RawMessage) {
				p.Field("email", me.User.Email)
				p.Field("token", me.Token.Name)
				p.Field("scopes", strings.Join(me.Token.Scopes, " "))
				if me.Token.ExpiresAt != "" {
					p.Field("expires", expiry(me.Token.ExpiresAt, time.Now()))
				}
				p.Field("made by", me.Token.MintedBy)
				p.Field("last used", ago(me.Token.LastUsedAt, time.Now()))
				p.Field("stored in", store.Describe(source))
				p.Field("api", a.BaseURL())
			})
		},
	}
}

func newAuthLogout(a *App) *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Sign tt out: revoke the stored token and forget it.",
		Long: `Sign tt out: revoke the stored token on tangotube.tv, then wipe it from the OS
keyring and the token file. A token in TANGOTUBE_TOKEN or --token is left
alone; unset it yourself.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			store := a.Store()
			token, _ := store.Stored()
			stored := token != ""
			revoked := false
			if stored {
				// Best effort: an unreachable site still gets a local sign-out.
				client := a.Client()
				client.Token = token
				_, err := client.Do(ctx(cmd), http.MethodDelete, "/api/v1/tokens/current", nil, nil, true)
				revoked = err == nil
			}
			if err := store.Delete(); err != nil {
				return a.Fail(Usage("Could not remove the token file: "+err.Error(), ""))
			}
			var summary string
			switch {
			case !stored:
				summary = "tt was not signed in."
			case revoked:
				summary = "Signed out. The token is revoked."
			default:
				summary = "Signed out here, but tangotube.tv did not answer, so revoke the token in Settings → Tokens."
			}
			var crumbs []string
			if a.Env("TANGOTUBE_TOKEN") != "" {
				summary += " TANGOTUBE_TOKEN is still set in this shell."
				crumbs = append(crumbs, "unset TANGOTUBE_TOKEN")
			}
			data := map[string]bool{"signed_out": stored, "revoked": revoked}
			return a.Printer().Success(output.NewEnvelope(data, summary, crumbs...), nil)
		},
	}
}

// ago turns an ISO time into "3 minutes ago", for a person at a terminal.
func ago(iso string, now time.Time) string {
	t, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		return iso
	}
	d := now.Sub(t)
	unit := func(n int, word string) string {
		if n == 1 {
			return "1 " + word + " ago"
		}
		return fmt.Sprintf("%d %ss ago", n, word)
	}
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return unit(int(d.Minutes()), "minute")
	case d < 24*time.Hour:
		return unit(int(d.Hours()), "hour")
	case d < 30*24*time.Hour:
		return unit(int(d.Hours()/24), "day")
	}
	return t.Format("2 January 2006")
}

func hostOf(base string) string {
	if u, err := url.Parse(base); err == nil && u.Host != "" {
		return u.Host
	}
	return base
}

// expiry says when an admin token lapses, as a date and a countdown, and says
// so plainly once it has.
func expiry(iso string, now time.Time) string {
	t, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		return iso
	}
	date := t.Local().Format("2 Jan 2006")
	days := int(t.Sub(now).Hours() / 24)
	switch {
	case !t.After(now):
		return "expired " + date + " · run tt auth login --admin"
	case days < 1:
		return date + " (today)"
	case days == 1:
		return date + " (tomorrow)"
	default:
		return fmt.Sprintf("%s (in %d days)", date, days)
	}
}
