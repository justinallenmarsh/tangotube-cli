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

// Playlist is one playlist; on its own page, with a page of its videos.
type Playlist struct {
	ID          string      `json:"id"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Visibility  string      `json:"visibility"`
	VideosCount int         `json:"videos_count"`
	Owner       string      `json:"owner"`
	Mine        bool        `json:"mine"`
	URL         string      `json:"url"`
	UpdatedAt   string      `json:"updated_at"`
	Videos      []api.Video `json:"videos"`
}

func newPlaylist(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "playlist",
		Short: "Make playlists of videos, put them in order, share them.",
		Long: `Make playlists of videos, put them in order, and share them. A playlist is
private, unlisted (anyone with the link), or public, as on the site. Anyone's
public or unlisted playlist can be read by its id; changing one needs your
token with write access (tt auth login).`,
		Args: cobra.NoArgs,
	}
	cmd.AddCommand(newPlaylistList(a), newPlaylistShow(a), newPlaylistCreate(a), newPlaylistRename(a),
		newPlaylistEdit(a), newPlaylistDelete(a), newPlaylistAdd(a), newPlaylistRemove(a), newPlaylistMove(a))
	return cmd
}

func playlistPath(id string, rest ...string) string {
	return "/api/v1/playlists/" + url.PathEscape(NormalizeID(id)) + strings.Join(rest, "")
}

func newPlaylistList(a *App) *cobra.Command {
	var sort, cursor string
	cmd := &cobra.Command{
		Use:         "list [QUERY]",
		Short:       "List your playlists.",
		Example:     "  tt playlist list\n  tt playlist list sunday --sort alphabetical",
		Annotations: map[string]string{listsThings: "yes"},
		Args:        cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			q := url.Values{"limit": {fmt.Sprint(a.limit(20))}}
			for k, v := range map[string]string{"q": strings.Join(args, " "), "sort": sort, "cursor": cursor} {
				if v != "" {
					q.Set(k, v)
				}
			}
			env, err := a.Client().Do(ctx(cmd), http.MethodGet, "/api/v1/playlists", q, nil, true)
			return a.Show(env, err, renderPlaylists)
		},
	}
	cmd.Flags().StringVar(&sort, "sort", "", "The `ORDER`: recent, oldest, alphabetical, or largest.")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Continue from the `CURSOR` the last page ended on.")
	return cmd
}

func renderPlaylists(p *output.Printer, raw json.RawMessage) {
	data := decode[struct {
		Playlists []Playlist `json:"playlists"`
	}](raw)
	if len(data.Playlists) == 0 {
		return
	}
	rows := make([][]string, 0, len(data.Playlists))
	links := make([]string, 0, len(data.Playlists))
	for _, pl := range data.Playlists {
		rows = append(rows, []string{pl.ID, pl.Title, fmt.Sprint(pl.VideosCount), pl.Visibility})
		links = append(links, pl.URL)
	}
	p.Table([]output.Column{
		{Header: "ID", ID: true, Links: links},
		{Header: "TITLE", Flex: true},
		{Header: "VIDEOS", Dim: true},
		{Header: "VISIBLE", Dim: true},
	}, rows)
}

func newPlaylistShow(a *App) *cobra.Command {
	var cursor string
	cmd := &cobra.Command{
		Use:   "show ID",
		Short: "Show a playlist and its videos, in order.",
		Long: `Show a playlist and its videos, in order. Yours, or anyone's public or
unlisted one: the id is the end of its tangotube.tv/playlists/ link.`,
		Example: "  tt playlist show di-sarli-for-sunday\n  tt playlist show https://tangotube.tv/playlists/di-sarli-for-sunday",
		Args:    exactArgs(1, "tt playlist show ID"),
		RunE: func(cmd *cobra.Command, args []string) error {
			q := url.Values{"limit": {fmt.Sprint(a.limit(50))}}
			if cursor != "" {
				q.Set("cursor", cursor)
			}
			env, err := a.Client().Get(ctx(cmd), playlistPath(args[0]), q)
			offset := 0
			if page, perr := strconv.Atoi(cursor); perr == nil && page > 1 {
				offset = (page - 1) * a.limit(50)
			}
			return a.ShowDetail(env, err, func(p *output.Printer, raw json.RawMessage) {
				renderPlaylist(p, raw, offset, true)
			})
		},
	}
	cmd.Flags().StringVar(&cursor, "cursor", "", "Continue from the `CURSOR` the last page ended on.")
	return cmd
}

// renderPlaylist prints a playlist's videos, numbered from offset+1, under its
// heading when the summary has not already named it.
func renderPlaylist(p *output.Printer, raw json.RawMessage, offset int, heading bool) {
	pl := decode[Playlist](raw)
	if heading {
		fmt.Fprintln(p.Out, p.Style.Bold(pl.Title))
		p.Field("id", p.Style.ID("", pl.ID))
		if pl.Description != "" {
			p.Para("about", pl.Description)
		}
		by := pl.Owner
		if pl.Mine {
			by = "you"
		}
		p.Field("by", by)
		p.Field("videos", fmt.Sprint(pl.VideosCount))
		p.Field("visible", pl.Visibility)
		p.FieldLink("link", pl.URL, pl.URL)
	}
	if len(pl.Videos) == 0 {
		return
	}
	fmt.Fprintln(p.Out)
	cols, rows := videoColumns(pl.Videos)
	cols = append([]output.Column{{Header: "#", Dim: true}}, cols...)
	for i := range rows {
		rows[i] = append([]string{fmt.Sprint(offset + i + 1)}, rows[i]...)
	}
	p.Table(cols, rows)
}

func renderPlaylistChange(p *output.Printer, raw json.RawMessage) {
	renderPlaylist(p, raw, 0, false)
}

func newPlaylistCreate(a *App) *cobra.Command {
	var description, visibility, video string
	cmd := &cobra.Command{
		Use:   "create TITLE",
		Short: "Make a playlist, private unless you say otherwise.",
		Example: `  tt playlist create "Di Sarli for Sunday"
  tt playlist create "Mundial finals" --visibility public --video uGwRPRusbC0`,
		Args: exactArgs(1, `tt playlist create "Di Sarli for Sunday"`),
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{"title": args[0]}
			for k, v := range map[string]string{"description": description, "visibility": visibility, "video_id": NormalizeID(video)} {
				if v != "" {
					body[k] = v
				}
			}
			env, err := a.Client().Do(ctx(cmd), http.MethodPost, "/api/v1/playlists", nil, body, true)
			return a.Show(env, err, func(p *output.Printer, raw json.RawMessage) {
				pl := decode[Playlist](raw)
				p.Field("id", p.Style.ID("", pl.ID))
				p.FieldLink("link", pl.URL, pl.URL)
			})
		},
	}
	f := cmd.Flags()
	f.StringVar(&description, "description", "", "What it is for, a `TEXT`.")
	f.StringVar(&visibility, "visibility", "", "Who can see it, a `VISIBILITY`: private, unlisted, or public.")
	f.StringVar(&video, "video", "", "Start it with this video, by `ID`.")
	return cmd
}

func patchPlaylist(a *App, cmd *cobra.Command, id string, body map[string]any) error {
	env, err := a.Client().Do(ctx(cmd), http.MethodPatch, playlistPath(id), nil, body, true)
	return a.Show(env, err, func(p *output.Printer, raw json.RawMessage) {
		pl := decode[Playlist](raw)
		p.FieldLink("link", pl.URL, pl.URL)
	})
}

func newPlaylistRename(a *App) *cobra.Command {
	return &cobra.Command{
		Use:     "rename ID TITLE",
		Short:   "Give one of your playlists a new title. Its id stays the same.",
		Example: `  tt playlist rename di-sarli-for-sunday "Di Sarli, Sunday night"`,
		Args:    exactArgs(2, `tt playlist rename ID "New title"`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return patchPlaylist(a, cmd, args[0], map[string]any{"title": args[1]})
		},
	}
}

func newPlaylistEdit(a *App) *cobra.Command {
	var title, description, visibility string
	cmd := &cobra.Command{
		Use:   "edit ID",
		Short: "Change a playlist's title, description, or who can see it.",
		Example: `  tt playlist edit di-sarli-for-sunday --visibility unlisted
  tt playlist edit di-sarli-for-sunday --description "For the Sunday practica"`,
		Args: exactArgs(1, "tt playlist edit ID --visibility public"),
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{}
			for _, name := range []string{"title", "description", "visibility"} {
				if cmd.Flags().Changed(name) {
					v, _ := cmd.Flags().GetString(name)
					body[name] = v
				}
			}
			if len(body) == 0 {
				return a.Fail(Usage("Say what to change: --title, --description, or --visibility",
					"tt playlist edit "+NormalizeID(args[0])+" --visibility public"))
			}
			return patchPlaylist(a, cmd, args[0], body)
		},
	}
	f := cmd.Flags()
	f.StringVar(&title, "title", "", "A new `TITLE`.")
	f.StringVar(&description, "description", "", "What it is for, a `TEXT`. Empty clears it.")
	f.StringVar(&visibility, "visibility", "", "Who can see it, a `VISIBILITY`: private, unlisted, or public.")
	return cmd
}

func newPlaylistDelete(a *App) *cobra.Command {
	return &cobra.Command{
		Use:     "delete ID",
		Short:   "Delete one of your playlists. The videos stay on TangoTube.",
		Example: "  tt playlist delete di-sarli-for-sunday",
		Args:    exactArgs(1, "tt playlist delete ID"),
		RunE: func(cmd *cobra.Command, args []string) error {
			env, err := a.Client().Do(ctx(cmd), http.MethodDelete, playlistPath(args[0]), nil, nil, true)
			return a.Show(env, err, nil)
		},
	}
}

func newPlaylistAdd(a *App) *cobra.Command {
	return &cobra.Command{
		Use:   "add ID VIDEO...",
		Short: "Add videos to the end of a playlist. One already there stays put.",
		Example: `  tt playlist add di-sarli-for-sunday uGwRPRusbC0
  tt playlist add di-sarli-for-sunday uGwRPRusbC0 n07s2yjCs-E`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return Usage("Name a playlist and at least one video", "tt playlist add ID uGwRPRusbC0")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			var env *output.Envelope
			var err error
			for _, video := range args[1:] {
				env, err = a.Client().Do(ctx(cmd), http.MethodPost, playlistPath(args[0], "/items"), nil,
					map[string]any{"video_id": NormalizeID(video)}, true)
				if err != nil {
					break
				}
			}
			return a.Show(env, err, renderPlaylistChange)
		},
	}
}

func newPlaylistRemove(a *App) *cobra.Command {
	return &cobra.Command{
		Use:     "remove ID VIDEO",
		Short:   "Take a video out of a playlist.",
		Example: "  tt playlist remove di-sarli-for-sunday uGwRPRusbC0",
		Args:    exactArgs(2, "tt playlist remove ID VIDEO"),
		RunE: func(cmd *cobra.Command, args []string) error {
			env, err := a.Client().Do(ctx(cmd), http.MethodDelete,
				playlistPath(args[0], "/items/", url.PathEscape(NormalizeID(args[1]))), nil, nil, true)
			return a.Show(env, err, renderPlaylistChange)
		},
	}
}

func newPlaylistMove(a *App) *cobra.Command {
	var to int
	cmd := &cobra.Command{
		Use:     "move ID VIDEO --to POSITION",
		Short:   "Move a video to a position in a playlist; 1 is first.",
		Example: "  tt playlist move di-sarli-for-sunday uGwRPRusbC0 --to 1",
		Args:    exactArgs(2, "tt playlist move ID VIDEO --to 1"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if to < 1 {
				return a.Fail(Usage("Say where to move it: --to 1 puts it first", "--to 1"))
			}
			env, err := a.Client().Do(ctx(cmd), http.MethodPut, playlistPath(args[0], "/order"), nil,
				map[string]any{"video_id": NormalizeID(args[1]), "position": to}, true)
			return a.Show(env, err, renderPlaylistChange)
		},
	}
	cmd.Flags().IntVar(&to, "to", 0, "The `POSITION` to move it to; 1 is first.")
	return cmd
}
