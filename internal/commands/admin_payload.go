package commands

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/justinallenmarsh/tangotube-cli/internal/output"
)

// The payload ledger: the jumpstart loop (export a work list, decide it,
// import the decisions) over the API, and the Payloads pile's reverts. A
// payload is its own undo, so tt admin undo does not apply to these verbs;
// tt admin payload revert does.

var payloadKinds = []string{"gender", "family", "song", "credit", "probe", "withdrawal"}

// maxPayloadRows and maxPayloadBytes are what one import takes; the API
// refuses more rows, and a file past the byte cap is not a session's work.
const (
	maxPayloadRows  = 5000
	maxPayloadBytes = 20 << 20
	payloadShown    = 20
)

func newAdminPayload(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "payload",
		Short: "The payload ledger: export a work list, import decisions, revert.",
		Long: `The jumpstart loop over the API. Export a queue's work list, decide each row
(its "decide" says the shape of the answer), then import the decisions: the
same idempotent importer as jumpstart:import, so importing the same rows twice
applies nothing new. Every row keeps the state it overwrote, so a wrong row or
a whole bad session is reverted.

Queues: gender, family, song, credit, probe, withdrawal. probe, credit and
withdrawal rows name appearances, which only mean the same thing in the
database they were exported from: export and import against the same one.`,
		Example: `  tt admin payload export gender --limit 500 -o gender-2026-09-24.jsonl
  tt admin payload import gender-2026-09-24-decided.jsonl --agent claude --dry-run
  tt admin payload status
  tt admin payload revert 12 --yes`,
		Args: cobra.NoArgs,
	}
	cmd.AddCommand(newPayloadExport(a), newPayloadImport(a), newPayloadStatus(a), newPayloadShow(a), newPayloadRevert(a))
	return cmd
}

func kindArg(example string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if err := exactArgs(1, example)(cmd, args); err != nil {
			return err
		}
		if !contains(payloadKinds, args[0]) {
			return Usage("the queue is one of "+strings.Join(payloadKinds, ", "), example)
		}
		return nil
	}
}

func newPayloadExport(a *App) *cobra.Command {
	var out string
	cmd := &cobra.Command{
		Use:   "export KIND",
		Short: "A queue's work list, each row with what to decide. -o saves it as JSON lines.",
		Long: `A queue's work list, as jumpstart:export writes it: each row carries ref (what
the decision applies to), the context to decide from, and decide, the shape of
the answer. --limit takes up to 3000 (500 by default). -o writes the rows to a
file, one JSON object a line, ready to decide and import.`,
		Example: `  tt admin payload export gender --limit 500 -o gender-2026-09-24.jsonl
  tt admin payload export family --limit 5`,
		Annotations: map[string]string{listsThings: "yes", limitUpTo: "3000 (500 by default)"},
		Args:        kindArg("tt admin payload export gender"),
		RunE: func(cmd *cobra.Command, args []string) error {
			q := url.Values{"kind": {args[0]}}
			if a.Flags.Limit > 0 {
				q.Set("limit", strconv.Itoa(a.Flags.Limit))
			}
			env, err := a.Client().Do(ctx(cmd), http.MethodGet, "/api/v1/admin/payloads/export", q, nil, true)
			if err == nil && out != "" {
				if werr := writeWorklist(out, env.Data); werr != nil {
					return a.Fail(werr)
				}
			}
			return a.Show(env, err, func(p *output.Printer, raw json.RawMessage) {
				d := decode[struct {
					Queue  int                          `json:"queue"`
					Decide map[string]string            `json:"decide"`
					Rows   []map[string]json.RawMessage `json:"rows"`
				}](raw)
				p.Field("queue", fmt.Sprintf("%s waiting · %d here", orDefault(thousands(d.Queue), "0"), len(d.Rows)))
				showDecide(p, d.Decide)
				if out != "" {
					p.Field("wrote", fmt.Sprintf("%d rows to %s", len(d.Rows), out))
					p.Line("next: decide each row, then tt admin payload import FILE --agent claude --dry-run")
					return
				}
				rows := make([][]string, 0, min(len(d.Rows), payloadShown))
				for _, row := range d.Rows[:min(len(d.Rows), payloadShown)] {
					rows = append(rows, []string{rawText(row["ref"]), aboutRow(row)})
				}
				p.Table([]output.Column{{Header: "REF", ID: true}, {Header: "ABOUT", Flex: true}}, rows)
				if len(d.Rows) > payloadShown {
					p.Line("and %d more: -o FILE saves them all", len(d.Rows)-payloadShown)
				}
			})
		},
	}
	cmd.Flags().StringVarP(&out, "out", "o", "", "Write the rows to `FILE`, one JSON object a line.")
	return cmd
}

