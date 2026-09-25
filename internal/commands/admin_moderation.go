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

	"github.com/justinallenmarsh/tangotube-cli/internal/output"
)

// The review queues: enrichment proposals, audio matches, identification
// suggestions, reports, clip tags, a person's trust, and recrediting a
// performance. Every verb is the one the web admin (or the model) already
// uses; tt only carries it.

// listQuery is the query every moderation list sends: its own filters,
// then --limit and --cursor.
func (a *App) listQuery(cursor string, pairs ...string) url.Values {
	q := url.Values{}
	for i := 0; i+1 < len(pairs); i += 2 {
		setIf(q, pairs[i], pairs[i+1])
	}
	if a.Flags.Limit > 0 {
		q.Set("limit", strconv.Itoa(a.Flags.Limit))
	}
	setIf(q, "cursor", cursor)
	return q
}

func cursorFlag(cmd *cobra.Command, cursor *string) {
	cmd.Flags().StringVar(cursor, "cursor", "", "Continue from the `CURSOR` the last page ended on.")
}

// showMore says how to get the next page, when there is one.
func showMore(p *output.Printer, raw json.RawMessage, command string) {
	next := decode[struct {
		Next *string `json:"next_cursor"`
	}](raw).Next
	if next != nil && *next != "" {
		p.Line("more: %s --cursor %s", command, *next)
	}
}

func idArg(verb, example string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if err := exactArgs(1, example)(cmd, args); err != nil {
			return err
		}
		if _, err := strconv.Atoi(args[0]); err != nil {
			return Usage(verb+" takes a number, from the list", example)
		}
		return nil
	}
}

