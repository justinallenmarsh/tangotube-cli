package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/justinallenmarsh/tangotube-cli/internal/output"
)

// The admin screens, read from the terminal. Each prints the few numbers a
// person looks for first; --json has everything the page has.

func adminReport(a *App, use, short, long, path string, example string, human func(*output.Printer, json.RawMessage)) *cobra.Command {
	return &cobra.Command{
		Use:     use,
		Short:   short,
		Long:    long,
		Example: example,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.adminDo(cmd, http.MethodGet, path, nil, nil, human)
		},
	}
}

func newAdminDashboard(a *App) *cobra.Command {
	return adminReport(a, "dashboard", "How TangoTube is doing: catalogue, imports, people, search, audio.",
		`How TangoTube is doing, the admin dashboard in one answer: the catalogue's size,
what arrived, how many people are using it, how search is answering, and the
audio pipeline. People are named, never with their email address.`,
		"/api/v1/admin/dashboard", "  tt admin dashboard\n  tt admin dashboard --jq '.health.enrichment_counts'",
		func(p *output.Printer, raw json.RawMessage) {
			d := decode[struct {
				Health struct {
					Total      int            `json:"total_videos"`
					Channels   int            `json:"total_channels"`
					Day        int            `json:"videos_imported_24h"`
					Week       int            `json:"videos_imported_7d"`
					Enrichment map[string]int `json:"enrichment_counts"`
				} `json:"health"`
				Activity struct {
					Tango    int `json:"tango_videos_count"`
					Users    int `json:"total_users"`
					Verified int `json:"verified_users"`
					DAU      int `json:"daily_active_users"`
					WAU      int `json:"weekly_active_users"`
					Signups  int `json:"signups_this_week"`
				} `json:"activity"`
				Search struct {
					Week     int     `json:"searches_7d"`
					CTR      float64 `json:"click_through_rate"`
					ZeroRate float64 `json:"zero_result_rate"`
				} `json:"search"`
				Audio struct {
					Today map[string]any `json:"today"`
				} `json:"audio"`
			}](raw)
			p.Field("videos", fmt.Sprintf("%s · %s tango · %s channels", figure(d.Health.Total), figure(d.Activity.Tango), figure(d.Health.Channels)))
			p.Field("imported", fmt.Sprintf("%s in 24h · %s in 7 days", figure(d.Health.Day), figure(d.Health.Week)))
			p.Field("named", fmt.Sprintf("%s with dancers · %s with a song · %s with an event",
				figure(d.Health.Enrichment["dancer"]), figure(d.Health.Enrichment["song"]), figure(d.Health.Enrichment["event"])))
			p.Field("people", fmt.Sprintf("%s accounts · %s verified · %s new this week", figure(d.Activity.Users), figure(d.Activity.Verified), figure(d.Activity.Signups)))
			p.Field("active", fmt.Sprintf("%s today · %s this week", figure(d.Activity.DAU), figure(d.Activity.WAU)))
			p.Field("search", fmt.Sprintf("%s in 7 days · %.1f%% clicked · %.1f%% found nothing", figure(d.Search.Week), d.Search.CTR, d.Search.ZeroRate))
			if d.Audio.Today != nil {
				p.Field("audio", "today: "+fmt.Sprintf("%v fingerprinted · %v applied · %v conflicts open",
					d.Audio.Today["fingerprinted"], d.Audio.Today["applied"], d.Audio.Today["conflicts_open"]))
			}
		})
}

