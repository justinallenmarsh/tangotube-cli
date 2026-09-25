package brand

import (
	"fmt"
	"math"
	"strings"
)

// Depth is how many colours the terminal can show.
type Depth int

const (
	// Mono is for NO_COLOR: the logo in the terminal's own foreground, as
	// half-block shapes with no panel behind them.
	Mono Depth = iota
	// Color256 maps each pixel to the nearest xterm-256 colour. The logo is
	// almost all greys, and the 24-step grey ramp holds them well.
	Color256
	// TrueColor paints every pixel as it is.
	TrueColor
)

// Render turns the canvas into terminal rows, indented by indent spaces.
func (c *Canvas) Render(depth Depth, indent int) string {
	var b strings.Builder
	pad := strings.Repeat(" ", indent)
	for y := 0; y+1 < c.H; y += 2 {
		b.WriteString(pad)
		if depth == Mono {
			b.WriteString(strings.TrimRight(c.monoRow(y), " "))
			b.WriteByte('\n')
			continue
		}
		var lastFG, lastBG string
		for x := 0; x < c.W; x++ {
			fg, bg := code(c.At(x, y), depth, 38), code(c.At(x, y+1), depth, 48)
			if fg != lastFG || bg != lastBG {
				b.WriteString("\x1b[" + fg + ";" + bg + "m")
				lastFG, lastBG = fg, bg
			}
			b.WriteString("▀")
		}
		b.WriteString("\x1b[0m\n")
	}
	return b.String()
}

// monoRow draws the lit pixels of one row pair: the couple and the letters,
// not the disc or the panel.
func (c *Canvas) monoRow(y int) string {
	var b strings.Builder
	for x := 0; x < c.W; x++ {
		top, bottom := lit(c.At(x, y)), lit(c.At(x, y+1))
		switch {
		case top && bottom:
			b.WriteString("█")
		case top:
			b.WriteString("▀")
		case bottom:
			b.WriteString("▄")
		default:
			b.WriteString(" ")
		}
	}
	return b.String()
}

func lit(v RGB) bool { return (v.R+v.G+v.B)/3 > 110 }

func code(v RGB, depth Depth, base int) string {
	r, g, b := clamp(v.R), clamp(v.G), clamp(v.B)
	if depth == TrueColor {
		return fmt.Sprintf("%d;2;%d;%d;%d", base, r, g, b)
	}
	return fmt.Sprintf("%d;5;%d", base, nearest256(r, g, b))
}

func clamp(f float64) int { return int(math.Max(0, math.Min(255, math.Round(f)))) }

// nearest256 picks the closer of the nearest grey-ramp step and the nearest
// colour-cube entry.
func nearest256(r, g, b int) int {
	levels := []int{0, 95, 135, 175, 215, 255}
	near := func(v int) int {
		best := 0
		for i, l := range levels {
			if abs(v-l) < abs(v-levels[best]) {
				best = i
			}
		}
		return best
	}
	cr, cg, cb := near(r), near(g), near(b)
	cube := 16 + 36*cr + 6*cg + cb
	cubeDist := sq(r-levels[cr]) + sq(g-levels[cg]) + sq(b-levels[cb])

	avg := (r + g + b) / 3
	step := int(math.Round(float64(avg-8) / 10))
	if step < 0 {
		step = 0
	}
	if step > 23 {
		step = 23
	}
	grey := 8 + 10*step
	greyDist := sq(r-grey) + sq(g-grey) + sq(b-grey)
	if greyDist < cubeDist {
		return 232 + step
	}
	return cube
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func sq(v int) int { return v * v }
