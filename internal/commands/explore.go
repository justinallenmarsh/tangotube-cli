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

// performance is GET /api/v1/performances/:id: one dance, every camera on it,
// and the couple's other dances that occasion.
type performance struct {
	ID       int64     `json:"id"`
	Dancers  []api.Ref `json:"dancers"`
	Couple   *api.Ref  `json:"couple"`
	Occasion struct {
		Event *api.Ref `json:"event"`
		Kind  string   `json:"kind"`
		Date  string   `json:"date"`
	} `json:"occasion"`
	Dance struct {
		Position  int       `json:"position"`
		Song      *api.Song `json:"song"`
		Orchestra *api.Ref  `json:"orchestra"`
		Year      *int      `json:"year"`
		Videos    []camera  `json:"videos"`
	} `json:"dance"`
	Session []struct {
		Position    int    `json:"position"`
		ID          string `json:"id"`
		Song        string `json:"song"`
		Orchestra   string `json:"orchestra"`
		VideosCount int    `json:"videos_count"`
		Current     bool   `json:"current"`
	} `json:"session"`
}

// camera is one video of a dance.
type camera struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Channel      string `json:"channel"`
	DurationS    *int   `json:"duration_s"`
	YoutubeViews *int64 `json:"youtube_views"`
	UploadedOn   string `json:"uploaded_on"`
	WatchURL     string `json:"watch_url"`
	Current      bool   `json:"current"`
}

func newPerformance(a *App) *cobra.Command {
	group := &cobra.Command{Use: "performance", Short: "Show one dance and every camera that filmed it.", Args: cobra.NoArgs}
	group.AddCommand(&cobra.Command{
		Use:   "show ID",
		Short: "Show one dance and every camera that filmed it.",
		Long: `Show one dance: every video of it (a final is often filmed from three
sides of the floor, and a channel sometimes posts one dance twice), the couple, the song and who wrote and sang it, the
occasion, and the couple's other dances that occasion, in the order they
danced them.

ID is the YouTube id of any video of the dance, or a performance id from
tt video show --jq .performance. With a performance id, the first dance is
shown.`,
		Example: `  tt performance show cPJ3MjWDUVY
  tt performance show cPJ3MjWDUVY --jq '.dance.videos[] | [.channel, .youtube_views]'
  tt video show cPJ3MjWDUVY --jq .performance`,
		Args: exactArgs(1, "tt performance show ID"),
		RunE: func(cmd *cobra.Command, args []string) error {
			env, err := a.Client().Get(ctx(cmd), "/api/v1/performances/"+NormalizeID(args[0]), nil)
			return a.ShowDetail(env, err, renderPerformance)
		},
	})
	return group
}

