// Package brand draws the TangoTube logo in a terminal: the couple in their
// embrace, inside the dark disc of the favicon, and the TangoTube wordmark.
//
// Nothing here is a hand-drawn approximation. The couple is rasterised from
// the favicon's own geometry (assets/brand/mark.svg), which is small enough to
// keep as numbers, and that is what lets the couple turn. The wordmark is
// sampled from wordmark.png, the site's wordmark (assets/brand/wordmark.svg)
// rendered white on black at 2620×497.
//
// A terminal cell holds two square pixels, one above the other: the upper half
// block ▀ painted in one colour over a background of another. The logo brings
// its own dark panel, so it is anti-aliased against a background it knows, and
// it reads the same on a light terminal as on a dark one.
package brand

import (
	"bytes"
	_ "embed"
	"image"
	"image/png"
	"math"
	"sync"
)

// RGB is a colour with float channels, 0–255.
type RGB struct{ R, G, B float64 }

// The palette: the favicon's black disc on the CLI's dark panel, and the two
// tones of the embrace. Where the two bodies overlap, and in the heads, the
// logo is brightest; a body on its own is a step darker.
var (
	Panel = RGB{20, 17, 14}
	Disc  = RGB{8, 8, 8}
	Light = RGB{244, 239, 230} // ivory
	Shade = RGB{176, 168, 156}
)

type pt struct{ x, y float64 }

// The favicon, in its own 501×501 viewBox.
var (
	leader   = [3]pt{{133.32, 193.19}, {288.6, 385.14}, {316.38, 138.53}}
	follower = [3]pt{{257.48, 156.12}, {166.58, 385.14}, {353.43, 385.14}}
	heads    = [2]struct {
		c pt
		r float64
	}{{pt{212.7, 130.27}, 21.3}, {pt{275.39, 112.45}, 20.93}}
	// The couple turn about the point between their feet.
	axis = pt{245, 385}
)

func inTriangle(p pt, t [3]pt) bool {
	side := func(a, b, c pt) float64 { return (a.x-c.x)*(b.y-c.y) - (b.x-c.x)*(a.y-c.y) }
	d1, d2, d3 := side(p, t[0], t[1]), side(p, t[1], t[2]), side(p, t[2], t[0])
	return !((d1 < 0 || d2 < 0 || d3 < 0) && (d1 > 0 || d2 > 0 || d3 > 0))
}

// Pose is how the couple stand. Turn is their width as they rotate about the
// axis: 1 is the logo, -1 the logo seen from behind, and it never reaches 0,
// because a couple mid-giro is narrow, not gone. Lean tilts them, in radians,
// the rock of a cadencia.
type Pose struct {
	Turn float64
	Lean float64
}

// Still is the logo as the site draws it.
var Still = Pose{Turn: 1}

// What a point of the favicon is. The logo is flat colour, four of them, and
// it stays flat in the terminal: every pixel is one of these, never a blend.
type part int

const (
	partPanel part = iota
	partDisc
	partBody  // one of the two bodies alone
	partLight // where the bodies overlap, and the heads
)

var partColour = [...]RGB{Panel, Disc, Shade, Light}

// markAt is what lies at a point in the favicon's viewBox.
func markAt(p pt, pose Pose, withHeads bool) part {
	bg := partPanel
	if math.Hypot(p.x-250.5, p.y-250.5) <= 250 {
		bg = partDisc
	}
	dx, dy := p.x-axis.x, p.y-axis.y
	cs, sn := math.Cos(-pose.Lean), math.Sin(-pose.Lean)
	dx, dy = dx*cs-dy*sn, dx*sn+dy*cs
	turn := pose.Turn
	if math.Abs(turn) < 0.2 {
		turn = math.Copysign(0.2, turn)
	}
	q := pt{axis.x + dx/turn, axis.y + dy}
	if withHeads {
		for _, h := range heads {
			if math.Hypot(q.x-h.c.x, q.y-h.c.y) <= h.r {
				return partLight
			}
		}
	}
	n := 0
	if inTriangle(q, leader) {
		n++
	}
	if inTriangle(q, follower) {
		n++
	}
	switch n {
	case 2:
		return partLight
	case 1:
		return partBody
	}
	return bg
}

// Canvas is a grid of square pixels. Its height is always even, so it maps
// onto whole terminal rows.
type Canvas struct {
	W, H int
	Px   []RGB
}

// NewCanvas is a w×h canvas filled with the panel colour.
func NewCanvas(w, h int) *Canvas {
	if h%2 == 1 {
		h++
	}
	c := &Canvas{W: w, H: h, Px: make([]RGB, w*h)}
	for i := range c.Px {
		c.Px[i] = Panel
	}
	return c
}

func (c *Canvas) At(x, y int) RGB { return c.Px[y*c.W+x] }

func (c *Canvas) set(x, y int, v RGB) {
	if x >= 0 && y >= 0 && x < c.W && y < c.H {
		c.Px[y*c.W+x] = v
	}
}

