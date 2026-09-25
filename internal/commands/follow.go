package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/spf13/cobra"

	"github.com/justinallenmarsh/tangotube-cli/internal/api"
	"github.com/justinallenmarsh/tangotube-cli/internal/output"
)

var followKinds = map[string]string{
	"dancer": "dancer", "dancers": "dancer",
	"channel": "channel", "channels": "channel",
	"event": "event", "events": "event",
}

// followTarget reads "dancer noelia hurtado" into dancer=noelia hurtado.
func followTarget(use string, args []string) (url.Values, error) {
	kind := followKinds[strings.ToLower(args[0])]
	if kind == "" {
		return nil, Usage("Follow a dancer, a channel, or an event", use)
	}
	name := strings.TrimSpace(strings.Join(args[1:], " "))
	if name == "" {
		return nil, Usage("Say which "+kind, use)
	}
	if kind == "channel" {
		name = NormalizeID(name)
	}
	return url.Values{kind: {name}}, nil
}

func followArgs(use string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) < 2 {
			return Usage("Say what to follow: dancer, channel, or event, then its name", use)
		}
		return nil
	}
}

type follow struct {
	Kind  string `json:"kind"`
	ID    string `json:"id"`
	Name  string `json:"name"`
	Level string `json:"level"`
	URL   string `json:"url"`
}

func newFollow(a *App) *cobra.Command {
	var level string
	const use = `tt follow dancer "noelia hurtado"`
	cmd := &cobra.Command{
		Use:   "follow dancer|channel|event NAME",
		Short: "Follow a dancer, channel, or event, and hear about their new videos.",
		Long: `Follow a dancer, a YouTube channel, or an event, and hear about their new
videos, as the Follow button on the site does. Following twice is following
once; --level changes how much you hear. Like on the site, your first follow
turns on the weekly email of new videos, which Settings turns off.`,
		Example: `  tt follow dancer "noelia hurtado"
  tt follow event planetango --level all_activity
  tt follow channel UCtdgMR0bmogczrZNpPaO66Q --level muted`,
		Args: followArgs(use),
		RunE: func(cmd *cobra.Command, args []string) error {
			target, err := followTarget(use, args)
			if err != nil {
				return a.Fail(err)
			}
			if level != "" {
				target.Set("level", strings.ReplaceAll(level, "-", "_"))
			}
			env, err := a.Client().Do(ctx(cmd), http.MethodPost, "/api/v1/follows", nil, valuesBody(target), true)
			return a.Show(env, err, func(p *output.Printer, raw json.RawMessage) {
				f := decode[follow](raw)
				p.Field("level", f.Level)
				p.FieldLink("page", f.URL, f.URL)
			})
		},
	}
	cmd.Flags().StringVar(&level, "level", "", "How much to hear, a `LEVEL`: personalized, all_activity, or muted.")
	return cmd
}

func newUnfollow(a *App) *cobra.Command {
	const use = `tt unfollow dancer "noelia hurtado"`
	return &cobra.Command{
		Use:     "unfollow dancer|channel|event NAME",
		Short:   "Stop following a dancer, channel, or event.",
		Example: "  tt unfollow dancer \"noelia hurtado\"\n  tt unfollow event planetango",
		Args:    followArgs(use),
		RunE: func(cmd *cobra.Command, args []string) error {
			target, err := followTarget(use, args)
			if err != nil {
				return a.Fail(err)
			}
			env, err := a.Client().Do(ctx(cmd), http.MethodDelete, "/api/v1/follows", target, nil, true)
			return a.Show(env, err, nil)
		},
	}
}

func newFollowing(a *App) *cobra.Command {
	var kind, cursor string
	var feed bool
	cmd := &cobra.Command{
		Use:   "following",
		Short: "Who you follow, or with --feed, their new videos.",
		Long: `Who you follow, or with --feed, what the dancers, channels, and events you
follow have posted in the last 30 days, grouped by who posted it, as the
site's Following page shows it.`,
		Example: `  tt following
  tt following --feed
  tt following --feed --kind dancers`,
		Annotations: map[string]string{listsThings: "yes"},
		Args:        cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			perPage := 50
			if feed {
				perPage = 5 // videos from each; the feed is a glance, not the catalogue
			}
			q := url.Values{"limit": {fmt.Sprint(a.limit(perPage))}}
			if kind != "" {
				q.Set("kind", strings.ToLower(kind))
			}
			if cursor != "" {
				q.Set("cursor", cursor)
			}
			if feed {
				q.Set("feed", "true")
				env, err := a.Client().Do(ctx(cmd), http.MethodGet, "/api/v1/me/following", q, nil, true)
				return a.Show(env, err, renderFeed)
			}
			env, err := a.Client().Do(ctx(cmd), http.MethodGet, "/api/v1/me/following", q, nil, true)
			return a.Show(env, err, renderFollowing)
		},
	}
	f := cmd.Flags()
	f.StringVar(&kind, "kind", "", "Only one `KIND`: dancers, channels, or events.")
	f.BoolVar(&feed, "feed", false, "Their new videos from the last 30 days, instead of who they are.")
	f.StringVar(&cursor, "cursor", "", "Continue from the `CURSOR` the last page ended on.")
	return cmd
}

func renderFollowing(p *output.Printer, raw json.RawMessage) {
	data := decode[struct {
		Follows []follow `json:"follows"`
	}](raw)
	if len(data.Follows) == 0 {
		return
	}
	rows := make([][]string, 0, len(data.Follows))
	links := make([]string, 0, len(data.Follows))
	for _, f := range data.Follows {
		rows = append(rows, []string{f.Name, f.Kind, f.ID, strings.ReplaceAll(f.Level, "_", " ")})
		links = append(links, f.URL)
	}
	p.Table([]output.Column{
		{Header: "NAME", ID: true, Flex: true, Links: links},
		{Header: "KIND", Dim: true},
		{Header: "ID", Flex: true},
		{Header: "HEARING", Dim: true},
	}, rows)
}

func renderFeed(p *output.Printer, raw json.RawMessage) {
	data := decode[struct {
		Groups []struct {
			Kind   string      `json:"kind"`
			Name   string      `json:"name"`
			Recent bool        `json:"recent"`
			Count  int         `json:"count"`
			Videos []api.Video `json:"videos"`
		} `json:"groups"`
	}](raw)
	// One set of widths for every group, so the columns line up down the feed.
	var cols []output.Column
	tables := make([][][]string, len(data.Groups))
	for i, g := range data.Groups {
		cols, tables[i] = videoColumns(g.Videos)
	}
	cols = p.Grid(cols, tables...)
	for i, g := range data.Groups {
		if i > 0 {
			fmt.Fprintln(p.Out)
		}
		when := "this month"
		if g.Recent {
			when = "this week"
		}
		fmt.Fprintf(p.Out, "%s  %s\n", p.Style.Bold(g.Name), p.Style.Dim(fmt.Sprintf("%s · %d new %s", g.Kind, g.Count, when)))
		_, rows := videoColumns(g.Videos)
		p.Rows(withLinks(cols, g.Videos), rows)
		if more := g.Count - len(g.Videos); more > 0 {
			fmt.Fprintln(p.Out, p.Style.Dim(fmt.Sprintf("and %d more", more)))
		}
	}
}

// withLinks is cols with the ID column linked to these videos.
func withLinks(cols []output.Column, videos []api.Video) []output.Column {
	out := append([]output.Column(nil), cols...)
	links := make([]string, 0, len(videos))
	for _, v := range videos {
		links = append(links, v.WatchURL)
	}
	out[0].Links = links
	return out
}
