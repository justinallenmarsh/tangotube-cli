package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/justinallenmarsh/tangotube-cli/internal/api"
	"github.com/justinallenmarsh/tangotube-cli/internal/output"
)

// identityRow is one fact about a video as the watch page shows it: what we
// have, how we know it, and what is waiting for a yes.
type identityRow struct {
	Fact     string `json:"fact"`
	State    string `json:"state"` // settled, proposed, missing
	Value    string `json:"value"`
	How      string `json:"how"`
	Proposal *struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	} `json:"proposal"`
	Suggestion *struct {
		ID int `json:"id"`
	} `json:"suggestion"`
	Credits []struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
		Role string `json:"role"`
	} `json:"credits"`
	// Dancers the video names that the dance's credits, and so their dancer
	// pages, don't list yet; Reach says when they will.
	OnlyOnVideo []struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	} `json:"only_on_video"`
	Reach string `json:"reach"`
}

// reachNote is when a dancer the video names reaches their dancer page.
func reachNote(reach string) string {
	switch reach {
	case "placing":
		return "on their dancer page once the video is placed, within 15 minutes"
	case "tonight":
		return "on their dancer page after the nightly rebuild, 03:10 UTC"
	case "stuck":
		return "not on their dancer page yet: this dance's credits aren't rebuilt automatically"
	}
	return ""
}

type suggestionRef struct {
	ID     int    `json:"id"`
	Fact   string `json:"fact"`
	Value  string `json:"value"`
	Role   string `json:"role"`
	Status string `json:"status"`
}

var suggestFacts = []string{"dancer", "song", "event", "kind"}

func newSuggest(a *App) *cobra.Command {
	var names = map[string]*string{}
	var role, precision, note, batch string
	var agree, isNew bool
	const use = `tt suggest uGwRPRusbC0 --dancer "noelia hurtado" --role follower`
	cmd := &cobra.Command{
		Use:   "suggest VIDEO --dancer|--song|--event|--kind NAME",
		Short: "Tell TangoTube who danced, to what song, where: as the watch page asks.",
		Long: `Tell TangoTube what a video is: a dancer, the song, the event, or the kind
of video. The site's rules decide what happens, and tt says which:

  applied            on the video now: you agreed with what a machine proposed,
                     or you have enough standing on the site. A dancer shows on
                     the video at once and on their dancer page once the
                     dance's credits are built; tt says when
  sent for review    somebody will check it, or it applies once three people agree

Name records by slug or exact name; a near miss lists the candidates instead
of guessing (tt identify search finds them). A dancer TangoTube does not
have yet needs --new. --agree says the song is the one the video already
proposes, which is two sources agreeing and applies at once.

--batch FILE sends several changes to one video, all or nothing: a JSON list
of {fact, op, song|dancer|event|kind|name}, op being add, replace, remove or
withdraw. "-" reads it from stdin.`,
		Example: `  tt suggest uGwRPRusbC0 --song la-mulateada-carlos-di-sarli --agree
  tt suggest uGwRPRusbC0 --dancer "noelia hurtado" --role follower
  tt suggest uGwRPRusbC0 --dancer "Ana Nueva" --new --note "her name is in the description"
  tt suggest uGwRPRusbC0 --batch changes.json`,
		Args: exactArgs(1, use),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/api/v1/videos/" + url.PathEscape(NormalizeID(args[0])) + "/suggestions"
			if batch != "" {
				changes, err := readChanges(batch, a.In)
				if err != nil {
					return a.Fail(err)
				}
				env, err := a.Client().Do(ctx(cmd), http.MethodPost, path+"/batch", nil, map[string]any{"changes": changes}, true)
				return a.Show(env, err, func(p *output.Printer, raw json.RawMessage) {
					renderIdentity(p, NormalizeID(args[0]), decode[struct {
						Identity []identityRow `json:"identity"`
					}](raw).Identity)
				})
			}

			body := map[string]any{}
			for _, fact := range suggestFacts {
				if v := strings.TrimSpace(*names[fact]); v != "" {
					if body["fact"] != nil {
						return a.Fail(Usage("Suggest one thing at a time, or several with --batch", use))
					}
					body["fact"] = fact
					body[fact] = v
				}
			}
			if body["fact"] == nil {
				return a.Fail(Usage("Say what you are naming: --dancer, --song, --event, or --kind", use))
			}
			for k, v := range map[string]string{"role": role, "precision": precision, "note": note} {
				if v != "" {
					body[k] = v
				}
			}
			if agree {
				body["agree"] = true
			}
			if isNew {
				body["new"] = true
			}
			env, err := a.Client().Do(ctx(cmd), http.MethodPost, path, nil, body, true)
			return a.Show(env, err, renderSuggestion)
		},
	}
	f := cmd.Flags()
	for _, fact := range suggestFacts {
		names[fact] = new(string)
	}
	f.StringVar(names["dancer"], "dancer", "", "A dancer who is in the video, a `NAME` or slug.")
	f.StringVar(names["song"], "song", "", "The song, a `SLUG` or exact title.")
	f.StringVar(names["event"], "event", "", "The festival or milonga, a `SLUG` or exact name.")
	f.StringVar(names["kind"], "kind", "", "What kind of video, a `KIND`: performance, class, practice, …")
	f.StringVar(&role, "role", "", "With --dancer, the `ROLE` they danced: leader or follower.")
	f.StringVar(&precision, "precision", "", "With --song, `HOW` sure: rendition (the tune and orchestra) or take (that recording).")
	f.StringVar(&note, "note", "", "A `NOTE` for whoever checks it: how you know.")
	f.BoolVar(&agree, "agree", false, "With --song: it is the song the video already proposes.")
	f.BoolVar(&isNew, "new", false, "With --dancer: somebody TangoTube does not have yet.")
	f.StringVar(&batch, "batch", "", "Send the changes in `FILE` (JSON), all or nothing; - for stdin.")
	return cmd
}