func newAdminPipeline(a *App) *cobra.Command {
	return adminReport(a, "pipeline", "Imports by day, channel syncs, and the audio pipeline.",
		`Imports by day for the last two weeks, how recently the busiest channels
synced, and the audio pipeline's queues, budget and failures.`,
		"/api/v1/admin/pipeline", "  tt admin pipeline\n  tt admin pipeline --jq '.audio.failures'",
		func(p *output.Printer, raw json.RawMessage) {
			d := decode[struct {
				Health struct {
					Volume []struct {
						Date  string `json:"date"`
						Count int    `json:"count"`
					} `json:"import_volume"`
					Sync []struct {
						Title string `json:"title"`
						Last  string `json:"last_import_at"`
					} `json:"channel_sync_recency"`
				} `json:"health"`
				Audio struct {
					Queues map[string]int `json:"queues"`
					Judged struct {
						Decided int `json:"decided"`
						Needed  int `json:"needed"`
					} `json:"judging"`
					Blocked any `json:"blocked"`
				} `json:"audio"`
			}](raw)
			rows := make([][]string, 0, len(d.Health.Volume))
			for _, v := range d.Health.Volume {
				rows = append(rows, []string{v.Date, figure(v.Count)})
			}
			p.Table([]output.Column{{Header: "DAY", Dim: true}, {Header: "IMPORTED", Right: true}}, rows)
			fmt.Fprintln(p.Out)
			now := time.Now()
			rows = rows[:0]
			for _, s := range d.Health.Sync {
				last := "not recorded"
				if s.Last != "" {
					last = ago(s.Last, now)
				}
				rows = append(rows, []string{s.Title, last})
			}
			p.Table([]output.Column{{Header: "CHANNEL", Flex: true}, {Header: "LAST IMPORT", Dim: true}}, rows)
			fmt.Fprintln(p.Out)
			p.Field("audio", fmt.Sprintf("%d to fetch · %d to match", d.Audio.Queues["audio_fetch"], d.Audio.Queues["audio"]))
			if blocked, ok := d.Audio.Blocked.(string); ok && blocked != "" {
				p.Field("blocked", blocked)
			}
		})
}

func newAdminIntake(a *App) *cobra.Command {
	return adminReport(a, "intake", "What arrived this week, how it was classified, what is undecided.",
		`What arrived this week, how the classifier labelled it, and samples of what it
refused and could not decide.`,
		"/api/v1/admin/intake", "  tt admin intake\n  tt admin intake --jq '.undecided[].youtube_id'",
		func(p *output.Printer, raw json.RawMessage) {
			d := decode[struct {
				ByForm []struct {
					Form    string `json:"form"`
					Verdict string `json:"verdict"`
					Count   int    `json:"count"`
				} `json:"by_form"`
				Undecided []struct {
					ID      string `json:"youtube_id"`
					Title   string `json:"title"`
					Channel string `json:"channel"`
				} `json:"undecided"`
			}](raw)
			if len(d.ByForm) > 0 {
				rows := [][]string{}
				for _, f := range d.ByForm {
					rows = append(rows, []string{strings.ReplaceAll(f.Form, "_", " "), f.Verdict, figure(f.Count)})
				}
				p.Table([]output.Column{{Header: "FORM"}, {Header: "VERDICT", Dim: true}, {Header: "VIDEOS", Right: true}}, rows)
				fmt.Fprintln(p.Out)
			}
			rows := [][]string{}
			for _, v := range d.Undecided {
				rows = append(rows, []string{v.ID, v.Title, v.Channel})
			}
			if len(rows) > 0 {
				p.Line("%s", p.Style.Dim("Undecided"))
				p.Table([]output.Column{{Header: "ID", ID: true}, {Header: "TITLE", Flex: true}, {Header: "CHANNEL", Dim: true, Flex: true}}, rows)
			}
		})
}

func newAdminSearchQuality(a *App) *cobra.Command {
	return adminReport(a, "search-quality", "How search is answering: clicks, rank, queries that found nothing.",
		`How search is answering people: volume, click-through, the rank of what they
clicked, the most-asked queries, and the queries that found nothing.`,
		"/api/v1/admin/search-quality", "  tt admin search-quality\n  tt admin search-quality --jq '.zero_result_queries'",
		func(p *output.Printer, raw json.RawMessage) {
			type q struct {
				Query    string  `json:"query"`
				Searches int     `json:"searches"`
				CTR      float64 `json:"ctr"`
			}
			d := decode[struct {
				Top  []q `json:"top_queries"`
				Zero []q `json:"zero_result_queries"`
			}](raw)
			for _, list := range []struct {
				head string
				rows []q
			}{{"TOP QUERY", d.Top}, {"FOUND NOTHING", d.Zero}} {
				if len(list.rows) == 0 {
					continue
				}
				rows := [][]string{}
				for _, r := range list.rows {
					rows = append(rows, []string{r.Query, figure(r.Searches), fmt.Sprintf("%.1f%%", r.CTR)})
				}
				p.Table([]output.Column{{Header: list.head, Flex: true}, {Header: "SEARCHES", Right: true}, {Header: "CLICKED", Right: true, Dim: true}}, rows)
				fmt.Fprintln(p.Out)
			}
		})
}