// writeWorklist writes an export's rows as JSON lines, each exactly as the
// API sent it.
func writeWorklist(path string, data json.RawMessage) error {
	d := decode[struct {
		Rows []json.RawMessage `json:"rows"`
	}](data)
	var b bytes.Buffer
	for _, row := range d.Rows {
		if err := json.Compact(&b, row); err != nil {
			return Usage("the export answered a row that is not JSON", "")
		}
		b.WriteByte('\n')
	}
	if err := os.WriteFile(path, b.Bytes(), 0o600); err != nil {
		return Usage("could not write "+path+": "+err.Error(), "")
	}
	return nil
}

// showDecide writes the answer's shape, a key to a line.
func showDecide(p *output.Printer, decide map[string]string) {
	keys := make([]string, 0, len(decide))
	for k := range decide {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for i, k := range keys {
		label := ""
		if i == 0 {
			label = "decide"
		}
		p.Field(label, k+" = "+decide[k])
	}
}

// aboutRow is what a work-list row is about, in the words its kind uses.
func aboutRow(row map[string]json.RawMessage) string {
	for _, k := range []string{"name", "raw_name", "dancer", "title"} {
		if v := rawText(row[k]); v != "" && v != "null" {
			if t := rawText(row["title"]); k != "title" && t != "" && t != "null" {
				return v + " · " + t
			}
			return v
		}
	}
	return ""
}

func rawText(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	return strings.TrimSpace(string(raw))
}

func newPayloadImport(a *App) *cobra.Command {
	var agent, kind string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "import FILE",
		Short: "Land decided rows through the jumpstart importer. --dry-run says what each would do.",
		Long: `Land a file of decided rows (JSON lines, or one JSON array) through the same
idempotent importer as jumpstart:import. The queue comes from the file name
(gender-2026-09-24-decided.jsonl is gender) unless --kind says it. Every row is
checked first: a malformed one refuses the whole file, each bad row named by
its line. --dry-run lands the rows and rolls them back, and says what each
would do. A payload is taken back with tt admin payload revert, not undo.`,
		Example: `  tt admin payload import gender-2026-09-24-decided.jsonl --agent claude --dry-run
  tt admin payload import gender-2026-09-24-decided.jsonl --agent claude`,
		Args: exactArgs(1, "tt admin payload import gender-2026-09-24-decided.jsonl --agent claude"),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := args[0]
			if kind == "" {
				kind = kindFromName(filepath.Base(path))
			}
			if !contains(payloadKinds, kind) {
				return Usage("say which queue with --kind, or name the file after it (gender-…jsonl)", "--kind "+strings.Join(payloadKinds, "|"))
			}
			if agent == "" {
				return Usage("say who decided the rows with --agent", "--agent claude")
			}
			rows, err := readDecisions(path)
			if err != nil {
				return a.Fail(err)
			}
			body := map[string]any{"kind": kind, "agent": agent, "rows": rows, "source": filepath.Base(path)}
			if dryRun {
				body["dry_run"] = true
			}
			return a.adminDo(cmd, http.MethodPost, "/api/v1/admin/payloads", nil, body, showPayloadImport)
		},
	}
	f := cmd.Flags()
	f.StringVar(&agent, "agent", "", "Who decided the rows, an `AGENT`: claude, grok or human.")
	f.StringVar(&kind, "kind", "", "The `QUEUE`, when the file name does not start with it.")
	f.BoolVar(&dryRun, "dry-run", false, "Say what each row would do, and write nothing.")
	return cmd
}

func kindFromName(name string) string {
	for _, k := range payloadKinds {
		if strings.HasPrefix(name, k) {
			return k
		}
	}
	return ""
}