// tt admin review
func newAdminReview(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "review",
		Short: "The enrichment review piles: list, accept, keep, reject.",
		Long: `The two piles on the coverage desk that a person settles:

  conflict  a confident panel disagrees with the song a video has
  panel     a panel named a song the matcher would not apply

accept takes the proposal (it cannot be undone). keep closes it and stamps
the video's song manual so no harvest moves it again; reject closes it and
leaves the video as it is. keep and reject undo with tt admin undo. The
other piles (holes, blank, miss, label, print, payloads) are read with
tt admin coverage --pile.`,
		Example: `  tt admin review list --pile conflict
  tt admin review keep 48213 --dry-run
  tt admin review accept 48213 --note "listened: it is Poema"`,
		Args: cobra.NoArgs,
	}
	var pile, q, cursor string
	list := &cobra.Command{
		Use:         "list",
		Short:       "A review pile, deepest first, each proposal with its tt commands.",
		Example:     "  tt admin review list --pile panel\n  tt admin review list --jq '.bills[].commands[]'",
		Annotations: map[string]string{listsThings: "yes"},
		Args:        cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			path := "/api/v1/admin/reviews"
			return a.adminDo(cmd, http.MethodGet, path, a.listQuery(cursor, "pile", pile, "q", q), nil, func(p *output.Printer, raw json.RawMessage) {
				d := decode[struct {
					Bills []struct {
						Headline string   `json:"headline"`
						Facts    string   `json:"facts"`
						Commands []string `json:"commands"`
						Evidence []struct {
							Label  string `json:"label"`
							Detail string `json:"detail"`
						} `json:"evidence"`
						Takes []struct {
							YoutubeID string `json:"youtube_id"`
						} `json:"takes"`
					} `json:"bills"`
				}](raw)
				rows := make([][]string, 0, len(d.Bills))
				verbs := []string{}
				for _, b := range d.Bills {
					id := ""
					for _, c := range b.Commands {
						if parts := strings.Fields(c); len(parts) >= 5 {
							id = parts[4]
							if !contains(verbs, parts[3]) {
								verbs = append(verbs, parts[3])
							}
						}
					}
					video := ""
					if len(b.Takes) > 0 {
						video = b.Takes[0].YoutubeID
					}
					if stored, proposed, split := strings.Cut(b.Facts, " vs "); split {
						rows = append(rows, []string{id, video, b.Headline, stored, proposed})
						continue
					}
					panel := ""
					if len(b.Evidence) > 0 {
						panel = b.Evidence[0].Detail
					}
					rows = append(rows, []string{id, video, b.Headline, panel})
				}
				cols := []output.Column{
					{Header: "ID", ID: true, Right: true}, {Header: "VIDEO", Dim: true}, {Header: "DANCERS", Flex: true},
				}
				if orDefault(pile, "conflict") == "conflict" {
					cols = append(cols, output.Column{Header: "STORED", Flex: true}, output.Column{Header: "PROPOSED", Flex: true})
				} else {
					cols = append(cols, output.Column{Header: "THE PANEL SAID", Flex: true})
				}
				p.Table(cols, rows)
				if len(verbs) > 0 {
					p.Line("decide: tt admin review %s ID", strings.Join(verbs, "|"))
				}
				showMore(p, raw, "tt admin review list --pile "+orDefault(pile, "conflict"))
			})
		},
	}
	f := list.Flags()
	f.StringVar(&pile, "pile", "conflict", "Which `PILE`: conflict or panel.")
	f.StringVar(&q, "q", "", "Narrow to a title or YouTube id (3+ characters), a `QUERY`.")
	cursorFlag(list, &cursor)
	cmd.AddCommand(list)
	for _, verb := range []struct{ name, short string }{
		{"accept", "Take the proposal: its song goes on the video. Cannot be undone."},
		{"keep", "Keep the video's song: close the proposal and stamp the song manual."},
		{"reject", "Close the proposal; the video stays as it is."},
	} {
		var w writeFlags
		var dancer string
		var create bool
		name := verb.name
		c := &cobra.Command{
			Use:     name + " ID",
			Short:   verb.short,
			Example: "  tt admin review " + name + " 48213 --dry-run",
			Args:    idArg("a proposal's id", "tt admin review "+name+" 48213"),
			RunE: func(cmd *cobra.Command, args []string) error {
				body := map[string]any{}
				setBody(body, "dancer", dancer)
				if create {
					body["create_dancer"] = true
				}
				return a.adminDo(cmd, http.MethodPost, "/api/v1/admin/reviews/"+args[0]+"/"+name, nil, w.body(body), showChange)
			},
		}
		w.bind(c, false)
		if name == "accept" {
			c.Flags().StringVar(&dancer, "dancer", "", "For a dancer proposal: the dancer it names, by `SLUG`.")
			c.Flags().BoolVar(&create, "create-dancer", false, "For a dancer proposal: create the dancer it names.")
		}
		cmd.AddCommand(c)
	}
	return cmd
}

func setBody(body map[string]any, key, value string) {
	if value != "" {
		body[key] = value
	}
}

