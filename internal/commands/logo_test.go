package commands

import (
	"bytes"
	"strings"
	"testing"

	"github.com/justinallenmarsh/tangotube-cli/internal/brand"
)

func homeApp(width, height int, env map[string]string) (*App, *bytes.Buffer) {
	var out bytes.Buffer
	a := &App{Out: &out, Err: &out, TTY: true, Width: width, Height: height,
		Env: func(k string) string { return env[k] }}
	return a, &out
}

func TestHomeShowsThePosterWhenThereIsRoom(t *testing.T) {
	a, out := homeApp(100, 50, map[string]string{"COLORTERM": "truecolor"})
	if err := a.home(); err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(out.String(), "▀"); n < brand.PosterCols*brand.PosterRows() {
		t.Fatalf("expected the poster, got %d half blocks", n)
	}
	for _, want := range []string{tagline, "Discover Tango Videos.", "next:"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %q", want)
		}
	}
}

func TestHomeDoesNotAnimateWithoutSomeoneWatching(t *testing.T) {
	// StdinTTY is false here, as it is when tt is run from a script.
	a, out := homeApp(100, 50, map[string]string{"COLORTERM": "truecolor"})
	_ = a.home()
	if strings.Contains(out.String(), "\x1b[?25l") {
		t.Fatal("the intro played with nobody at the keyboard")
	}
}

func TestHomeFallsBackOnSmallTerminals(t *testing.T) {
	a, out := homeApp(100, 20, map[string]string{})
	_ = a.home()
	if rows := strings.Count(out.String(), "\n"); rows > 20 {
		t.Errorf("a 20-row terminal got %d rows", rows)
	}
	a, out = homeApp(40, 50, map[string]string{})
	_ = a.home()
	if !strings.Contains(out.String(), "TangoTube") || strings.Contains(out.String(), "▀▀▀▀") {
		t.Errorf("a 40-column terminal should get the name in words:\n%s", out.String())
	}
}

func TestHomeUnderNoColorDrawsShapesOnly(t *testing.T) {
	a, out := homeApp(100, 50, map[string]string{"NO_COLOR": "1"})
	_ = a.home()
	if strings.Contains(out.String(), "\x1b") {
		t.Fatal("NO_COLOR output carries an escape")
	}
	if !strings.ContainsAny(out.String(), "▀▄█") {
		t.Fatal("no logo drawn")
	}
}

func TestDanceNeedsATerminal(t *testing.T) {
	a, _ := homeApp(100, 50, map[string]string{})
	a.TTY = false
	err := newDanceCmd(a).RunE(newDanceCmd(a), nil)
	if err == nil || !strings.Contains(err.Error(), "terminal") {
		t.Fatalf("got %v", err)
	}
}
