// Package output turns API envelopes into what the reader needs: a table at a
// terminal, the envelope itself in a pipe, the data alone under --quiet.
package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Envelope is the shape of every /api/v1 response and of every JSON document
// tt writes.
type Envelope struct {
	OK          bool            `json:"ok"`
	Data        json.RawMessage `json:"data,omitempty"`
	Summary     string          `json:"summary,omitempty"`
	Breadcrumbs []string        `json:"breadcrumbs,omitempty"`
	Error       *Error          `json:"error,omitempty"`

	// Raw holds the bytes the server sent, so a pipe gets them untouched.
	Raw []byte `json:"-"`
}

// Error is the error half of an envelope.
type Error struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Hint      string `json:"hint,omitempty"`
	Retryable bool   `json:"retryable"`
}

// Error codes shared by the API and the CLI.
const (
	CodeUsage     = "usage"
	CodeNotFound  = "not_found"
	CodeAuth      = "auth"
	CodeForbidden = "forbidden"
	CodeRateLimit = "rate_limit"
	CodeNetwork   = "network"
	CodeAPI       = "api"
)

var exitCodes = map[string]int{
	CodeUsage: 1, CodeNotFound: 2, CodeAuth: 3, CodeForbidden: 4,
	CodeRateLimit: 5, CodeNetwork: 6, CodeAPI: 7,
}

// ExitCode is the process exit status for an error code. Unknown codes are
// API errors: something answered, and it was not what tt expected.
func ExitCode(code string) int {
	if n, ok := exitCodes[code]; ok {
		return n
	}
	return 7
}

// NewEnvelope builds a success envelope from any JSON-encodable value.
func NewEnvelope(data any, summary string, breadcrumbs ...string) *Envelope {
	raw, _ := json.Marshal(data)
	return &Envelope{OK: true, Data: raw, Summary: summary, Breadcrumbs: breadcrumbs}
}

// Fail builds an error envelope. Messages are sentences, and end like one,
// the way the API's do.
func Fail(code, message, hint string) *Envelope {
	return &Envelope{Error: &Error{Code: code, Message: sentence(message), Hint: hint,
		Retryable: code == CodeNetwork || code == CodeRateLimit}}
}

func sentence(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || strings.HasSuffix(s, ".") || strings.HasSuffix(s, "?") || strings.HasSuffix(s, "!") {
		return s
	}
	return s + "."
}

// Mode is how a command's result reaches the reader.
type Mode int

const (
	Human Mode = iota // a terminal: tables, colour, next steps
	JSON              // the envelope
	Quiet             // .data alone
)

// Printer writes envelopes in one mode.
type Printer struct {
	Out    io.Writer
	Err    io.Writer
	Mode   Mode
	Style  Style
	Pretty bool   // indent JSON (someone asked for --json at a terminal)
	JQ     string // filter .data through this jq expression
	Width  int    // terminal columns

	// Detail views open with their own heading; the summary would repeat it.
	HideSummary bool
}

// Success writes a successful envelope. human renders it for a terminal; it
// may be nil when the summary and breadcrumbs say everything.
func (p *Printer) Success(env *Envelope, human func(*Printer, json.RawMessage)) error {
	if p.JQ != "" {
		return RunJQ(p.Out, env.Data, p.JQ)
	}
	switch p.Mode {
	case Quiet:
		return p.writeJSON(env.Data)
	case JSON:
		return p.writeEnvelope(env)
	}
	if env.Summary != "" && !p.HideSummary {
		for _, line := range wrapWords(env.Summary, p.width()) {
			fmt.Fprintln(p.Out, p.Style.Dim(line))
		}
	}
	if human != nil {
		human(p, env.Data)
	}
	p.Breadcrumbs(env.Breadcrumbs)
	return nil
}

// Failure writes an error envelope. Machines read stdout, so in JSON modes the
// envelope goes there; people read stderr.
func (p *Printer) Failure(env *Envelope) {
	e := env.Error
	if e == nil {
		e = &Error{Code: CodeAPI, Message: "The API answered without saying what went wrong"}
		env.Error = e
	}
	if p.Mode != Human || p.JQ != "" {
		_ = p.writeEnvelope(env)
		return
	}
	for i, line := range wrapWords(e.Message, p.width()-2) {
		lead := "  "
		if i == 0 {
			lead = p.Style.Error("✗ ")
		}
		fmt.Fprintln(p.Err, lead+line)
	}
	if e.Hint != "" {
		fmt.Fprintln(p.Err, p.Style.Dim("  try: ")+p.Style.Gold(e.Hint))
	}
}