func orDefault(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

// tt admin audio
func newAdminAudio(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "audio",
		Short: "Audio match review: matches, verdict, rerun.",
		Long: `Audio match review, as the admin audio page does it. A verdict is a fact
check: right, wrong or unsure. wrong also rejects the proposal and takes the
song back if audio put it there; --song names the right one (--rendition when
you know the tune and orchestra but not the take). A verdict is recorded and
cannot be undone with tt admin undo; the song change is a payload row, which
the Payloads pile on /admin/coverage reverts.`,
		Example: `  tt admin audio matches
  tt admin audio verdict 3120 wrong --song poema-canaro --dry-run
  tt admin audio rerun 3120 --refingerprint`,
		Args: cobra.NoArgs,
	}
	var status, cursor string
	var judged bool
	matches := &cobra.Command{
		Use:         "matches",
		Short:       "The latest audio match per video; by default matched and not yet judged.",
		Example:     "  tt admin audio matches\n  tt admin audio matches --status conflict --judged",
		Annotations: map[string]string{listsThings: "yes"},
		Args:        cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			q := a.listQuery(cursor, "status", status)
			if judged {
				q.Set("judged", "true")
			}
			return a.adminDo(cmd, http.MethodGet, "/api/v1/admin/audio/matches", q, nil, func(p *output.Printer, raw json.RawMessage) {
				d := decode[struct {
					Matches []struct {
						ID     int      `json:"id"`
						Status string   `json:"status"`
						BER    *float64 `json:"ber"`
						Song   *struct {
							Title     string `json:"title"`
							Orchestra string `json:"orchestra"`
						} `json:"song"`
						Video struct {
							ID    string `json:"id"`
							Title string `json:"title"`
							Song  string `json:"song"`
						} `json:"video"`
						CreatedAt string `json:"created_at"`
					} `json:"matches"`
				}](raw)
				now := time.Now()
				rows := make([][]string, 0, len(d.Matches))
				for _, m := range d.Matches {
					heard, ber := "", ""
					if m.Song != nil {
						heard = join(" · ", m.Song.Title, m.Song.Orchestra)
					}
					if m.BER != nil {
						ber = fmt.Sprintf("%.3f", *m.BER)
					}
					rows = append(rows, []string{strconv.Itoa(m.ID), m.Video.ID, m.Video.Title, heard, ber, ago(m.CreatedAt, now)})
				}
				p.Table([]output.Column{
					{Header: "ID", ID: true, Right: true}, {Header: "VIDEO", Dim: true}, {Header: "TITLE", Flex: true},
					{Header: "AUDIO HEARD", Flex: true}, {Header: "BER", Right: true, Dim: true}, {Header: "WHEN", Dim: true},
				}, rows)
				showMore(p, raw, "tt admin audio matches")
			})
		},
	}
	f := matches.Flags()
	f.StringVar(&status, "status", "", "Only this `STATUS`: matched (default), weak, no_match, conflict or error.")
	f.BoolVar(&judged, "judged", false, "Include matches somebody has already judged.")
	cursorFlag(matches, &cursor)

	var vw writeFlags
	var song string
	var rendition bool
	verdict := &cobra.Command{
		Use:     "verdict ID right|wrong|unsure",
		Short:   "Say whether audio heard the right song. Recorded; not undoable here.",
		Example: "  tt admin audio verdict 3120 right\n  tt admin audio verdict 3120 wrong --song poema-canaro --dry-run",
		Args: func(cmd *cobra.Command, args []string) error {
			if err := exactArgs(2, "tt admin audio verdict 3120 right")(cmd, args); err != nil {
				return err
			}
			if !contains([]string{"right", "wrong", "unsure"}, args[1]) {
				return Usage("the verdict is right, wrong or unsure", "tt admin audio verdict 3120 right")
			}
			return idArg("a match's id", "tt admin audio verdict 3120 right")(cmd, args[:1])
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{"verdict": args[1]}
			setBody(body, "song", song)
			if rendition {
				body["rendition"] = true
			}
			return a.adminDo(cmd, http.MethodPost, "/api/v1/admin/audio/matches/"+args[0]+"/verdict", nil, vw.body(body), showChange)
		},
	}
	vw.bind(verdict, false)
	verdict.Flags().StringVar(&song, "song", "", "wrong: the song it really is, by `SLUG`.")
	verdict.Flags().BoolVar(&rendition, "rendition", false, "wrong: you know the tune and orchestra, not which recording.")

	var rw writeFlags
	var refingerprint bool
	rerun := &cobra.Command{
		Use:     "rerun ID",
		Short:   "Run the matcher on the video again, in the background.",
		Example: "  tt admin audio rerun 3120 --refingerprint",
		Args:    idArg("a match's id", "tt admin audio rerun 3120"),
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{}
			if refingerprint {
				body["refingerprint"] = true
			}
			return a.adminDo(cmd, http.MethodPost, "/api/v1/admin/audio/matches/"+args[0]+"/rerun", nil, rw.body(body), showChange)
		},
	}
	rw.bind(rerun, false)
	rerun.Flags().BoolVar(&refingerprint, "refingerprint", false, "Fetch the audio again before matching.")
	cmd.AddCommand(matches, verdict, rerun)
	return cmd
}

