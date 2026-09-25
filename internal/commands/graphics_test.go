package commands

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

func TestGraphicsFollowsTheTerminal(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want graphics
	}{
		{"kitty", map[string]string{"TERM": "xterm-kitty"}, kittyGraphics},
		{"ghostty", map[string]string{"TERM_PROGRAM": "ghostty"}, kittyGraphics},
		{"konsole", map[string]string{"KONSOLE_VERSION": "230804"}, kittyGraphics},
		{"iterm2", map[string]string{"TERM_PROGRAM": "iTerm.app"}, itermGraphics},
		{"wezterm", map[string]string{"TERM_PROGRAM": "WezTerm"}, itermGraphics},
		{"terminal.app", map[string]string{"TERM_PROGRAM": "Apple_Terminal"}, noGraphics},
		{"inside tmux", map[string]string{"TERM_PROGRAM": "iTerm.app", "TMUX": "/tmp/tmux-1/default,1,0"}, noGraphics},
		{"override", map[string]string{"TERM_PROGRAM": "Apple_Terminal", "TT_IMAGES": "kitty"}, kittyGraphics},
		{"turned off", map[string]string{"TERM": "xterm-kitty", "TT_IMAGES": "none"}, noGraphics},
	}
	for _, tc := range cases {
		a := &App{TTY: true, Env: func(k string) string { return tc.env[k] }}
		if got := a.graphics(); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
	piped := &App{TTY: false, Env: func(k string) string { return map[string]string{"TERM": "xterm-kitty"}[k] }}
	if piped.graphics() != noGraphics {
		t.Error("piped output must never carry an image")
	}
}

// kitty's protocol: the first chunk carries the keys, every chunk is at most
// 4096 bytes of base64, m=1 on all but the last, and the payload reassembles.
func TestKittyChunks(t *testing.T) {
	data := bytes.Repeat([]byte("tango"), 3000) // 15000 bytes → 20000 base64
	var out strings.Builder
	writeKitty(&out, data, size{cols: 58}, 1, true)
	chunks := strings.Split(strings.TrimSuffix(out.String(), "\x1b\\"), "\x1b\\")
	if len(chunks) != 5 {
		t.Fatalf("%d chunks, want 5", len(chunks))
	}
	var payload strings.Builder
	for i, c := range chunks {
		if !strings.HasPrefix(c, "\x1b_G") {
			t.Fatalf("chunk %d does not open with APC G: %q", i, c[:10])
		}
		keys, body, _ := strings.Cut(strings.TrimPrefix(c, "\x1b_G"), ";")
		if len(body) > 4096 {
			t.Fatalf("chunk %d is %d bytes", i, len(body))
		}
		last := i == len(chunks)-1
		if last != strings.HasSuffix(keys, "m=0") || !last != strings.HasSuffix(keys, "m=1") {
			t.Fatalf("chunk %d keys %q", i, keys)
		}
		if i == 0 && keys != "a=T,f=100,i=1,p=1,q=2,c=58,C=1,m=1" {
			t.Fatalf("first chunk keys %q", keys)
		}
		payload.WriteString(body)
	}
	got, err := base64.StdEncoding.DecodeString(payload.String())
	if err != nil || !bytes.Equal(got, data) {
		t.Fatal("payload does not reassemble")
	}
}

func TestITermInlineImage(t *testing.T) {
	var out strings.Builder
	writeITerm(&out, []byte("png"), size{rows: 5})
	want := "\x1b]1337;File=inline=1;size=3;height=5;preserveAspectRatio=1:cG5n\a"
	if out.String() != want {
		t.Fatalf("got %q", out.String())
	}
}

func TestHomeDrawsAnImageWhereTheTerminalCan(t *testing.T) {
	a, out := homeApp(100, 60, map[string]string{"TERM_PROGRAM": "iTerm.app", "COLORTERM": "truecolor"})
	a.Cell = func() (float64, float64) { return 10, 21 }
	if err := a.home(); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	if !strings.Contains(s, "\x1b]1337;File=inline=1") {
		t.Fatal("no inline image")
	}
	if strings.Contains(s, "▀▀▀") {
		t.Fatal("drew half blocks as well as the image")
	}
	if !strings.Contains(s, tagline) {
		t.Fatal("tagline missing")
	}
}
