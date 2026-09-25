package output

import (
	"bytes"
	"strings"
	"testing"
)

func sample() *Envelope {
	env := NewEnvelope(map[string]any{"videos": []map[string]string{{"id": "2ByaUQXeAeo"}}}, "1 video", "tt video show 2ByaUQXeAeo")
	env.Raw = []byte(`{"ok":true,"data":{"videos":[{"id":"2ByaUQXeAeo"}]},"summary":"1 video","breadcrumbs":["tt video show 2ByaUQXeAeo"]}`)
	return env
}

func TestJSONModeWritesTheServerEnvelopeOnOneLine(t *testing.T) {
	var out bytes.Buffer
	p := &Printer{Out: &out, Mode: JSON}
	if err := p.Success(sample(), nil); err != nil {
		t.Fatal(err)
	}
	raw := sample()
	raw.Raw = []byte("{\n  \"ok\": true,\n  \"data\": {}\n}")
	out.Reset()
	_ = p.Success(raw, nil)
	if got := out.String(); got != `{"ok":true,"data":{}}`+"\n" {
		t.Fatalf("got %s", got)
	}
}

func TestQuietModeWritesOnlyData(t *testing.T) {
	var out bytes.Buffer
	p := &Printer{Out: &out, Mode: Quiet}
	_ = p.Success(sample(), nil)
	if got := strings.TrimSpace(out.String()); got != `{"videos":[{"id":"2ByaUQXeAeo"}]}` {
		t.Fatalf("got %s", got)
	}
}

func TestHumanModeShowsSummaryAndNextSteps(t *testing.T) {
	var out bytes.Buffer
	p := &Printer{Out: &out, Mode: Human}
	_ = p.Success(sample(), nil)
	if !strings.HasPrefix(out.String(), "1 video\n") || !strings.Contains(out.String(), "next: tt video show 2ByaUQXeAeo") {
		t.Fatalf("got %q", out.String())
	}
	if strings.Contains(out.String(), "\x1b[") {
		t.Fatal("colour without a style")
	}
}

func TestJQFiltersDataAndPrintsStringsBare(t *testing.T) {
	var out bytes.Buffer
	p := &Printer{Out: &out, Mode: JSON, JQ: ".videos[].id"}
	_ = p.Success(sample(), nil)
	if out.String() != "2ByaUQXeAeo\n" {
		t.Fatalf("got %q", out.String())
	}
}

func TestFailureGoesToStdoutForMachinesAndStderrForPeople(t *testing.T) {
	var out, errOut bytes.Buffer
	env := Fail(CodeNotFound, "No video with that id", `tt search "di sarli"`)
	(&Printer{Out: &out, Err: &errOut, Mode: JSON}).Failure(env)
	if !strings.Contains(out.String(), `"code":"not_found"`) || errOut.Len() != 0 {
		t.Fatalf("json: %q / %q", out.String(), errOut.String())
	}
	out.Reset()
	(&Printer{Out: &out, Err: &errOut, Mode: Human}).Failure(env)
	if out.Len() != 0 || !strings.Contains(errOut.String(), "No video with that id") || !strings.Contains(errOut.String(), `try: tt search "di sarli"`) {
		t.Fatalf("human: %q / %q", out.String(), errOut.String())
	}
}

func TestExitCodes(t *testing.T) {
	want := map[string]int{"usage": 1, "not_found": 2, "auth": 3, "forbidden": 4, "rate_limit": 5, "network": 6, "api": 7, "surprise": 7}
	for code, n := range want {
		if got := ExitCode(code); got != n {
			t.Errorf("%s: %d, want %d", code, got, n)
		}
	}
}

func TestStylePaintsOnlyWhenEnabled(t *testing.T) {
	if (Style{}).Error("✗") != "✗" {
		t.Fatal("disabled style painted")
	}
	if got := (Style{Enabled: true, TrueColor: true}).Error("✗"); got != "\x1b[38;2;196;30;58m✗\x1b[0m" {
		t.Fatalf("got %q", got)
	}
	if got := (Style{Enabled: true}).Gold("x"); !strings.HasPrefix(got, "\x1b[38;5;") {
		t.Fatalf("256-colour fallback: %q", got)
	}
}