// adminSuggestion is one identification suggestion as the API lists it.
type adminSuggestion struct {
	ID     int    `json:"id"`
	Type   string `json:"type"`
	Status string `json:"status"`
	Names  string `json:"names"`
	Role   string `json:"role"`
	Video  struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	} `json:"video"`
	Author struct {
		Name       string `json:"name"`
		Trust      string `json:"trust"`
		TrustScore int    `json:"trust_score"`
	} `json:"author"`
	Confirmations int    `json:"confirmations"`
	CreatedAt     string `json:"created_at"`
}

// tt admin suggestions
func newAdminSuggestions(a *App) *cobra.Command {
	var status, cursor string
	var pending bool
	cmd := &cobra.Command{
		Use:   "suggestions",
		Short: "Identification suggestions waiting for a decision, newest first.",
		Long: `Identification suggestions: the dancers, songs, events and kinds people named
on videos. By default the pending ones, which wait because their author has
not yet earned the trust to apply them. --status accepted, rejected or all
lists the others. Decide one with tt admin suggestion accept|reject ID.`,
		Example:     "  tt admin suggestions\n  tt admin suggestions --status all --limit 50",
		Annotations: map[string]string{listsThings: "yes"},
		Args:        cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if pending {
				status = "pending"
			}
			all := status != "" && status != "pending"
			return a.adminDo(cmd, http.MethodGet, "/api/v1/admin/suggestions", a.listQuery(cursor, "status", status), nil, func(p *output.Printer, raw json.RawMessage) {
				d := decode[struct {
					Suggestions []adminSuggestion `json:"suggestions"`
				}](raw)
				now := time.Now()
				rows := make([][]string, 0, len(d.Suggestions))
				for _, s := range d.Suggestions {
					what := join(" · ", s.Names, s.Role)
					row := []string{strconv.Itoa(s.ID), s.Type, what, s.Video.ID, s.Video.Title, s.Author.Name, strconv.Itoa(s.Author.TrustScore)}
					if all {
						row = append(row, s.Status)
					}
					rows = append(rows, append(row, ago(s.CreatedAt, now)))
				}
				cols := []output.Column{
					{Header: "ID", ID: true, Right: true}, {Header: "FACT"}, {Header: "NAMES", Flex: true}, {Header: "VIDEO", Dim: true},
					{Header: "TITLE", Flex: true}, {Header: "BY", Flex: true}, {Header: "TRUST", Right: true, Dim: true},
				}
				if all {
					cols = append(cols, output.Column{Header: "STATUS", Dim: true})
				}
				p.Table(append(cols, output.Column{Header: "WHEN", Dim: true}), rows)
				showMore(p, raw, "tt admin suggestions")
			})
		},
	}
	f := cmd.Flags()
	f.StringVar(&status, "status", "", "Only this `STATUS`: pending (default), accepted, rejected or all.")
	f.BoolVar(&pending, "pending", false, "Only the pending ones (the default).")
	cursorFlag(cmd, &cursor)
	return cmd
}

// tt admin suggestion accept|reject
func newAdminSuggestion(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "suggestion",
		Short: "Decide one identification suggestion: accept or reject.",
		Long: `Decide one pending identification suggestion.

accept applies it the way a trusted author's applies: the credit or song goes
on the video through the ledger, the author's trust score rises a point, and
they are told. A dancer credit is also recredited onto the performance, so it
reaches the dancer's page now. reject lowers the author's score a point and
tells them. Neither can be undone with tt admin undo.`,
		Example: "  tt admin suggestion accept 9120 --dry-run\n  tt admin suggestion reject 9121 --note \"not in this video\"",
		Args:    cobra.NoArgs,
	}
	for _, verb := range []struct{ name, short string }{
		{"accept", "Apply a suggestion and raise its author's trust. Not undoable."},
		{"reject", "Reject a suggestion and lower its author's trust. Not undoable."},
	} {
		var w writeFlags
		name := verb.name
		c := &cobra.Command{
			Use:     name + " ID",
			Short:   verb.short,
			Example: "  tt admin suggestion " + name + " 9120 --dry-run",
			Args:    idArg("a suggestion's id", "tt admin suggestion "+name+" 9120"),
			RunE: func(cmd *cobra.Command, args []string) error {
				return a.adminDo(cmd, http.MethodPost, "/api/v1/admin/suggestions/"+args[0]+"/"+name, nil, w.body(nil), showChange)
			},
		}
		w.bind(c, false)
		cmd.AddCommand(c)
	}
	return cmd
}

