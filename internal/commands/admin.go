package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/justinallenmarsh/tangotube-cli/internal/output"
)

// tt admin: operating TangoTube. Every command needs an admin token, which
// only an operator can hold (tt auth login --admin). Writes take --dry-run to
// see the change first, destructive ones ask for --yes, and each one is
// recorded so tt admin actions lists it and tt admin undo can put it back.
func newAdmin(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "admin",
		Short: "Operate TangoTube: pipelines, records, the audit log. Operators only.",
		Long: `Operate TangoTube from the terminal: ask how the catalogue and its pipelines
are doing, change records, and undo what was changed.

Every command needs an admin token, which only an operator's account can
approve, and which lapses after 90 days:

  tt auth login --admin

Every change takes --dry-run to show what would happen first. Anything
destructive or hard to reverse asks for --yes. Every change is recorded with
its before and after: tt admin actions lists them, and tt admin undo ID puts
one back, unless something has changed the same field since.`,
		Example: `  tt admin dashboard
  tt admin video hide uGwRPRusbC0 --note "reupload" --dry-run
  tt admin dancer edit noelia-hurtado --bio "Born in Buenos Aires." --dry-run
  tt admin actions --since 24h
  tt admin undo 812`,
		Args: cobra.NoArgs,
	}
	cmd.AddCommand(
		newAdminDashboard(a), newAdminCoverage(a), newAdminPipeline(a), newAdminIntake(a),
		newAdminSearchQuality(a), newAdminChannels(a), newAdminDesk(a), newAdminPrecision(a), newAdminJobs(a),
		newAdminDescribe(a), newAdminQuery(a),
		newAdminActions(a), newAdminAction(a), newAdminUndo(a), newAdminVideo(a),
		newAdminDancer(a), newAdminChannel(a), newAdminClip(a), newAdminChampionships(a), newAdminImage(a),
		newAdminReview(a), newAdminAudio(a), newAdminSuggestions(a), newAdminSuggestion(a), newAdminReportsInbox(a),
		newAdminReportDecide(a), newAdminTags(a), newAdminTag(a), newAdminUser(a), newAdminPerformance(a), newAdminPayload(a),
		newAdminJob(a), newAdminCron(a), newAdminRebuild(a), newAdminAnnouncement(a),
	)
	cmd.AddCommand(newAdminCatalogue(a)...)
	return cmd
}

// adminDo calls an admin endpoint with the token and shows the answer.
func (a *App) adminDo(cmd *cobra.Command, method, path string, q url.Values, body any, human func(*output.Printer, json.RawMessage)) error {
	env, err := a.Client().Do(ctx(cmd), method, path, q, body, true)
	return a.Show(env, err, human)
}

// adminAction is one row of the audit log, as the API returns it.
type adminAction struct {
	ID      int    `json:"id"`
	Action  string `json:"action"`
	Subject struct {
		Type  string `json:"type"`
		Label string `json:"label"`
		Ref   any    `json:"ref"`
	} `json:"subject"`
	By         string         `json:"by"`
	At         string         `json:"at"`
	Before     map[string]any `json:"before"`
	After      map[string]any `json:"after"`
	Note       string         `json:"note"`
	RevertedAt string         `json:"reverted_at"`
	RevertedBy string         `json:"reverted_by"`
	Undoable   bool           `json:"undoable"`
}

// changeRows is one "field  before → after" row per field that moved, the
// fields lined up, each row cut to fit room columns. One line per field
// rather than one long clause: an edit to a date and a Spotify id ran off
// the terminal as a single " · "-joined row.
func changeRows(before, after map[string]any, room int) []string {
	var keys []string
	width := 0
	for _, k := range sortedKeys(after, before) {
		if plainValue(before[k]) == plainValue(after[k]) {
			continue
		}
		keys = append(keys, k)
		width = max(width, output.DisplayWidth(k))
	}
	rows := make([]string, 0, len(keys))
	for _, k := range keys {
		label := k + strings.Repeat(" ", width-output.DisplayWidth(k)) + "  "
		from, to := plainValue(before[k]), plainValue(after[k])
		if left := room - output.DisplayWidth(label) - 3; output.DisplayWidth(from)+output.DisplayWidth(to) > left {
			half := max(left/2, 8)
			if output.DisplayWidth(from) <= half {
				to = output.Truncate(to, max(left-output.DisplayWidth(from), 8))
			} else if output.DisplayWidth(to) <= half {
				from = output.Truncate(from, max(left-output.DisplayWidth(to), 8))
			} else {
				from, to = output.Truncate(from, half), output.Truncate(to, max(left-half, 8))
			}
		}
		rows = append(rows, label+from+" → "+to)
	}
	return rows
}

// showChanges writes changeRows under one label, a field to a line.
func showChanges(p *output.Printer, label string, before, after map[string]any) {
	for i, row := range changeRows(before, after, p.Columns()-13) {
		if i > 0 {
			label = ""
		}
		p.Field(label, row)
	}
}