// readChanges reads a batch: a JSON list of changes, or {"changes": [...]}.
func readChanges(name string, stdin io.Reader) ([]any, error) {
	var raw []byte
	var err error
	if name == "-" {
		raw, err = io.ReadAll(stdin)
	} else {
		raw, err = os.ReadFile(name)
	}
	if err != nil {
		return nil, Usage("Could not read "+name+": "+err.Error(), "tt suggest VIDEO --batch changes.json")
	}
	var list []any
	if json.Unmarshal(raw, &list) == nil {
		return list, nil
	}
	var wrapped struct {
		Changes []any `json:"changes"`
	}
	if json.Unmarshal(raw, &wrapped) == nil && wrapped.Changes != nil {
		return wrapped.Changes, nil
	}
	return nil, Usage(name+" is not a JSON list of changes",
		`[{"fact": "song", "op": "add", "song": "la-mulateada-carlos-di-sarli"}]`)
}

func renderSuggestion(p *output.Printer, raw json.RawMessage) {
	d := decode[struct {
		Suggestion suggestionRef `json:"suggestion"`
		Outcome    string        `json:"outcome"`
		Verified   *bool         `json:"verified"`
		Fact       identityRow   `json:"fact"`
	}](raw)
	s := d.Suggestion
	p.Field(s.Fact, join(" · ", s.Value, s.Role))
	switch {
	case d.Outcome == "applied" && d.Verified != nil && *d.Verified:
		p.Field("status", p.Style.Ok("applied")+p.Style.Dim(" · verified"))
	case d.Outcome == "applied":
		p.Field("status", p.Style.Ok("applied")+p.Style.Dim(" · unverified until others agree"))
	default:
		p.Field("status", p.Style.Gold("waiting for review")+p.Style.Dim(fmt.Sprintf(" · suggestion %d", s.ID)))
	}
}

func newConfirm(a *App) *cobra.Command {
	return &cobra.Command{
		Use:   "confirm VIDEO",
		Short: "Say the dancers credited on a video are right.",
		Long: `Say the dancers credited on a video are right, as the watch page's "yes,
that's them" does. Once per person; when enough people confirm, the credits
count as verified.`,
		Example: "  tt confirm uGwRPRusbC0",
		Args:    exactArgs(1, "tt confirm VIDEO"),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/api/v1/videos/" + url.PathEscape(NormalizeID(args[0])) + "/confirmation"
			env, err := a.Client().Do(ctx(cmd), http.MethodPost, path, nil, map[string]any{}, true)
			return a.Show(env, err, func(p *output.Printer, raw json.RawMessage) {
				d := decode[struct {
					Dancers identityRow `json:"dancers"`
				}](raw)
				p.Field("dancers", d.Dancers.Value)
			})
		},
	}
}

