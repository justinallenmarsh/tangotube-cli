package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/justinallenmarsh/tangotube-cli/internal/api"
	"github.com/justinallenmarsh/tangotube-cli/internal/output"
)

func newClip(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clip",
		Short: "Create short practice clips from any video.",
		Long: `Create short practice clips from any video. Loop movements, tag techniques,
and build your personal library.

A clip is a start and an end on one performance, two seconds to five minutes,
tagged with the steps in it. Reading clips needs no account; making and
deleting them needs a token with write access (tt auth login).`,
		Args: cobra.NoArgs,
	}
	cmd.AddCommand(newClipList(a), newClipShow(a), newClipCreate(a), newClipEdit(a), newClipDelete(a), newClipTags(a), newClipTagSuggest(a))
	return cmd
}

func newClipList(a *App) *cobra.Command {
	var video, technique, style, dancer, cursor string
	var mine bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List practice clips, on one video or across the catalog.",
		Long: `List practice clips: the ones on a video, the ones tagged with a step, or
your own with --mine.`,
		Example: `  tt clip list --video uGwRPRusbC0
  tt clip list --technique sacada --dancer "noelia hurtado"
  tt clip list --mine`,
		Annotations: map[string]string{listsThings: "yes"},
		Args:        cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			q := url.Values{}
			set := func(k, v string) {
				if v != "" {
					q.Set(k, v)
				}
			}
			set("video", NormalizeID(video))
			set("technique", tagName(technique))
			set("style", tagName(style))
			set("dancer", dancer)
			set("cursor", cursor)
			if mine {
				q.Set("mine", "true")
			}
			q.Set("limit", fmt.Sprint(a.limit(20)))
			env, err := a.Client().Do(ctx(cmd), http.MethodGet, "/api/v1/clips", q, nil, mine)
			return a.Show(env, err, renderClips)
		},
	}
	f := cmd.Flags()
	f.StringVar(&video, "video", "", "Only clips on this video, by `ID`.")
	f.StringVar(&technique, "technique", "", "Only clips tagged with this `STEP`, like sacada.")
	f.StringVar(&style, "style", "", "Only clips tagged with this `STYLE`: milonguero, salon, nuevo, or stage.")
	f.StringVar(&dancer, "dancer", "", "Only clips of this dancer: a `NAME` or slug.")
	f.BoolVar(&mine, "mine", false, "Only your clips, private ones included.")
	f.StringVar(&cursor, "cursor", "", "Continue from the `CURSOR` the last page ended on.")
	return cmd
}

func renderClips(p *output.Printer, raw json.RawMessage) {
	data := decode[struct {
		Clips []api.Clip `json:"clips"`
	}](raw)
	if len(data.Clips) == 0 {
		p.Line("No clips here yet. Make the first with tt clip create.")
		return
	}
	rows := make([][]string, 0, len(data.Clips))
	var clipLinks, loopLinks []string
	for _, c := range data.Clips {
		rows = append(rows, []string{c.ID, c.VideoID, c.Range(), c.Title, strings.Join(c.Tags, " ")})
		clipLinks = append(clipLinks, c.ClipURL)
		loopLinks = append(loopLinks, c.WatchURL)
	}
	p.Table([]output.Column{
		{Header: "ID", ID: true, Flex: true, Links: clipLinks},
		{Header: "VIDEO", Dim: true, Links: loopLinks},
		{Header: "LOOP", Dim: true},
		{Header: "TITLE", Flex: true},
		{Header: "TAGS", Flex: true},
	}, rows)
}

func newClipShow(a *App) *cobra.Command {
	return &cobra.Command{
		Use:     "show ID",
		Short:   "Show one practice clip.",
		Example: `  tt clip show parallel-cross-sacada-turn-linear-exit`,
		Args:    exactArgs(1, "tt clip show ID"),
		RunE: func(cmd *cobra.Command, args []string) error {
			env, err := a.Client().Get(ctx(cmd), "/api/v1/clips/"+url.PathEscape(NormalizeID(args[0])), nil)
			return a.ShowDetail(env, err, renderClip)
		},
	}
}

func renderClip(p *output.Printer, raw json.RawMessage) {
	c := decode[api.Clip](raw)
	fmt.Fprintln(p.Out, p.Style.Bold(c.Title))
	renderClipFields(p, c)
}

