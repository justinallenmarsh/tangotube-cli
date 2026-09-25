package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/justinallenmarsh/tangotube-cli/internal/output"
)

// tt admin job: start an allowlisted job, retry a failed one, discard one
// still waiting.
func newAdminJob(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "job",
		Short: "Start an allowlisted job, retry a failed one, discard a waiting one.",
		Long: `Start, retry and discard background jobs. tt admin jobs shows the queues, the
schedule and what failed.

Only the jobs on the allowlist run (tt admin job run --list). Each states what
it costs before it starts; the expensive ones (every channel, a full reindex,
the audio sweep) ask for --yes. A job already waiting or running is not queued
twice, an expensive one waits an hour before it runs again, and one operator
starts at most ten jobs in ten minutes.`,
		Example: `  tt admin job run --list
  tt admin job run reindex --arg mode=recent
  tt admin job run reindex --arg mode=full --dry-run
  tt admin job retry 3f2c6a2e-0d7b-4a53-9a55-5c1e8f0f4b1a`,
		Args: cobra.NoArgs,
	}
	cmd.AddCommand(newAdminJobRun(a), newAdminJobRetry(a), newAdminJobDiscard(a))
	return cmd
}

func newAdminJobRun(a *App) *cobra.Command {
	var w writeFlags
	var pairs []string
	var list bool
	cmd := &cobra.Command{
		Use:   "run NAME",
		Short: "Start one allowlisted job now, its cost stated first.",
		Long: `Start one allowlisted job now, in the background: channel-sync, place-new-videos,
reindex, refresh-couples, harvest-descriptions, harvest-panels, audio-sweep,
fingerprint, sitemap or availability-check. --list says what each does and the
--arg each takes. Recorded; not undoable (tt admin job discard takes a job off
the queue before it starts).`,
		Example: `  tt admin job run --list
  tt admin job run sitemap
  tt admin job run channel-sync --arg channel=UCtdgMR0bmogczrZNpPaO66Q
  tt admin job run reindex --arg mode=full --dry-run`,
		Args: func(cmd *cobra.Command, args []string) error {
			if list {
				return cobra.NoArgs(cmd, args)
			}
			return exactArgs(1, "tt admin job run --list")(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if list {
				return a.adminDo(cmd, http.MethodGet, "/api/v1/admin/jobs/runs", nil, nil, showRuns)
			}
			given := map[string]any{}
			for _, pair := range pairs {
				k, v, ok := strings.Cut(pair, "=")
				if !ok || strings.TrimSpace(k) == "" {
					return Usage("--arg is NAME=VALUE, like mode=full", "tt admin job run reindex --arg mode=full")
				}
				given[strings.TrimSpace(k)] = strings.TrimSpace(v)
			}
			body := map[string]any{}
			if len(given) > 0 {
				body["args"] = given
			}
			return a.adminDo(cmd, http.MethodPost, "/api/v1/admin/jobs/run/"+url.PathEscape(args[0]), nil, w.body(body), showJobStart)
		},
	}
	cmd.Flags().StringArrayVar(&pairs, "arg", nil, "A job's `NAME=VALUE`, repeatable: mode=full, channel=ID, video=ID.")
	cmd.Flags().BoolVar(&list, "list", false, "The jobs tt runs, what each does, and the --arg each takes.")
	w.bind(cmd, true)
	return cmd
}

func newAdminJobRetry(a *App) *cobra.Command {
	var w writeFlags
	cmd := &cobra.Command{
		Use:   "retry ID",
		Short: "Run a failed job again. Recorded; not undoable.",
		Long: `Run a failed job again, as the GoodJob dashboard retries it: a new attempt on the
same job. ID is the job's id from tt admin jobs. Recorded; not undoable.`,
		Example: "  tt admin job retry 3f2c6a2e-0d7b-4a53-9a55-5c1e8f0f4b1a",
		Args:    exactArgs(1, "tt admin job retry ID (from tt admin jobs)"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.adminDo(cmd, http.MethodPost, "/api/v1/admin/jobs/"+url.PathEscape(args[0])+"/retry", nil, w.body(nil), showJobChange)
		},
	}
	w.bind(cmd, false)
	return cmd
}

func newAdminJobDiscard(a *App) *cobra.Command {
	var w writeFlags
	cmd := &cobra.Command{
		Use:   "discard ID",
		Short: "Take a waiting job off the queue so it never runs. Asks for --yes.",
		Long: `Take a job that is still waiting off the queue, so it never runs. A failed job is
already off it, and a running one is left to finish. Recorded; not undoable.
Asks for --yes.`,
		Example: "  tt admin job discard 3f2c6a2e-0d7b-4a53-9a55-5c1e8f0f4b1a --dry-run",
		Args:    exactArgs(1, "tt admin job discard ID"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.adminDo(cmd, http.MethodPost, "/api/v1/admin/jobs/"+url.PathEscape(args[0])+"/discard", nil, w.body(nil), showJobChange)
		},
	}
	w.bind(cmd, true)
	return cmd
}

// tt admin cron: switch a scheduled job off and on.
func newAdminCron(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cron",
		Short: "Pause or resume a scheduled job. Recorded; tt admin undo puts it back.",
		Long: `Pause or resume a scheduled job, by its key in tt admin jobs. The switch is
GoodJob's own, so the /good_job dashboard shows the same. A paused job keeps its
place in the schedule and does not run until resumed. Recorded; tt admin undo
puts it back.`,
		Example: "  tt admin cron pause generate_sitemap --dry-run\n  tt admin cron resume generate_sitemap",
		Args:    cobra.NoArgs,
	}
	for _, verb := range []struct{ name, short string }{
		{"pause", "Stop a scheduled job from running until resumed."},
		{"resume", "Let a paused scheduled job run again."},
	} {
		var w writeFlags
		name := verb.name
		c := &cobra.Command{
			Use:     name + " KEY",
			Short:   verb.short,
			Example: "  tt admin cron " + name + " generate_sitemap",
			Args:    exactArgs(1, "tt admin cron "+name+" generate_sitemap (keys: tt admin jobs)"),
			RunE: func(cmd *cobra.Command, args []string) error {
				return a.adminWrite(cmd, http.MethodPost, "/api/v1/admin/cron/"+url.PathEscape(args[0])+"/"+name, &w, nil)
			},
		}
		w.bind(c, false)
		cmd.AddCommand(c)
	}
	return cmd
}