// Breadcrumbs prints the next commands worth running.
func (p *Printer) Breadcrumbs(crumbs []string) {
	if len(crumbs) == 0 {
		return
	}
	fmt.Fprintln(p.Out)
	for _, c := range crumbs {
		fmt.Fprintln(p.Out, p.Style.Gold("next: "+c))
	}
}

func (p *Printer) writeEnvelope(env *Envelope) error {
	if len(env.Raw) > 0 {
		return p.writeJSON(env.Raw) // the server's bytes, one line (or indented at a terminal)
	}
	raw, err := json.Marshal(env)
	if err != nil {
		return err
	}
	return p.writeJSON(raw)
}

func (p *Printer) writeJSON(raw json.RawMessage) error {
	if len(raw) == 0 {
		raw = json.RawMessage("null")
	}
	var buf bytes.Buffer
	if p.Pretty {
		if err := json.Indent(&buf, raw, "", "  "); err != nil {
			buf.Write(raw)
		}
	} else if err := json.Compact(&buf, raw); err != nil {
		buf.Write(raw)
	}
	buf.WriteByte('\n')
	_, err := p.Out.Write(buf.Bytes())
	return err
}

// Line writes one line of primary text.
func (p *Printer) Line(format string, args ...any) {
	fmt.Fprintln(p.Out, p.Style.Text(fmt.Sprintf(format, args...)))
}

// Field writes an indented "label  value" line, the label dimmed.
func (p *Printer) Field(label, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	fmt.Fprintf(p.Out, "  %s %s\n", p.Style.Dim(fmt.Sprintf("%-10s", label)), p.Style.Text(value))
}

// FieldLink is a Field whose value is a link to url.
func (p *Printer) FieldLink(label, value, url string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	fmt.Fprintf(p.Out, "  %s %s\n", p.Style.Dim(fmt.Sprintf("%-10s", label)), p.Style.Text(p.Style.Link(url, value)))
}

// Words writes a Field whose value is a list, wrapped to the terminal and
// lined up under the first word. Words may carry colour; widths are measured
// on what the terminal shows.
func (p *Printer) Words(label string, words []string) {
	if len(words) == 0 {
		return
	}
	const indent = 13 // "  " + a ten-column label + " "
	room := p.width() - indent
	if room < 20 {
		room = 20
	}
	var lines []string
	line := ""
	for _, w := range words {
		switch {
		case line == "":
			line = w
		case visibleLen(line)+1+visibleLen(w) > room:
			lines = append(lines, line)
			line = w
		default:
			line += " " + w
		}
	}
	lines = append(lines, line)
	for i, l := range lines {
		if i > 0 {
			label = ""
		}
		p.Field(label, l)
	}
}

// Items writes a Field whose value is a list of phrases joined by " · ",
// wrapped to the terminal between phrases, never inside one, and lined up
// under the first. Phrases may carry colour or links; widths are measured on
// what the terminal shows.
func (p *Printer) Items(label string, items []string) {
	var kept []string
	for _, it := range items {
		if strings.TrimSpace(StripEscapes(it)) != "" {
			kept = append(kept, it)
		}
	}
	if len(kept) == 0 {
		return
	}
	room := p.valueRoom()
	sep := p.Style.Dim(" · ")
	var lines []string
	line := ""
	for _, it := range kept {
		switch {
		case line == "":
			line = it
		case visibleLen(line)+3+visibleLen(it) > room:
			lines = append(lines, line)
			line = it
		default:
			line += sep + it
		}
	}
	lines = append(lines, line)
	for i, l := range lines {
		if i > 0 {
			label = ""
		}
		p.Field(label, l)
	}
}

// Para writes a Field whose value is prose, wrapped under the value column.
func (p *Printer) Para(label, text string) {
	p.Words(label, strings.Fields(text))
}

// valueRoom is how many columns a Field's value has before the edge.
func (p *Printer) valueRoom() int {
	const indent = 13 // "  " + a ten-column label + " "
	room := p.width() - indent
	if room < 20 {
		room = 20
	}
	return room
}

// wrapWords breaks text at spaces so no line is wider than width: a summary
// that says where a change went can run past one line.
func wrapWords(text string, width int) []string {
	var lines []string
	cur := ""
	for _, w := range strings.Fields(text) {
		switch {
		case cur == "":
			cur = w
		case DisplayWidth(cur)+1+DisplayWidth(w) > width:
			lines = append(lines, cur)
			cur = w
		default:
			cur += " " + w
		}
	}
	return append(lines, cur)
}