// renderClipFields is a clip without its heading, for when the summary line
// above ("Saved “Sacada at 1:12” · 1:12–1:18") already names it.
func renderClipFields(p *output.Printer, c api.Clip) {
	p.Field("id", p.Style.ID("", c.ID))
	p.Field("video", c.VideoID)
	p.Field("loop", fmt.Sprintf("%s %s", c.Range(), p.Style.Dim(fmt.Sprintf("(%ds)", c.EndS-c.StartS))))
	p.Field("tags", strings.Join(c.Tags, " "))
	if c.Description != "" {
		p.Para("about", c.Description)
	}
	p.Field("visible", c.Visibility)
	p.FieldLink("clip", c.ClipURL, c.ClipURL)
	p.FieldLink("watch", c.WatchURL, c.WatchURL)
}

func newClipCreate(a *App) *cobra.Command {
	var start, end, title, visibility string
	var tags []string
	cmd := &cobra.Command{
		Use:   "create VIDEO --start TIME --end TIME",
		Short: "Make a practice clip: a loop on one video, tagged with its steps.",
		Long: `Make a practice clip: a start and an end on one video, tagged with the steps
in it. Times are m:ss, h:mm:ss, or plain seconds. A clip runs two seconds to
five minutes.

Clips are private unless you say otherwise, like on the site. Tags come from
the site's vocabulary; tt clip tags lists them.`,
		Example: `  tt clip create uGwRPRusbC0 --start 1:12 --end 1:18 --tag sacada
  tt clip create uGwRPRusbC0 --start 0:45 --end 1:05 --tag sacada,cruzada --tag salon --title "Sacada into the cross"
  tt clip create uGwRPRusbC0 --start 72 --end 78 --tag boleo --visibility public`,
		Args: exactArgs(1, "tt clip create uGwRPRusbC0 --start 1:12 --end 1:18 --tag sacada"),
		RunE: func(cmd *cobra.Command, args []string) error {
			video := NormalizeID(args[0])
			if start == "" || end == "" {
				return a.Fail(Usage("A clip needs a start and an end: --start and --end",
					"tt clip create "+video+" --start 1:12 --end 1:18 --tag sacada"))
			}
			startS, err := ParseTime(start)
			if err != nil {
				return a.Fail(Usage("--start: "+err.Error(), "--start 1:12"))
			}
			endS, err := ParseTime(end)
			if err != nil {
				return a.Fail(Usage("--end: "+err.Error(), "--end 1:18"))
			}
			if endS <= startS {
				return a.Fail(Usage("The clip has to end after it starts", "--start 1:12 --end 1:18"))
			}
			body := map[string]any{"start_s": startS, "end_s": endS, "tags": splitTags(tags)}
			if title != "" {
				body["title"] = title
			}
			if visibility != "" {
				body["visibility"] = visibility
			}
			env, err := a.Client().Do(ctx(cmd), http.MethodPost,
				"/api/v1/videos/"+url.PathEscape(video)+"/clips", nil, body, true)
			return a.Show(env, err, func(p *output.Printer, raw json.RawMessage) {
				renderClipFields(p, decode[api.Clip](raw))
			})
		},
	}
	f := cmd.Flags()
	f.StringVar(&start, "start", "", "Where the loop starts, a `TIME`: 1:12, 0:01:12, or 72.")
	f.StringVar(&end, "end", "", "Where the loop ends, a `TIME`.")
	f.StringSliceVar(&tags, "tag", nil, "A `STEP` or style in the clip. Repeat it, or separate with commas.")
	f.StringVar(&title, "title", "", "A `TITLE`. Defaults to the first tag and the start time.")
	f.StringVar(&visibility, "visibility", "", "Who can see it, a `VISIBILITY`: private, unlisted, or public. Defaults to private.")
	return cmd
}

