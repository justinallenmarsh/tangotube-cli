// Package commands is the tt command tree.
package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/justinallenmarsh/tangotube-cli/internal/api"
	"github.com/justinallenmarsh/tangotube-cli/internal/auth"
	"github.com/justinallenmarsh/tangotube-cli/internal/output"
)

// Build facts, set by the linker.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// Flags are the global flags every command understands.
type Flags struct {
	JSON    bool
	Quiet   bool
	Agent   bool
	Verbose bool
	Limit   int
	APIURL  string
	Token   string
	JQ      string
}

// App is what every command shares: where to write, how to reach the API.
type App struct {
	Out, Err io.Writer
	In       io.Reader
	Env      func(string) string
	Flags    Flags
	TTY      bool // stdout is a terminal
	StdinTTY bool // stdin is a terminal: someone is there to watch
	// Plain is a terminal that cannot read escape sequences: an old Windows
	// console. It gets no colour, links, pictures or animation.
	Plain  bool
	Width  int
	Height int
	Home   string
	Cwd    string

	// Cell reports a terminal cell's size in pixels. Tests swap it out; nil
	// asks the terminal.
	Cell func() (float64, float64)

	// Browser opens a URL. Tests swap it out.
	Browser func(string) error
	// NewStore builds the token store for a base URL. Tests swap it out.
	NewStore func(baseURL string) *auth.Store
}

// NewApp wires an App to the real process.
func NewApp() *App {
	home, _ := os.UserHomeDir()
	cwd, _ := os.Getwd()
	tty := term.IsTerminal(int(os.Stdout.Fd()))
	width, height, plain := 0, 0, false
	if tty {
		width, height, _ = term.GetSize(int(os.Stdout.Fd()))
		plain = !enableEscapes(os.Stdout)
	}
	return &App{
		Out: os.Stdout, Err: os.Stderr, In: os.Stdin, Env: os.Getenv,
		TTY: tty, StdinTTY: term.IsTerminal(int(os.Stdin.Fd())), Plain: plain, Width: width, Height: height,
		Home: home, Cwd: cwd,
		Browser: openBrowser, NewStore: auth.NewStore,
	}
}

// ExitError carries a process exit status up to main.
type ExitError struct{ Code int }

func (e *ExitError) Error() string { return fmt.Sprintf("exit %d", e.Code) }

// Interactive is true when a person is at the keyboard and may be asked
// things: a terminal on stdout and no --agent, --json, or --quiet.
func (a *App) Interactive() bool {
	return a.TTY && !a.Flags.Agent && !a.Flags.JSON && !a.Flags.Quiet && a.Flags.JQ == ""
}

// Printer is the output for this invocation.
func (a *App) Printer() *output.Printer {
	p := &output.Printer{Out: a.Out, Err: a.Err, Width: a.Width, JQ: a.Flags.JQ}
	switch {
	case a.Flags.Quiet:
		p.Mode = output.Quiet
	case a.Flags.JSON || a.Flags.Agent || !a.TTY:
		p.Mode = output.JSON
	default:
		p.Mode = output.Human
	}
	p.Pretty = a.TTY && !a.Flags.Agent
	escapes := p.Mode == output.Human && a.Env("TERM") != "dumb" && !a.Plain
	links := escapes && a.Env("TT_NO_LINKS") == ""
	if escapes && a.Env("NO_COLOR") == "" {
		ct := a.Env("COLORTERM")
		p.Style = output.Style{
			Enabled: true,
			// Windows Terminal draws 24-bit colour and says so only with WT_SESSION.
			TrueColor: ct == "truecolor" || ct == "24bit" || a.Env("WT_SESSION") != "",
			DarkBG:    darkBackground(a.Env("COLORFGBG")),
		}
	}
	p.Style.Links = links
	return p
}

// darkBackground reads COLORFGBG ("15;0"): a background below 7 (or 8) is dark.
func darkBackground(v string) bool {
	parts := strings.Split(v, ";")
	if len(parts) < 2 {
		return false
	}
	bg := parts[len(parts)-1]
	return bg == "0" || bg == "8" || (len(bg) == 1 && bg < "7")
}

// BaseURL is --api-url, TANGOTUBE_API_URL, TANGOTUBE_DEV=1, or the live site.
func (a *App) BaseURL() string {
	switch {
	case a.Flags.APIURL != "":
		return strings.TrimRight(a.Flags.APIURL, "/")
	case a.Env("TANGOTUBE_API_URL") != "":
		return strings.TrimRight(a.Env("TANGOTUBE_API_URL"), "/")
	case a.Env("TANGOTUBE_DEV") == "1":
		return api.DevBaseURL
	}
	return api.DefaultBaseURL
}

// Store is the token store for the current API.
func (a *App) Store() *auth.Store { return a.NewStore(a.BaseURL()) }

// Client is an API client carrying whatever token can be found.
func (a *App) Client() *api.Client {
	token, source := a.Store().Resolve(a.Flags.Token)
	c := api.New(a.BaseURL(), token, Version)
	c.StoredToken = source == auth.SourceKeyring || source == auth.SourceFile
	c.Warn = a.Err
	if a.Flags.Verbose {
		c.Debug = a.Err
	}
	return c
}

// Show prints the result of an API call: the envelope on success, the error
// envelope and its exit status on failure.
func (a *App) Show(env *output.Envelope, err error, human func(*output.Printer, json.RawMessage)) error {
	return a.show(a.Printer(), env, err, human)
}

// ShowDetail is Show for one thing: its renderer supplies the heading.
func (a *App) ShowDetail(env *output.Envelope, err error, human func(*output.Printer, json.RawMessage)) error {
	p := a.Printer()
	p.HideSummary = true
	return a.show(p, env, err, human)
}

func (a *App) show(p *output.Printer, env *output.Envelope, err error, human func(*output.Printer, json.RawMessage)) error {
	if err != nil {
		return a.Fail(err)
	}
	if err := p.Success(env, human); err != nil {
		p.Failure(output.Fail(output.CodeUsage, err.Error(), ""))
		return &ExitError{Code: 1}
	}
	return nil
}

// Fail prints an error and returns the exit status for it.
func (a *App) Fail(err error) error {
	p := a.Printer()
	if apiErr, ok := api.AsError(err); ok {
		p.Failure(apiErr.Envelope)
		return &ExitError{Code: output.ExitCode(apiErr.Code())}
	}
	var exit *ExitError
	if errors.As(err, &exit) {
		return err
	}
	p.Failure(output.Fail(output.CodeUsage, err.Error(), ""))
	return &ExitError{Code: 1}
}

// Usage is a usage error with a hint.
func Usage(message, hint string) error { return api.Fail(output.CodeUsage, message, hint) }

func ctx(cmd *cobra.Command) context.Context { return cmd.Context() }

// decode unmarshals envelope data for a human renderer.
func decode[T any](raw json.RawMessage) T {
	var v T
	_ = json.Unmarshal(raw, &v)
	return v
}

// limit is --limit when given, else the fallback.
func (a *App) limit(fallback int) int {
	if a.Flags.Limit > 0 {
		return a.Flags.Limit
	}
	return fallback
}
