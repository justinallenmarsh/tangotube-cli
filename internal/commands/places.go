package commands

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"

	"github.com/justinallenmarsh/tangotube-cli/internal/api"
	"github.com/justinallenmarsh/tangotube-cli/internal/output"
)

func newDancer(a *App) *cobra.Command {
	var timeline, tour, repertoire bool
	cmd := &cobra.Command{Use: "dancer", Short: "Show a dancer: titles, partners, the orchestras they dance to.", Args: cobra.NoArgs}
	show := &cobra.Command{
		Use:   "show SLUG",
		Short: "Show a dancer: titles, partners, the orchestras they dance to.",
		Long: `Show a dancer: world titles, the years they have been filmed, who they
dance with, the orchestras they dance to most, and their most-watched
performances.

--timeline adds their videos year by year and each partnership's years.
--tour adds the cities they have danced in and each performance on the road.
--repertoire adds the songs they danced, year by year.

Slugs are what the site uses in /dancers/SLUG, like noelia-hurtado.`,
		Example: `  tt dancer show noelia-hurtado
  tt dancer show sebastian-achaval --timeline --tour
  tt dancers --champion --jq '.dancers[].slug'`,
		Args: exactArgs(1, "tt dancer show SLUG"),
		RunE: func(cmd *cobra.Command, args []string) error {
			var include []string
			for name, on := range map[string]bool{"timeline": timeline, "tour": tour, "repertoire": repertoire} {
				if on {
					include = append(include, name)
				}
			}
			var q url.Values
			if len(include) > 0 {
				q = url.Values{"include": {strings.Join(sortedStrings(include), ",")}}
			}
			env, err := a.Client().Get(ctx(cmd), "/api/v1/dancers/"+url.PathEscape(NormalizeID(args[0])), q)
			return a.ShowDetail(env, err, renderDancer)
		},
	}
	fl := show.Flags()
	fl.BoolVar(&timeline, "timeline", false, "Add their videos year by year, and each partnership's years.")
	fl.BoolVar(&tour, "tour", false, "Add the cities and occasions they have danced at.")
	fl.BoolVar(&repertoire, "repertoire", false, "Add the songs they danced, year by year.")
	cmd.AddCommand(show)
	return cmd
}