// readDecisions reads a decided file: JSON lines, or one JSON array. A line
// that is not JSON is named by its number, as the API names a bad row.
func readDecisions(path string) ([]json.RawMessage, error) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return nil, Usage("no file at "+path, "tt admin payload export gender -o gender.jsonl")
	}
	if info.Size() > maxPayloadBytes {
		return nil, Usage(fmt.Sprintf("%s is %s; an import is at most 20 MB, so split it", path, humanSize(int(info.Size()))), "")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, Usage("could not read "+path+": "+err.Error(), "")
	}
	var rows []json.RawMessage
	if trimmed := bytes.TrimSpace(raw); len(trimmed) > 0 && trimmed[0] == '[' {
		if err := json.Unmarshal(trimmed, &rows); err != nil {
			return nil, Usage(path+" is not a JSON array: "+err.Error(), "one JSON object a line also works")
		}
	} else {
		scanner := bufio.NewScanner(bytes.NewReader(raw))
		scanner.Buffer(make([]byte, 0, 64<<10), maxPayloadBytes)
		var bad []string
		for line := 1; scanner.Scan(); line++ {
			text := bytes.TrimSpace(scanner.Bytes())
			if len(text) == 0 {
				continue
			}
			if !json.Valid(text) {
				bad = append(bad, fmt.Sprintf("line %d", line))
				continue
			}
			rows = append(rows, json.RawMessage(append([]byte(nil), text...)))
		}
		if len(bad) > 0 {
			return nil, Usage(fmt.Sprintf("%s is not JSON on %s; nothing was imported", path, strings.Join(bad[:min(len(bad), 10)], ", ")), "one JSON object a line")
		}
	}
	if len(rows) == 0 {
		return nil, Usage(path+" has no rows", "")
	}
	if len(rows) > maxPayloadRows {
		return nil, Usage(fmt.Sprintf("%s has %d rows; one import takes %d, so split it", path, len(rows), maxPayloadRows), "")
	}
	return rows, nil
}

type payloadSummary struct {
	ID         int      `json:"id"`
	Kind       string   `json:"kind"`
	Agent      string   `json:"agent"`
	Source     string   `json:"source"`
	ImportedBy string   `json:"imported_by"`
	CreatedAt  string   `json:"created_at"`
	RevertedAt string   `json:"reverted_at"`
	Rows       int      `json:"rows"`
	Applied    int      `json:"applied"`
	Skipped    int      `json:"skipped"`
	Reverted   int      `json:"reverted"`
	Approval   *float64 `json:"approval"`
}

// showPayloadImport renders an import or its dry run: the tally, a line a
// row for the first rows, and how to take it back.
func showPayloadImport(p *output.Printer, raw json.RawMessage) {
	d := decode[struct {
		DryRun  bool                       `json:"dry_run"`
		Earlier *int                       `json:"earlier"`
		Payload *payloadSummary            `json:"payload"`
		Tally   map[string]json.RawMessage `json:"tally"`
		Rows    []struct {
			Ref    string         `json:"ref"`
			Result string         `json:"result"`
			Reason string         `json:"reason"`
			Before map[string]any `json:"before"`
		} `json:"rows"`
		Action *adminAction `json:"action"`
	}](raw)
	if d.Payload != nil {
		p.Field("payload", fmt.Sprintf("%d · %s · %s", d.Payload.ID, d.Payload.Kind, d.Payload.Agent))
	} else if d.Earlier != nil {
		p.Field("payload", fmt.Sprintf("%d already holds these rows", *d.Earlier))
	}
	p.Field(orDefault(map[bool]string{true: "would"}[d.DryRun], "rows"), tallyLine(d.Tally, d.DryRun))
	rows := make([][]string, 0, payloadShown)
	for _, r := range d.Rows[:min(len(d.Rows), payloadShown)] {
		what := r.Reason
		if r.Result == "applied" {
			what = "overwrites " + compactState(r.Before)
		}
		result := strings.ReplaceAll(r.Result, "_", " ")
		if would, ok := map[string]string{"applied": "would apply", "skipped": "would skip"}[r.Result]; ok && d.DryRun {
			result = would
		}
		rows = append(rows, []string{r.Ref, result, what})
	}
	p.Table([]output.Column{{Header: "REF", ID: true}, {Header: "RESULT"}, {Header: "", Dim: true, Flex: true}}, rows)
	if len(d.Rows) > payloadShown {
		p.Line("and %d more rows: --json lists each", len(d.Rows)-payloadShown)
	}
	switch {
	case d.DryRun:
		p.Field("undo", "not with tt admin undo; tt admin payload revert takes it back")
	case d.Action != nil:
		p.Field("recorded", fmt.Sprintf("action %d", d.Action.ID))
		p.Field("revert", fmt.Sprintf("tt admin payload revert %d --yes", d.Payload.ID))
	}
}

