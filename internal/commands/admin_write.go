package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/justinallenmarsh/tangotube-cli/internal/output"
)

// writeFlags are the three every tt admin change takes: --dry-run to see it,
// --yes where it is destructive, --note to say why.
type writeFlags struct {
	dryRun, yes bool
	note        string
	confirm     bool // whether this verb takes --yes
}

func (w *writeFlags) bind(cmd *cobra.Command, confirm bool) {
	w.confirm = confirm
	f := cmd.Flags()
	f.BoolVar(&w.dryRun, "dry-run", false, "Show what would change, and change nothing.")
	f.StringVar(&w.note, "note", "", "Why, a `NOTE` kept with the record.")
	if confirm {
		f.BoolVar(&w.yes, "yes", false, "Go ahead: this one is destructive or cannot be undone.")
	}
}

func (w *writeFlags) body(body map[string]any) map[string]any {
	if body == nil {
		body = map[string]any{}
	}
	if w.dryRun {
		body["dry_run"] = true
	}
	if w.yes {
		body["confirm"] = true
	}
	if w.note != "" {
		body["note"] = w.note
	}
	return body
}

// adminWrite sends one change and shows what it did or would do.
func (a *App) adminWrite(cmd *cobra.Command, method, path string, w *writeFlags, body map[string]any) error {
	return a.adminDo(cmd, method, path, nil, w.body(body), showChange)
}

// showChange renders any admin write: the fields that would change or did,
// whether it can be undone, and the action that recorded it.
func showChange(p *output.Printer, raw json.RawMessage) {
	d := decode[struct {
		DryRun   bool                       `json:"dry_run"`
		Would    map[string]json.RawMessage `json:"would"`
		Undoable *bool                      `json:"undoable"`
		Moves    map[string]json.RawMessage `json:"moves"`
		Action   *adminAction               `json:"action"`
		Actions  []adminAction              `json:"actions"`
	}](raw)
	if len(d.Would) > 0 {
		from, to := map[string]any{}, map[string]any{}
		for k, raw := range d.Would {
			var pair []any
			if json.Unmarshal(raw, &pair) == nil && len(pair) == 2 {
				from[k], to[k] = pair[0], pair[1]
			} else {
				to[k] = string(raw)
			}
		}
		label := "changed"
		if d.DryRun {
			label = "would"
		}
		showChanges(p, label, from, to)
	}
	for _, k := range moveOrder {
		raw, ok := d.Moves[k]
		if !ok {
			continue
		}
		var move struct {
			Move int `json:"move"`
			Drop int `json:"drop"`
		}
		if string(raw) == "true" {
			p.Field(moveLabels[k], "the folded name becomes an alias")
			continue
		}
		if json.Unmarshal(raw, &move) != nil || move.Move+move.Drop == 0 {
			continue
		}
		line := fmt.Sprintf("%d move", move.Move)
		if move.Drop > 0 {
			line += fmt.Sprintf(", %d already held (dropped)", move.Drop)
		}
		p.Field(moveLabels[k], line)
	}
	if d.DryRun && d.Undoable != nil && !*d.Undoable {
		p.Field("undo", "not undoable")
	}
	if len(d.Actions) > 1 {
		// One decision that recorded several rows (a review's keep): each
		// is undone on its own.
		for i, act := range d.Actions {
			label := ""
			if i == 0 {
				label = "recorded"
			}
			undo := "not undoable"
			if act.Undoable {
				undo = fmt.Sprintf("tt admin undo %d", act.ID)
			}
			p.Field(label, fmt.Sprintf("action %d · %s · %s", act.ID, act.Action, undo))
		}
		return
	}
	if d.Action != nil {
		if len(d.Action.After) > 0 && len(d.Would) == 0 && len(d.Moves) == 0 {
			showChanges(p, "changed", d.Action.Before, d.Action.After)
		}
		p.Field("recorded", fmt.Sprintf("action %d", d.Action.ID))
		if !d.Action.Undoable {
			p.Field("undo", "not undoable")
		}
	}
}

// moveOrder and moveLabels say what a dancer merge moves, busiest first,
// in words that fit the field column.
var moveOrder = []string{"video_credits", "occasion_credits", "couples", "aliases", "name_alias",
	"championship_titles", "followers", "images", "suggestions", "partnerships"}

var moveLabels = map[string]string{
	"video_credits": "videos", "occasion_credits": "occasions", "couples": "couples", "aliases": "aliases",
	"name_alias": "name", "championship_titles": "titles", "followers": "followers", "images": "pictures",
	"suggestions": "suggestions", "partnerships": "partners",
}

// editFlag is one field an edit command can set: the flag, the API's name
// for it, and how to read it.
type editFlag struct {
	flag, key, usage string
	kind             string // "", "bool", "number", "time" (seconds or m:ss) or "list" (comma-separated)
}

// newAdminEdit is `tt admin NOUN edit ID --field ...`: only the flags given
// are sent, and "none" clears a field.
func newAdminEdit(a *App, noun, plural, idName, example string, fields []editFlag) *cobra.Command {
	var w writeFlags
	values := map[string]*string{}
	cmd := &cobra.Command{
		Use:     "edit " + idName,
		Short:   fmt.Sprintf("Change a %s's facts. Recorded; tt admin undo puts it back.", noun),
		Long:    fmt.Sprintf("Change a %s's facts. Only the flags you give are sent; \"none\" clears a field.\nRecorded with the before and after, and tt admin undo puts it back unless\nthe field has changed again since.", noun),
		Example: example,
		Args:    exactArgs(1, firstLine(example)),
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{}
			for _, f := range fields {
				if !cmd.Flags().Changed(f.flag) {
					continue
				}
				raw := *values[f.flag]
				switch f.kind {
				case "bool":
					v, err := strconv.ParseBool(raw)
					if err != nil {
						return Usage(fmt.Sprintf("--%s is true or false", f.flag), "")
					}
					body[f.key] = v
				case "number":
					if raw == "none" {
						body[f.key] = nil
						continue
					}
					v, err := strconv.ParseFloat(raw, 64)
					if err != nil {
						return Usage(fmt.Sprintf("--%s is a number", f.flag), "")
					}
					body[f.key] = v
				case "time":
					v, err := ParseTime(raw)
					if err != nil {
						return Usage(fmt.Sprintf("--%s is seconds or m:ss", f.flag), "")
					}
					body[f.key] = v
				case "list":
					body[f.key] = splitList(raw)
				default:
					body[f.key] = raw
				}
			}
			if len(body) == 0 {
				return Usage("say what to change", firstLine(example))
			}
			return a.adminWrite(cmd, http.MethodPatch, "/api/v1/admin/"+plural+"/"+url.PathEscape(args[0]), &w, body)
		},
	}
	for _, f := range fields {
		values[f.flag] = cmd.Flags().String(f.flag, "", f.usage)
	}
	w.bind(cmd, false)
	return cmd
}

func firstLine(example string) string {
	for i, r := range example {
		if r == '\n' {
			return trimIndent(example[:i])
		}
	}
	return trimIndent(example)
}

func trimIndent(s string) string {
	for len(s) > 0 && s[0] == ' ' {
		s = s[1:]
	}
	return s
}