func newAdminChannels(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "channels",
		Short: "The channel scorecard: the biggest channels and how well they are named.",
		Long: `The channel scorecard: the channels with the most tango videos, how many of
those have dancers, a song and an event, and when each last synced. Also the
active channels nobody has reviewed yet.`,
		Example:     "  tt admin channels --limit 20",
		Annotations: map[string]string{listsThings: "yes"},
		Args:        cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			q := url.Values{}
			if a.Flags.Limit > 0 {
				q.Set("limit", strconv.Itoa(a.Flags.Limit))
			}
			return a.adminDo(cmd, http.MethodGet, "/api/v1/admin/channels", q, nil, func(p *output.Printer, raw json.RawMessage) {
				d := decode[struct {
					Channels []struct {
						Title    string             `json:"title"`
						Slug     string             `json:"youtube_slug"`
						Videos   int                `json:"videos_count"`
						Synced   string             `json:"last_synced"`
						Coverage map[string]float64 `json:"enrichment_coverage"`
					} `json:"channels"`
				}](raw)
				now := time.Now()
				rows := [][]string{}
				for _, c := range d.Channels {
					rows = append(rows, []string{c.Title, figure(c.Videos),
						fmt.Sprintf("%.0f%%", c.Coverage["with_dancers"]), fmt.Sprintf("%.0f%%", c.Coverage["with_song"]),
						fmt.Sprintf("%.0f%%", c.Coverage["with_event"]), ago(c.Synced, now)})
				}
				p.Table([]output.Column{{Header: "CHANNEL", Flex: true}, {Header: "VIDEOS", Right: true},
					{Header: "DANCERS", Right: true}, {Header: "SONG", Right: true}, {Header: "EVENT", Right: true},
					{Header: "SYNCED", Dim: true}}, rows)
			})
		},
	}
	return cmd
}

func newAdminCoverage(a *App) *cobra.Command {
	var pile, window, clock, q string
	cmd := &cobra.Command{
		Use:   "coverage",
		Short: "The coverage desk: the heartbeat, or one pile of work with --pile.",
		Long: `The coverage desk. Without --pile, the heartbeat: how much of the catalogue is
named, and every alarm. With --pile, that pile of work, deepest first.

Piles: holes, blank, panel, conflict, miss, label, print, payloads.`,
		Example: `  tt admin coverage
  tt admin coverage --pile conflict
  tt admin coverage --pile blank --window 7d --jq '.work.bills[0]'`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			v := url.Values{}
			setIf(v, "pile", pile)
			setIf(v, "window", window)
			setIf(v, "clock", clock)
			setIf(v, "q", q)
			return a.adminDo(cmd, http.MethodGet, "/api/v1/admin/coverage", v, nil, func(p *output.Printer, raw json.RawMessage) {
				d := decode[struct {
					Heartbeat *struct {
						Lights []struct {
							Label string `json:"label"`
							Value any    `json:"value"`
							Note  string `json:"note"`
						} `json:"lights"`
						Alarms []map[string]any `json:"alarms"`
					} `json:"heartbeat"`
					Work *struct {
						Piles []struct {
							Key   string `json:"key"`
							Label string `json:"label"`
							Count int    `json:"count"`
						} `json:"piles"`
						Pile  string `json:"pile"`
						Bills []struct {
							Headline string `json:"headline"`
							Facts    string `json:"facts"`
							Takes    []struct {
								ID string `json:"youtube_id"`
							} `json:"takes"`
						} `json:"bills"`
					} `json:"work"`
				}](raw)
				if h := d.Heartbeat; h != nil {
					rows := [][]string{}
					for _, l := range h.Lights {
						rows = append(rows, []string{l.Label, plainValue(l.Value), l.Note})
					}
					p.Table([]output.Column{{Header: "LIGHT"}, {Header: "NOW", Right: true}, {Header: "", Dim: true, Flex: true}}, rows)
					p.Field("alarms", figure(len(h.Alarms)))
				}
				if w := d.Work; w != nil {
					rows := [][]string{}
					for _, pl := range w.Piles {
						mark := ""
						if pl.Key == w.Pile {
							mark = "●"
						}
						rows = append(rows, []string{mark, pl.Key, pl.Label, figure(pl.Count)})
					}
					p.Table([]output.Column{{Header: ""}, {Header: "PILE", ID: true}, {Header: "", Flex: true}, {Header: "WAITING", Right: true}}, rows)
					fmt.Fprintln(p.Out)
					rows = rows[:0]
					for _, b := range w.Bills {
						id := ""
						if len(b.Takes) > 0 {
							id = b.Takes[0].ID
						}
						rows = append(rows, []string{id, b.Headline, b.Facts})
					}
					p.Table([]output.Column{{Header: "VIDEO", ID: true}, {Header: "WHO", Flex: true}, {Header: "WHAT", Flex: true, Dim: true}}, rows)
				}
			})
		},
	}
	f := cmd.Flags()
	f.StringVar(&pile, "pile", "", "Open this `PILE` of work instead of the heartbeat.")
	f.StringVar(&window, "window", "", "Only the last `WINDOW`: 24h, 7d or all.")
	f.StringVar(&clock, "clock", "", "Count from when a video was matched or arrived (`CLOCK`: matched, arrived).")
	f.StringVar(&q, "q", "", "Narrow the pile to `TEXT` (3 characters or more).")
	return cmd
}