func newClipEdit(a *App) *cobra.Command {
	var title, description, visibility, start, end string
	var tags, addTags, removeTags []string
	cmd := &cobra.Command{
		Use:   "edit ID",
		Short: "Change one of your clips: its title, loop, tags, or who can see it.",
		Long: `Change one of your practice clips. What you do not say stays as it is, and
the clip keeps its id and its link. --tag replaces the tags; --add-tag and
--remove-tag change them.`,
		Example: `  tt clip edit sacada-at-1-12 --title "Sacada into the cross"
  tt clip edit sacada-at-1-12 --start 1:10 --end 1:20
  tt clip edit sacada-at-1-12 --add-tag boleo --visibility public`,
		Args: exactArgs(1, `tt clip edit ID --title "Sacada into the cross"`),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := NormalizeID(args[0])
			body := map[string]any{}
			changed := cmd.Flags().Changed
			if changed("title") {
				body["title"] = title
			}
			if changed("description") {
				body["description"] = description
			}
			if changed("visibility") {
				body["visibility"] = visibility
			}
			for name, value := range map[string]string{"start": start, "end": end} {
				if !changed(name) {
					continue
				}
				s, err := ParseTime(value)
				if err != nil {
					return a.Fail(Usage("--"+name+": "+err.Error(), "--"+name+" 1:12"))
				}
				body[name+"_s"] = s
			}
			if changed("tag") {
				body["tags"] = splitTags(tags)
			}
			if changed("add-tag") {
				body["add_tags"] = splitTags(addTags)
			}
			if changed("remove-tag") {
				body["remove_tags"] = splitTags(removeTags)
			}
			if len(body) == 0 {
				return a.Fail(Usage("Say what to change: --title, --description, --start, --end, --visibility, or tags",
					"tt clip edit "+id+` --title "Sacada into the cross"`))
			}
			env, err := a.Client().Do(ctx(cmd), http.MethodPatch, "/api/v1/clips/"+url.PathEscape(id), nil, body, true)
			return a.Show(env, err, func(p *output.Printer, raw json.RawMessage) {
				renderClipFields(p, decode[api.Clip](raw))
			})
		},
	}
	f := cmd.Flags()
	f.StringVar(&title, "title", "", "A new `TITLE`.")
	f.StringVar(&description, "description", "", "What to watch for, a `TEXT`. Empty clears it.")
	f.StringVar(&start, "start", "", "A new start, a `TIME`: 1:12, 0:01:12, or 72.")
	f.StringVar(&end, "end", "", "A new end, a `TIME`.")
	f.StringVar(&visibility, "visibility", "", "Who can see it, a `VISIBILITY`: private, unlisted, or public.")
	f.StringSliceVar(&tags, "tag", nil, "Replace the tags with these `STEPS`. Repeat it, or separate with commas.")
	f.StringSliceVar(&addTags, "add-tag", nil, "Add a `STEP` to the tags.")
	f.StringSliceVar(&removeTags, "remove-tag", nil, "Take a `STEP` off the tags.")
	return cmd
}

func splitTags(raw []string) []string {
	tags := []string{}
	for _, t := range raw {
		for _, part := range strings.Split(t, ",") {
			if name := tagName(part); name != "" {
				tags = append(tags, name)
			}
		}
	}
	return tags
}

func newClipDelete(a *App) *cobra.Command {
	return &cobra.Command{
		Use:     "delete ID",
		Short:   "Delete one of your practice clips.",
		Example: `  tt clip delete sacada-at-1-12`,
		Args:    exactArgs(1, "tt clip delete ID"),
		RunE: func(cmd *cobra.Command, args []string) error {
			env, err := a.Client().Do(ctx(cmd), http.MethodDelete, "/api/v1/clips/"+url.PathEscape(NormalizeID(args[0])), nil, nil, true)
			return a.Show(env, err, nil)
		},
	}
}

func newClipTags(a *App) *cobra.Command {
	return &cobra.Command{
		Use:   "tags",
		Short: "List the steps and styles a clip can be tagged with.",
		Long: `List the vocabulary for clip tags: techniques (sacada, boleo, volcada…) and
styles (milonguero, salon, nuevo, stage). These are the only values --tag,
--technique, and --style accept.`,
		Example: `  tt clip tags
  tt clip tags --jq '.techniques[]'`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			env, err := a.Client().Get(ctx(cmd), "/api/v1/tags", nil)
			return a.Show(env, err, func(p *output.Printer, raw json.RawMessage) {
				data := decode[struct {
					Techniques []string `json:"techniques"`
					Styles     []string `json:"styles"`
				}](raw)
				p.Words("steps", data.Techniques)
				p.Words("styles", data.Styles)
			})
		},
	}
}

// ParseTime reads 1:12, 0:01:12, 72, or 72s as seconds.
func ParseTime(s string) (int, error) {
	s = strings.TrimSuffix(strings.TrimSpace(s), "s")
	if s == "" {
		return 0, fmt.Errorf("a time is needed, like 1:12")
	}
	parts := strings.Split(s, ":")
	if len(parts) > 3 {
		return 0, fmt.Errorf("%q is not a time; use m:ss, like 1:12", s)
	}
	total := 0
	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return 0, fmt.Errorf("%q is not a time; use m:ss, like 1:12", s)
		}
		if i > 0 && (n > 59 || len(part) != 2) {
			return 0, fmt.Errorf("%q is not a time; seconds and minutes after a colon run 00–59", s)
		}
		total = total*60 + n
	}
	return total, nil
}
