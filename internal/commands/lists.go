package commands

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/justinallenmarsh/tangotube-cli/internal/output"
)

// listFlag is one filter a catalogue list takes: its flag, the query
// parameter it becomes, and what it means.
type listFlag struct {
	name, param, usage string
	kind               string // "string" (default), "int", or "bool"
}

// listCommand is "tt dancers", "tt events", …: the site's index pages, a
// page at a time. A bare word is the same as --q.
func listCommand(a *App, use, short, long, example, path string, flags []listFlag, render func(*output.Printer, json.RawMessage)) *cobra.Command {
	strs := map[string]*string{}
	ints := map[string]*int{}
	bools := map[string]*bool{}
	var q, cursor string
	cmd := &cobra.Command{
		Use:         use + " [QUERY]",
		Short:       short,
		Long:        long,
		Example:     example,
		Annotations: map[string]string{listsThings: "yes"},
		Args:        cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			params := url.Values{}
			if len(args) == 1 && strings.TrimSpace(args[0]) != "" {
				params.Set("q", args[0])
			}
			if q != "" {
				params.Set("q", q)
			}
			for _, f := range flags {
				switch {
				case strs[f.name] != nil && *strs[f.name] != "":
					params.Set(f.param, *strs[f.name])
				case ints[f.name] != nil && *ints[f.name] != 0:
					params.Set(f.param, fmt.Sprint(*ints[f.name]))
				case bools[f.name] != nil && *bools[f.name]:
					params.Set(f.param, "1")
				}
			}
			if cursor != "" {
				params.Set("cursor", cursor)
			}
			params.Set("limit", fmt.Sprint(a.limit(20)))
			env, err := a.Client().Get(ctx(cmd), path, params)
			return a.Show(env, err, render)
		},
	}
	fl := cmd.Flags()
	fl.StringVar(&q, "q", "", "Only names matching these `WORDS`.")
	for _, f := range flags {
		switch f.kind {
		case "int":
			ints[f.name] = fl.Int(f.name, 0, f.usage)
		case "bool":
			bools[f.name] = fl.Bool(f.name, false, f.usage)
		default:
			strs[f.name] = fl.String(f.name, "", f.usage)
		}
	}
	fl.StringVar(&cursor, "cursor", "", "Continue from the `CURSOR` the last page ended on.")
	return cmd
}

func sortFlag(orders string) listFlag {
	return listFlag{name: "sort", param: "sort", usage: "The `ORDER`: " + orders + "."}
}

func newDancers(a *App) *cobra.Command {
	return listCommand(a, "dancers", "List dancers, most watched first.",
		`List the dancers TangoTube knows, most watched first: their slug, how many
videos they are in, and whether they have won the Mundial.`,
		`  tt dancers --champion
  tt dancers --role follower --sort name
  tt dancers noelia`,
		"/api/v1/dancers", []listFlag{
			{name: "role", param: "role", usage: "Only dancers who mostly dance this `ROLE`: leader or follower."},
			{name: "champion", param: "champion", usage: "Only Mundial de Tango champions.", kind: "bool"},
			sortFlag("popular, name, recent, followers, or partners"),
		}, renderDancers)
}

func newCouples(a *App) *cobra.Command {
	return listCommand(a, "couples", "List couples, most watched first.",
		`List the partnerships TangoTube has filmed, most watched first.`,
		`  tt couples --min-videos 100
  tt couples noelia`,
		"/api/v1/couples", []listFlag{
			{name: "min-videos", param: "min_videos", usage: "Only couples filmed at least `N` times.", kind: "int"},
			sortFlag("popular, name, recent, or most-videos"),
		}, renderCouples)
}

func newOrchestras(a *App) *cobra.Command {
	return listCommand(a, "orchestras", "List orchestras, most danced first.",
		`List the orchestras dancers dance to, most danced first, with how many of
their recordings TangoTube knows.`,
		`  tt orchestras --era golden-age
  tt orchestras --sort most-songs`,
		"/api/v1/orchestras", []listFlag{
			{name: "era", param: "era", usage: "`ERA`: golden-age (recorded before 1960) or contemporary."},
			sortFlag("popular, name, most-songs, or most-videos"),
		}, renderOrchestras)
}

func newSongs(a *App) *cobra.Command {
	return listCommand(a, "songs", "List recordings, most danced first.",
		`List recordings, most danced first: the orchestra, the singer, the year it
was recorded, and how many performances were danced to it.`,
		`  tt songs --orchestra "di sarli" --decade 1950
  tt songs --genre vals --sort newest`,
		"/api/v1/songs", []listFlag{
			{name: "orchestra", param: "orchestra", usage: "Only this orchestra's recordings: a `NAME` or slug."},
			{name: "genre", param: "genre", usage: "Only this `GENRE`: tango, vals, or milonga."},
			{name: "decade", param: "decade", usage: "Only recordings from this `DECADE`, like 1940.", kind: "int"},
			{name: "letter", param: "letter", usage: "Only titles starting with this `LETTER`."},
			sortFlag("popular, name, newest, or oldest"),
		}, renderSongs)
}