// tallyLine is an import's outcome on one line: 4 applied · 1 skipped
// (human 1), or in a dry run's words, apply 4 · skip 1 (human 1).
func tallyLine(tally map[string]json.RawMessage, dryRun bool) string {
	var parts []string
	for _, k := range []string{"applied", "skipped", "applied_earlier", "reverted_earlier"} {
		v, ok := tally[k]
		if !ok {
			continue
		}
		count := string(v)
		if k == "skipped" {
			var reasons map[string]int
			_ = json.Unmarshal(v, &reasons)
			count = reasonList(reasons)
		}
		switch {
		case dryRun && k == "applied":
			parts = append(parts, "apply "+count)
		case dryRun && k == "skipped":
			parts = append(parts, "skip "+count)
		default:
			n, rest, _ := strings.Cut(count, " ")
			parts = append(parts, strings.TrimSpace(n+" "+strings.ReplaceAll(k, "_", " ")+" "+rest))
		}
	}
	if len(parts) == 0 {
		return "nothing"
	}
	return strings.Join(parts, " · ")
}

func reasonList(reasons map[string]int) string {
	keys := make([]string, 0, len(reasons))
	total := 0
	for k, n := range reasons {
		keys = append(keys, k)
		total += n
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s %d", k, reasons[k]))
	}
	return fmt.Sprintf("%d (%s)", total, strings.Join(parts, ", "))
}

// decisionLine is a decided row with its answer first and the evidence
// last: gender female, confidence 0.95, evidence given name Constanza.
func decisionLine(decision map[string]any) string {
	answers := []string{"gender", "dance_form", "decision", "verdict", "dancer_id"}
	var parts []string
	for _, k := range answers {
		if v, ok := decision[k]; ok {
			parts = append(parts, k+" "+plainValue(v))
		}
	}
	for _, k := range sortedKeys(decision) {
		if !contains(answers, k) {
			parts = append(parts, k+" "+plainValue(decision[k]))
		}
	}
	return strings.Join(parts, ", ")
}

// compactState is a before-state on one line: gender none, gender_source none.
func compactState(state map[string]any) string {
	keys := sortedKeys(state)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+" "+plainValue(state[k]))
	}
	if len(parts) == 0 {
		return "nothing"
	}
	return strings.Join(parts, ", ")
}

func newPayloadStatus(a *App) *cobra.Command {
	var kind, cursor string
	var counts bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: "The payloads landed, newest first; --counts adds how many rows wait in each queue.",
		Long: `The payloads landed, newest first, with rows applied, skipped and reverted.
--counts also counts the rows waiting in each queue; that walks every
exporter's scope and takes a few seconds, so it is only done when asked.`,
		Example:     "  tt admin payload status\n  tt admin payload status --counts --kind gender",
		Annotations: map[string]string{listsThings: "yes"},
		Args:        cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			q := a.listQuery(cursor, "kind", kind)
			if counts {
				q.Set("counts", "true")
			}
			return a.adminDo(cmd, http.MethodGet, "/api/v1/admin/payloads", q, nil, func(p *output.Printer, raw json.RawMessage) {
				d := decode[struct {
					Queues   map[string]int   `json:"queues"`
					Counted  bool             `json:"counted"`
					Payloads []payloadSummary `json:"payloads"`
				}](raw)
				if d.Counted || d.Queues != nil {
					waiting := [][]string{}
					for _, k := range payloadKinds {
						if n, ok := d.Queues[k]; ok {
							waiting = append(waiting, []string{k, orDefault(thousands(n), "0")})
						}
					}
					p.Table([]output.Column{{Header: "QUEUE"}, {Header: "WAITING", Right: true}}, waiting)
				} else {
					p.Line("%s", p.Style.Dim("queues not counted; --counts counts them (a few seconds)"))
				}
				p.Line("")
				now := time.Now()
				rows := make([][]string, 0, len(d.Payloads))
				for _, pl := range d.Payloads {
					approval := ""
					if pl.Approval != nil {
						approval = fmt.Sprintf("%.0f%%", *pl.Approval*100)
					}
					rows = append(rows, []string{strconv.Itoa(pl.ID), pl.Kind, pl.Agent, ago(pl.CreatedAt, now), pl.ImportedBy,
						strconv.Itoa(pl.Applied), strconv.Itoa(pl.Skipped), strconv.Itoa(pl.Reverted), approval})
				}
				p.Table([]output.Column{
					{Header: "PAYLOAD", ID: true, Right: true}, {Header: "KIND"}, {Header: "AGENT"}, {Header: "WHEN", Dim: true},
					{Header: "BY", Dim: true, Flex: true}, {Header: "APPLIED", Right: true}, {Header: "SKIPPED", Right: true},
					{Header: "REVERTED", Right: true}, {Header: "APPROVAL", Right: true},
				}, rows)
				showMore(p, raw, "tt admin payload status")
			})
		},
	}
	cmd.Flags().StringVar(&kind, "kind", "", "Only this `QUEUE`: "+strings.Join(payloadKinds, ", ")+".")
	cmd.Flags().BoolVar(&counts, "counts", false, "Also count the rows waiting in each queue (a few seconds).")
	cursorFlag(cmd, &cursor)
	return cmd
}