// tt admin reports
func newAdminReportsInbox(a *App) *cobra.Command {
	var status, cursor string
	var open bool
	cmd := &cobra.Command{
		Use:   "reports",
		Short: "The report inbox: what people flagged as wrong, newest first.",
		Long: `The report inbox: what people flagged as wrong, newest first. By default the
open ones; --status resolved, dismissed or all lists the others. Close one
with tt admin report resolve|dismiss ID --note.`,
		Example:     "  tt admin reports\n  tt admin reports --status all --jq '.reports[] | {id, kind, note}'",
		Annotations: map[string]string{listsThings: "yes"},
		Args:        cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if open {
				status = "open"
			}
			all := status != "" && status != "open"
			return a.adminDo(cmd, http.MethodGet, "/api/v1/admin/reports", a.listQuery(cursor, "status", status), nil, func(p *output.Printer, raw json.RawMessage) {
				d := decode[struct {
					Reports []struct {
						ID     int    `json:"id"`
						Kind   string `json:"kind"`
						Status string `json:"status"`
						Note   string `json:"note"`
						Video  *struct {
							ID    string `json:"id"`
							Title string `json:"title"`
						} `json:"video"`
						Target *struct {
							Says string `json:"says"`
						} `json:"target"`
						Reporter  string `json:"reporter"`
						CreatedAt string `json:"created_at"`
					} `json:"reports"`
				}](raw)
				now := time.Now()
				rows := make([][]string, 0, len(d.Reports))
				for _, r := range d.Reports {
					video, about, by := "", "", r.Reporter
					if r.Video != nil {
						video, about = r.Video.ID, r.Video.Title
					}
					if r.Target != nil && r.Target.Says != "" {
						about = r.Target.Says
					}
					if by == "" {
						by = "anonymous"
					}
					row := []string{strconv.Itoa(r.ID), strings.ReplaceAll(r.Kind, "_", " "), video, about, r.Note, by}
					if all {
						row = append(row, r.Status)
					}
					rows = append(rows, append(row, ago(r.CreatedAt, now)))
				}
				cols := []output.Column{
					{Header: "ID", ID: true, Right: true}, {Header: "KIND"}, {Header: "VIDEO", Dim: true},
					{Header: "ABOUT", Flex: true}, {Header: "NOTE", Flex: true}, {Header: "BY", Dim: true},
				}
				if all {
					cols = append(cols, output.Column{Header: "STATUS", Dim: true})
				}
				p.Table(append(cols, output.Column{Header: "WHEN", Dim: true}), rows)
				showMore(p, raw, "tt admin reports")
			})
		},
	}
	f := cmd.Flags()
	f.StringVar(&status, "status", "", "Only this `STATUS`: open (default), resolved, dismissed or all.")
	f.BoolVar(&open, "open", false, "Only the open ones (the default).")
	cursorFlag(cmd, &cursor)
	return cmd
}

// tt admin report resolve|dismiss
func newAdminReportDecide(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Close one report: resolve or dismiss, with --note.",
		Long: `Close one open report. resolve says what was done; dismiss says why nothing
was. --note is required either way, and is kept on the report. A credit or a
fact hidden by reports shows again once its reports are closed. tt admin undo
reopens the report.`,
		Example: "  tt admin report resolve 77 --note \"credit withdrawn\" --dry-run\n  tt admin report dismiss 78 --note \"the credit is right\"",
		Args:    cobra.NoArgs,
	}
	for _, verb := range []struct{ name, short string }{
		{"resolve", "Close a report with what was done about it."},
		{"dismiss", "Close a report with why nothing was done."},
	} {
		var w writeFlags
		name := verb.name
		c := &cobra.Command{
			Use:     name + " ID",
			Short:   verb.short,
			Example: fmt.Sprintf("  tt admin report %s 77 --note \"…\" --dry-run", name),
			Args:    idArg("a report's id", "tt admin report "+name+" 77 --note \"…\""),
			RunE: func(cmd *cobra.Command, args []string) error {
				if strings.TrimSpace(w.note) == "" {
					return Usage("say what was done, or why not, with --note", "tt admin report "+name+" "+args[0]+" --note \"…\"")
				}
				return a.adminWrite(cmd, http.MethodPost, "/api/v1/admin/reports/"+args[0]+"/"+name, &w, nil)
			},
		}
		w.bind(c, false)
		cmd.AddCommand(c)
	}
	return cmd
}

