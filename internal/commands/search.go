package commands

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/justinallenmarsh/tangotube-cli/internal/api"
	"github.com/justinallenmarsh/tangotube-cli/internal/output"
)

// filters are what narrows a search, and the facets over it: the same flags,
// the same query parameters.
type filters struct {
	dancer, leader, follower, orchestra, song, event, couple, channel string
	genre, kind                                                       string
	year, yearFrom, yearTo, uploaded                                  int
	hd                                                                bool
}

func (f *filters) register(fl *pflag.FlagSet) {
	fl.StringVar(&f.dancer, "dancer", "", "Only videos with this dancer: a `NAME` or slug.")
	fl.StringVar(&f.leader, "leader", "", "Only videos where this `NAME` leads.")
	fl.StringVar(&f.follower, "follower", "", "Only videos where this `NAME` follows.")
	fl.StringVar(&f.couple, "couple", "", "Only videos of this couple: both `NAMES`, or the slug.")
	fl.StringVar(&f.orchestra, "orchestra", "", "Only videos danced to this orchestra: a `NAME` or slug.")
	fl.StringVar(&f.song, "song", "", "Only videos danced to this song: a `TITLE` or slug.")
	fl.StringVar(&f.event, "event", "", "Only videos filmed at this festival or milonga: a `NAME` or slug.")
	fl.StringVar(&f.channel, "channel", "", "Only videos from this YouTube channel: its `NAME` or id.")
	fl.StringVar(&f.genre, "genre", "", "Only this `GENRE`: tango, vals, or milonga.")
	fl.StringVar(&f.kind, "kind", "", "Only this `KIND` of video: performance, class, workshop, interview, competition…")
	fl.IntVar(&f.year, "year", 0, "The `YEAR` the music was recorded.")
	fl.IntVar(&f.yearFrom, "year-from", 0, "Music recorded in this `YEAR` or later.")
	fl.IntVar(&f.yearTo, "year-to", 0, "Music recorded in this `YEAR` or earlier.")
	fl.IntVar(&f.uploaded, "uploaded", 0, "The `YEAR` the video went up on YouTube.")
	fl.BoolVar(&f.hd, "hd", false, "Only videos in HD.")
}

func (f *filters) set(q url.Values) {
	for key, v := range map[string]string{
		"dancer": f.dancer, "leader": f.leader, "follower": f.follower, "couple": f.couple,
		"orchestra": f.orchestra, "song": f.song, "event": f.event, "channel": f.channel,
		"genre": f.genre, "kind": f.kind,
	} {
		if v != "" {
			q.Set(key, v)
		}
	}
	for key, v := range map[string]int{"year": f.year, "year_from": f.yearFrom, "year_to": f.yearTo, "uploaded": f.uploaded} {
		if v != 0 {
			q.Set(key, fmt.Sprint(v))
		}
	}
	if f.hd {
		q.Set("hd", "1")
	}
}

type searchFlags struct {
	filters
	style, technique, sort, cursor string
}

func newSearch(a *App) *cobra.Command {
	var f searchFlags
	cmd := &cobra.Command{
		Use:   "search [QUERY]",
		Short: "Search dancers, orchestras, songs… and find the performance.",
		Long: `Search dancers, orchestras, songs… and find the performance.

Type what you would type into the search box on tangotube.tv. Narrow it with
the flags: a dancer, an orchestra, a song, the year the music was recorded.
Names work as well as slugs — "noelia hurtado" and "noelia-hurtado" are the
same dancer. Names typed into the query are read as filters too: "di sarli
facundo" searches Carlos Di Sarli and a Facundo, and says which Facundo.

--sort orders the whole catalogue the way the site's sections do: trending,
popular, newest, oldest, hidden-gems. A sort, a kind or a channel alone is
enough to browse.

--technique and --style search practice clips that dancers have tagged, and
return the videos those clips come from, each with its clips attached.`,
		Example: `  tt search "di sarli facundo"
  tt search --leader "carlitos" --follower "noelia hurtado"
  tt search --orchestra "di sarli" --year-from 1951 --year-to 1954
  tt search --sort hidden-gems --kind class
  tt search "sacada" --technique sacada --dancer "noelia hurtado"
  tt search "noelia" --json --jq '.videos[].id'`,
		Annotations: map[string]string{listsThings: "yes"},
		Args:        cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			q := url.Values{}
			if len(args) == 1 && strings.TrimSpace(args[0]) != "" {
				q.Set("q", args[0])
			}
			f.filters.set(q)
			for key, v := range map[string]string{
				"cursor": f.cursor, "sort": f.sort,
				"style": tagName(f.style), "technique": tagName(f.technique),
			} {
				if v != "" {
					q.Set(key, v)
				}
			}
			if len(q) == 0 {
				return a.Fail(Usage("Search needs something to look for: a query, a filter, or a sort", `tt search "di sarli"`))
			}
			q.Set("limit", fmt.Sprint(a.limit(20)))
			env, err := a.Client().Get(ctx(cmd), "/api/v1/search", q)
			return a.Show(env, err, renderSearch)
		},
	}
	fl := cmd.Flags()
	f.filters.register(fl)
	fl.StringVar(&f.sort, "sort", "", "The `ORDER`: browse, trending, popular, newest, oldest, or hidden-gems.")
	fl.StringVar(&f.style, "style", "", "A `STYLE` tagged on practice clips: milonguero, salon, nuevo, or stage.")
	fl.StringVar(&f.technique, "technique", "", "A `STEP` tagged on practice clips, like sacada or boleo (tt clip tags).")
	fl.StringVar(&f.cursor, "cursor", "", "Continue from the `CURSOR` the last page ended on.")
	return cmd
}