func newAdminDesk(a *App) *cobra.Command {
	return &cobra.Command{
		Use:   "desk VIDEO",
		Short: "Everything the record holds about one video, and where each fact came from.",
		Long: `Everything the record holds about one video: what each source said (the
title, the YouTube Music panel, audio fingerprints), what the ledger did with
it, the fact checks, and the matching methods' standing. --json has it all.`,
		Example: "  tt admin desk EJv04w-mZaM\n  tt admin desk EJv04w-mZaM --jq '.song.sources'",
		Args:    exactArgs(1, "tt admin desk EJv04w-mZaM"),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/api/v1/admin/desk/" + url.PathEscape(NormalizeID(args[0]))
			return a.adminDo(cmd, http.MethodGet, path, nil, nil, func(p *output.Printer, raw json.RawMessage) {
				d := decode[struct {
					Video struct {
						ID       string `json:"youtube_id"`
						Title    string `json:"title"`
						Channel  string `json:"channel"`
						Uploaded string `json:"upload_date"`
						Hidden   bool   `json:"hidden"`
						Reviewed bool   `json:"reviewed"`
					} `json:"video"`
					Song struct {
						Record struct {
							Song       any     `json:"song"`
							Method     string  `json:"method"`
							Confidence float64 `json:"confidence"`
						} `json:"record"`
					} `json:"song"`
					Credits []map[string]any `json:"credits"`
					Ledger  []map[string]any `json:"ledger"`
				}](raw)
				p.Field("video", join(" · ", d.Video.Title, d.Video.ID))
				p.Field("channel", join(" · ", d.Video.Channel, d.Video.Uploaded))
				p.Field("state", fmt.Sprintf("hidden %t · reviewed %t", d.Video.Hidden, d.Video.Reviewed))
				song := plainValue(d.Song.Record.Song)
				if d.Song.Record.Method != "" {
					song += fmt.Sprintf(" · %s %.2f", d.Song.Record.Method, d.Song.Record.Confidence)
				}
				p.Field("song", song)
				p.Field("credits", figure(len(d.Credits)))
				p.Field("ledger", figure(len(d.Ledger))+" entries")
			})
		},
	}
}

func newAdminPrecision(a *App) *cobra.Command {
	return adminReport(a, "precision", "How right each matching method has been, from probe judgments.",
		`How right each matching method has been, measured by probe judgments of facts
drawn at random. A method needs 200 checks before its precision counts; then
0.98 lets it apply alone and 0.90 lets it corroborate.`,
		"/api/v1/admin/precision", "  tt admin precision",
		func(p *output.Printer, raw json.RawMessage) {
			d := decode[struct {
				Methods []struct {
					Method    string   `json:"method"`
					Fact      string   `json:"fact"`
					Precision *float64 `json:"precision"`
					Checked   int      `json:"checked"`
					Standing  string   `json:"standing"`
				} `json:"methods"`
			}](raw)
			rows := [][]string{}
			for _, m := range d.Methods {
				prec := "—"
				if m.Precision != nil {
					prec = fmt.Sprintf("%.3f", *m.Precision)
				}
				rows = append(rows, []string{m.Method, m.Fact, prec, figure(m.Checked), m.Standing})
			}
			p.Table([]output.Column{{Header: "METHOD"}, {Header: "FACT", Dim: true}, {Header: "PRECISION", Right: true},
				{Header: "CHECKED", Right: true}, {Header: "STANDING", Flex: true}}, rows)
		})
}

