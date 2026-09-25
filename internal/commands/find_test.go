package commands

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/justinallenmarsh/tangotube-cli/internal/output"
)

// fixture is the data of a recorded envelope in e2e/fixtures.
func fixture(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile("../../e2e/fixtures/" + name)
	if err != nil {
		t.Fatal(err)
	}
	var env struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatal(err)
	}
	return string(env.Data)
}

func plain(t *testing.T, fn func(*output.Printer, json.RawMessage), name string) string {
	t.Helper()
	return output.StripEscapes(render(t, false, fn, fixture(t, name)))
}

func TestListsAreSlugFirstWithCountsOnTheRight(t *testing.T) {
	out := plain(t, renderDancers, "list_dancers.json")
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if !strings.HasPrefix(lines[0], "SLUG") || !strings.HasSuffix(lines[0], "VIDEOS") {
		t.Fatalf("header: %q", lines[0])
	}
	// The counts end in the same column.
	for _, line := range lines[1:] {
		if len([]rune(line)) != len([]rune(lines[0])) {
			t.Fatalf("counts do not line up on the right:\n%s", out)
		}
	}
	if !strings.Contains(out, "champion") {
		t.Fatalf("champions are marked:\n%s", out)
	}
}

func TestChampionsNameTheCouple(t *testing.T) {
	out := plain(t, renderChampions, "list_champions.json")
	if !strings.Contains(out, "2005  Tango de Salón") || !strings.Contains(out, "Sebastián Achaval") {
		t.Fatalf("got:\n%s", out)
	}
}

func TestFacetsCountAndDrawTheDecades(t *testing.T) {
	out := plain(t, renderFacets, "facets.json")
	for _, want := range []string{"  leader     ", "  recorded   ", "1940s  █"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "orchestra  ") {
		t.Error("the facet the search is filtered on is not counted")
	}
}

func TestBarsDrawInEighths(t *testing.T) {
	cases := map[[3]int]string{{8, 8, 1}: "█", {1, 8, 1}: "▏", {4, 8, 1}: "▌", {0, 8, 4}: "", {16, 16, 3}: "███"}
	for in, want := range cases {
		if got := bar(in[0], in[1], in[2]); got != want {
			t.Errorf("bar%v = %q, want %q", in, got, want)
		}
	}
}

func TestHomeLinesUpEverySection(t *testing.T) {
	out := plain(t, renderHome, "home.json")
	col, rows := -1, 0
	for _, line := range strings.Split(out, "\n") {
		// A video row: an 11-character id, then the dancers, then the orchestra.
		r := []rune(line)
		if len(r) < 14 || string(r[11:13]) != "  " {
			continue
		}
		orchestra := -1
		for i := 15; i < len(r); i++ {
			if r[i-2] == ' ' && r[i-1] == ' ' && r[i] != ' ' {
				orchestra = i
				break
			}
		}
		if orchestra < 0 {
			continue // no orchestra on this row
		}
		rows++
		if col < 0 {
			col = orchestra
		} else if orchestra != col {
			t.Fatalf("sections drift (%d vs %d):\n%s", orchestra, col, out)
		}
	}
	if rows < 3 {
		t.Fatalf("only %d rows compared:\n%s", rows, out)
	}
	if !strings.Contains(out, "more: tt search") {
		t.Fatalf("each section says how to see all of it:\n%s", out)
	}
}

func TestDancerExtrasReadInOrderBeforeTheVideos(t *testing.T) {
	out := plain(t, renderDancer, "dancer_sebastian-achaval_include.json")
	last := -1
	for _, h := range []string{"Year by year", "Partnerships", "On tour", "Repertoire", "ID           DANCERS"} {
		i := strings.Index(out, h)
		if i < last {
			t.Fatalf("%q is missing or out of order in:\n%s", h, out)
		}
		last = i
	}
	if !strings.Contains(out, "more (--jq '.tour.performances[]'") {
		t.Errorf("says there is more on the road:\n%s", out)
	}
}

func TestVideoShowMarksThisDanceOfTheSession(t *testing.T) {
	out := plain(t, renderVideo, "video_bUFFLZVvttk.json")
	for _, want := range []string{"session    dance 1 of 3 · TangoLovers Festival", "bUFFLZVvttk  Se dice de mi", "← this one", "2  zvLSjzWVvf0"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestMachineTranslationIsSaid(t *testing.T) {
	out := plain(t, renderSong, "song_volver-a-sonar-carlos-di-sarli_lyrics.json")
	if !strings.Contains(out, "In English · machine translation") {
		t.Fatalf("got:\n%s", out)
	}
}

func TestResolveListsTheReadingFirst(t *testing.T) {
	out := plain(t, renderResolve, "resolve.json")
	lines := strings.Split(out, "\n")
	if !strings.HasPrefix(lines[1], "noelia-hurtado") || !strings.HasPrefix(lines[2], "carlos-di-sarli") {
		t.Fatalf("got:\n%s", out)
	}
}

func TestVersionsNameThisRecordingThenTheOthers(t *testing.T) {
	out := plain(t, renderVersions, "song_versions_todo-es-amor-fulvio-salamanca.json")
	if !strings.Contains(out, "this one   Fulvio Salamanca") || !strings.Contains(out, "todo-es-amor-rodolfo-biagi") {
		t.Fatalf("got:\n%s", out)
	}
}

func TestEventShowsWhereWhoAndWhat(t *testing.T) {
	out := plain(t, renderEvent, "event_planetango.json")
	for _, want := range []string{"Planetango  planetango", "where      Moscow, Russia", "editions   2026", "danced to  Juan D'Arienzo"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}
