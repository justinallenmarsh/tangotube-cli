package commands

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/justinallenmarsh/tangotube-cli/internal/output"
)

var firstSteps = []string{
	`tt search "di sarli noelia"`,
	`tt clip tags`,
	`tt setup`,
}

// NewRoot builds the whole tt command tree.
func NewRoot(a *App) *cobra.Command {
	root := &cobra.Command{
		Use:   "tt",
		Short: "Argentine tango videos, in the terminal and in your coding agent's hands.",
		Long: `Argentine tango videos, in the terminal and in your coding agent's hands.

Search dancers, orchestras, songs… and find the performance. Make a practice
clip of the eight seconds you want to loop. At a terminal the output is made
for reading. Piped, a command that returns data writes JSON.`,
		Example: `  tt search "di sarli noelia"
  tt search "sacada" --technique sacada --dancer "noelia hurtado"
  tt clip create uGwRPRusbC0 --start 1:12 --end 1:18 --tag sacada`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if f := cmd.Flags().Lookup("limit"); f != nil && f.Changed && (a.Flags.Limit < 1 || a.Flags.Limit > limitMax(cmd)) {
				return Usage(fmt.Sprintf("--limit is a number from 1 to %d", limitMax(cmd)), cmd.CommandPath()+" --limit 20")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.home()
		},
	}
	f := root.PersistentFlags()
	f.BoolVar(&a.Flags.JSON, "json", false, "Write the JSON envelope, even at a terminal.")
	f.BoolVar(&a.Flags.Quiet, "quiet", false, "Write only the data, as JSON.")
	f.BoolVar(&a.Flags.Agent, "agent", false, "Run for a coding agent: JSON out, never prompt, never open a browser.")
	f.BoolVar(&a.Flags.Verbose, "verbose", false, "Show each API request on stderr.")
	f.IntVar(&a.Flags.Limit, "limit", 0, "How many results to return, a `NUMBER` up to 50.")
	f.StringVar(&a.Flags.APIURL, "api-url", "", "Talk to another TangoTube at this `URL`, like http://localhost:3000.")
	f.StringVar(&a.Flags.Token, "token", "", "Use this `TOKEN` instead of the stored one.")
	f.StringVar(&a.Flags.JQ, "jq", "", "Filter the data with a jq `EXPRESSION`, like '.videos[].id'.")

	f.BoolP("help", "h", false, "Show help for a command.")
	root.SetOut(a.Out)
	root.SetErr(a.Err)
	root.SetIn(a.In)
	root.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		return Usage(flagError(err), cmd.CommandPath()+" --help")
	})
	cobra.AddTemplateFunc("globalFlags", globalFlags)
	root.SetHelpTemplate(helpTemplate)
	root.SetUsageTemplate(usageTemplate)
	root.CompletionOptions.HiddenDefaultCmd = true

	root.AddCommand(
		newSearch(a), newFacets(a), newResolve(a), newHome(a),
		newVideo(a), newDancer(a), newCouple(a), newOrchestra(a), newSong(a), newEvent(a), newChannel(a),
		newDancers(a), newCouples(a), newOrchestras(a), newSongs(a), newEvents(a), newChannels(a),
		newSingers(a), newChampions(a),
		newClip(a), newLike(a), newUnlike(a), newLikes(a), newHistory(a), newPlaylist(a),
		newFollow(a), newUnfollow(a), newFollowing(a),
		newPractice(a), newSavedSearch(a), newNotifications(a), newImport(a),
		newQueue(a), newIdentify(a), newSuggest(a), newConfirm(a), newAgree(a), newReport(a),
		newPerformance(a), newPartners(a), newAdmin(a),
		newOpen(a), newAuth(a), newDoctor(a), newSetup(a), newSkill(a),
		newCommands(a), newMCP(a), newVersion(a), newUpgrade(a), newDanceCmd(a),
	)
	return root
}

func (a *App) home() error {
	p := a.Printer()
	if p.Mode != output.Human || p.JQ != "" {
		env := output.NewEnvelope(map[string]any{
			"name":    "tt",
			"version": Version,
			"api_url": a.BaseURL(),
		}, "TangoTube CLI "+Version, firstSteps...)
		return p.Success(env, nil)
	}
	fmt.Fprintln(a.Out)
	a.masthead(p)
	fmt.Fprintln(a.Out)
	fmt.Fprintln(a.Out, p.Style.Text("Discover Tango Videos."))
	p.Breadcrumbs(firstSteps)
	return nil
}

const helpTemplate = `{{with or .Long .Short}}{{. | trimTrailingWhitespaces}}{{end}}

{{.UsageString}}`

const usageTemplate = `Usage
  {{if .Runnable}}{{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}{{if .Runnable}}
{{end}}  {{.CommandPath}} COMMAND{{end}}{{if .HasExample}}

Examples
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}

Commands{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }}  {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

Global flags
{{globalFlags . | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableSubCommands}}

Run "{{.CommandPath}} COMMAND --help" to learn a command.{{end}}
`

// listsThings marks a command that returns a list, so --limit means something
// to it. Help leaves --limit off every other command.
const listsThings = "lists"

// limitUpTo, on a command that takes more than 50, is how many it takes and
// its default, as help says it: "3000 (500 by default)".
const limitUpTo = "limit"

// limitMax is the most --limit a command takes: 50 unless it says more.
func limitMax(cmd *cobra.Command) int {
	if n, err := strconv.Atoi(strings.Fields(cmd.Annotations[limitUpTo] + " 50")[0]); err == nil {
		return n
	}
	return 50
}

func globalFlags(cmd *cobra.Command) string {
	shown := pflag.NewFlagSet(cmd.Name(), pflag.ContinueOnError)
	cmd.InheritedFlags().VisitAll(func(f *pflag.Flag) {
		if f.Name == "limit" && cmd.Annotations[listsThings] == "" {
			return
		}
		if f.Name == "limit" && cmd.Annotations[limitUpTo] != "" {
			own := *f
			own.Usage = "How many results to return, a `NUMBER` up to " + cmd.Annotations[limitUpTo] + "."
			f = &own
		}
		shown.AddFlag(f)
	})
	return shown.FlagUsages()
}

var badFlagValue = regexp.MustCompile(`invalid argument "(.*)" for "(?:-\w, )?(--[\w-]+)" flag`)

// flagError rewrites pflag's parser errors as a sentence a person can act on:
// `"abc" is not a number for --limit`.
func flagError(err error) string {
	if m := badFlagValue.FindStringSubmatch(err.Error()); m != nil {
		return fmt.Sprintf("%q is not a number for %s", m[1], m[2])
	}
	return err.Error()
}
