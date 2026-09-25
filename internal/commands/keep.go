package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/justinallenmarsh/tangotube-cli/internal/api"
	"github.com/justinallenmarsh/tangotube-cli/internal/output"
)

func newPractice(a *App) *cobra.Command {
	var technique, style, genre, dancer, orchestra, sort, cursor string
	cmd := &cobra.Command{
		Use:   "practice [QUERY]",
		Short: "Your practice list: the clips you have saved to loop.",
		Long: `Your practice list: the clips you have saved to loop, as the site's Practice
page keeps them. tt practice add saves a clip to it; rm takes one off.`,
		Example: `  tt practice
  tt practice --technique sacada
  tt practice add sacada-at-1-12
  tt practice rm sacada-at-1-12`,
		Annotations: map[string]string{listsThings: "yes"},
		Args:        cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			q := url.Values{"limit": {fmt.Sprint(a.limit(20))}}
			for k, v := range map[string]string{"q": strings.Join(args, " "), "technique": tagName(technique), "style": tagName(style),
				"genre": genre, "dancer": dancer, "orchestra": orchestra, "sort": sort, "cursor": cursor} {
				if v != "" {
					q.Set(k, v)
				}
			}
			env, err := a.Client().Do(ctx(cmd), http.MethodGet, "/api/v1/me/practice", q, nil, true)
			return a.Show(env, err, renderClips)
		},
	}
	f := cmd.Flags()
	f.StringVar(&technique, "technique", "", "Only clips tagged with this `STEP`, like sacada.")
	f.StringVar(&style, "style", "", "Only clips tagged with this `STYLE`.")
	f.StringVar(&genre, "genre", "", "Only this `GENRE`: tango, vals, or milonga.")
	f.StringVar(&dancer, "dancer", "", "Only clips of this `DANCER`.")
	f.StringVar(&orchestra, "orchestra", "", "Only clips danced to this `ORCHESTRA`.")
	f.StringVar(&sort, "sort", "", "The `ORDER`: recent, oldest, or popular.")
	f.StringVar(&cursor, "cursor", "", "Continue from the `CURSOR` the last page ended on.")
	cmd.AddCommand(&cobra.Command{
		Use:     "add CLIP",
		Short:   "Save a clip to your practice list. Saving twice is saving once.",
		Example: "  tt practice add sacada-at-1-12",
		Args:    exactArgs(1, "tt practice add CLIP"),
		RunE: func(cmd *cobra.Command, args []string) error {
			env, err := a.Client().Do(ctx(cmd), http.MethodPost, "/api/v1/me/practice", nil,
				map[string]any{"clip_id": NormalizeID(args[0])}, true)
			return a.Show(env, err, func(p *output.Printer, raw json.RawMessage) { renderClipFields(p, decode[api.Clip](raw)) })
		},
	}, &cobra.Command{
		Use:     "rm CLIP",
		Short:   "Take a clip off your practice list.",
		Example: "  tt practice rm sacada-at-1-12",
		Args:    exactArgs(1, "tt practice rm CLIP"),
		RunE: func(cmd *cobra.Command, args []string) error {
			env, err := a.Client().Do(ctx(cmd), http.MethodDelete, "/api/v1/me/practice/"+url.PathEscape(NormalizeID(args[0])), nil, nil, true)
			return a.Show(env, err, nil)
		},
	})
	return cmd
}

type savedSearch struct {
	ID      int               `json:"id"`
	Name    string            `json:"name"`
	Filters map[string]string `json:"filters"`
	Sort    string            `json:"sort"`
	Command string            `json:"command"`
	URL     string            `json:"url"`
}

func newSavedSearch(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "saved-search",
		Short: "Name a search to run again, as Save this search does on the site.",
		Long: `Name a search to run again, as "Save this search" does on the site. add takes
tt search's query and flags; each saved search comes back with the tt search
command that runs it, and run runs it.`,
		Args: cobra.NoArgs,
	}
	cmd.AddCommand(newSavedSearchList(a), newSavedSearchAdd(a), newSavedSearchRun(a), newSavedSearchRm(a))
	return cmd
}

func newSavedSearchList(a *App) *cobra.Command {
	return &cobra.Command{
		Use:         "list",
		Short:       "List your saved searches, each with the command that runs it.",
		Example:     "  tt saved-search list",
		Annotations: map[string]string{listsThings: "yes"},
		Args:        cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			env, err := a.Client().Do(ctx(cmd), http.MethodGet, "/api/v1/me/saved_searches",
				url.Values{"limit": {fmt.Sprint(a.limit(50))}}, nil, true)
			return a.Show(env, err, func(p *output.Printer, raw json.RawMessage) {
				data := decode[struct {
					SavedSearches []savedSearch `json:"saved_searches"`
				}](raw)
				if len(data.SavedSearches) == 0 {
					return
				}
				rows := make([][]string, 0, len(data.SavedSearches))
				links := make([]string, 0, len(data.SavedSearches))
				for _, s := range data.SavedSearches {
					rows = append(rows, []string{s.Name, s.Command})
					links = append(links, s.URL)
				}
				p.Table([]output.Column{{Header: "NAME", ID: true, Links: links}, {Header: "RUNS", Flex: true}}, rows)
			})
		},
	}
}