func newEvents(a *App) *cobra.Command {
	return listCommand(a, "events", "List festivals, championships and marathons.",
		`List the festivals, championships, marathons and milongas TangoTube has
videos from, most filmed first.`,
		`  tt events --country Argentina
  tt events --continent europe --sort recent`,
		"/api/v1/events", []listFlag{
			{name: "country", param: "country", usage: "Only events in this `COUNTRY`."},
			{name: "continent", param: "continent", usage: "Only events on this `CONTINENT`."},
			{name: "category", param: "category", usage: "Only this `KIND` of event, like festival or marathon."},
			sortFlag("popular, name, recent, followers, or most-dancers"),
		}, renderEvents)
}

func newChannels(a *App) *cobra.Command {
	return listCommand(a, "channels", "List the YouTube channels that film tango.",
		`List the YouTube channels TangoTube follows, by how many tango videos each
has. The slug is the channel's YouTube id.`,
		`  tt channels
  tt channels prischepov`,
		"/api/v1/channels", []listFlag{sortFlag("popular, name, recent, or followers")}, renderChannels)
}

func newSingers(a *App) *cobra.Command {
	return listCommand(a, "singers", "List singers and the orchestras they sang with.",
		`List the singers of the recordings TangoTube knows, with the orchestras
they recorded with most.`,
		`  tt singers --orchestra "di sarli"`,
		"/api/v1/singers", []listFlag{
			{name: "orchestra", param: "orchestra", usage: "Only singers who recorded with this `ORCHESTRA`."},
			sortFlag("popular or name"),
		}, renderSingers)
}

func newChampions(a *App) *cobra.Command {
	cmd := listCommand(a, "champions", "List Mundial de Tango champions, newest first.",
		`List the titles of the Mundial de Tango in Buenos Aires, newest first:
the year, the category (Tango de Pista, once called Salón, and Tango
Escenario) and the couple who won it.`,
		`  tt champions --year 2019
  tt champions --category escenario`,
		"/api/v1/champions", []listFlag{
			{name: "year", param: "year", usage: "Only this `YEAR`'s Mundial.", kind: "int"},
			{name: "category", param: "category", usage: "`CATEGORY`: pista (salon) or escenario."},
		}, renderChampions)
	cmd.Use = "champions"
	cmd.Args = cobra.NoArgs
	cmd.Flags().MarkHidden("q")
	return cmd
}

// Rows of the lists. Each list is slug-first, like search is id-first: the
// thing to paste into the next command.

type listItem struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Title       string `json:"title"`
	Role        string `json:"role"`
	Champion    bool   `json:"champion"`
	VideosCount int    `json:"videos_count"`
	SongsCount  int    `json:"songs_count"`
	Genre       string `json:"genre"`
	Year        *int   `json:"year"`
	Singer      string `json:"singer"`
	City        string `json:"city"`
	Country     string `json:"country"`
	Category    string `json:"category"`
	StartDate   string `json:"start_date"`
	EndDate     string `json:"end_date"`
	Orchestra   *struct {
		Name string `json:"name"`
	} `json:"orchestra"`
	Orchestras []struct {
		Name string `json:"name"`
	} `json:"orchestras"`
	Links struct {
		TangoTube string `json:"tangotube"`
		YouTube   string `json:"youtube"`
	} `json:"links"`
}

func (i listItem) label() string {
	if i.Name != "" {
		return i.Name
	}
	return i.Title
}

func items(raw json.RawMessage, key string) []listItem {
	var page map[string]json.RawMessage
	_ = json.Unmarshal(raw, &page)
	var out []listItem
	_ = json.Unmarshal(page[key], &out)
	return out
}

// slugColumn is the list's first column: the slug, linked to its page.
func slugColumn(list []listItem) output.Column {
	links := make([]string, len(list))
	for i, it := range list {
		links[i] = it.Links.TangoTube
	}
	return output.Column{Header: "SLUG", ID: true, Links: links}
}

func emptyList(p *output.Printer, list []listItem) bool {
	if len(list) == 0 {
		p.Line("Nothing matches. Try fewer filters.")
		return true
	}
	return false
}

func renderDancers(p *output.Printer, raw json.RawMessage) {
	list := items(raw, "dancers")
	if emptyList(p, list) {
		return
	}
	rows := make([][]string, 0, len(list))
	for _, d := range list {
		role := d.Role
		if role == "neither" || role == "both" {
			role = ""
		}
		if d.Champion {
			role = join(" · ", role, "champion")
		}
		rows = append(rows, []string{d.Slug, d.Name, role, thousands(d.VideosCount)})
	}
	p.Table([]output.Column{slugColumn(list), {Header: "DANCER", Flex: true}, {Header: "", Dim: true},
		{Header: "VIDEOS", Dim: true, Right: true}}, rows)
}