func TestTableFitsTheTerminal(t *testing.T) {
	var out bytes.Buffer
	p := &Printer{Out: &out, Width: 40}
	p.Table([]Column{{Header: "ID"}, {Header: "DANCERS", Flex: true}, {Header: "YEAR"}},
		[][]string{{"2ByaUQXeAeo", "Carlitos Espinoza & Noelia Hurtado and friends", "1941"}})
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if n := len([]rune(line)); n > 40 {
			t.Fatalf("line of %d runes: %q", n, line)
		}
	}
	if !strings.Contains(out.String(), "…") {
		t.Fatal("expected truncation")
	}
}

func TestTableCellRunsOnIntoEmptyCellsBesideIt(t *testing.T) {
	var out bytes.Buffer
	p := &Printer{Out: &out, Width: 70}
	cols := []Column{{Header: "ID"}, {Header: "DANCERS", Flex: true}, {Header: "ORCHESTRA", Flex: true}, {Header: "SONG", Flex: true}, {Header: "YEAR"}}
	p.Table(cols, [][]string{
		{"ilxOpAK7n7g", "Leandro Oliver & Ricardo Calvo & Cyntia Palacios", "", "", ""},
		{"Ld5HIqcsa7M", "Carlitos Espinoza & Noelia Hurtado", "Osvaldo Pugliese", "Berretín", "1979"},
		{"2S_EPrCMyiU", "Carlitos Espinoza & Noelia Hurtado & a friend", "", "Sorbos amargos", "1942"},
	})
	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	if !strings.Contains(lines[1], "Leandro Oliver & Ricardo Calvo & Cyntia Palacios") {
		t.Fatalf("the names should run on into the empty columns: %q", lines[1])
	}
	for _, line := range lines {
		if n := DisplayWidth(line); n > 70 {
			t.Fatalf("line of %d: %q", n, line)
		}
	}
	// A spill into one empty column leaves the next one where it was.
	at := func(line, word string) int { return DisplayWidth(line[:strings.Index(line, word)]) }
	if at(lines[3], "Sorbos") != at(lines[2], "Berretín") {
		t.Fatalf("SONG moved:\n%s\n%s", lines[2], lines[3])
	}
}

func TestTableAlignsWideCharacters(t *testing.T) {
	var buf bytes.Buffer
	p := &Printer{Out: &buf, Width: 100}
	p.Table([]Column{{Header: "TITLE"}, {Header: "TAGS"}}, [][]string{
		{"❤️ 八 ochos — Ñandú 🎻", "sacada"},
		{"plain", "boleo"},
	})
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	col := func(line, word string) int { return DisplayWidth(line[:strings.Index(line, word)]) }
	if col(lines[1], "sacada") != col(lines[2], "boleo") || col(lines[0], "TAGS") != col(lines[2], "boleo") {
		t.Fatalf("columns drift:\n%s", buf.String())
	}
}

func TestCountsLineUpOnTheRight(t *testing.T) {
	var buf bytes.Buffer
	p := &Printer{Out: &buf, Width: 100}
	p.Table([]Column{{Header: "NAME"}, {Header: "VIDEOS", Right: true}, {Header: "YEAR"}}, [][]string{
		{"Juan D'Arienzo", "12,815", "1935"},
		{"Carlos Di Sarli", "812", "1941"},
	})
	want := "NAME             VIDEOS  YEAR\nJuan D'Arienzo   12,815  1935\nCarlos Di Sarli     812  1941\n"
	if buf.String() != want {
		t.Fatalf("got:\n%s\nwant:\n%s", buf.String(), want)
	}
}

