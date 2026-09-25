package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/justinallenmarsh/tangotube-cli/internal/api"
	"github.com/justinallenmarsh/tangotube-cli/internal/output"
)

// CommandInfo describes one command for tt commands --json.
type CommandInfo struct {
	Path     string     `json:"path"`
	Use      string     `json:"use"`
	Summary  string     `json:"summary"`
	Examples []string   `json:"examples,omitempty"`
	Flags    []FlagInfo `json:"flags,omitempty"`
}

// FlagInfo describes one flag.
type FlagInfo struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Usage   string `json:"usage"`
	Default string `json:"default,omitempty"`
}

func newCommands(a *App) *cobra.Command {
	return &cobra.Command{
		Use:   "commands",
		Short: "List every tt command, with its flags. Agents: use --json.",
		Long: `List every tt command. With --json it is a machine-readable map of the
whole CLI — commands, flags, examples — for an agent that wants the lay of
the land in one call.`,
		Example: `  tt commands
  tt commands --json --jq '.commands[].path'`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			infos := collect(cmd.Root())
			var global []FlagInfo
			cmd.Root().PersistentFlags().VisitAll(func(f *pflag.Flag) { global = append(global, flagInfo(f)) })
			env := output.NewEnvelope(map[string]any{"commands": infos, "global_flags": global},
				fmt.Sprintf("%d commands", len(infos)), "tt search --help")
			return a.Printer().Success(env, func(p *output.Printer, _ json.RawMessage) {
				rows := make([][]string, 0, len(infos))
				for _, c := range infos {
					rows = append(rows, []string{c.Path, c.Summary})
				}
				p.Table([]output.Column{{Header: "COMMAND", ID: true}, {Header: "WHAT IT DOES", Flex: true}}, rows)
			})
		},
	}
}

func collect(cmd *cobra.Command) []CommandInfo {
	var out []CommandInfo
	for _, c := range cmd.Commands() {
		if c.Hidden || c.Name() == "help" {
			continue
		}
		if c.Runnable() {
			info := CommandInfo{Path: c.CommandPath(), Use: c.UseLine(), Summary: c.Short}
			for _, ex := range strings.Split(c.Example, "\n") {
				if ex = strings.TrimSpace(ex); ex != "" {
					info.Examples = append(info.Examples, ex)
				}
			}
			c.LocalNonPersistentFlags().VisitAll(func(f *pflag.Flag) { info.Flags = append(info.Flags, flagInfo(f)) })
			out = append(out, info)
		}
		out = append(out, collect(c)...)
	}
	return out
}

func flagInfo(f *pflag.Flag) FlagInfo {
	def := f.DefValue
	if def == "false" || def == "0" || def == "[]" {
		def = ""
	}
	return FlagInfo{Name: "--" + f.Name, Type: f.Value.Type(), Usage: f.Usage, Default: def}
}

func newVersion(a *App) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the tt version.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			data := map[string]string{"version": Version, "commit": Commit, "date": Date, "go": runtime.Version(), "platform": runtime.GOOS + "/" + runtime.GOARCH}
			env := output.NewEnvelope(data, "")
			return a.Printer().Success(env, func(p *output.Printer, _ json.RawMessage) {
				fmt.Fprintf(p.Out, "%s %s %s\n", p.Style.Mark("tt"), p.Style.Text(Version), p.Style.Dim(fmt.Sprintf("(%s, %s, %s)", Commit, Date, data["platform"])))
			})
		},
	}
}

// latestReleaseURL answers {"tag_name": "v0.2.0"}. TANGOTUBE_RELEASES_URL
// points it elsewhere for tests.
const latestReleaseURL = "https://api.github.com/repos/justinallenmarsh/tangotube-cli/releases/latest"

// upgradePlan is how this copy of tt was installed, and so how it upgrades.
type upgradePlan struct {
	Method  string `json:"method"` // script, mise, go, source
	Command string `json:"command"`
}

func planUpgrade(self, home string, env func(string) string) upgradePlan {
	// Windows paths compare as slashes, so one set of rules reads both.
	slash := func(p string) string { return strings.TrimRight(strings.ReplaceAll(p, `\`, "/"), "/") }
	gobins := []string{env("GOBIN")}
	if gopath := env("GOPATH"); gopath != "" {
		gobins = append(gobins, gopath+"/bin")
	}
	gobins = append(gobins, home+"/go/bin")
	self = slash(self)
	dir := self[:max(strings.LastIndex(self, "/"), 0)]
	switch {
	case strings.Contains(self, "/mise/installs/"):
		return upgradePlan{"mise", "mise upgrade github:justinallenmarsh/tangotube-cli"}
	case Version == "dev" || strings.HasSuffix(Version, "-dirty"):
		return upgradePlan{"source", "git pull && make build"}
	}
	for _, bin := range gobins {
		if bin != "" && dir == slash(bin) {
			return upgradePlan{"go", "go install github.com/justinallenmarsh/tangotube-cli/cmd/tt@latest"}
		}
	}
	return upgradePlan{"script", installCommand}
}

func latestRelease(c context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(c, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub answered HTTP %d", resp.StatusCode)
	}
	var release struct {
		Tag string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil || release.Tag == "" {
		return "", fmt.Errorf("GitHub did not name a release")
	}
	return release.Tag, nil
}

func newUpgrade(a *App) *cobra.Command {
	return &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade tt to the latest release.",
		Long: `Upgrade tt to the latest release. tt asks GitHub for the newest release first,
and says so when you already have it.

A tt installed by the install script is replaced where it lives. One
installed by mise or go install is upgraded the same way it came, and tt
prints that command instead of running someone else's package manager. Under
--agent, or piped, it only reports what it would do.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			self, _ := os.Executable()
			plan := planUpgrade(self, a.Home, a.Env)
			url := a.Env("TANGOTUBE_RELEASES_URL")
			if url == "" {
				url = latestReleaseURL
			}
			latest, err := latestRelease(ctx(cmd), url)
			if err != nil {
				return a.Fail(api.Fail(output.CodeNetwork, "Could not ask GitHub for the latest tt: "+err.Error(), plan.Command))
			}
			data := map[string]string{"current": Version, "latest": latest, "method": plan.Method, "command": plan.Command}
			if strings.TrimPrefix(latest, "v") == strings.TrimPrefix(Version, "v") {
				return a.Printer().Success(output.NewEnvelope(data, "tt "+Version+" is the latest release."), nil)
			}
			summary := fmt.Sprintf("tt %s is out; this is %s.", latest, Version)
			if !a.Interactive() || plan.Method != "script" {
				return a.Printer().Success(output.NewEnvelope(data, summary+" Upgrade with the command below.", plan.Command), nil)
			}
			run := installer()
			run.Stdout, run.Stderr, run.Stdin = a.Out, a.Err, a.In
			run.Env = append(os.Environ(), "TANGOTUBE_BIN_DIR="+dirOf(self), "TANGOTUBE_SKIP_SETUP=1", "TANGOTUBE_VERSION="+latest)
			if err := run.Run(); err != nil {
				return a.Fail(api.Fail(output.CodeNetwork, "The install script failed: "+err.Error(), installCommand))
			}
			return nil
		},
	}
}

func dirOf(path string) string {
	if i := strings.LastIndex(path, string(os.PathSeparator)); i > 0 {
		return path[:i]
	}
	return "."
}