// tagName writes a step the way the vocabulary does: "Media Luna" → media_luna.
func tagName(s string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(s)), " ", "_")
}

type searchData struct {
	Videos     []api.Video                `json:"videos"`
	Total      int                        `json:"total"`
	NextCursor *string                    `json:"next_cursor"`
	Filters    map[string]json.RawMessage `json:"filters"`
}

// filterOrder is how a dancer reads a search: who, to what, where.
var filterOrder = []string{"dancer", "leader", "follower", "couple", "orchestra", "song", "event", "channel"}

func renderSearch(p *output.Printer, raw json.RawMessage) {
	data := decode[searchData](raw)
	renderAlternatives(p, data.Filters)
	if len(data.Videos) == 0 {
		p.Line("Nothing yet. Try fewer filters, or a name spelled the way the site spells it.")
		return
	}
	videoTable(p, data.Videos)
	var clips [][]string
	var clipLinks, loopLinks []string
	for _, v := range data.Videos {
		for _, c := range v.Clips {
			clips = append(clips, []string{c.ID, c.VideoID, c.Range(), strings.Join(c.Tags, " ")})
			clipLinks = append(clipLinks, c.ClipURL)
			loopLinks = append(loopLinks, c.WatchURL)
		}
	}
	if len(clips) > 0 {
		fmt.Fprintln(p.Out)
		p.Table([]output.Column{
			{Header: "CLIP", ID: true, Flex: true, Links: clipLinks},
			{Header: "VIDEO", Dim: true, Links: loopLinks},
			{Header: "LOOP", Dim: true},
			{Header: "TAGS", Flex: true},
		}, clips)
	}
}

// renderAlternatives says which name a guess landed on and what else it could
// have been, as slugs a person can paste straight back into --dancer:
// "dancer: Facundo Piñero · also facundo-de-la-cruz, facundo-penalva".
func renderAlternatives(p *output.Printer, filters map[string]json.RawMessage) {
	for _, key := range filterOrder {
		raw, ok := filters[key]
		if !ok {
			continue
		}
		var ref api.Ref
		if json.Unmarshal(raw, &ref) != nil || len(ref.Also) == 0 {
			continue
		}
		var slugs []string
		for _, alt := range ref.Also {
			slugs = append(slugs, alt.Slug)
		}
		line := fmt.Sprintf("%s: %s · also %s", key, ref.Label(), strings.Join(slugs, ", "))
		fmt.Fprintln(p.Out, p.Style.Dim(output.Truncate(line, p.Columns())))
	}
}

// videoTable is the house table: ID, DANCERS, ORCHESTRA, SONG, YEAR.
func videoTable(p *output.Printer, videos []api.Video) {
	cols, rows := videoColumns(videos)
	p.Table(cols, rows)
}

func videoColumns(videos []api.Video) ([]output.Column, [][]string) {
	rows := make([][]string, 0, len(videos))
	links := make([]string, 0, len(videos))
	for _, v := range videos {
		rows = append(rows, []string{v.ID, v.DancerNames(), v.Orchestra.Label(), v.Song.Label(), v.YearText()})
		links = append(links, v.WatchURL)
	}
	return []output.Column{
		{Header: "ID", ID: true, Links: links},
		{Header: "DANCERS", Flex: true},
		{Header: "ORCHESTRA", Flex: true},
		{Header: "SONG", Flex: true},
		{Header: "YEAR", Dim: true},
	}, rows
}