func newAdminJobs(a *App) *cobra.Command {
	return adminReport(a, "jobs", "Background work: queues, the schedule, and what failed.",
		`Background work: how deep each queue is, every scheduled job with its last and
next run and whether it is paused, and the latest failures by job, error and
the error's first line, never a job's arguments. Read-only; tt admin job, cron
and rebuild change it.`,
		"/api/v1/admin/jobs", "  tt admin jobs\n  tt admin jobs --jq '.failures[] | {id, job, error_class}'",
		func(p *output.Printer, raw json.RawMessage) {
			d := decode[struct {
				Queues []struct {
					Name      string `json:"name"`
					Queued    int    `json:"queued"`
					Running   int    `json:"running"`
					Scheduled int    `json:"scheduled"`
					Failed    int    `json:"failed"`
				} `json:"queues"`
				Cron []struct {
					Key     string `json:"key"`
					Job     string `json:"job"`
					Every   string `json:"schedule"`
					Enabled bool   `json:"enabled"`
					Last    string `json:"last_run_at"`
					Error   string `json:"last_error"`
					Next    string `json:"next_at"`
				} `json:"cron"`
				Failures []struct {
					ID         string `json:"id"`
					Job        string `json:"job"`
					ErrorClass string `json:"error_class"`
					Error      string `json:"error"`
					At         string `json:"finished_at"`
				} `json:"failures"`
			}](raw)
			rows := [][]string{}
			for _, q := range d.Queues {
				rows = append(rows, []string{q.Name, figure(q.Queued), figure(q.Running), figure(q.Scheduled), figure(q.Failed)})
			}
			p.Table([]output.Column{{Header: "QUEUE"}, {Header: "QUEUED", Right: true}, {Header: "RUNNING", Right: true},
				{Header: "LATER", Right: true}, {Header: "FAILED", Right: true}}, rows)
			fmt.Fprintln(p.Out)
			now := time.Now()
			rows = rows[:0]
			for _, c := range d.Cron {
				last := "—"
				if c.Last != "" {
					last = ago(c.Last, now)
				}
				if c.Error != "" {
					last += " · failed"
				}
				next := until(c.Next, now)
				if !c.Enabled {
					next = "paused"
				}
				rows = append(rows, []string{c.Key, c.Every, last, next})
			}
			p.Table([]output.Column{{Header: "SCHEDULED", Flex: true}, {Header: "CRON", Dim: true}, {Header: "LAST RUN"}, {Header: "NEXT"}}, rows)
			if len(d.Failures) > 0 {
				fmt.Fprintln(p.Out)
				rows = rows[:0]
				for _, f := range d.Failures {
					rows = append(rows, []string{ago(f.At, now), f.Job, f.ID})
				}
				// The error goes on its own line under each failure: beside a
				// job's id it had a dozen columns left.
				cols := p.Grid([]output.Column{{Header: "FAILED", Dim: true}, {Header: "JOB"}, {Header: "ID", ID: true}}, rows)
				p.Table(cols, nil)
				for i, f := range d.Failures {
					p.Rows(cols, rows[i:i+1])
					why := join(": ", strings.TrimPrefix(f.ErrorClass, f.Job+"::"), f.Error)
					p.Line("  %s", p.Style.Dim(output.Truncate(why, p.Columns()-2)))
				}
			}
		})
}

// until is "in 12m" for a time ahead.
func until(iso string, now time.Time) string {
	t, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		return iso
	}
	d := t.Sub(now)
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("in %dm", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("in %dh", int(d.Hours()))
	default:
		return fmt.Sprintf("in %dd", int(d.Hours()/24))
	}
}

