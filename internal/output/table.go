package output

import (
	"fmt"
	"sort"
	"strings"
)

// Column is one column of a table. Flex columns give up width first when the
// terminal is narrow.
type Column struct {
	Header string
	Flex   bool
	ID     bool // ids and slugs: gold where the cell links (Links), primary text otherwise
	Dim    bool // years, durations
	Right  bool // counts: right-aligned, so their digits line up
	// Min is the narrowest the column may be: tables printed one after another
	// line up when they share it.
	Min int
	// Width, when set, is exactly how wide the column is (see Grid).
	Width int
	// Links, when set, holds a URL for each row: the cell becomes a link to it.
	Links []string
}

// Table writes rows under dimmed headers, fitted to the terminal width.
func (p *Printer) Table(cols []Column, rows [][]string) { p.table(cols, rows, true) }

// Rows is a Table without its header row: for short runs under a heading of
// their own.
func (p *Printer) Rows(cols []Column, rows [][]string) { p.table(cols, rows, false) }

func (p *Printer) table(cols []Column, rows [][]string, header bool) {
	widths := make([]int, len(cols))
	for i, c := range cols {
		if header {
			widths[i] = DisplayWidth(c.Header)
		}
		widths[i] = max(widths[i], c.Min)
	}
	for _, row := range rows {
		for i, cell := range row {
			if n := DisplayWidth(cell); n > widths[i] {
				widths[i] = n
			}
		}
	}
	for i, c := range cols {
		if c.Width > 0 {
			widths[i] = c.Width
		}
	}
	fit(cols, widths, p.width(), rows)

	var head []string
	for i, c := range cols {
		if c.Right {
			head = append(head, padLeft(c.Header, widths[i]))
			continue
		}
		head = append(head, pad(c.Header, widths[i]))
	}
	if header {
		fmt.Fprintln(p.Out, p.Style.Dim(strings.TrimRight(strings.Join(head, "  "), " ")))
	}

	for r, row := range rows {
		var cells []string
		for i := 0; i < len(cols); i++ {
			c := cols[i]
			// A cell with nothing to its right on this row may run on into
			// those empty cells rather than stop at its own column: a video
			// with no song credited shows every dancer's name.
			width, last := widths[i], i
			if !c.Right {
				for last+1 < len(cols) && row[last+1] == "" && !cols[last+1].Right {
					last++
					width += 2 + widths[last]
				}
			}
			text := Truncate(row[i], width)
			gap := strings.Repeat(" ", max(0, width-DisplayWidth(text)))
			lead := ""
			if c.Right {
				lead, gap = gap, ""
			}
			if last == len(cols)-1 {
				gap = ""
			}
			link := ""
			if r < len(c.Links) {
				link = c.Links[r]
			}
			switch {
			case c.ID:
				text = p.Style.ID(link, text)
			case link != "":
				text = p.Style.Text(p.Style.Link(link, text))
			case c.Dim:
				text = p.Style.Dim(text)
			default:
				text = p.Style.Text(text)
			}
			cells = append(cells, lead+text+gap)
			i = last
		}
		fmt.Fprintln(p.Out, strings.TrimRight(strings.Join(cells, "  "), " "))
	}
}

// Columns is how wide the terminal is, or 100 when nobody said.
func (p *Printer) Columns() int { return p.width() }

func (p *Printer) width() int {
	if p.Width > 0 {
		return p.Width
	}
	return 100
}

// Widths is how wide each column's widest cell is: pass it back as each
// column's Min to line several tables up.
func Widths(cols []Column, groups ...[][]string) []int {
	widths := make([]int, len(cols))
	for _, rows := range groups {
		for _, row := range rows {
			for i, cell := range row {
				widths[i] = max(widths[i], DisplayWidth(cell))
			}
		}
	}
	return widths
}

// Grid fits one set of widths to every group of rows and pins them on the
// columns, so tables printed one after another line up.
func (p *Printer) Grid(cols []Column, groups ...[][]string) []Column {
	widths := Widths(cols, groups...)
	var all [][]string
	for i, c := range cols {
		widths[i] = max(widths[i], c.Min)
	}
	for _, rows := range groups {
		all = append(all, rows...)
	}
	fit(cols, widths, p.width(), all)
	out := append([]Column(nil), cols...)
	for i := range out {
		out[i].Width = widths[i]
	}
	return out
}

// fit shrinks flex columns until the row fits, a character at a time. First
// it trims what an outlier alone asks for — a column wide for one long song
// title comes down to its typical cell, the biggest excess first — and only
// then shrinks the widest, so no column is starved to save another's slack.
// Pinned columns (Width) are left alone.
func fit(cols []Column, widths []int, total int, rows [][]string) {
	used := func() int {
		n := 2 * (len(widths) - 1)
		for _, w := range widths {
			n += w
		}
		return n
	}
	floor := make([]int, len(cols))
	typical := make([]int, len(cols))
	for i, c := range cols {
		floor[i] = max(8, DisplayWidth(c.Header))
		typical[i] = max(floor[i], medianWidth(rows, i))
	}
	shrink := func(pick func(i int) int) bool {
		best, bestScore := -1, 0
		for i, c := range cols {
			if !c.Flex || c.Width > 0 || widths[i] <= floor[i] {
				continue
			}
			if score := pick(i); score > 0 && (best < 0 || score > bestScore) {
				best, bestScore = i, score
			}
		}
		if best < 0 {
			return false
		}
		widths[best]--
		return true
	}
	for used() > total && shrink(func(i int) int { return widths[i] - typical[i] }) {
	}
	for used() > total && shrink(func(i int) int { return widths[i] }) {
	}
}

// medianWidth is the middle width of column i's filled cells, leaving out
// cells with an empty one beside them: those run on and need no room.
func medianWidth(rows [][]string, i int) int {
	var ws []int
	for _, row := range rows {
		if row[i] != "" && (i == len(row)-1 || row[i+1] != "") {
			ws = append(ws, DisplayWidth(row[i]))
		}
	}
	if len(ws) == 0 {
		return 0
	}
	sort.Ints(ws)
	return ws[len(ws)/2]
}

func padLeft(s string, width int) string {
	if n := visibleLen(s); n < width {
		return strings.Repeat(" ", width-n) + s
	}
	return s
}

func pad(s string, width int) string {
	if n := visibleLen(s); n < width {
		return s + strings.Repeat(" ", width-n)
	}
	return s
}