// tt admin tags pending
func newAdminTags(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tags",
		Short: "Clip tag suggestions: pending.",
		Args:  cobra.NoArgs,
	}
	var cursor string
	pending := &cobra.Command{
		Use:         "pending",
		Short:       "Tags people suggested for clips, most suggested first, and the blocked ones.",
		Example:     "  tt admin tags pending",
		Annotations: map[string]string{listsThings: "yes"},
		Args:        cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.adminDo(cmd, http.MethodGet, "/api/v1/admin/tags/pending", a.listQuery(cursor), nil, func(p *output.Printer, raw json.RawMessage) {
				d := decode[struct {
					Tags []struct {
						Name        string   `json:"name"`
						Suggestions int      `json:"suggestions"`
						Clips       []string `json:"clips"`
						LastAt      string   `json:"last_at"`
					} `json:"tags"`
					Blocked []string `json:"blocked"`
				}](raw)
				now := time.Now()
				rows := make([][]string, 0, len(d.Tags))
				for _, t := range d.Tags {
					rows = append(rows, []string{t.Name, strconv.Itoa(t.Suggestions), strings.Join(t.Clips, ", "), ago(t.LastAt, now)})
				}
				p.Table([]output.Column{
					{Header: "TAG", ID: true}, {Header: "TIMES", Right: true}, {Header: "CLIPS", Flex: true, Dim: true}, {Header: "LAST", Dim: true},
				}, rows)
				if len(d.Blocked) > 0 {
					p.Field("blocked", strings.Join(d.Blocked, ", "))
				}
				showMore(p, raw, "tt admin tags pending")
			})
		},
	}
	cursorFlag(pending, &cursor)
	cmd.AddCommand(pending)
	return cmd
}

// tt admin tag accept|reject|block|unblock
func newAdminTag(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tag",
		Short: "Decide a suggested clip tag: accept, reject, block, unblock.",
		Long: `Decide a clip tag by name, for every suggestion of it at once.

  accept   into the vocabulary and onto every clip it was suggested for;
           each suggester's trust rises a point
  reject   close its suggestions; people may suggest it again
  block    take it off every clip and never take it again (asks for --yes)
  unblock  let it be suggested again; the clips are not tagged again

All four are recorded; none undoes with tt admin undo, and each says what
takes it back instead.`,
		Example: "  tt admin tag accept americana --dry-run\n  tt admin tag block spam_tag --yes",
		Args:    cobra.NoArgs,
	}
	for _, verb := range []struct {
		name, short string
		confirm     bool
	}{
		{"accept", "Accept a tag into the vocabulary and onto the clips it was suggested for.", false},
		{"reject", "Close every suggestion of a tag.", false},
		{"block", "Block a tag: off every clip, never suggested again. Asks for --yes.", true},
		{"unblock", "Let a blocked tag be suggested again.", false},
	} {
		var w writeFlags
		name := verb.name
		c := &cobra.Command{
			Use:     name + " NAME",
			Short:   verb.short,
			Example: "  tt admin tag " + name + " americana --dry-run",
			Args:    exactArgs(1, "tt admin tag "+name+" americana"),
			RunE: func(cmd *cobra.Command, args []string) error {
				return a.adminWrite(cmd, http.MethodPost, "/api/v1/admin/tags/"+url.PathEscape(args[0])+"/"+name, &w, nil)
			},
		}
		w.bind(c, verb.confirm)
		cmd.AddCommand(c)
	}
	return cmd
}