// renderPerformance reads like the video page, then lists the cameras, then
// the night.
func renderPerformance(p *output.Printer, raw json.RawMessage) {
	perf := decode[performance](raw)
	d := perf.Dance
	names := perf.Couple.Label()
	if names == "" {
		names = api.Video{Dancers: perf.Dancers}.DancerNames()
	}
	fmt.Fprintln(p.Out, p.Style.Bold(join(" · ", names, d.Song.Label())))

	p.Field("song", join(" · ", d.Song.Label(), d.Orchestra.Label(), intText(d.Year), songGenre(d.Song)))
	if d.Song != nil {
		p.Items("credits", credits(d.Song))
	}
	occasion := join(" · ", perf.Occasion.Kind, longDate(perf.Occasion.Date))
	if e := perf.Occasion.Event; e != nil {
		occasion = join(" · ", join(", ", e.Label(), e.City, e.Country), occasion)
	}
	p.Field("occasion", occasion)

	fmt.Fprintln(p.Out)
	rows := make([][]string, 0, len(d.Videos))
	links := make([]string, 0, len(d.Videos))
	for _, v := range d.Videos {
		var length, views string
		if v.DurationS != nil {
			length = api.Clock(*v.DurationS)
		}
		if v.YoutubeViews != nil && *v.YoutubeViews > 0 {
			views = compactCount(*v.YoutubeViews)
		}
		mark := ""
		if v.Current {
			mark = "← this one"
		}
		rows = append(rows, []string{v.ID, v.Channel, length, views, mark})
		links = append(links, v.WatchURL)
	}
	p.Table([]output.Column{
		{Header: "video", ID: true, Links: links},
		{Header: "channel", Flex: true},
		{Header: "length", Dim: true, Right: true},
		{Header: "views", Right: true},
		{Header: "", Dim: true},
	}, rows)

	if len(perf.Session) > 1 {
		fmt.Fprintln(p.Out)
		p.Field("session", fmt.Sprintf("dance %d of %d", d.Position, len(perf.Session)))
		for _, s := range perf.Session {
			music := join(" · ", s.Song, s.Orchestra)
			if music == "" {
				music = "song not yet known"
			}
			cams := ""
			if s.VideosCount > 1 {
				cams = p.Style.Dim(fmt.Sprintf("  %d videos", s.VideosCount))
			}
			n := fmt.Sprint(s.Position)
			if s.Current {
				p.Field("", p.Style.Bold(n)+"  "+p.Style.ID("", s.ID)+"  "+p.Style.Bold(music)+cams+p.Style.Dim("  ← this one"))
				continue
			}
			p.Field("", p.Style.Dim(n)+"  "+p.Style.ID("", s.ID)+"  "+music+cams)
		}
	}
}

// partnerGraph is GET /api/v1/dancers/:slug/partners.
type partnerGraph struct {
	Dancer struct {
		Name        string `json:"name"`
		Slug        string `json:"slug"`
		VideosCount int    `json:"videos_count"`
	} `json:"dancer"`
	Depth         int           `json:"depth"`
	PartnersCount int           `json:"partners_count"`
	Nodes         []partnerNode `json:"nodes"`
	Edges         []struct {
		A      int64 `json:"a"`
		B      int64 `json:"b"`
		Videos int   `json:"videos"`
	} `json:"edges"`
}

type partnerNode struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Slug   string `json:"slug"`
	Ring   int    `json:"ring"`
	Videos int    `json:"videos"`
	Couple string `json:"couple"`
}

func newPartners(a *App) *cobra.Command {
	var depth int
	cmd := &cobra.Command{
		Use:   "partners DANCER",
		Short: "Show who a dancer has danced with, and who those partners dance with.",
		Long: `Show a dancer's partners on camera, most filmed together first: the
network the site draws on /explore.

--depth 2 adds, under each of their top partners, the other dancers that
partner dances with, and names the dancers several of them share: the
people who make a scene one scene.

DANCER is a slug, like noelia-hurtado.`,
		Example: `  tt partners noelia-hurtado
  tt partners sebastian-achaval --depth 2
  tt partners noelia-hurtado --jq '.nodes[] | select(.ring == 1) | .couple'`,
		Args: exactArgs(1, "tt partners DANCER"),
		RunE: func(cmd *cobra.Command, args []string) error {
			q := url.Values{"depth": {fmt.Sprint(depth)}}
			env, err := a.Client().Get(ctx(cmd), "/api/v1/dancers/"+url.PathEscape(NormalizeID(args[0]))+"/partners", q)
			return a.ShowDetail(env, err, renderPartners)
		},
	}
	cmd.Flags().IntVar(&depth, "depth", 1, "1 for partners, 2 to add their partners too.")
	return cmd
}

// partnersShown is how many partners a terminal lists; --jq has them all.
const partnersShown = 20