func newSavedSearchAdd(a *App) *cobra.Command {
	var f filters
	var sort string
	cmd := &cobra.Command{
		Use:   "add NAME [QUERY]",
		Short: "Save a search under a name. Saving under a name again replaces it.",
		Example: `  tt saved-search add "Di Sarli vals" --orchestra "di sarli" --genre vals
  tt saved-search add "Noelia classes" noelia --kind class --sort newest`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 || len(args) > 2 {
				return Usage("Name the search, then what it looks for", `tt saved-search add "Di Sarli vals" --orchestra "di sarli" --genre vals`)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			q := url.Values{"name": {args[0]}}
			if len(args) == 2 {
				q.Set("q", args[1])
			}
			f.set(q)
			if sort != "" {
				q.Set("sort", sort)
			}
			env, err := a.Client().Do(ctx(cmd), http.MethodPost, "/api/v1/me/saved_searches", nil, valuesBody(q), true)
			return a.Show(env, err, func(p *output.Printer, raw json.RawMessage) {
				s := decode[savedSearch](raw)
				p.Field("runs", s.Command)
				p.FieldLink("site", s.URL, s.URL)
			})
		},
	}
	f.register(cmd.Flags())
	cmd.Flags().StringVar(&sort, "sort", "", "The `ORDER`, as tt search takes it.")
	return cmd
}

// searchParams are a saved search's filters as tt search sends them.
var searchParams = map[string]string{"query": "q", "category": "kind", "recorded_from": "year_from", "recorded_to": "year_to"}

func newSavedSearchRun(a *App) *cobra.Command {
	return &cobra.Command{
		Use:         "run NAME",
		Short:       "Run a saved search.",
		Example:     `  tt saved-search run "Di Sarli vals"`,
		Annotations: map[string]string{listsThings: "yes"},
		Args:        exactArgs(1, `tt saved-search run "Di Sarli vals"`),
		RunE: func(cmd *cobra.Command, args []string) error {
			env, err := a.Client().Do(ctx(cmd), http.MethodGet, "/api/v1/me/saved_searches", url.Values{"limit": {"50"}}, nil, true)
			if err != nil {
				return a.Fail(err)
			}
			data := decode[struct {
				SavedSearches []savedSearch `json:"saved_searches"`
			}](env.Data)
			var found *savedSearch
			for i, s := range data.SavedSearches {
				if strings.EqualFold(s.Name, args[0]) || strconv.Itoa(s.ID) == args[0] {
					found = &data.SavedSearches[i]
				}
			}
			if found == nil {
				return a.Fail(api.Fail(output.CodeNotFound, fmt.Sprintf("No saved search called “%s”", args[0]), "tt saved-search list"))
			}
			q := url.Values{"limit": {fmt.Sprint(a.limit(20))}}
			for k, v := range found.Filters {
				if name, ok := searchParams[k]; ok {
					k = name
				}
				q.Set(k, v)
			}
			if found.Sort != "" {
				q.Set("sort", found.Sort)
			}
			env, err = a.Client().Get(ctx(cmd), "/api/v1/search", q)
			return a.Show(env, err, renderSearch)
		},
	}
}

func newSavedSearchRm(a *App) *cobra.Command {
	return &cobra.Command{
		Use:     "rm NAME",
		Short:   "Delete a saved search, by its name or id.",
		Example: `  tt saved-search rm "Di Sarli vals"`,
		Args:    exactArgs(1, `tt saved-search rm "Di Sarli vals"`),
		RunE: func(cmd *cobra.Command, args []string) error {
			env, err := a.Client().Do(ctx(cmd), http.MethodDelete, "/api/v1/me/saved_searches/"+url.PathEscape(args[0]), nil, nil, true)
			return a.Show(env, err, nil)
		},
	}
}

func newNotifications(a *App) *cobra.Command {
	var unread bool
	var cursor string
	cmd := &cobra.Command{
		Use:   "notifications",
		Short: "What the bell on the site would tell you, newest first.",
		Long: `What the bell on the site would tell you, newest first: new videos from who
you follow, likes on your clips, what happened to your suggestions. tt
notifications read marks them all read.`,
		Example: `  tt notifications
  tt notifications --unread
  tt notifications read`,
		Annotations: map[string]string{listsThings: "yes"},
		Args:        cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			q := url.Values{"limit": {fmt.Sprint(a.limit(20))}}
			if unread {
				q.Set("unread", "true")
			}
			if cursor != "" {
				q.Set("cursor", cursor)
			}
			env, err := a.Client().Do(ctx(cmd), http.MethodGet, "/api/v1/me/notifications", q, nil, true)
			return a.Show(env, err, renderNotifications)
		},
	}
	cmd.Flags().BoolVar(&unread, "unread", false, "Only the ones you have not read.")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Continue from the `CURSOR` the last page ended on.")
	cmd.AddCommand(&cobra.Command{
		Use:     "read",
		Short:   "Mark every notification read.",
		Example: "  tt notifications read",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			env, err := a.Client().Do(ctx(cmd), http.MethodPost, "/api/v1/me/notifications/read", nil, nil, true)
			return a.Show(env, err, nil)
		},
	})
	return cmd
}

func renderNotifications(p *output.Printer, raw json.RawMessage) {
	data := decode[struct {
		Notifications []struct {
			Message string `json:"message"`
			Read    bool   `json:"read"`
			At      string `json:"at"`
			URL     string `json:"url"`
		} `json:"notifications"`
	}](raw)
	if len(data.Notifications) == 0 {
		return
	}
	rows := make([][]string, 0, len(data.Notifications))
	links := make([]string, 0, len(data.Notifications))
	for _, n := range data.Notifications {
		mark := " "
		if !n.Read {
			mark = "•"
		}
		rows = append(rows, []string{mark, ago(n.At, time.Now()), n.Message})
		links = append(links, n.URL)
	}
	p.Rows([]output.Column{{ID: true}, {Dim: true}, {Flex: true, Links: links}}, rows)
}
