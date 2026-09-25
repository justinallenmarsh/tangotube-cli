package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/justinallenmarsh/tangotube-cli/internal/api"
	"github.com/justinallenmarsh/tangotube-cli/internal/output"
)

// Check is one line of tt doctor.
type Check struct {
	Name   string `json:"name"`
	Status string `json:"status"` // ok, warn, fail
	Detail string `json:"detail"`
	Hint   string `json:"hint,omitempty"`
}

func newDoctor(a *App) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check that tt can reach TangoTube, and say how to fix what it can't.",
		Long: `Check that tt is installed, can reach TangoTube, knows who you are, and that
your coding agents have the skill. Every problem comes with the command that
fixes it.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			checks := a.runChecks(cmd)
			failed, warned := 0, 0
			for _, c := range checks {
				switch c.Status {
				case "fail":
					failed++
				case "warn":
					warned++
				}
			}
			summary := "Everything is in step."
			switch {
			case failed > 0:
				summary = fmt.Sprintf("%d problem%s to fix.", failed, plural(failed))
			case warned > 0:
				summary = fmt.Sprintf("Working. %d thing%s worth a look.", warned, plural(warned))
			}
			var crumbs []string
			for _, c := range checks {
				if c.Hint != "" && c.Status != "ok" {
					crumbs = append(crumbs, c.Hint)
				}
			}
			env := output.NewEnvelope(map[string]any{"checks": checks, "api_url": a.BaseURL(), "version": Version}, summary, crumbs...)
			exit := 0
			for _, c := range checks {
				if c.Status == "fail" && exit == 0 {
					code := output.CodeNetwork
					if c.Name == "token" {
						code = output.CodeAuth
					}
					exit = output.ExitCode(code)
					env.OK = false
					env.Error = &output.Error{Code: code, Message: c.Name + ": " + c.Detail, Hint: c.Hint, Retryable: code == output.CodeNetwork}
				}
			}
			p := a.Printer()
			if p.Mode == output.Human && p.JQ == "" {
				fmt.Fprintln(a.Out)
				a.banner(p)
			}
			if err := p.Success(env, func(p *output.Printer, _ json.RawMessage) {
				fmt.Fprintln(p.Out)
				for _, c := range checks {
					mark := p.Style.Ok("✓")
					switch c.Status {
					case "warn":
						mark = p.Style.Gold("•")
					case "fail":
						mark = p.Style.Error("✗")
					}
					fmt.Fprintf(p.Out, "  %s %s %s\n", mark, p.Style.Text(fmt.Sprintf("%-8s", c.Name)), p.Style.Dim(c.Detail))
				}
			}); err != nil {
				return err
			}
			if exit != 0 {
				return &ExitError{Code: exit}
			}
			return nil
		},
	}
}

func (a *App) runChecks(cmd *cobra.Command) []Check {
	var checks []Check

	// The binary.
	self, _ := os.Executable()
	onPath, err := exec.LookPath("tt")
	switch {
	case err != nil:
		checks = append(checks, Check{"tt", "warn", fmt.Sprintf("%s at %s, not on your PATH", Version, tilde(self, a.Home)), installCommand})
	default:
		checks = append(checks, Check{"tt", "ok", fmt.Sprintf("%s at %s", Version, tilde(onPath, a.Home)), ""})
	}

	// The API.
	client := a.Client()
	token := client.Token
	client.Token = "" // reachability is a public question
	start := time.Now()
	_, err = client.Get(ctx(cmd), "/api/v1/tags", nil)
	apiOK := err == nil
	if apiOK {
		checks = append(checks, Check{"api", "ok", fmt.Sprintf("%s answered in %s", a.BaseURL(), time.Since(start).Round(time.Millisecond)), ""})
	} else {
		checks = append(checks, Check{"api", "fail", fmt.Sprintf("%s: %s", a.BaseURL(), err.Error()), "tt doctor --verbose"})
	}

	// The token.
	_, source := a.Store().Resolve(a.Flags.Token)
	switch {
	case token == "":
		checks = append(checks, Check{"token", "warn", "none — search still works; making clips needs one", "tt auth login"})
	case !apiOK:
		checks = append(checks, Check{"token", "warn", "found in " + a.Store().Describe(source) + ", not checked", ""})
	default:
		client.Token = token
		env, err := client.Do(ctx(cmd), "GET", "/api/v1/me", nil, nil, true)
		if err != nil {
			hint := "tt auth login"
			if apiErr, ok := api.AsError(err); ok && apiErr.Code() != output.CodeAuth {
				hint = "tt doctor --verbose"
			}
			checks = append(checks, Check{"token", "fail", "rejected: " + err.Error(), hint})
		} else {
			me := decode[meData](env.Data)
			checks = append(checks, Check{"token", "ok", fmt.Sprintf("%s, %s, from %s", me.User.Name, strings.Join(me.Token.Scopes, "+"), a.Store().Describe(source)), ""})
		}
	}

	// The skill, for every agent that lives here.
	found := false
	for _, t := range a.skillTargets("") {
		if !t.detected {
			continue
		}
		found = true
		path := filepath.Join(t.dir, "SKILL.md")
		switch installed, current := skillState(path); {
		case !installed:
			checks = append(checks, Check{"skill", "warn", t.agent + ": not installed", "tt setup " + t.agent})
		case !current:
			checks = append(checks, Check{"skill", "warn", t.agent + ": older than this tt", "tt setup " + t.agent})
		default:
			checks = append(checks, Check{"skill", "ok", t.agent + ": " + tilde(path, a.Home), ""})
		}
	}
	if !found {
		checks = append(checks, Check{"skill", "ok", "no coding agents found on this machine", ""})
	}
	return checks
}
