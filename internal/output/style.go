package output

import (
	"fmt"
	"strings"

	"github.com/mattn/go-runewidth"
)

// The TangoTube palette. Five colours, each with one job.
const (
	Ivory     = "#F4EFE6" // primary text on dark terminals
	Dim       = "#8A8580" // year, duration, hints
	Gold      = "#C4A35A" // breadcrumbs, next:, ids that are links
	Bandoneon = "#C41E3A" // wordmark and errors, nothing else: red reads as wrong
	Ok        = "#6B8F71" // doctor pass
)

// Style paints strings for a terminal. The zero value paints nothing, which is
// what a pipe, a file, or NO_COLOR gets.
type Style struct {
	Enabled   bool
	TrueColor bool
	DarkBG    bool
	// Links makes ids and addresses clickable, with OSC 8 hyperlinks. It is
	// separate from colour: NO_COLOR is about colour, and a link is not one.
	Links bool
}

// Link makes text a hyperlink to url in terminals that know OSC 8 (iTerm2,
// Ghostty, WezTerm, kitty, GNOME Terminal, Windows Terminal, VS Code). The
// rest ignore the escape and show the text.
func (s Style) Link(url, text string) string {
	if !s.Links || url == "" || text == "" {
		return text
	}
	return "\x1b]8;;" + url + "\x1b\\" + text + "\x1b]8;;\x1b\\"
}

func (s Style) paint(hex, text string, bold bool) string {
	if !s.Enabled || text == "" {
		return text
	}
	var codes []string
	if bold {
		codes = append(codes, "1")
	}
	if hex != "" {
		r, g, b := rgb(hex)
		if s.TrueColor {
			codes = append(codes, fmt.Sprintf("38;2;%d;%d;%d", r, g, b))
		} else {
			codes = append(codes, fmt.Sprintf("38;5;%d", xterm256(r, g, b)))
		}
	}
	if len(codes) == 0 {
		return text
	}
	return "\x1b[" + strings.Join(codes, ";") + "m" + text + "\x1b[0m"
}

// Text is primary text: ivory where the background is known to be dark, the
// terminal's own foreground everywhere else, so a light terminal stays legible.
func (s Style) Text(t string) string {
	if s.DarkBG {
		return s.paint(Ivory, t, false)
	}
	return t
}

func (s Style) Bold(t string) string {
	if s.DarkBG {
		return s.paint(Ivory, t, true)
	}
	return s.paint("", t, true)
}

func (s Style) Dim(t string) string   { return s.paint(Dim, t, false) }
func (s Style) Gold(t string) string  { return s.paint(Gold, t, false) }
func (s Style) Error(t string) string { return s.paint(Bandoneon, t, false) }
func (s Style) Mark(t string) string  { return s.paint(Bandoneon, t, true) }

// ID paints an id or a slug: gold, and a link to url, where the terminal
// makes links and there is somewhere to go; primary text otherwise. Never
// bandoneón: an id is not an error.
func (s Style) ID(url, text string) string {
	if s.Links && url != "" && text != "" {
		return s.Gold(s.Link(url, text))
	}
	return s.Text(text)
}
func (s Style) Ok(t string) string { return s.paint(Ok, t, false) }

func rgb(hex string) (int, int, int) {
	var r, g, b int
	fmt.Sscanf(strings.TrimPrefix(hex, "#"), "%02x%02x%02x", &r, &g, &b)
	return r, g, b
}

// xterm256 maps a colour onto the 6×6×6 cube for terminals without truecolor.
func xterm256(r, g, b int) int {
	q := func(v int) int {
		if v < 48 {
			return 0
		}
		if v < 115 {
			return 1
		}
		return (v - 35) / 40
	}
	return 16 + 36*q(r) + 6*q(g) + q(b)
}

// visibleLen counts what a terminal shows: display cells, not bytes or runes
// (八 and 🎻 take two), and no escapes.
func visibleLen(s string) int { return DisplayWidth(StripEscapes(s)) }

// StripEscapes drops what a terminal does not print: colour (CSI … m) and
// hyperlinks (OSC 8 … ST). A URL has letters in it, so an OSC has to be read
// to its terminator, not to the first letter.
func StripEscapes(s string) string {
	var plain strings.Builder
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		if rs[i] != '\x1b' || i+1 >= len(rs) {
			plain.WriteRune(rs[i])
			continue
		}
		switch rs[i+1] {
		case '[':
			i += 2
			for i < len(rs) && !(rs[i] >= '@' && rs[i] <= '~') {
				i++
			}
		case ']':
			i += 2
			for i < len(rs) && rs[i] != '\a' && !(rs[i] == '\x1b' && i+1 < len(rs) && rs[i+1] == '\\') {
				i++
			}
			if i < len(rs) && rs[i] == '\x1b' {
				i++
			}
		default:
			i++
		}
	}
	return plain.String()
}

// DisplayWidth is how many terminal cells plain text takes. A character
// followed by the emoji variation selector (❤️) is drawn as an emoji, two
// cells, even where its bare form (❤) takes one.
func DisplayWidth(s string) int {
	n := runewidth.StringWidth(s)
	prev := rune(0)
	for _, r := range s {
		if r == '\uFE0F' && prev != 0 && runewidth.RuneWidth(prev) == 1 {
			n++
		}
		prev = r
	}
	return n
}

// Truncate shortens plain text to width cells, ending in an ellipsis.
func Truncate(s string, width int) string {
	if width <= 0 || DisplayWidth(s) <= width {
		return s
	}
	if width == 1 {
		return "…"
	}
	return runewidth.Truncate(s, width, "…")
}