// tt admin rebuild: one rebuild step, as a job.
func newAdminRebuild(a *App) *cobra.Command {
	var w writeFlags
	var list bool
	cmd := &cobra.Command{
		Use:   "rebuild STEP",
		Short: "Run one rebuild step in the background, its cost stated first. Asks for --yes.",
		Long: `Run one rebuild step in the background, as bin/rails rebuild:STEP does: occasions,
editions, appearances, views, dances, links, pairings, establish, partnerships,
refresh-music, refresh-families or refresh-roles. Each builds only what is not
built, so running one again is safe, and each walks a whole table, so each
states its cost and asks for --yes. --list shows the steps and their costs.
Recorded; not undoable.

A dancer merge rebuilds the kept dancer's partnerships itself; tt admin rebuild
partnerships is for the whole catalogue.`,
		Example: `  tt admin rebuild --list
  tt admin rebuild partnerships --dry-run
  tt admin rebuild partnerships --yes`,
		Args: func(cmd *cobra.Command, args []string) error {
			if list {
				return cobra.NoArgs(cmd, args)
			}
			return exactArgs(1, "tt admin rebuild --list")(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if list {
				return a.adminDo(cmd, http.MethodGet, "/api/v1/admin/jobs/runs", nil, nil, showRebuilds)
			}
			return a.adminDo(cmd, http.MethodPost, "/api/v1/admin/rebuilds/"+url.PathEscape(args[0]), nil, w.body(nil), showJobStart)
		},
	}
	cmd.Flags().BoolVar(&list, "list", false, "The rebuild steps and what each costs.")
	w.bind(cmd, true)
	return cmd
}

type jobRuns struct {
	Runs []struct {
		Name  string            `json:"name"`
		About string            `json:"about"`
		Args  map[string]string `json:"args"`
	} `json:"runs"`
	Rebuilds []struct {
		Step string `json:"step"`
		Runs string `json:"runs"`
		Cost string `json:"cost"`
	} `json:"rebuilds"`
}

func showRuns(p *output.Printer, raw json.RawMessage) {
	d := decode[jobRuns](raw)
	for i, r := range d.Runs {
		if i > 0 {
			p.Line("")
		}
		p.Line("%s", p.Style.Bold(r.Name))
		p.Para("does", r.About)
		keys := make([]string, 0, len(r.Args))
		for k := range r.Args {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for i, k := range keys {
			label := ""
			if i == 0 {
				label = "--arg"
			}
			p.Para(label, k+": "+r.Args[k])
		}
	}
}

func showRebuilds(p *output.Printer, raw json.RawMessage) {
	d := decode[jobRuns](raw)
	for i, r := range d.Rebuilds {
		if i > 0 {
			p.Line("")
		}
		p.Line("%s  %s", p.Style.Bold(r.Step), p.Style.Dim(r.Runs))
		p.Para("costs", r.Cost)
	}
}

// showJobStart renders a job run or a rebuild: what it is, where it runs,
// what it costs, and what was recorded.
func showJobStart(p *output.Printer, raw json.RawMessage) {
	d := decode[struct {
		Run    string       `json:"run"`
		Job    string       `json:"job"`
		Queue  string       `json:"queue"`
		Cost   string       `json:"cost"`
		DryRun bool         `json:"dry_run"`
		Action *adminAction `json:"action"`
	}](raw)
	p.Field("job", fmt.Sprintf("%s · %s on %s", d.Run, d.Job, d.Queue))
	p.Para("costs", d.Cost)
	if d.Action != nil {
		p.Field("recorded", fmt.Sprintf("action %d", d.Action.ID))
	}
	if d.DryRun || d.Action != nil {
		p.Field("undo", "not undoable")
	}
}

// showJobChange renders a retry or a discard.
func showJobChange(p *output.Printer, raw json.RawMessage) {
	d := decode[struct {
		Job struct {
			ID         string `json:"id"`
			Job        string `json:"job"`
			Queue      string `json:"queue"`
			ErrorClass string `json:"error_class"`
			Error      string `json:"error"`
		} `json:"job"`
	}](raw)
	p.Field("job", join(" · ", d.Job.Job, d.Job.Queue, d.Job.ID))
	if d.Job.Error != "" {
		p.Para("error", join(": ", strings.TrimPrefix(d.Job.ErrorClass, d.Job.Job+"::"), d.Job.Error))
	}
	showChange(p, raw)
}
