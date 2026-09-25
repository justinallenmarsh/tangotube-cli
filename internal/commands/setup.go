package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/justinallenmarsh/tangotube-cli/internal/output"
	"github.com/justinallenmarsh/tangotube-cli/skills"
)

type skillTarget struct {
	agent    string
	dir      string // where SKILL.md goes
	detected bool   // the agent lives on this machine or in this project
}

// skillTargets lists where the tangotube skill can go. only narrows to one
// agent by name.
func (a *App) skillTargets(only string) []skillTarget {
	var targets []skillTarget
	for _, agent := range []string{"claude", "codex", "grok"} {
		home := filepath.Join(a.Home, "."+agent)
		targets = append(targets, skillTarget{agent, filepath.Join(home, "skills", "tangotube"), isDir(home)})
	}
	for _, local := range []struct{ agent, dir string }{
		{"claude", filepath.Join(a.Cwd, ".claude", "skills")},
		{"agents", filepath.Join(a.Cwd, ".agents", "skills")},
	} {
		if isDir(local.dir) {
			targets = append(targets, skillTarget{local.agent + " (this project)", filepath.Join(local.dir, "tangotube"), true})
		}
	}
	if only == "" {
		return targets
	}
	var picked []skillTarget
	for _, t := range targets {
		if strings.HasPrefix(t.agent, only) {
			t.detected = true
			picked = append(picked, t)
		}
	}
	return picked
}

func newSetup(a *App) *cobra.Command {
	return &cobra.Command{
		Use:   "setup [claude|codex|grok]",
		Short: "Teach your coding agent to use tt: install the tangotube skill.",
		Long: `Teach your coding agent to use tt. setup copies the tangotube skill into
~/.claude/skills, ~/.codex/skills, and ~/.grok/skills for every agent it finds,
and into this project's .claude/skills or .agents/skills when those exist.

Name an agent to install for it even if tt cannot see it yet.`,
		Example: `  tt setup
  tt setup claude
  tt setup codex --agent`,
		ValidArgs: []string{"claude", "codex", "grok"},
		Args:      cobra.MatchAll(cobra.MaximumNArgs(1), cobra.OnlyValidArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			only := ""
			if len(args) == 1 {
				only = args[0]
			}
			var installed []string
			for _, t := range a.skillTargets(only) {
				if !t.detected {
					continue
				}
				if err := os.MkdirAll(t.dir, 0o755); err != nil {
					return a.Fail(Usage("Could not make "+t.dir+": "+err.Error(), ""))
				}
				path := filepath.Join(t.dir, "SKILL.md")
				if err := os.WriteFile(path, skills.TangoTube, 0o644); err != nil {
					return a.Fail(Usage("Could not write "+path+": "+err.Error(), ""))
				}
				installed = append(installed, path)
			}
			if len(installed) == 0 {
				return a.Fail(Usage("No coding agent found in ~/.claude, ~/.codex, or ~/.grok", "tt setup claude"))
			}
			summary := fmt.Sprintf("Installed the tangotube skill in %d place%s.", len(installed), plural(len(installed)))
			env := output.NewEnvelope(map[string]any{"installed": installed}, summary, `tt search "di sarli"`, "tt doctor")
			return a.Printer().Success(env, func(p *output.Printer, _ json.RawMessage) {
				for _, path := range installed {
					fmt.Fprintln(p.Out, "  "+p.Style.Ok("✓")+" "+p.Style.Text(tilde(path, a.Home)))
				}
			})
		},
	}
}

func newSkill(a *App) *cobra.Command {
	return &cobra.Command{
		Use:   "skill",
		Short: "Print the tangotube agent skill.",
		Long: `Print the tangotube agent skill — the SKILL.md that tt setup installs — so you
can read it, or pipe it wherever your agent keeps its skills.`,
		Example: `  tt skill
  tt skill > .claude/skills/tangotube/SKILL.md`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if a.Flags.JSON || a.Flags.Agent || a.Flags.Quiet || a.Flags.JQ != "" {
				env := output.NewEnvelope(map[string]string{"name": "tangotube", "content": string(skills.TangoTube)}, "The tangotube skill", "tt setup")
				return a.Printer().Success(env, nil)
			}
			_, err := a.Out.Write(skills.TangoTube)
			return err
		},
	}
}

func skillState(path string) (installed, current bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, false
	}
	return true, bytes.Equal(raw, skills.TangoTube)
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func tilde(path, home string) string {
	if home != "" && strings.HasPrefix(path, home+string(os.PathSeparator)) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}