func newAgree(a *App) *cobra.Command {
	return &cobra.Command{
		Use:   "agree VIDEO SUGGESTION",
		Short: "Agree with somebody's answer that is waiting on a video.",
		Long: `Agree with somebody else's answer that is waiting on a video: tt video show
VIDEO --identity lists them with their numbers. Three people agreeing puts it
on the video, whoever wrote it.`,
		Example: "  tt agree uGwRPRusbC0 4812",
		Args:    exactArgs(2, "tt agree VIDEO SUGGESTION"),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/api/v1/videos/" + url.PathEscape(NormalizeID(args[0])) + "/suggestions/" + url.PathEscape(args[1]) + "/agree"
			env, err := a.Client().Do(ctx(cmd), http.MethodPost, path, nil, map[string]any{}, true)
			return a.Show(env, err, func(p *output.Printer, raw json.RawMessage) {
				d := decode[struct {
					Suggestion    suggestionRef `json:"suggestion"`
					Confirmations int           `json:"confirmations"`
					Needed        int           `json:"needed"`
				}](raw)
				p.Field(d.Suggestion.Fact, d.Suggestion.Value)
				p.Field("agreed", fmt.Sprintf("%d of %d", d.Confirmations, d.Needed))
			})
		},
	}
}

var reportKinds = "wrong_dancer, wrong_role, wrong_song, wrong_event, wrong_kind, not_tango, duplicate, or other"

func newReport(a *App) *cobra.Command {
	var kind, dancer, note string
	const use = "tt report uGwRPRusbC0 --kind not_tango"
	cmd := &cobra.Command{
		Use:   "report VIDEO --kind KIND",
		Short: "Say something on a video is wrong, as the site's report button does.",
		Long: `Say something on a video is wrong, as the site's report button does. No
account needed; a signed-in report weighs more, and enough weight takes the
fact down until somebody looks. --dancer reports that one dancer's credit.

Kinds: ` + reportKinds + `.`,
		Example: `  tt report uGwRPRusbC0 --kind not_tango
  tt report uGwRPRusbC0 --kind wrong_dancer --dancer "noelia hurtado" --note "that is her sister"`,
		Args: exactArgs(1, use),
		RunE: func(cmd *cobra.Command, args []string) error {
			if kind == "" {
				return a.Fail(Usage("Say what is wrong with --kind: "+reportKinds, use))
			}
			body := map[string]any{"video_id": NormalizeID(args[0]), "kind": strings.ReplaceAll(kind, "-", "_")}
			if dancer != "" {
				body["dancer"] = dancer
			}
			if note != "" {
				body["note"] = note
			}
			env, err := a.Client().Do(ctx(cmd), http.MethodPost, "/api/v1/reports", nil, body, false)
			return a.Show(env, err, func(p *output.Printer, raw json.RawMessage) {
				d := decode[struct {
					Report struct {
						Kind   string  `json:"kind"`
						About  string  `json:"about"`
						Dancer string  `json:"dancer"`
						Weight float64 `json:"weight"`
						HideAt float64 `json:"hide_at"`
					} `json:"report"`
				}](raw)
				p.Field("report", join(" · ", strings.ReplaceAll(d.Report.Kind, "_", " "), d.Report.Dancer))
				if d.Report.HideAt > 0 {
					p.Field("weight", fmt.Sprintf("%g of the %g that takes it down until somebody looks", d.Report.Weight, d.Report.HideAt))
				}
			})
		},
	}
	f := cmd.Flags()
	f.StringVar(&kind, "kind", "", "What is wrong, a `KIND`: "+reportKinds+".")
	f.StringVar(&dancer, "dancer", "", "The credited `DANCER` who is wrong, by name or slug.")
	f.StringVar(&note, "note", "", "A `NOTE` for whoever looks at it.")
	return cmd
}

