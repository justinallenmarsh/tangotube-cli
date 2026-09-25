package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/justinallenmarsh/tangotube-cli/internal/api"
	"github.com/justinallenmarsh/tangotube-cli/internal/output"
)

// collectionFlags are the filters the site's liked and history pages share.
// Names are read the way tt search reads them; slugs are taken as given.
type collectionFlags struct {
	leader, follower, orchestra, genre, event, channel, kind, year, sort, cursor string
}

func (c *collectionFlags) add(cmd *cobra.Command, sorts string) {
	f := cmd.Flags()
	f.StringVar(&c.leader, "leader", "", "Only videos this `DANCER` leads.")
	f.StringVar(&c.follower, "follower", "", "Only videos this `DANCER` follows.")
	f.StringVar(&c.orchestra, "orchestra", "", "Only this `ORCHESTRA`, by name or slug.")
	f.StringVar(&c.genre, "genre", "", "Only this `GENRE`: tango, vals, or milonga.")
	f.StringVar(&c.event, "event", "", "Only videos filmed at this `EVENT`.")
	f.StringVar(&c.channel, "channel", "", "Only this YouTube `CHANNEL`, by name or id.")
	f.StringVar(&c.kind, "kind", "", "Only this `KIND` of video: performance, class, practice…")
	f.StringVar(&c.year, "year", "", "Only videos filmed in this `YEAR`.")
	f.StringVar(&c.sort, "sort", "", "The `ORDER`: "+sorts+".")
	f.StringVar(&c.cursor, "cursor", "", "Continue from the `CURSOR` the last page ended on.")
}

func (c *collectionFlags) query(a *App, q string) url.Values {
	v := url.Values{}
	for key, value := range map[string]string{
		"q": q, "leader": c.leader, "follower": c.follower, "orchestra": c.orchestra, "genre": strings.ToLower(c.genre),
		"event": c.event, "channel": c.channel, "category": c.kind, "year": c.year,
		"sort": strings.ReplaceAll(c.sort, "-", "_"), "cursor": c.cursor,
	} {
		if value = strings.TrimSpace(value); value != "" {
			v.Set(key, value)
		}
	}
	v.Set("limit", fmt.Sprint(a.limit(20)))
	return v
}

// likeTarget is the body naming what to like: a video by default, a clip
// with --clip or when a tangotube.tv/clips/ link is pasted.
func likeTarget(arg string, clip bool) url.Values {
	id := NormalizeID(arg)
	if clip || strings.Contains(arg, "/clips/") {
		return url.Values{"clip_id": {id}}
	}
	return url.Values{"video_id": {id}}
}

func newLike(a *App) *cobra.Command {
	var clip bool
	cmd := &cobra.Command{
		Use:   "like ID",
		Short: "Like a video, or a practice clip with --clip.",
		Long: `Like a video, or a practice clip with --clip, as the heart on the site does.
Liking twice is liking once. Liking someone else's clip tells them, the way
the site does. Needs a token with write access (tt auth login).`,
		Example: `  tt like uGwRPRusbC0
  tt like --clip sacada-at-1-12
  tt like https://tangotube.tv/clips/sacada-at-1-12`,
		Args: exactArgs(1, "tt like uGwRPRusbC0"),
		RunE: func(cmd *cobra.Command, args []string) error {
			env, err := a.Client().Do(ctx(cmd), http.MethodPost, "/api/v1/likes", nil, valuesBody(likeTarget(args[0], clip)), true)
			return a.Show(env, err, renderLiked)
		},
	}
	cmd.Flags().BoolVar(&clip, "clip", false, "The ID is a practice clip's, not a video's.")
	return cmd
}

func newUnlike(a *App) *cobra.Command {
	var clip bool
	cmd := &cobra.Command{
		Use:     "unlike ID",
		Short:   "Take back a like on a video, or on a practice clip with --clip.",
		Example: "  tt unlike uGwRPRusbC0\n  tt unlike --clip sacada-at-1-12",
		Args:    exactArgs(1, "tt unlike uGwRPRusbC0"),
		RunE: func(cmd *cobra.Command, args []string) error {
			env, err := a.Client().Do(ctx(cmd), http.MethodDelete, "/api/v1/likes", likeTarget(args[0], clip), nil, true)
			return a.Show(env, err, nil)
		},
	}
	cmd.Flags().BoolVar(&clip, "clip", false, "The ID is a practice clip's, not a video's.")
	return cmd
}

func renderLiked(p *output.Printer, raw json.RawMessage) {
	data := decode[struct {
		Video *api.Video `json:"video"`
		Clip  *api.Clip  `json:"clip"`
	}](raw)
	switch {
	case data.Video != nil:
		p.FieldLink("watch", data.Video.WatchURL, data.Video.WatchURL)
	case data.Clip != nil:
		p.FieldLink("clip", data.Clip.ClipURL, data.Clip.ClipURL)
	}
}

func newLikes(a *App) *cobra.Command {
	var c collectionFlags
	var clips bool
	cmd := &cobra.Command{
		Use:   "likes [QUERY]",
		Short: "List the videos you have liked, or the clips with --clips.",
		Long: `List the videos you have liked, newest like first, filtered the way the
site's liked page filters: by a word in the title, a dancer, an orchestra,
an event, a channel, a year.`,
		Example: `  tt likes
  tt likes --orchestra "di sarli" --sort popular
  tt likes --clips`,
		Annotations: map[string]string{listsThings: "yes"},
		Args:        cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			q := c.query(a, strings.Join(args, " "))
			if clips {
				q.Set("kind", "clips")
				env, err := a.Client().Do(ctx(cmd), http.MethodGet, "/api/v1/me/likes", q, nil, true)
				return a.Show(env, err, renderClips)
			}
			env, err := a.Client().Do(ctx(cmd), http.MethodGet, "/api/v1/me/likes", q, nil, true)
			return a.Show(env, err, renderVideoList)
		},
	}
	c.add(cmd, "recent (when you liked it), newest (when it was filmed), or popular")
	cmd.Flags().BoolVar(&clips, "clips", false, "List the practice clips you have liked instead.")
	return cmd
}