// tt admin user show|trust|supporter
func newAdminUser(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user",
		Short: "A person's standing: show, trust, supporter. Never roles or passwords.",
		Long: `A person's standing, found by their address: their trust (which decides
whether their suggestions apply without review) and whether they support
TangoTube. Roles, passwords, addresses and tokens are not here, by design.

Trust tiers: pending below 5, auto_apply from 5 (applied, unverified),
trusted from 20 or when --trusted is set (applied and verified). trust and
supporter are recorded and undo with tt admin undo.`,
		Example: `  tt admin user show ana@example.com
  tt admin user trust ana@example.com --trusted true --dry-run
  tt admin user supporter ana@example.com --on`,
		Args: cobra.NoArgs,
	}
	show := &cobra.Command{
		Use:     "show EMAIL",
		Short:   "A person's trust, supporter flag, and what they have sent.",
		Example: "  tt admin user show ana@example.com",
		Args:    exactArgs(1, "tt admin user show ana@example.com"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.adminDo(cmd, http.MethodGet, "/api/v1/admin/users/"+url.PathEscape(args[0]), nil, nil, func(p *output.Printer, raw json.RawMessage) {
				showAdminUser(p, raw)
			})
		},
	}
	var tw writeFlags
	var trusted string
	var score int
	trust := &cobra.Command{
		Use:     "trust EMAIL",
		Short:   "Set whether a person is trusted, or their trust score. Undoable.",
		Example: "  tt admin user trust ana@example.com --trusted true --dry-run\n  tt admin user trust ana@example.com --score 5",
		Args:    exactArgs(1, "tt admin user trust ana@example.com --trusted true"),
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{}
			if cmd.Flags().Changed("trusted") {
				v, err := strconv.ParseBool(trusted)
				if err != nil {
					return Usage("--trusted is true or false", "")
				}
				body["trusted"] = v
			}
			if cmd.Flags().Changed("score") {
				if score < 0 {
					return Usage("--score is 0 or more", "")
				}
				body["score"] = score
			}
			if len(body) == 0 {
				return Usage("say --trusted or --score", "tt admin user trust "+args[0]+" --trusted true")
			}
			return a.adminWrite(cmd, http.MethodPatch, "/api/v1/admin/users/"+url.PathEscape(args[0])+"/trust", &tw, body)
		},
	}
	tw.bind(trust, false)
	trust.Flags().StringVar(&trusted, "trusted", "", "`true` to trust them whatever their score, false to go by the score.")
	trust.Flags().IntVar(&score, "score", 0, "Set the trust `SCORE` (accepted suggestions raise it, rejected ones lower it).")

	var sw writeFlags
	var on, off bool
	supporter := &cobra.Command{
		Use:     "supporter EMAIL",
		Short:   "Mark a person a supporter, or not. Undoable.",
		Example: "  tt admin user supporter ana@example.com --on --dry-run",
		Args:    exactArgs(1, "tt admin user supporter ana@example.com --on"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if on == off {
				return Usage("say --on or --off", "tt admin user supporter "+args[0]+" --on")
			}
			return a.adminWrite(cmd, http.MethodPatch, "/api/v1/admin/users/"+url.PathEscape(args[0])+"/supporter", &sw, map[string]any{"supporter": on})
		},
	}
	sw.bind(supporter, false)
	supporter.Flags().BoolVar(&on, "on", false, "They support TangoTube.")
	supporter.Flags().BoolVar(&off, "off", false, "They no longer do.")
	cmd.AddCommand(show, trust, supporter)
	return cmd
}