func newClipTagSuggest(a *App) *cobra.Command {
	return &cobra.Command{
		Use:   "tag-suggest CLIP STEP",
		Short: "Suggest a step name the tags do not have yet.",
		Long: `Suggest a step name the tags do not have yet, for a clip. An admin adds new
steps to the vocabulary, which also tags the clip. Three a day; a step that
already exists is added with tt clip edit CLIP --add-tag STEP instead.`,
		Example: "  tt clip tag-suggest sacada-1-cuando-el-amor-muere calesita",
		Args:    exactArgs(2, "tt clip tag-suggest CLIP STEP"),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/api/v1/clips/" + url.PathEscape(args[0]) + "/tag_suggestions"
			env, err := a.Client().Do(ctx(cmd), http.MethodPost, path, nil, map[string]any{"name": args[1]}, true)
			return a.Show(env, err, nil)
		},
	}
}

func newQueue(a *App) *cobra.Command {
	var fact, dancer, orchestra, event, channel, cursor string
	var proposed bool
	cmd := &cobra.Command{
		Use:   "queue",
		Short: "Videos with something still to name, most watched first.",
		Long: `Videos with something still to name: no song, no dancers, no event, most
watched first, with what is still open on each. --proposed keeps the ones
where an answer is already waiting for a yes, the quickest help there is.

Narrow it to the videos you know: an orchestra, a dancer, an event, a channel.`,
		Example: `  tt queue --proposed
  tt queue --fact song --orchestra "di sarli"
  tt queue --dancer "noelia hurtado" --fact event`,
		Annotations: map[string]string{listsThings: "yes"},
		Args:        cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			q := url.Values{"limit": {fmt.Sprint(a.limit(10))}}
			for k, v := range map[string]string{"fact": fact, "dancer": dancer, "orchestra": orchestra, "event": event, "channel": channel, "cursor": cursor} {
				if v != "" {
					q.Set(k, v)
				}
			}
			if proposed {
				q.Set("proposed", "true")
			}
			env, err := a.Client().Get(ctx(cmd), "/api/v1/identify/queue", q)
			return a.Show(env, err, renderQueue)
		},
	}
	f := cmd.Flags()
	f.StringVar(&fact, "fact", "", "Only videos missing this `FACT`: song, dancers, or event.")
	f.StringVar(&dancer, "dancer", "", "Only videos with this `DANCER`.")
	f.StringVar(&orchestra, "orchestra", "", "Only videos to this `ORCHESTRA`.")
	f.StringVar(&event, "event", "", "Only videos from this `EVENT`.")
	f.StringVar(&channel, "channel", "", "Only videos on this `CHANNEL`, by title or YouTube id.")
	f.BoolVar(&proposed, "proposed", false, "Only videos where an answer is waiting for a yes.")
	f.StringVar(&cursor, "cursor", "", "Continue from the `CURSOR` the last page ended on.")
	return cmd
}

func renderQueue(p *output.Printer, raw json.RawMessage) {
	d := decode[struct {
		Videos []struct {
			Video api.Video     `json:"video"`
			Open  []identityRow `json:"open"`
		} `json:"videos"`
	}](raw)
	if len(d.Videos) == 0 {
		return
	}
	rows := make([][]string, 0, len(d.Videos))
	links := make([]string, 0, len(d.Videos))
	for _, r := range d.Videos {
		var open, waiting []string
		for _, o := range r.Open {
			open = append(open, o.Fact)
			if o.State == "proposed" && o.Proposal != nil {
				waiting = append(waiting, o.Proposal.Name)
			}
		}
		rows = append(rows, []string{r.Video.ID, r.Video.DancerNames(), strings.Join(open, ", "), strings.Join(waiting, "; ")})
		links = append(links, r.Video.WatchURL)
	}
	p.Table([]output.Column{
		{Header: "ID", ID: true, Links: links},
		{Header: "DANCERS", Flex: true},
		{Header: "OPEN"},
		{Header: "PROPOSED", Flex: true},
	}, rows)
}

func newIdentify(a *App) *cobra.Command {
	group := &cobra.Command{
		Use:   "identify",
		Short: "Find the record to name a video with.",
		Args:  cobra.NoArgs,
	}
	const use = `tt identify search song "la mulateada"`
	group.AddCommand(&cobra.Command{
		Use:   "search song|dancer|event QUERY",
		Short: "Find the song, dancer, or event to name a video with, and its slug.",
		Long: `Find the song, dancer, or event to name a video with, the way the watch
page's picker does, and the slug tt suggest takes. Songs come with their
orchestra and each recording's year, so a take can be told apart.`,
		Example: `  tt identify search song "la mulateada"
  tt identify search dancer noelia
  tt identify search event "planeta tango"`,
		Annotations: map[string]string{listsThings: "yes"},
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return Usage("Say what kind of thing, then what to look for", use)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			q := url.Values{"kind": {strings.ToLower(args[0])}, "q": {strings.Join(args[1:], " ")}, "limit": {fmt.Sprint(a.limit(10))}}
			env, err := a.Client().Get(ctx(cmd), "/api/v1/identify/search", q)
			return a.Show(env, err, renderCandidates)
		},
	})
	return group
}