// Mark draws the disc and the couple, size pixels across, with its top-left
// corner at x, y. Each pixel takes the colour that covers most of it, from
// 6×6 samples, so edges stay hard. The heads are the smallest shapes, and
// sampling chips them into odd shapes, so they are set after: a round dot of
// a 4×4 rounded dot, and below 32 pixels, where a head is about two pixels
// across, a 2×2 dot, both snapped to the grid.
func (c *Canvas) Mark(x0, y0, size int, pose Pose) {
	const ss = 6
	scale := 501 / float64(size)
	small := size < 32
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			var votes [4]int
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					votes[markAt(pt{(float64(x) + (float64(sx)+0.5)/ss) * scale, (float64(y) + (float64(sy)+0.5)/ss) * scale}, pose, false)]++
				}
			}
			c.set(x0+x, y0+y, partColour[winner(votes, ss*ss)])
		}
	}
	for _, h := range heads {
		centre := posed(h.c, pose)
		cx, cy := centre.x/scale, centre.y/scale
		if small {
			hx, hy := int(math.Round(cx-1)), int(math.Round(cy-1))
			for dy := 0; dy < 2; dy++ {
				for dx := 0; dx < 2; dx++ {
					c.set(x0+hx+dx, y0+hy+dy, Light)
				}
			}
			continue
		}
		// A 4×4 dot with its corners off: the roundest a head gets in pixels,
		// and the same shape for both heads.
		hx, hy := int(math.Round(cx-2)), int(math.Round(cy-2))
		for dy := 0; dy < 4; dy++ {
			for dx := 0; dx < 4; dx++ {
				if (dx == 0 || dx == 3) && (dy == 0 || dy == 3) {
					continue
				}
				c.set(x0+hx+dx, y0+hy+dy, Light)
			}
		}
	}
}

// posed is where a point of the logo ends up with the couple in a pose.
func posed(p pt, pose Pose) pt {
	turn := pose.Turn
	if math.Abs(turn) < 0.2 {
		turn = math.Copysign(0.2, turn)
	}
	dx, dy := (p.x-axis.x)*turn, p.y-axis.y
	cs, sn := math.Cos(pose.Lean), math.Sin(pose.Lean)
	return pt{axis.x + dx*cs - dy*sn, axis.y + dx*sn + dy*cs}
}

func winner(votes [4]int, total int) part {
	if votes[partLight]*100 >= total*40 {
		return partLight
	}
	best := partPanel
	for p := partDisc; p <= partLight; p++ {
		if votes[p] > votes[best] {
			best = p
		}
	}
	return best
}

//go:embed wordmark.png
var wordmarkPNG []byte

var (
	wordmarkOnce sync.Once
	wordmarkImg  *image.Gray
)

func wordmarkMask() *image.Gray {
	wordmarkOnce.Do(func() {
		src, err := png.Decode(bytes.NewReader(wordmarkPNG))
		if err != nil {
			panic("brand: wordmark.png: " + err.Error())
		}
		b := src.Bounds()
		g := image.NewGray(b)
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				g.Set(x, y, src.At(x, y))
			}
		}
		wordmarkImg = g
	})
	return wordmarkImg
}

// The size of wordmark.png. A test holds the file to it, so the lockups can
// be measured without decoding the image.
const wordmarkW, wordmarkH = 2620, 497

// WordmarkHeight is how many pixels tall the wordmark is at a given width.
func WordmarkHeight(width int) int {
	return int(math.Ceil(float64(wordmarkH) * float64(width) / float64(wordmarkW)))
}

var (
	coverageMu sync.Mutex
	coverage   = map[int][]float64{}
)

// wordmarkCoverage is how much of each pixel the letters cover, at a width.
// Sampling the mask is the slow part of a frame, so it happens once a width.
func wordmarkCoverage(width int) []float64 {
	coverageMu.Lock()
	defer coverageMu.Unlock()
	if cov, ok := coverage[width]; ok {
		return cov
	}
	m := wordmarkMask()
	b := m.Bounds()
	d := float64(b.Dx()) / float64(width)
	h := WordmarkHeight(width)
	cov := make([]float64, width*h)
	for y := 0; y < h; y++ {
		for x := 0; x < width; x++ {
			sum, n := 0.0, 0.0
			for sy := int(float64(y) * d); sy < int(math.Ceil(float64(y+1)*d)) && sy < b.Dy(); sy++ {
				for sx := int(float64(x) * d); sx < int(math.Ceil(float64(x+1)*d)) && sx < b.Dx(); sx++ {
					sum += float64(m.GrayAt(sx, sy).Y) / 255
					n++
				}
			}
			if n > 0 {
				cov[y*width+x] = sum / n
			}
		}
	}
	coverage[width] = cov
	return cov
}

// Wordmark draws "TangoTube" width pixels wide at x, y. Only the first
// reveal (0–1) of it, from the left, is drawn: the wordmark walking in.
func (c *Canvas) Wordmark(x0, y0, width int, reveal float64) {
	cov := wordmarkCoverage(width)
	h := WordmarkHeight(width)
	shown := int(math.Round(reveal * float64(width)))
	for y := 0; y < h; y++ {
		for x := 0; x < shown; x++ {
			if cov[y*width+x] >= 0.35 {
				c.set(x0+x, y0+y, Light)
			}
		}
	}
}