// changeCell is a change in one table cell: the whole change when one field
// moved, the fields' names when several did.
func changeCell(before, after map[string]any) string {
	rows := changeRows(before, after, 1<<20)
	if len(rows) == 1 {
		return strings.Join(strings.Fields(rows[0]), " ")
	}
	var keys []string
	for _, k := range sortedKeys(after, before) {
		if plainValue(before[k]) != plainValue(after[k]) {
			keys = append(keys, k)
		}
	}
	return strings.Join(keys, ", ")
}

func sortedKeys(maps ...map[string]any) []string {
	seen := map[string]bool{}
	var keys []string
	for _, m := range maps {
		for k := range m {
			if !seen[k] {
				seen[k] = true
				keys = append(keys, k)
			}
		}
	}
	sort.Strings(keys)
	return keys
}

func plainValue(v any) string {
	switch t := v.(type) {
	case nil:
		return "none"
	case string:
		if t == "" {
			return `""`
		}
		return t
	default:
		raw, _ := json.Marshal(t)
		return string(raw)
	}
}

func newAdminActions(a *App) *cobra.Command {
	var since, kind, cursor string
	var mine, kinds bool
	cmd := &cobra.Command{
		Use:   "actions",
		Short: "List recorded operator changes, newest first.",
		Long: `List recorded operator changes, newest first: what changed, on what, by whom,
and whether tt admin undo can put it back. --kind narrows to one verb or a
family of them (video, video.hide, payload); --since takes a date or an age.
--kinds lists every verb the log holds and whether tt admin undo takes it.`,
		Example: `  tt admin actions --since 24h
  tt admin actions --kind video.hide --mine
  tt admin actions --kinds
  tt admin actions --jq '.actions[] | select(.undoable) | .id'`,
		Annotations: map[string]string{listsThings: "yes"},
		Args:        cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if kinds {
				return a.adminDo(cmd, http.MethodGet, "/api/v1/admin/actions/kinds", nil, nil, showActionKinds)
			}
			q := a.listQuery(cursor, "since", since, "kind", kind)
			if mine {
				q.Set("mine", "true")
			}
			return a.adminDo(cmd, http.MethodGet, "/api/v1/admin/actions", q, nil, func(p *output.Printer, raw json.RawMessage) {
				d := decode[struct {
					Actions []adminAction `json:"actions"`
				}](raw)
				rows := make([][]string, 0, len(d.Actions))
				now := time.Now()
				for _, act := range d.Actions {
					state := "undo"
					switch {
					case act.RevertedAt != "":
						state = "undone"
					case !act.Undoable:
						state = ""
					}
					rows = append(rows, []string{strconv.Itoa(act.ID), ago(act.At, now), act.By, act.Action,
						act.Subject.Label, changeCell(act.Before, act.After), state})
				}
				p.Table([]output.Column{
					{Header: "ID", ID: true, Right: true}, {Header: "WHEN", Dim: true}, {Header: "BY", Dim: true, Flex: true},
					{Header: "ACTION"}, {Header: "ON", Flex: true}, {Header: "CHANGE"}, {Header: "", Dim: true},
				}, rows)
				showMore(p, raw, strings.TrimSpace("tt admin actions "+actionsFilters(since, kind, mine)))
			})
		},
	}
	f := cmd.Flags()
	f.StringVar(&since, "since", "", "Only changes since a `WHEN`: a date (2026-09-01) or an age (24h, 7d, 2w).")
	f.StringVar(&kind, "kind", "", "Only this `VERB` or its family, like video or video.hide.")
	f.BoolVar(&mine, "mine", false, "Only your own changes.")
	f.BoolVar(&kinds, "kinds", false, "List the verbs the log holds, how often, and which undo.")
	cursorFlag(cmd, &cursor)
	return cmd
}

// actionsFilters repeats the filters a page was asked with, for the next.
func actionsFilters(since, kind string, mine bool) string {
	var parts []string
	if since != "" {
		parts = append(parts, "--since "+since)
	}
	if kind != "" {
		parts = append(parts, "--kind "+kind)
	}
	if mine {
		parts = append(parts, "--mine")
	}
	return strings.Join(parts, " ")
}

func showActionKinds(p *output.Printer, raw json.RawMessage) {
	d := decode[struct {
		Kinds []struct {
			Kind     string `json:"kind"`
			Count    int    `json:"count"`
			LastAt   string `json:"last_at"`
			Undoable bool   `json:"undoable"`
		} `json:"kinds"`
	}](raw)
	now := time.Now()
	rows := make([][]string, 0, len(d.Kinds))
	for _, k := range d.Kinds {
		undo := ""
		if k.Undoable {
			undo = "undoable"
		}
		rows = append(rows, []string{k.Kind, strconv.Itoa(k.Count), ago(k.LastAt, now), undo})
	}
	p.Table([]output.Column{{Header: "KIND", ID: true}, {Header: "COUNT", Right: true}, {Header: "LAST", Dim: true}, {Header: "", Dim: true}}, rows)
}