func renderVideoList(p *output.Printer, raw json.RawMessage) {
	data := decode[struct {
		Videos []api.Video `json:"videos"`
	}](raw)
	if len(data.Videos) > 0 {
		videoTable(p, data.Videos)
	}
}

func newHistory(a *App) *cobra.Command {
	var c collectionFlags
	cmd := &cobra.Command{
		Use:   "history [QUERY]",
		Short: "What you have watched, when, and how far in.",
		Long: `What you have watched on TangoTube, most recent first, with the site's
history filters. tt history add records a watch (backdated with --at);
rm takes one video out; clear empties it, and asks for --yes.`,
		Example: `  tt history
  tt history --orchestra pugliese --sort popular
  tt history add uGwRPRusbC0 --at 2024-05-02T21:30
  tt history rm uGwRPRusbC0
  tt history clear --yes`,
		Annotations: map[string]string{listsThings: "yes"},
		Args:        cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			env, err := a.Client().Do(ctx(cmd), http.MethodGet, "/api/v1/me/history", c.query(a, strings.Join(args, " ")), nil, true)
			return a.Show(env, err, renderHistory)
		},
	}
	c.add(cmd, "recent, oldest, or popular")
	cmd.AddCommand(newHistoryAdd(a), newHistoryRm(a), newHistoryClear(a))
	return cmd
}

type watch struct {
	WatchedAt string    `json:"watched_at"`
	ProgressS int       `json:"progress_s"`
	Completed bool      `json:"completed"`
	Video     api.Video `json:"video"`
}

func renderHistory(p *output.Printer, raw json.RawMessage) {
	data := decode[struct {
		Watches []watch `json:"watches"`
	}](raw)
	if len(data.Watches) == 0 {
		return
	}
	videos := make([]api.Video, 0, len(data.Watches))
	for _, w := range data.Watches {
		videos = append(videos, w.Video)
	}
	cols, rows := videoColumns(videos)
	cols = append([]output.Column{{Header: "WATCHED", Dim: true}}, cols...)
	for i, w := range data.Watches {
		rows[i] = append([]string{watchedOn(w.WatchedAt)}, rows[i]...)
	}
	p.Table(cols, rows)
}

// watchedOn is "Sep 20", with the year when it is not this one.
func watchedOn(iso string) string {
	t, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		return ""
	}
	t = t.Local()
	if t.Year() != time.Now().Year() {
		return t.Format("Jan 2 2006")
	}
	return t.Format("Jan 2")
}

func newHistoryAdd(a *App) *cobra.Command {
	var at string
	cmd := &cobra.Command{
		Use:   "add VIDEO",
		Short: "Record that you watched a video, now or --at another time.",
		Example: `  tt history add uGwRPRusbC0
  tt history add uGwRPRusbC0 --at 2024-05-02T21:30`,
		Args: exactArgs(1, "tt history add uGwRPRusbC0"),
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{"video_id": NormalizeID(args[0])}
			if at != "" {
				body["at"] = at
			}
			env, err := a.Client().Do(ctx(cmd), http.MethodPost, "/api/v1/me/history", nil, body, true)
			return a.Show(env, err, func(p *output.Printer, raw json.RawMessage) {
				w := decode[watch](raw)
				p.Field("watched", watchedOn(w.WatchedAt))
				p.FieldLink("watch", w.Video.WatchURL, w.Video.WatchURL)
			})
		},
	}
	cmd.Flags().StringVar(&at, "at", "", "When you watched it, a `TIME` like 2024-05-02 or 2024-05-02T21:30. Defaults to now.")
	return cmd
}

func newHistoryRm(a *App) *cobra.Command {
	return &cobra.Command{
		Use:     "rm VIDEO",
		Short:   "Take one video out of your history.",
		Example: `  tt history rm uGwRPRusbC0`,
		Args:    exactArgs(1, "tt history rm uGwRPRusbC0"),
		RunE: func(cmd *cobra.Command, args []string) error {
			env, err := a.Client().Do(ctx(cmd), http.MethodDelete, "/api/v1/me/history/"+url.PathEscape(NormalizeID(args[0])), nil, nil, true)
			return a.Show(env, err, nil)
		},
	}
}

func newHistoryClear(a *App) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "clear --yes",
		Short: "Empty your whole watch history. There is no undo.",
		Long: `Empty your whole watch history, as the site's "Clear history" does. There is
no undo, so it asks for --yes.`,
		Example: `  tt history clear --yes`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !yes {
				return a.Fail(Usage("Clearing your history cannot be undone; say --yes", "tt history clear --yes"))
			}
			env, err := a.Client().Do(ctx(cmd), http.MethodDelete, "/api/v1/me/history", url.Values{"confirm": {"true"}}, nil, true)
			return a.Show(env, err, nil)
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "Yes, clear all of it.")
	return cmd
}

// valuesBody is a form's values as a JSON body.
func valuesBody(v url.Values) map[string]any {
	body := map[string]any{}
	for k := range v {
		body[k] = v.Get(k)
	}
	return body
}
