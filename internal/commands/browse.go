package commands

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/justinallenmarsh/tangotube-cli/internal/api"
	"github.com/justinallenmarsh/tangotube-cli/internal/output"
)

func newFacets(a *App) *cobra.Command {
	var f filters
	cmd := &cobra.Command{
		Use:   "facets [QUERY]",
		Short: "Count what a search holds: dancers, orchestras, songs, years…",
		Long: `Count what a search holds before reading it: the leaders, followers,
couples, orchestras, songs, events, channels, genres and kinds of video in
it, most performances first, and the years its music was recorded.

It takes the same query and filters as tt search. With none, it counts the
whole catalogue. Counts are performances: twelve cameras on one dance count
once. --limit is how many of each to show.`,
		Example: `  tt facets --orchestra "di sarli"
  tt facets --dancer "noelia hurtado" --jq '.facets.orchestra[:3]'
  tt facets "vals" --genre vals --limit 5`,
		Annotations: map[string]string{listsThings: "yes"},
		Args:        cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			q := url.Values{}
			if len(args) == 1 && strings.TrimSpace(args[0]) != "" {
				q.Set("q", args[0])
			}
			f.set(q)
			q.Set("limit", fmt.Sprint(a.limit(8)))
			env, err := a.Client().Get(ctx(cmd), "/api/v1/facets", q)
			return a.Show(env, err, renderFacets)
		},
	}
	f.register(cmd.Flags())
	return cmd
}

type facetOption struct {
	Value  any    `json:"value"`
	Label  string `json:"label"`
	Count  int    `json:"count"`
	Detail string `json:"detail"`
}

// facetOrder is how a dancer reads a body of work: who, to what, where, when.
// Each facet is labelled with the flag that narrows to it.
var facetOrder = []struct{ key, label string }{
	{"kind", "kind"}, {"genre", "genre"},
	{"leader", "leader"}, {"follower", "follower"}, {"couple", "couple"}, {"dancer", "dancer"},
	{"orchestra", "orchestra"}, {"song", "song"}, {"event", "event"}, {"channel", "channel"},
	{"year", "uploaded"},
}

func renderFacets(p *output.Printer, raw json.RawMessage) {
	var data struct {
		Total  int                      `json:"total"`
		Facets map[string][]facetOption `json:"facets"`
	}
	_ = json.Unmarshal(raw, &data)
	if data.Total == 0 {
		p.Line("Nothing to count. Try fewer filters.")
		return
	}
	for _, f := range facetOrder {
		options := data.Facets[f.key]
		phrases := make([]string, 0, len(options))
		for _, o := range options {
			phrases = append(phrases, o.Label+" "+p.Style.Dim(thousands(o.Count)))
		}
		p.Items(f.label, phrases)
	}
	if rec := data.Facets["recorded"]; len(rec) > 0 {
		fmt.Fprintln(p.Out)
		decades(p, rec)
	}
}

// decades draws the recording years as a bar per decade: where the music of
// a search comes from, at a glance.
func decades(p *output.Printer, years []facetOption) {
	counts := map[int]int{}
	for _, y := range years {
		var year int
		switch v := y.Value.(type) {
		case float64:
			year = int(v)
		case string:
			fmt.Sscan(v, &year)
		}
		if year > 0 {
			counts[year/10*10] += y.Count
		}
	}
	keys := make([]int, 0, len(counts))
	most := 0
	for k, n := range counts {
		keys = append(keys, k)
		most = max(most, n)
	}
	sort.Ints(keys)
	width := min(40, p.Columns()-24)
	for i, d := range keys {
		label := ""
		if i == 0 {
			label = "recorded"
		}
		p.Field(label, fmt.Sprintf("%ds  %s %s", d, p.Style.Gold(bar(counts[d], most, width)), p.Style.Dim(thousands(counts[d]))))
	}
}

// bar is n of most, drawn in eighths of a cell so small counts still show.
func bar(n, most, width int) string {
	if most == 0 || n == 0 {
		return ""
	}
	eighths := n * width * 8 / most
	if eighths == 0 {
		eighths = 1
	}
	s := strings.Repeat("█", eighths/8)
	if r := eighths % 8; r > 0 {
		s += string([]rune("▏▎▍▌▋▊▉")[r-1])
	}
	return s
}