func newAdminAction(a *App) *cobra.Command {
	return &cobra.Command{
		Use:     "action ID",
		Short:   "Show one recorded change in full: before, after, note, undo.",
		Example: "  tt admin action 812",
		Args:    exactArgs(1, "tt admin action 812"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.adminDo(cmd, http.MethodGet, "/api/v1/admin/actions/"+url.PathEscape(args[0]), nil, nil, func(p *output.Printer, raw json.RawMessage) {
				showAdminAction(p, decode[struct {
					Action adminAction `json:"action"`
				}](raw).Action)
			})
		},
	}
}

func showAdminAction(p *output.Printer, act adminAction) {
	p.Field("action", fmt.Sprintf("%d · %s", act.ID, act.Action))
	p.Field("on", join(" · ", act.Subject.Label, plainRef(act.Subject.Ref)))
	p.Field("by", join(" · ", act.By, ago(act.At, time.Now())))
	showChanges(p, "change", act.Before, act.After)
	if act.Note != "" {
		p.Field("note", act.Note)
	}
	switch {
	case act.RevertedAt != "":
		p.Field("undone", join(" · ", act.RevertedBy, ago(act.RevertedAt, time.Now())))
	case act.Undoable:
		p.Field("undo", fmt.Sprintf("tt admin undo %d", act.ID))
	default:
		p.Field("undo", "not undoable")
	}
}

func plainRef(ref any) string {
	if ref == nil {
		return ""
	}
	return plainValue(ref)
}

func newAdminUndo(a *App) *cobra.Command {
	var dryRun, yes bool
	cmd := &cobra.Command{
		Use:   "undo ID",
		Short: "Put a recorded change back, if nothing has changed it since.",
		Long: `Put a recorded change back. It only runs while every field the change touched
still holds the value the change left: if anyone or anything has changed one
since, tt says which and leaves it alone. Undoing another operator's change
asks for --yes.`,
		Example: `  tt admin undo 812 --dry-run
  tt admin undo 812`,
		Args: exactArgs(1, "tt admin undo 812"),
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{}
			if dryRun {
				body["dry_run"] = true
			}
			if yes {
				body["confirm"] = true
			}
			return a.adminDo(cmd, http.MethodPost, "/api/v1/admin/actions/"+url.PathEscape(args[0])+"/undo", nil, body, func(p *output.Printer, raw json.RawMessage) {
				d := decode[struct {
					Action adminAction    `json:"action"`
					Would  map[string]any `json:"would_restore"`
				}](raw)
				if d.Would != nil {
					showChanges(p, "undo", d.Action.After, d.Would)
					return
				}
				showChanges(p, "restored", d.Action.After, d.Action.Before)
			})
		},
	}
	f := cmd.Flags()
	f.BoolVar(&dryRun, "dry-run", false, "Show what would be put back, and change nothing.")
	f.BoolVar(&yes, "yes", false, "Go ahead with undoing another operator's change.")
	return cmd
}

func newAdminVideo(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "video",
		Short: "Operator changes to videos: hide, feature, label, stamp, reidentify, import.",
		Args:  cobra.NoArgs,
	}
	for _, verb := range []struct{ name, short, long string }{
		{"hide", "Take a video out of search, the home page and every list.",
			"Take a video out of search, the home page and every list. It stays reachable by\nits id, and tt admin undo brings it back."},
		{"unhide", "Put a hidden video back in search and the lists.", "Put a hidden video back in search and the lists."},
	} {
		var note string
		var dryRun bool
		name := verb.name
		c := &cobra.Command{
			Use:     name + " VIDEO",
			Short:   verb.short,
			Long:    verb.long,
			Example: fmt.Sprintf("  tt admin video %s uGwRPRusbC0 --note \"reupload\" --dry-run", name),
			Args:    exactArgs(1, "tt admin video "+name+" uGwRPRusbC0"),
			RunE: func(cmd *cobra.Command, args []string) error {
				body := map[string]any{}
				if note != "" {
					body["note"] = note
				}
				if dryRun {
					body["dry_run"] = true
				}
				path := "/api/v1/admin/videos/" + url.PathEscape(NormalizeID(args[0])) + "/" + name
				return a.adminDo(cmd, http.MethodPost, path, nil, body, func(p *output.Printer, raw json.RawMessage) {
					d := decode[struct {
						Video struct {
							ID       string `json:"id"`
							Title    string `json:"title"`
							Hidden   bool   `json:"hidden"`
							WatchURL string `json:"watch_url"`
						} `json:"video"`
						Action *adminAction `json:"action"`
					}](raw)
					p.FieldLink("video", d.Video.Title, d.Video.WatchURL)
					p.Field("hidden", strconv.FormatBool(d.Video.Hidden))
					if d.Action != nil {
						p.Field("recorded", fmt.Sprintf("action %d", d.Action.ID))
					}
				})
			},
		}
		c.Flags().StringVar(&note, "note", "", "Why, a `NOTE` kept with the record.")
		c.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would change, and change nothing.")
		cmd.AddCommand(c)
	}
	cmd.AddCommand(newAdminVideoVerbs(a)...)
	return cmd
}

func setIf(q url.Values, key, value string) {
	if value != "" {
		q.Set(key, value)
	}
}
