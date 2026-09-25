package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/justinallenmarsh/tangotube-cli/internal/output"
)

const identityRows = `[
  {"fact": "song", "state": "proposed", "value": "La mulateada · Carlos Di Sarli", "how": "proposed by the YouTube panel",
   "proposal": {"name": "La mulateada · Carlos Di Sarli", "slug": "la-mulateada-carlos-di-sarli"}},
  {"fact": "dancers", "state": "proposed", "value": "Roxana Suárez", "how": "proposed by a person",
   "proposal": {"name": "Roxana Suárez"}, "suggestion": {"id": 17}},
  {"fact": "event", "state": "missing"},
  {"fact": "kind", "state": "settled", "value": "Performance", "how": "a person"}
]`

func TestIdentityOffersTheYesEachWaitingAnswerNeeds(t *testing.T) {
	var rows []identityRow
	if err := json.Unmarshal([]byte(identityRows), &rows); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	renderIdentity(&output.Printer{Out: &out, Width: 120}, "wCJInTctvnw", rows)
	plain := output.StripEscapes(out.String())
	for _, want := range []string{
		"tt suggest wCJInTctvnw --song la-mulateada-carlos-di-sarli --agree",
		"tt agree wCJInTctvnw 17",
		"event      not known yet",
		"Performance  a person",
	} {
		if !strings.Contains(plain, want) {
			t.Errorf("missing %q in:\n%s", want, plain)
		}
	}
}

// A dancer applied to the video but not yet on the dance's credits is named,
// with when their dancer page catches up -- "applied" alone would overstate it.
func TestIdentityNamesDancersOnlyOnTheVideo(t *testing.T) {
	const rowsJSON = `[{"fact": "dancers", "state": "settled", "value": "Carlitos Espinoza", "how": "a person",
	  "only_on_video": [{"name": "Noelia Hurtado", "slug": "noelia-hurtado"}], "reach": "stuck"}]`
	var rows []identityRow
	if err := json.Unmarshal([]byte(rowsJSON), &rows); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	renderIdentity(&output.Printer{Out: &out, Width: 140}, "wCJInTctvnw", rows)
	plain := output.StripEscapes(out.String())
	if !strings.Contains(plain, "Noelia Hurtado  on the video; not on their dancer page yet") {
		t.Errorf("missing the waiting dancer in:\n%s", plain)
	}
	for reach, want := range map[string]string{"placing": "within 15 minutes", "tonight": "03:10 UTC", "": ""} {
		if got := reachNote(reach); !strings.Contains(got, want) || (want == "" && got != "") {
			t.Errorf("reachNote(%q) = %q", reach, got)
		}
	}
}

func TestReadChangesTakesAListOrAWrappedList(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"list.json":    `[{"fact": "song", "op": "add", "song": "x"}]`,
		"wrapped.json": `{"changes": [{"fact": "song", "op": "add", "song": "x"}]}`,
	} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		changes, err := readChanges(path, nil)
		if err != nil || len(changes) != 1 {
			t.Errorf("%s: %v %v", name, changes, err)
		}
	}
	if _, err := readChanges("-", strings.NewReader(`{"fact": "song"}`)); err == nil {
		t.Error("a lone change is not a batch")
	}
	if _, err := readChanges(filepath.Join(dir, "missing.json"), nil); err == nil {
		t.Error("a missing file is an error")
	}
}