func sortedStrings(s []string) []string {
	order := map[string]int{"timeline": 0, "tour": 1, "repertoire": 2}
	out := append([]string(nil), s...)
	for i := range out {
		for j := i + 1; j < len(out); j++ {
			if order[out[j]] < order[out[i]] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

type dancerExtras struct {
	Timeline []struct {
		Year         int `json:"year"`
		Videos       int `json:"videos"`
		Performances int `json:"performances"`
		Competitions int `json:"competitions"`
		Workshops    int `json:"workshops"`
	} `json:"timeline"`
	Partnerships []struct {
		Partner string `json:"partner"`
		From    int    `json:"from"`
		To      int    `json:"to"`
		Videos  int    `json:"videos"`
	} `json:"partnerships"`
	Repertoire []struct {
		Year  int `json:"year"`
		Songs []struct {
			Title     string `json:"title"`
			Orchestra string `json:"orchestra"`
			Count     int    `json:"count"`
		} `json:"songs"`
	} `json:"repertoire"`
	Tour *struct {
		PerformancesCount int `json:"performances_count"`
		Cities            []struct {
			City    string `json:"city"`
			Country string `json:"country"`
			Count   int    `json:"count"`
		} `json:"cities"`
		Performances []struct {
			ID      string `json:"id"`
			Event   string `json:"event"`
			City    string `json:"city"`
			Country string `json:"country"`
			Year    int    `json:"year"`
			Partner string `json:"partner"`
			Dances  int    `json:"dances"`
		} `json:"performances"`
	} `json:"tour"`
}

// renderDancer is the dancer's page, then whatever --timeline, --tour and
// --repertoire asked for, each under a heading of its own.
func renderDancer(p *output.Printer, raw json.RawMessage) {
	e := entityFacts(p, raw)
	x := decode[dancerExtras](raw)

	if len(x.Timeline) > 0 {
		heading(p, "Year by year")
		most := 0
		for _, y := range x.Timeline {
			most = max(most, y.Videos)
		}
		for _, y := range x.Timeline {
			detail := join(" · ", countOf(y.Performances, "performance"), countOf(y.Competitions, "competition"), countOf(y.Workshops, "workshop"))
			count := fmt.Sprintf("%*s", len(thousands(most)), thousands(y.Videos))
			p.Field(fmt.Sprint(y.Year), count+"  "+p.Style.Gold(fmt.Sprintf("%-24s", bar(y.Videos, most, 24)))+"  "+p.Style.Dim(detail))
		}
	}
	if len(x.Partnerships) > 0 {
		heading(p, "Partnerships")
		rows := make([][]string, 0, len(x.Partnerships))
		for _, pa := range x.Partnerships {
			rows = append(rows, []string{pa.Partner, (&api.Years{From: pa.From, To: pa.To}).Text(), thousands(pa.Videos)})
		}
		indented(p, []output.Column{{Header: "PARTNER", Flex: true}, {Header: "YEARS", Dim: true}, {Header: "VIDEOS", Dim: true, Right: true}}, rows)
	}
	if x.Tour != nil {
		heading(p, "On tour")
		if len(x.Tour.Cities) == 0 && len(x.Tour.Performances) == 0 {
			p.Field("", "No festival performances credited yet.")
		}
		var cities []string
		for _, c := range x.Tour.Cities {
			cities = append(cities, join(", ", c.City, c.Country)+" "+p.Style.Dim(thousands(c.Count)))
		}
		p.Items("cities", cities)
		if len(x.Tour.Performances) > 0 {
			fmt.Fprintln(p.Out)
			rows := make([][]string, 0, tourRows)
			for i, t := range x.Tour.Performances {
				if i == tourRows {
					break
				}
				rows = append(rows, []string{t.ID, intText(&t.Year), t.Event, join(", ", t.City, t.Country), t.Partner, thousands(t.Dances)})
			}
			indented(p, []output.Column{{Header: "ID", ID: true}, {Header: "YEAR", Dim: true}, {Header: "EVENT", Flex: true},
				{Header: "WHERE", Flex: true}, {Header: "WITH", Flex: true}, {Header: "DANCES", Dim: true, Right: true}}, rows)
			if more := x.Tour.PerformancesCount - len(rows); more > 0 {
				p.Field("", p.Style.Dim(fmt.Sprintf("and %s more (--jq '.tour.performances[]' has the newest 20)", thousands(more))))
			}
		}
	}
	if len(x.Repertoire) > 0 {
		heading(p, "Repertoire")
		years := x.Repertoire
		if len(years) > repertoireYears {
			years = years[len(years)-repertoireYears:]
		}
		for _, y := range years {
			var songs []string
			for i, s := range y.Songs {
				if i == 3 {
					break
				}
				songs = append(songs, s.Title+" "+p.Style.Dim(s.Orchestra))
			}
			p.Items(fmt.Sprint(y.Year), songs)
		}
		if earlier := len(x.Repertoire) - len(years); earlier > 0 {
			p.Field("", p.Style.Dim(fmt.Sprintf("and %d earlier years (--jq '.repertoire[]')", earlier)))
		}
	}
	entityVideos(p, e)
}

// A terminal shows the newest of the road and of the repertoire; --json has
// the rest.
const (
	tourRows        = 10
	repertoireYears = 5
)

// heading opens a block of a detail view: a blank line, then its name.
func heading(p *output.Printer, name string) {
	fmt.Fprintln(p.Out)
	fmt.Fprintln(p.Out, "  "+p.Style.Bold(name))
}

// indented is a table sitting under a heading, in line with the fields.
func indented(p *output.Printer, cols []output.Column, rows [][]string) {
	var buf strings.Builder
	q := *p
	q.Out = &buf
	q.Width = p.Columns() - 2
	q.Table(cols, rows)
	for _, line := range strings.Split(strings.TrimRight(buf.String(), "\n"), "\n") {
		fmt.Fprintln(p.Out, "  "+line)
	}
}

func countOf(n int, noun string) string {
	if n == 0 {
		return ""
	}
	return fmt.Sprintf("%s %s%s", thousands(n), noun, plural(n))
}

func newEvent(a *App) *cobra.Command {
	var year int
	cmd := &cobra.Command{Use: "event", Short: "Show a festival or championship and who danced there.", Args: cobra.NoArgs}
	show := &cobra.Command{
		Use:   "show SLUG",
		Short: "Show a festival or championship and who danced there.",
		Long: `Show a festival, championship or marathon: where and when, the years
TangoTube has videos from, the couples and dancers filmed there most, the
orchestras and songs they danced to, similar events, and its most-watched
performances. --year keeps to one edition.`,
		Example: `  tt event show planetango
  tt event show planetango --year 2019
  tt events --country Argentina`,
		Args: exactArgs(1, "tt event show SLUG"),
		RunE: func(cmd *cobra.Command, args []string) error {
			var q url.Values
			if year != 0 {
				q = url.Values{"year": {fmt.Sprint(year)}}
			}
			env, err := a.Client().Get(ctx(cmd), "/api/v1/events/"+url.PathEscape(NormalizeID(args[0])), q)
			return a.ShowDetail(env, err, renderEvent)
		},
	}
	show.Flags().IntVar(&year, "year", 0, "Only this `YEAR`'s edition.")
	cmd.AddCommand(show)
	return cmd
}

type named struct {
	Slug  string `json:"slug"`
	Name  string `json:"name"`
	Title string `json:"title"`
}

func names(list []named, n int) []string {
	var out []string
	for i, x := range list {
		if i == n {
			break
		}
		if x.Name != "" {
			out = append(out, x.Name)
		} else {
			out = append(out, x.Title)
		}
	}
	return out
}

func renderEvent(p *output.Printer, raw json.RawMessage) {
	var e struct {
		listItem
		Year  *int `json:"year"`
		Years []struct {
			Year   int `json:"year"`
			Videos int `json:"videos"`
		} `json:"years"`
		Dancers       []named     `json:"dancers"`
		Couples       []named     `json:"couples"`
		Orchestras    []named     `json:"orchestras"`
		SimilarEvents []named     `json:"similar_events"`
		Songs         []listItem  `json:"songs"`
		Videos        []api.Video `json:"videos"`
	}
	_ = json.Unmarshal(raw, &e)
	fmt.Fprintln(p.Out, p.Style.Bold(e.Title)+"  "+p.Style.ID("", e.Slug))
	p.Field("where", join(", ", e.City, e.Country))
	p.Field("when", dateSpan(e.StartDate, e.EndDate))
	p.Field("kind", e.Category)
	p.Field("videos", thousands(e.VideosCount))
	var years []string
	for _, y := range e.Years {
		years = append(years, fmt.Sprint(y.Year)+" "+p.Style.Dim(thousands(y.Videos)))
	}
	p.Items("editions", years)
	p.Items("couples", names(e.Couples, 6))
	p.Items("dancers", names(e.Dancers, 6))
	p.Items("danced to", names(e.Orchestras, 6))
	var songs []string
	for i, s := range e.Songs {
		if i == 5 {
			break
		}
		orchestra := ""
		if s.Orchestra != nil {
			orchestra = s.Orchestra.Name
		}
		songs = append(songs, s.Title+" "+p.Style.Dim(orchestra))
	}
	p.Items("songs", songs)
	p.Items("similar", names(e.SimilarEvents, 4))
	p.FieldLink("page", e.Links.TangoTube, e.Links.TangoTube)
	if len(e.Videos) > 0 {
		fmt.Fprintln(p.Out)
		videoTable(p, e.Videos)
	}
}

func newChannel(a *App) *cobra.Command {
	return showGroup(a, "channel", "Show a YouTube channel's trending and newest tango videos.",
		`Show a YouTube channel TangoTube follows: how many tango videos it has, its
trending videos, and its newest. The slug is the channel's YouTube id, as in
youtube.com/channel/ID; tt channels lists them.`,
		`  tt channel show UCCoOxQMnmwZ-jhezbLfSUgQ
  tt channels --jq '.channels[0].slug'`, "/api/v1/channels", renderChannel)
}

func renderChannel(p *output.Printer, raw json.RawMessage) {
	var c struct {
		listItem
		Trending []api.Video `json:"trending"`
		Recent   []api.Video `json:"recent"`
	}
	_ = json.Unmarshal(raw, &c)
	fmt.Fprintln(p.Out, p.Style.Bold(c.Title)+"  "+p.Style.ID("", c.Slug))
	if c.VideosCount > 0 {
		p.Field("videos", thousands(c.VideosCount)+" tango videos")
	}
	p.FieldLink("youtube", c.Links.YouTube, c.Links.YouTube)
	p.FieldLink("page", c.Links.TangoTube, c.Links.TangoTube)
	if len(c.Trending) > 0 {
		heading(p, "Trending")
		indentedVideos(p, c.Trending)
	}
	if len(c.Recent) > 0 {
		heading(p, "Newest")
		indentedVideos(p, c.Recent)
	}
}

func indentedVideos(p *output.Printer, videos []api.Video) {
	cols, rows := videoColumns(videos)
	indented(p, cols, rows)
}

func newSongVersions(a *App) *cobra.Command {
	return &cobra.Command{
		Use:   "versions SLUG",
		Short: "List the other recordings of the same song.",
		Long: `List the other recordings of the same composition: the same tango by other
orchestras, or by the same orchestra with another singer, most danced first.`,
		Example: `  tt song versions todo-es-amor-fulvio-salamanca
  tt song versions volver-a-sonar-carlos-di-sarli --jq '.versions[].slug'`,
		Args: exactArgs(1, "tt song versions SLUG"),
		RunE: func(cmd *cobra.Command, args []string) error {
			env, err := a.Client().Get(ctx(cmd), "/api/v1/songs/"+url.PathEscape(NormalizeID(args[0]))+"/versions", nil)
			return a.ShowDetail(env, err, renderVersions)
		},
	}
}

func renderVersions(p *output.Printer, raw json.RawMessage) {
	var v struct {
		Song        listItem   `json:"song"`
		Composition string     `json:"composition"`
		Versions    []listItem `json:"versions"`
	}
	_ = json.Unmarshal(raw, &v)
	title := v.Composition
	if title == "" {
		title = v.Song.Title
	}
	fmt.Fprintln(p.Out, p.Style.Bold(title))
	orchestra := ""
	if v.Song.Orchestra != nil {
		orchestra = v.Song.Orchestra.Name
	}
	p.Field("this one", join(" · ", orchestra, v.Song.Singer, intText(v.Song.Year))+"  "+p.Style.ID("", v.Song.Slug))
	fmt.Fprintln(p.Out)
	if len(v.Versions) == 0 {
		p.Line("No other recording of it yet.")
		return
	}
	songTable(p, v.Versions)
}