func newAdminDescribe(a *App) *cobra.Command {
	return &cobra.Command{
		Use:   "describe [TYPE]",
		Short: "A kind of record: its table, columns, allowed values, links and verbs.",
		Long: `A kind of record, read from the schema: its table and columns (for writing a
tt admin query), the values each enum allows, what it links to, and the
tt admin verbs that change it. Without TYPE, the kinds there are.`,
		Example: "  tt admin describe\n  tt admin describe dancer\n  tt admin describe video --jq '.columns[] | select(.values) | {name, values}'",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/api/v1/admin/describe"
			if len(args) == 1 {
				path += "/" + url.PathEscape(strings.ToLower(args[0]))
			}
			return a.adminDo(cmd, http.MethodGet, path, nil, nil, func(p *output.Printer, raw json.RawMessage) {
				d := decode[struct {
					Types   []string `json:"types"`
					Table   string   `json:"table"`
					ID      string   `json:"id"`
					Columns []struct {
						Name   string   `json:"name"`
						Type   string   `json:"type"`
						Null   bool     `json:"null"`
						Values []string `json:"values"`
					} `json:"columns"`
					Links []struct {
						Name  string `json:"name"`
						Kind  string `json:"kind"`
						Table string `json:"table"`
						Key   string `json:"foreign_key"`
					} `json:"links"`
					Verbs []struct {
						Verb     string   `json:"verb"`
						Changes  []string `json:"changes"`
						Undoable bool     `json:"undoable"`
					} `json:"verbs"`
				}](raw)
				if d.Table == "" {
					p.Words("types", d.Types)
					return
				}
				p.Field("table", d.Table)
				p.Field("id", d.ID)
				fmt.Fprintln(p.Out)
				rows := [][]string{}
				for _, c := range d.Columns {
					typ := c.Type
					if !c.Null {
						typ += " not null"
					}
					rows = append(rows, []string{c.Name, typ, strings.Join(c.Values, " ")})
				}
				p.Table([]output.Column{{Header: "COLUMN", ID: true}, {Header: "TYPE", Dim: true}, {Header: "VALUES", Flex: true}}, rows)
				if len(d.Links) > 0 {
					fmt.Fprintln(p.Out)
					rows = rows[:0]
					for _, l := range d.Links {
						rows = append(rows, []string{l.Name, strings.ReplaceAll(l.Kind, "_", " "), l.Table, l.Key})
					}
					p.Table([]output.Column{{Header: "LINK"}, {Header: "", Dim: true}, {Header: "TABLE"}, {Header: "KEY", Dim: true}}, rows)
				}
				fmt.Fprintln(p.Out)
				if len(d.Verbs) == 0 {
					p.Field("verbs", "none yet: read with tt admin query")
					return
				}
				verbs := []string{}
				for _, v := range d.Verbs {
					verbs = append(verbs, v.Verb)
				}
				p.Items("verbs", verbs)
			})
		},
	}
}

func newAdminQuery(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   `query "SQL"`,
		Short: "Ask the catalogue a question in SQL. Read-only.",
		Long: `Ask the catalogue a question in SQL: one SELECT, read-only, stopped after 5
seconds, at most 200 rows (--limit up to 2,000). Catalogue tables only:
videos, dancers, couples, songs, orchestras, events, channels, performances
and the pipelines' own tables; never accounts, tokens, sessions, likes,
watches or playlists. tt admin describe TYPE lists a table's columns.

Pass - to read the SQL from stdin.`,
		Example: `  tt admin query "SELECT dance_form, count(*) FROM videos GROUP BY 1 ORDER BY 2 DESC"
  tt admin query "SELECT name FROM dancers WHERE bio IS NULL" --limit 50
  tt admin query - < question.sql`,
		Annotations: map[string]string{listsThings: "yes", limitUpTo: "2000 (200 by default)"},
		Args:        exactArgs(1, `tt admin query "SELECT count(*) FROM videos"`),
		RunE: func(cmd *cobra.Command, args []string) error {
			sql := args[0]
			if sql == "-" {
				raw, err := io.ReadAll(a.In)
				if err != nil {
					return a.Fail(Usage("Could not read the SQL from stdin", `tt admin query "SELECT 1"`))
				}
				sql = string(raw)
			}
			body := map[string]any{"sql": sql}
			if a.Flags.Limit > 0 {
				body["limit"] = a.Flags.Limit
			}
			return a.adminDo(cmd, http.MethodPost, "/api/v1/admin/query", nil, body, func(p *output.Printer, raw json.RawMessage) {
				d := decode[struct {
					Columns []string `json:"columns"`
					Rows    [][]any  `json:"rows"`
				}](raw)
				cols := make([]output.Column, len(d.Columns))
				for i, c := range d.Columns {
					cols[i] = output.Column{Header: strings.ToUpper(c), Flex: true}
				}
				rows := make([][]string, 0, len(d.Rows))
				for _, r := range d.Rows {
					row := make([]string, len(r))
					for i, v := range r {
						switch t := v.(type) {
						case float64:
							if t == float64(int64(t)) {
								row[i] = strconv.FormatInt(int64(t), 10)
								cols[i].Right = true
								cols[i].Flex = false
							} else {
								row[i] = strconv.FormatFloat(t, 'f', -1, 64)
							}
						default:
							row[i] = strings.ReplaceAll(plainValue(v), "\n", " ")
						}
					}
					rows = append(rows, row)
				}
				p.Table(cols, rows)
			})
		},
	}
	return cmd
}

// figure is thousands for a report, where 0 is an answer and says so.
func figure(n int) string {
	if n == 0 {
		return "0"
	}
	return thousands(n)
}