func newHome(a *App) *cobra.Command {
	return &cobra.Command{
		Use:   "home",
		Short: "What the front page of tangotube.tv shows today.",
		Long: `What the front page of tangotube.tv shows today: trending this week, new
uploads, the featured dancer, orchestra, song, event and channel,
conversations, classes, hidden gems and the Mundial. Each section names the
tt search that shows all of it.`,
		Example: `  tt home
  tt home --jq '.sections[] | {title, search}'`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			env, err := a.Client().Get(ctx(cmd), "/api/v1/home", nil)
			return a.ShowDetail(env, err, renderHome)
		},
	}
}

// homeRows is how many videos of each section a terminal shows; --json has
// them all.
const homeRows = 3

func renderHome(p *output.Printer, raw json.RawMessage) {
	var data struct {
		Sections []struct {
			Title    string      `json:"title"`
			Subtitle string      `json:"subtitle"`
			Search   string      `json:"search"`
			Videos   []api.Video `json:"videos"`
		} `json:"sections"`
	}
	_ = json.Unmarshal(raw, &data)
	// One grid for the whole page: every section's rows line up.
	var cols []output.Column
	groups := make([][][]string, len(data.Sections))
	for i, s := range data.Sections {
		if len(s.Videos) > homeRows {
			data.Sections[i].Videos = s.Videos[:homeRows]
		}
		cols, groups[i] = videoColumns(data.Sections[i].Videos)
	}
	grid := p.Grid(cols, groups...)
	for i, s := range data.Sections {
		if i > 0 {
			fmt.Fprintln(p.Out)
		}
		fmt.Fprintln(p.Out, p.Style.Bold(s.Title))
		if s.Subtitle != "" {
			fmt.Fprintln(p.Out, p.Style.Dim(s.Subtitle))
		}
		_, rows := videoColumns(s.Videos)
		p.Rows(withLinks(grid, s.Videos), rows)
		if s.Search != "" {
			fmt.Fprintln(p.Out, p.Style.Gold("more: "+s.Search))
		}
	}
}

func newResolve(a *App) *cobra.Command {
	return &cobra.Command{
		Use:   "resolve NAME",
		Short: "Say what a typed name means before searching with it.",
		Long: `Say what a typed name means before searching with it: which dancers,
couples, orchestras, songs, events and channels it matches, and how
tt search would read it as filters. "di sarli noelia" reads as the orchestra
Carlos Di Sarli and the dancer Noelia Hurtado.

When nothing matches, it offers the spelling it thinks you meant.`,
		Example: `  tt resolve "di sarli noelia"
  tt resolve "chicho" --jq '.matches[] | select(.kind == "dancer") | .slug'`,
		Args: exactArgs(1, `tt resolve "di sarli"`),
		RunE: func(cmd *cobra.Command, args []string) error {
			env, err := a.Client().Get(ctx(cmd), "/api/v1/resolve", url.Values{"q": {args[0]}})
			return a.Show(env, err, renderResolve)
		},
	}
}

func renderResolve(p *output.Printer, raw json.RawMessage) {
	var data struct {
		Matches []struct {
			Kind   string `json:"kind"`
			Slug   string `json:"slug"`
			Name   string `json:"name"`
			Detail string `json:"detail"`
		} `json:"matches"`
		Reading []struct {
			Filter string `json:"filter"`
			Label  string `json:"label"`
		} `json:"reading"`
		Remainder  string `json:"remainder"`
		DidYouMean string `json:"did_you_mean"`
	}
	_ = json.Unmarshal(raw, &data)
	if data.DidYouMean != "" {
		p.Line("Did you mean “%s”?", data.DidYouMean)
		return
	}
	// The summary already says how the words read; what is left is the rest.
	if data.Remainder != "" {
		p.Field("and words", "“"+data.Remainder+"”")
		fmt.Fprintln(p.Out)
	}
	if len(data.Matches) == 0 {
		return
	}
	rows := make([][]string, 0, len(data.Matches))
	for _, m := range data.Matches {
		rows = append(rows, []string{m.Slug, m.Name, m.Kind, m.Detail})
	}
	p.Table([]output.Column{{Header: "SLUG", ID: true}, {Header: "NAME", Flex: true},
		{Header: "KIND", Dim: true}, {Header: "", Dim: true, Flex: true}}, rows)
}
