package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/justinallenmarsh/tangotube-cli/internal/output"
)

// A song edit that moves the recording date and the Spotify id: one field to
// a line, the fields lined up, never one " · "-joined row off the edge.
func TestShowChangeWritesOneFieldPerLine(t *testing.T) {
	var out bytes.Buffer
	p := &output.Printer{Out: &out, Width: 100}
	raw := json.RawMessage(`{"changed":true,"action":{"id":812,"undoable":true,
		"before":{"date":"1940-10-08","spotify_track_id":"3n3Ppam7vgaVa1iaRUc9Lp","title":"Volver a soñar"},
		"after":{"date":"1941-06-24","spotify_track_id":"6rqhFgbbKwnb9MLmUQDhG6","title":"Volver a soñar"}}}`)
	showChange(p, raw)

	want := "" +
		"  changed    date              1940-10-08 → 1941-06-24\n" +
		"             spotify_track_id  3n3Ppam7vgaVa1iaRUc9Lp → 6rqhFgbbKwnb9MLmUQDhG6\n" +
		"  recorded   action 812\n"
	if out.String() != want {
		t.Errorf("got\n%s\nwant\n%s", out.String(), want)
	}
}

func TestShowChangeDryRunUsesTheSameRows(t *testing.T) {
	var out bytes.Buffer
	p := &output.Printer{Out: &out, Width: 100}
	showChange(p, json.RawMessage(`{"dry_run":true,"undoable":false,"would":{"bio":[null,"Born in Buenos Aires."],"gender":["male","female"]}}`))

	want := "" +
		"  would      bio     none → Born in Buenos Aires.\n" +
		"             gender  male → female\n" +
		"  undo       not undoable\n"
	if out.String() != want {
		t.Errorf("got\n%s\nwant\n%s", out.String(), want)
	}
}

func TestChangeRowsFitTheRoomAndSkipWhatDidNotMove(t *testing.T) {
	long := strings.Repeat("letra de tango ", 20)
	rows := changeRows(map[string]any{"lyrics": "", "primary": true},
		map[string]any{"lyrics": long, "primary": true}, 87)
	if len(rows) != 1 {
		t.Fatalf("want only the field that moved, got %q", rows)
	}
	if w := output.DisplayWidth(rows[0]); w > 87 {
		t.Errorf("row is %d wide, room is 87: %q", w, rows[0])
	}
	if !strings.HasPrefix(rows[0], `lyrics  "" → letra de tango`) || !strings.HasSuffix(rows[0], "…") {
		t.Errorf("row = %q", rows[0])
	}
}

func TestChangeCellNamesTheFieldsWhenSeveralMoved(t *testing.T) {
	before := map[string]any{"date": "1940-10-08", "spotify_track_id": "a"}
	if got := changeCell(before, map[string]any{"date": "1941-06-24", "spotify_track_id": "b"}); got != "date, spotify_track_id" {
		t.Errorf("got %q", got)
	}
	if got := changeCell(before, map[string]any{"date": "1941-06-24", "spotify_track_id": "a"}); got != "date 1940-10-08 → 1941-06-24" {
		t.Errorf("got %q", got)
	}
}

// Keeping a stored song records two rows, each undone on its own; the
// answer names both and how to undo each.
func TestShowChangeNamesEveryRowADecisionRecorded(t *testing.T) {
	var out bytes.Buffer
	p := &output.Printer{Out: &out, Width: 100}
	showChange(p, json.RawMessage(`{"changed":true,"action":{"id":90,"action":"video.song.keep","undoable":true},
		"actions":[{"id":90,"action":"video.song.keep","undoable":true},{"id":91,"action":"enrichment.reject","undoable":true}]}`))

	want := "" +
		"  recorded   action 90 · video.song.keep · tt admin undo 90\n" +
		"             action 91 · enrichment.reject · tt admin undo 91\n"
	if out.String() != want {
		t.Errorf("got\n%s\nwant\n%s", out.String(), want)
	}
}

// Two tools of one name and an MCP client sees only one of them.
func TestAdminToolNamesAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, tool := range allAdminTools() {
		if seen[tool.Name] {
			t.Errorf("two admin tools are called %s", tool.Name)
		}
		seen[tool.Name] = true
	}
}

// A decided file with a line that is not JSON names the line and sends
// nothing; a JSON array works as well as JSON lines.
func TestReadDecisionsNamesBadLines(t *testing.T) {
	dir := t.TempDir()
	lines := dir + "/gender-decided.jsonl"
	_ = os.WriteFile(lines, []byte("{\"ref\":\"dancer:1\",\"gender\":null}\n\n{\"ref\":\"dancer:2\",\n"), 0o600)
	if _, err := readDecisions(lines); err == nil || !strings.Contains(err.Error(), "line 3") {
		t.Errorf("want line 3 named, got %v", err)
	}
	array := dir + "/family.json"
	_ = os.WriteFile(array, []byte(`[{"ref":"video:1","dance_form":"folklore"},{"ref":"video:2","dance_form":"other"}]`), 0o600)
	rows, err := readDecisions(array)
	if err != nil || len(rows) != 2 {
		t.Errorf("rows = %d, err = %v", len(rows), err)
	}
	if kindFromName("withdrawal-2026-09-24-decided.jsonl") != "withdrawal" || kindFromName("decided.jsonl") != "" {
		t.Error("the queue comes from the file name's start")
	}
}