func renderCandidates(p *output.Printer, raw json.RawMessage) {
	d := decode[struct {
		Kind       string `json:"kind"`
		Candidates []struct {
			Slug       string `json:"slug"`
			Name       string `json:"name"`
			Videos     int    `json:"videos"`
			Orchestra  string `json:"orchestra"`
			Genre      string `json:"genre"`
			City       string `json:"city"`
			Country    string `json:"country"`
			Recordings []struct {
				Year int `json:"year"`
			} `json:"recordings"`
		} `json:"candidates"`
	}](raw)
	if len(d.Candidates) == 0 {
		return
	}
	var rows [][]string
	for _, c := range d.Candidates {
		var years []string
		for _, r := range c.Recordings {
			if r.Year > 0 {
				years = append(years, fmt.Sprint(r.Year))
			}
		}
		switch d.Kind {
		case "song":
			rows = append(rows, []string{c.Slug, c.Name, c.Orchestra, c.Genre, strings.Join(years, " "), fmt.Sprint(c.Videos)})
		case "event":
			rows = append(rows, []string{c.Slug, c.Name, join(", ", c.City, c.Country), fmt.Sprint(c.Videos)})
		default:
			rows = append(rows, []string{c.Slug, c.Name, fmt.Sprint(c.Videos)})
		}
	}
	cols := []output.Column{{Header: "SLUG", ID: true}, {Header: "NAME", Flex: true}}
	switch d.Kind {
	case "song":
		cols = append(cols, output.Column{Header: "ORCHESTRA", Flex: true}, output.Column{Header: "GENRE", Dim: true}, output.Column{Header: "RECORDED", Dim: true})
	case "event":
		cols = append(cols, output.Column{Header: "WHERE", Flex: true})
	}
	p.Table(append(cols, output.Column{Header: "VIDEOS", Right: true}), rows)
}

// renderIdentity is what TangoTube knows about a video and how: the watch
// page's identity rows, with the yes each waiting answer needs.
func renderIdentity(p *output.Printer, id string, rows []identityRow) {
	if len(rows) == 0 {
		return
	}
	fmt.Fprintln(p.Out)
	fmt.Fprintln(p.Out, p.Style.Bold("What we know"))
	for _, r := range rows {
		switch r.State {
		case "settled":
			value := r.Value
			if r.How != "" {
				value += "  " + p.Style.Dim(r.How)
			}
			p.Field(r.Fact, value)
		case "proposed":
			name := r.Value
			if r.Proposal != nil {
				name = r.Proposal.Name
			}
			p.Field(r.Fact, p.Style.Gold(name)+"  "+p.Style.Dim(r.How+", waiting for a yes"))
			switch {
			case r.Suggestion != nil:
				p.Field("", p.Style.Gold(fmt.Sprintf("if that is right: tt agree %s %d", id, r.Suggestion.ID)))
			case r.Fact == "song" && r.Proposal != nil && r.Proposal.Slug != "":
				p.Field("", p.Style.Gold(fmt.Sprintf("if that is right: tt suggest %s --song %s --agree", id, r.Proposal.Slug)))
			}
		default:
			p.Field(r.Fact, p.Style.Dim("not known yet"))
		}
		if len(r.OnlyOnVideo) > 0 {
			names := make([]string, 0, len(r.OnlyOnVideo))
			for _, d := range r.OnlyOnVideo {
				names = append(names, d.Name)
			}
			// Wrapped, since the note can outrun the terminal: names plain,
			// where they are dim.
			words := strings.Fields(strings.Join(names, " & "))
			words[len(words)-1] += " " // two spaces before the note, as on every row
			for _, w := range strings.Fields("on the video; " + reachNote(r.Reach)) {
				words = append(words, p.Style.Dim(w))
			}
			p.Words("", words)
		}
	}
}