func TestLinksAreOSC8AndOnlyWhenAsked(t *testing.T) {
	on := Style{Links: true}
	if got := on.Link("https://tangotube.tv/watch?v=uGwRPRusbC0", "uGwRPRusbC0"); got != "\x1b]8;;https://tangotube.tv/watch?v=uGwRPRusbC0\x1b\\uGwRPRusbC0\x1b]8;;\x1b\\" {
		t.Fatalf("got %q", got)
	}
	if got := (Style{}).Link("https://tangotube.tv", "x"); got != "x" {
		t.Fatalf("links off should leave the text alone, got %q", got)
	}
}

// A URL has an m in it (…/tangotube.com, …/milonga). Reading an OSC to the first
// letter, the way a colour code ends, would swallow the text and break widths.
func TestWidthSkipsHyperlinks(t *testing.T) {
	s := Style{Links: true, Enabled: true, TrueColor: true}
	cell := s.ID("https://www.youtube.com/watch?v=milonga1234", "milonga1234")
	if n := visibleLen(cell); n != 11 {
		t.Fatalf("visible width %d, want 11", n)
	}
}

func TestTableLinksTheIDAndKeepsColumnsAligned(t *testing.T) {
	var out strings.Builder
	p := &Printer{Out: &out, Width: 80, Style: Style{Links: true}}
	p.Table([]Column{
		{Header: "ID", Links: []string{"https://tangotube.tv/watch?v=a", "https://tangotube.tv/watch?v=bbb"}},
		{Header: "SONG"},
	}, [][]string{{"a", "Bahía Blanca"}, {"bbb", "Gallo ciego"}})
	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	if !strings.Contains(lines[1], "\x1b]8;;https://tangotube.tv/watch?v=a\x1b\\a\x1b]8;;\x1b\\") {
		t.Fatalf("no link on the id: %q", lines[1])
	}
	if strings.Index(StripEscapes(lines[1]), "Bahía") != strings.Index(StripEscapes(lines[2]), "Gallo") {
		t.Fatalf("columns moved:\n%s\n%s", StripEscapes(lines[1]), StripEscapes(lines[2]))
	}
}

func TestALongSummaryWrapsToTheTerminal(t *testing.T) {
	var out bytes.Buffer
	p := &Printer{Out: &out, Mode: Human, Width: 40}
	env := sample()
	env.Summary = "Applied. The video names them now; their dancer page lists it after the nightly rebuild, at 03:10 UTC"
	_ = p.Success(env, nil)
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if DisplayWidth(line) > 40 {
			t.Errorf("line wider than 40: %q", line)
		}
	}
	if !strings.Contains(out.String(), "at 03:10 UTC") {
		t.Fatalf("got %q", out.String())
	}
}

// Red is for errors. An id painted bandoneón read as one: a column of
// payload ids looked like a column of failures.
func TestIDsAreGoldWhenTheyLinkAndIvoryOtherwiseNeverRed(t *testing.T) {
	const red, gold, ivory = "38;2;196;30;58", "38;2;196;163;90", "38;2;244;239;230"
	var out strings.Builder
	p := &Printer{Out: &out, Width: 80, Style: Style{Enabled: true, TrueColor: true, DarkBG: true, Links: true}}
	p.Table([]Column{{Header: "PAYLOAD", ID: true, Right: true}, {Header: "VIDEO", ID: true, Links: []string{"https://tangotube.tv/watch?v=a"}}},
		[][]string{{"17", "uGwRPRusbC0"}})
	row := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")[1]
	if strings.Contains(row, red) {
		t.Fatalf("an id in red: %q", row)
	}
	if !strings.Contains(row, "\x1b["+ivory+"m17") {
		t.Fatalf("an id with nowhere to go should be ivory: %q", row)
	}
	if !strings.Contains(row, "\x1b["+gold+"m\x1b]8;;https://tangotube.tv/watch?v=a") {
		t.Fatalf("a linked id should be gold: %q", row)
	}
	if got := (Style{Enabled: true, TrueColor: true, DarkBG: true}).ID("https://tangotube.tv", "x"); strings.Contains(got, gold) {
		t.Fatalf("no links in this terminal, so nothing to click: %q", got)
	}
}