func newPayloadShow(a *App) *cobra.Command {
	var state, cursor string
	cmd := &cobra.Command{
		Use:         "show ID",
		Short:       "One payload: its counts, why rows skipped, and its rows.",
		Example:     "  tt admin payload show 12\n  tt admin payload show 12 --state skipped",
		Annotations: map[string]string{listsThings: "yes"},
		Args:        idArg("a payload's id", "tt admin payload show 12"),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/api/v1/admin/payloads/" + args[0]
			return a.adminDo(cmd, http.MethodGet, path, a.listQuery(cursor, "state", state), nil, func(p *output.Printer, raw json.RawMessage) {
				d := decode[struct {
					Payload payloadSummary `json:"payload"`
					Skipped map[string]int `json:"skipped"`
					Rows    []struct {
						ID       int            `json:"id"`
						Ref      string         `json:"ref"`
						State    string         `json:"state"`
						Skipped  string         `json:"skipped"`
						Decision map[string]any `json:"decision"`
					} `json:"rows"`
				}](raw)
				pl := d.Payload
				p.Field("payload", fmt.Sprintf("%d · %s · %s", pl.ID, pl.Kind, pl.Agent))
				p.Field("from", pl.Source)
				p.Field("landed", join(" · ", ago(pl.CreatedAt, time.Now()), pl.ImportedBy))
				p.Field("rows", fmt.Sprintf("%d · %d applied · %d skipped · %d reverted", pl.Rows, pl.Applied, pl.Skipped, pl.Reverted))
				if len(d.Skipped) > 0 {
					p.Field("skipped", reasonList(d.Skipped))
				}
				if pl.RevertedAt != "" {
					p.Field("reverted", ago(pl.RevertedAt, time.Now()))
				}
				p.Line("")
				rows := make([][]string, 0, len(d.Rows))
				for _, r := range d.Rows {
					st := r.State
					if r.Skipped != "" {
						st = "skipped: " + r.Skipped
					}
					rows = append(rows, []string{strconv.Itoa(r.ID), r.Ref, st, decisionLine(r.Decision)})
				}
				p.Table([]output.Column{{Header: "ROW", ID: true, Right: true}, {Header: "REF"}, {Header: "STATE"}, {Header: "DECISION", Flex: true}}, rows)
				showMore(p, raw, "tt admin payload show "+args[0])
			})
		},
	}
	cmd.Flags().StringVar(&state, "state", "", "Only rows that are `STATE`: applied, skipped or reverted.")
	cursorFlag(cmd, &cursor)
	return cmd
}

func newPayloadRevert(a *App) *cobra.Command {
	var w writeFlags
	var row string
	cmd := &cobra.Command{
		Use:   "revert ID",
		Short: "Put back what a payload, or one row of it, overwrote. Needs --yes.",
		Long: `Put back what a payload's applied rows overwrote, as the Payloads pile's revert
does, or only one row with --row (its id from tt admin payload show, or its
ref). Each row is put back only where nothing has changed it since. A reverted
row stays reverted: importing the file again never re-applies it.`,
		Example: `  tt admin payload revert 12 --dry-run
  tt admin payload revert 12 --row dancer:4821 --yes`,
		Args: idArg("a payload's id", "tt admin payload revert 12 --yes"),
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{}
			setBody(body, "row_id", row)
			return a.adminWrite(cmd, http.MethodPost, "/api/v1/admin/payloads/"+args[0]+"/revert", &w, body)
		},
	}
	cmd.Flags().StringVar(&row, "row", "", "Only this `ROW`: its id or its ref (dancer:4821).")
	w.bind(cmd, true)
	return cmd
}