func showAdminUser(p *output.Printer, raw json.RawMessage) {
	u := decode[struct {
		User struct {
			Name        string `json:"name"`
			Slug        string `json:"slug"`
			MemberSince string `json:"member_since"`
			Trust       string `json:"trust"`
			TrustScore  int    `json:"trust_score"`
			Trusted     bool   `json:"trusted"`
			Supporter   bool   `json:"supporter"`
			Suggestions struct {
				Pending, Accepted, Rejected int
			} `json:"suggestions"`
			TagSuggestions int            `json:"tag_suggestions"`
			Reports        map[string]int `json:"reports"`
		} `json:"user"`
	}](raw).User
	p.Field("name", join(" · ", u.Name, u.Slug))
	trust := fmt.Sprintf("%s, score %d", strings.ReplaceAll(u.Trust, "_", " "), u.TrustScore)
	if u.Trusted {
		trust += " (trusted by hand)"
	}
	p.Field("trust", trust)
	p.Field("supporter", map[bool]string{true: "yes", false: "no"}[u.Supporter])
	p.Field("suggested", fmt.Sprintf("%d pending, %d accepted, %d rejected", u.Suggestions.Pending, u.Suggestions.Accepted, u.Suggestions.Rejected))
	p.Field("tags", fmt.Sprintf("%d suggested", u.TagSuggestions))
	p.Field("reports", fmt.Sprintf("%d open, %d resolved, %d dismissed", u.Reports["open"], u.Reports["resolved"], u.Reports["dismissed"]))
	p.Field("since", u.MemberSince)
}

// tt admin performance recredit
func newAdminPerformance(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "performance",
		Short: "Operator changes to performances: recredit.",
		Args:  cobra.NoArgs,
	}
	var w writeFlags
	recredit := &cobra.Command{
		Use:   "recredit ID",
		Short: "Bring a performance's credits, and its dancers' pages, up to date with its videos.",
		Long: `Bring a performance's credits up to date with its videos' credits.

A dancer named on a video shows on the video at once, but reaches their dancer
page, pairings and search only through the performance's own credits, which
are built once and not rebuilt when a video gains a dancer later. recredit adds
the credits the videos carry and the performance lacks, and reindexes it.

It only adds: a credit on the performance that no video carries any more is
named and left alone. ID is a performance id or the YouTube id of any of its
videos. Recorded; not undoable (report a wrong credit to take it back).
A suggestion's dancer credit recredits its performance on its own; this is
for credits that arrived another way.`,
		Example: "  tt admin performance recredit uGwRPRusbC0 --dry-run\n  tt admin performance recredit 51234",
		Args:    exactArgs(1, "tt admin performance recredit uGwRPRusbC0"),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			if _, err := strconv.Atoi(id); err != nil {
				id = NormalizeID(id)
			}
			return a.adminDo(cmd, http.MethodPost, "/api/v1/admin/performances/"+url.PathEscape(id)+"/recredit", nil, w.body(nil), showRecredit)
		},
	}
	w.bind(recredit, false)
	cmd.AddCommand(recredit)
	return cmd
}

type recreditCredit struct {
	DancerSlug string `json:"dancer_slug"`
	DancerName string `json:"dancer_name"`
	Part       string `json:"part"`
}

func showRecredit(p *output.Printer, raw json.RawMessage) {
	d := decode[struct {
		DryRun      bool             `json:"dry_run"`
		Missing     []recreditCredit `json:"missing"`
		Added       []recreditCredit `json:"added"`
		Unsupported []recreditCredit `json:"unsupported"`
		Performance struct {
			Videos []string `json:"videos"`
		} `json:"performance"`
		Action *adminAction `json:"action"`
	}](raw)
	label, credits := "adds", d.Added
	if d.DryRun {
		label, credits = "would add", d.Missing
	}
	p.Field("videos", strings.Join(d.Performance.Videos, " "))
	for i, c := range credits {
		if i > 0 {
			label = ""
		}
		p.Field(label, fmt.Sprintf("%s (%s)", c.DancerName, c.DancerSlug))
	}
	for i, c := range d.Unsupported {
		label := ""
		if i == 0 {
			label = "left alone"
		}
		p.Field(label, fmt.Sprintf("%s (%s), on no video", c.DancerName, c.DancerSlug))
	}
	if d.Action != nil {
		p.Field("recorded", fmt.Sprintf("action %d", d.Action.ID))
	}
	if d.DryRun || d.Action != nil {
		p.Field("undo", "not undoable")
	}
}