func renderCouples(p *output.Printer, raw json.RawMessage) {
	list := items(raw, "couples")
	if emptyList(p, list) {
		return
	}
	rows := make([][]string, 0, len(list))
	for _, c := range list {
		rows = append(rows, []string{c.Slug, c.Name, thousands(c.VideosCount)})
	}
	p.Table([]output.Column{slugColumn(list), {Header: "COUPLE", Flex: true}, {Header: "VIDEOS", Dim: true, Right: true}}, rows)
}

func renderOrchestras(p *output.Printer, raw json.RawMessage) {
	list := items(raw, "orchestras")
	if emptyList(p, list) {
		return
	}
	rows := make([][]string, 0, len(list))
	for _, o := range list {
		rows = append(rows, []string{o.Slug, o.Name, thousands(o.SongsCount), thousands(o.VideosCount)})
	}
	p.Table([]output.Column{slugColumn(list), {Header: "ORCHESTRA", Flex: true},
		{Header: "SONGS", Dim: true, Right: true}, {Header: "VIDEOS", Dim: true, Right: true}}, rows)
}

func renderSongs(p *output.Printer, raw json.RawMessage) {
	list := items(raw, "songs")
	if emptyList(p, list) {
		return
	}
	songTable(p, list)
}

func songTable(p *output.Printer, list []listItem) {
	rows := make([][]string, 0, len(list))
	for _, s := range list {
		orchestra := ""
		if s.Orchestra != nil {
			orchestra = s.Orchestra.Name
		}
		rows = append(rows, []string{s.Slug, s.Title, orchestra, s.Singer, intText(s.Year), thousands(s.VideosCount)})
	}
	p.Table([]output.Column{slugColumn(list), {Header: "SONG", Flex: true}, {Header: "ORCHESTRA", Flex: true},
		{Header: "SINGER", Flex: true}, {Header: "YEAR", Dim: true}, {Header: "VIDEOS", Dim: true, Right: true}}, rows)
}

func renderEvents(p *output.Printer, raw json.RawMessage) {
	list := items(raw, "events")
	if emptyList(p, list) {
		return
	}
	rows := make([][]string, 0, len(list))
	for _, e := range list {
		rows = append(rows, []string{e.Slug, e.Title, join(", ", e.City, e.Country), monthYear(e.StartDate), thousands(e.VideosCount)})
	}
	p.Table([]output.Column{slugColumn(list), {Header: "EVENT", Flex: true}, {Header: "WHERE", Flex: true},
		{Header: "WHEN", Dim: true}, {Header: "VIDEOS", Dim: true, Right: true}}, rows)
}

func renderChannels(p *output.Printer, raw json.RawMessage) {
	list := items(raw, "channels")
	if emptyList(p, list) {
		return
	}
	rows := make([][]string, 0, len(list))
	youtube := make([]string, 0, len(list))
	for _, c := range list {
		rows = append(rows, []string{c.Slug, c.Title, thousands(c.VideosCount)})
		youtube = append(youtube, c.Links.YouTube)
	}
	p.Table([]output.Column{slugColumn(list), {Header: "CHANNEL", Flex: true, Links: youtube},
		{Header: "VIDEOS", Dim: true, Right: true}}, rows)
}

func renderSingers(p *output.Printer, raw json.RawMessage) {
	list := items(raw, "singers")
	if emptyList(p, list) {
		return
	}
	rows := make([][]string, 0, len(list))
	for _, s := range list {
		var names []string
		for _, o := range s.Orchestras {
			names = append(names, o.Name)
		}
		rows = append(rows, []string{s.Slug, s.Name, strings.Join(names, ", "), thousands(s.SongsCount)})
	}
	p.Table([]output.Column{{Header: "SLUG", ID: true}, {Header: "SINGER", Flex: true},
		{Header: "WITH", Flex: true}, {Header: "SONGS", Dim: true, Right: true}}, rows)
}

type title struct {
	Year      int    `json:"year"`
	Label     string `json:"label"`
	City      string `json:"city"`
	Country   string `json:"country"`
	Champions []struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	} `json:"champions"`
}

func renderChampions(p *output.Printer, raw json.RawMessage) {
	var page struct {
		Champions []title `json:"champions"`
	}
	_ = json.Unmarshal(raw, &page)
	if len(page.Champions) == 0 {
		p.Line("No Mundial titles match. Try another year.")
		return
	}
	rows := make([][]string, 0, len(page.Champions))
	for _, t := range page.Champions {
		var names []string
		for _, c := range t.Champions {
			names = append(names, c.Name)
		}
		champions := strings.Join(names, " & ")
		if champions == "" {
			champions = "—"
		}
		rows = append(rows, []string{fmt.Sprint(t.Year), t.Label, champions, join(", ", t.City, t.Country)})
	}
	p.Table([]output.Column{{Header: "YEAR", ID: true}, {Header: "CATEGORY"}, {Header: "CHAMPIONS", Flex: true},
		{Header: "WHERE", Dim: true, Flex: true}}, rows)
}

// monthYear writes a date the short way a list has room for: "Aug 2017".
func monthYear(iso string) string {
	t, err := time.Parse("2006-01-02", iso)
	if err != nil {
		return ""
	}
	return t.Format("Jan 2006")
}