// renderPartners draws the network as a tree: the dancer, each partner with
// the videos they share, and at depth 2 who else each partner dances with.
func renderPartners(p *output.Printer, raw json.RawMessage) {
	g := decode[partnerGraph](raw)
	head := p.Style.Bold(g.Dancer.Name) + "  " + p.Style.ID("", g.Dancer.Slug)
	if g.PartnersCount > 0 {
		head += p.Style.Dim(fmt.Sprintf(" · %d partner%s", g.PartnersCount, plural(g.PartnersCount)))
	}
	fmt.Fprintln(p.Out, head)

	byID := map[int64]partnerNode{}
	var partners []partnerNode
	for _, n := range g.Nodes {
		byID[n.ID] = n
		if n.Ring == 1 {
			partners = append(partners, n)
		}
	}
	if len(partners) == 0 {
		p.Line("  No partnership on camera yet.")
		return
	}

	// Who each dancer dances with inside the picture, heaviest first.
	links := map[int64][]partnerNode{}
	weight := map[[2]int64]int{}
	for _, e := range g.Edges {
		for _, pair := range [][2]int64{{e.A, e.B}, {e.B, e.A}} {
			if other, ok := byID[pair[1]]; ok && other.Ring != 0 {
				links[pair[0]] = append(links[pair[0]], other)
				weight[pair] = e.Videos
			}
		}
	}

	shown := partners
	if len(shown) > partnersShown {
		shown = shown[:partnersShown]
	}
	nameWidth := 0
	for _, n := range shown {
		nameWidth = max(nameWidth, output.DisplayWidth(n.Name))
	}
	count := 0
	for _, n := range shown {
		count = max(count, len(thousands(n.Videos)))
	}

	for i, n := range shown {
		last := i == len(shown)-1 && len(partners) == len(shown)
		branch, stem := "├─ ", "│    "
		if last {
			branch, stem = "└─ ", "     "
		}
		together := fmt.Sprintf("%*s videos", count, thousands(n.Videos))
		fmt.Fprintln(p.Out, p.Style.Dim(branch)+n.Name+strings.Repeat(" ", nameWidth-output.DisplayWidth(n.Name))+"  "+p.Style.Dim(together+"  "+n.Slug))
		if g.Depth < 2 {
			continue
		}
		others := links[n.ID]
		sort.SliceStable(others, func(x, y int) bool {
			return weight[[2]int64{n.ID, others[x].ID}] > weight[[2]int64{n.ID, others[y].ID}]
		})
		var names []string
		for j, o := range others {
			if j == 3 {
				names = append(names, fmt.Sprintf("%d other%s", len(others)-3, plural(len(others)-3)))
				break
			}
			names = append(names, o.Name+" "+p.Style.Dim(fmt.Sprint(weight[[2]int64{n.ID, o.ID}])))
		}
		if len(names) > 0 {
			fmt.Fprintln(p.Out, p.Style.Dim(stem+"also with ")+strings.Join(names, p.Style.Dim(" · ")))
		}
	}
	if rest := len(partners) - len(shown); rest > 0 {
		fmt.Fprintln(p.Out, p.Style.Dim(fmt.Sprintf("└─ and %d more (--jq '.nodes[] | select(.ring == 1)')", rest)))
	}

	if g.Depth >= 2 {
		shared(p, g.Nodes, links)
	}
}

// shared names the dancers beyond the partners who dance with several of
// them: why the outer ring is there at all.
func shared(p *output.Printer, nodes []partnerNode, links map[int64][]partnerNode) {
	type bridge struct {
		name string
		n    int
	}
	var bridges []bridge
	for _, n := range nodes {
		if n.Ring != 2 {
			continue
		}
		k := 0
		for _, o := range links[n.ID] {
			if o.Ring == 1 {
				k++
			}
		}
		if k >= 2 {
			bridges = append(bridges, bridge{n.Name, k})
		}
	}
	if len(bridges) == 0 {
		return
	}
	sort.SliceStable(bridges, func(i, j int) bool { return bridges[i].n > bridges[j].n })
	var items []string
	for i, b := range bridges {
		if i == 8 {
			items = append(items, fmt.Sprintf("%d more", len(bridges)-8))
			break
		}
		items = append(items, b.name+" "+p.Style.Dim(fmt.Sprintf("(%d)", b.n)))
	}
	fmt.Fprintln(p.Out)
	p.Field("shared", p.Style.Dim("dancers who partner two or more of them, and how many"))
	p.Items("", items)
}
