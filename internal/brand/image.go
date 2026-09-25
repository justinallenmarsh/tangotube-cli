package brand

import (
	"image"
	"math"
	"runtime"
	"sync"
)

// Picture is the logo as a real image, for terminals that can show one (see
// the graphics protocols in internal/commands). Half blocks give a terminal
// about 80×60 pixels to draw with, and at that size the logo can only be
// blocky or blurry. An image has the screen's own pixels: the couple's edges
// are smooth and the wordmark's hairlines survive.
//
// The layout matches the poster: the disc over the wordmark on a dark panel.
// w and h are the image's size in pixels; pose and reveal work as they do on
// the poster.
func Picture(w, h int, pose Pose, reveal float64) *image.RGBA {
	lay := pictureLayers(w, h)
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	if reveal >= 1 {
		copy(img.Pix, lay.full.Pix)
	} else {
		copy(img.Pix, lay.blank.Pix)
		lay.paintWordmark(img, reveal)
	}
	drawMarkAA(img, lay.discX, lay.discY, lay.disc, pose)
	return img
}

// layers is what does not move between frames, kept per size: the empty
// panel, the panel with the whole wordmark on it, and the wordmark's coverage.
// Only the couple are drawn fresh each frame, and they are a fraction of it.
type layers struct {
	blank, full        *image.RGBA
	discX, discY, disc float64
	wordX, wordY       int
	wordW, wordH       int
	cover              []float64
}

var (
	layersMu    sync.Mutex
	layersCache = map[[2]int]*layers{}
)

func pictureLayers(w, h int) *layers {
	layersMu.Lock()
	defer layersMu.Unlock()
	if l, ok := layersCache[[2]int{w, h}]; ok {
		return l
	}
	pad := 0.06 * float64(h)
	gap := 0.05 * float64(h)
	wordW := math.Min(0.84*float64(w), 2.6*float64(h))
	wordH := wordW * wordmarkH / wordmarkW
	disc := float64(h) - 2*pad - gap - wordH
	l := &layers{
		discX: (float64(w) - disc) / 2, discY: pad, disc: disc,
		wordX: int(math.Round((float64(w) - wordW) / 2)), wordY: int(math.Round(pad + disc + gap)),
		wordW: int(math.Round(wordW)), wordH: int(math.Ceil(wordH)),
	}
	l.cover = wordmarkBox(l.wordW, l.wordH)
	l.blank = image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(l.blank.Pix); i += 4 {
		l.blank.Pix[i], l.blank.Pix[i+1], l.blank.Pix[i+2], l.blank.Pix[i+3] = uint8(Panel.R), uint8(Panel.G), uint8(Panel.B), 255
	}
	l.full = image.NewRGBA(l.blank.Rect)
	copy(l.full.Pix, l.blank.Pix)
	l.paintWordmark(l.full, 1)
	layersCache[[2]int{w, h}] = l
	return l
}

// paintWordmark lays the wordmark's first reveal (0–1), from the left, onto img.
func (l *layers) paintWordmark(img *image.RGBA, reveal float64) {
	shown := int(math.Round(reveal * float64(l.wordW)))
	for y := 0; y < l.wordH; y++ {
		for x := 0; x < shown; x++ {
			if a := l.cover[y*l.wordW+x]; a > 0 {
				blend(img, l.wordX+x, l.wordY+y, RGB{Panel.R + (Light.R-Panel.R)*a, Panel.G + (Light.G-Panel.G)*a, Panel.B + (Light.B-Panel.B)*a})
			}
		}
	}
}

// drawMarkAA paints the disc and the couple, size pixels across at (x0, y0),
// averaging 3×3 samples a pixel. Rows go to every core: a frame of the intro
// has to be ready in a few milliseconds.
func drawMarkAA(img *image.RGBA, x0, y0, size float64, pose Pose) {
	const ss = 3
	scale := 501 / size
	sampler := newPoseSampler(pose)
	top, bottom := int(math.Floor(y0)), int(math.Ceil(y0+size))
	left, right := int(math.Floor(x0)), int(math.Ceil(x0+size))

	rows := make(chan int, bottom-top)
	for y := top; y < bottom; y++ {
		rows <- y
	}
	close(rows)
	var wg sync.WaitGroup
	for n := 0; n < runtime.NumCPU(); n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for y := range rows {
				for x := left; x < right; x++ {
					var acc RGB
					for sy := 0; sy < ss; sy++ {
						for sx := 0; sx < ss; sx++ {
							p := pt{
								(float64(x) - x0 + (float64(sx)+0.5)/ss) * scale,
								(float64(y) - y0 + (float64(sy)+0.5)/ss) * scale,
							}
							v := partColour[sampler.at(p)]
							acc.R += v.R
							acc.G += v.G
							acc.B += v.B
						}
					}
					blend(img, x, y, RGB{acc.R / ss / ss, acc.G / ss / ss, acc.B / ss / ss})
				}
			}
		}()
	}
	wg.Wait()
}

// poseSampler is markAt with the pose's trigonometry done once, not once a
// sample.
type poseSampler struct {
	cos, sin, turn float64
}

func newPoseSampler(pose Pose) poseSampler {
	turn := pose.Turn
	if math.Abs(turn) < 0.2 {
		turn = math.Copysign(0.2, turn)
	}
	return poseSampler{math.Cos(-pose.Lean), math.Sin(-pose.Lean), turn}
}

func (s poseSampler) at(p pt) part {
	bg := partPanel
	if math.Hypot(p.x-250.5, p.y-250.5) <= 250 {
		bg = partDisc
	}
	dx, dy := p.x-axis.x, p.y-axis.y
	dx, dy = dx*s.cos-dy*s.sin, dx*s.sin+dy*s.cos
	q := pt{axis.x + dx/s.turn, axis.y + dy}
	for _, h := range heads {
		if math.Hypot(q.x-h.c.x, q.y-h.c.y) <= h.r {
			return partLight
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

// wordmarkBox is how much of each pixel of a w×h wordmark the letters cover,
// each the average of the 2620-pixel mask under it: a box filter, which is
// what keeps the thin strokes of a Bodoni as thin grey lines rather than gaps.
func wordmarkBox(w, h int) []float64 {
	m := wordmarkMask()
	b := m.Bounds()
	d := float64(b.Dx()) / float64(w)
	cover := make([]float64, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			sx0, sy0 := float64(x)*d, float64(y)*d
			sum, n := 0.0, 0.0
			for sy := int(sy0); sy < int(math.Min(float64(b.Dy()), math.Ceil(sy0+d))); sy++ {
				for sx := int(sx0); sx < int(math.Min(float64(b.Dx()), math.Ceil(sx0+d))); sx++ {
					sum += float64(m.GrayAt(sx, sy).Y) / 255
					n++
				}
			}
			if n > 0 {
				cover[y*w+x] = sum / n
			}
		}
	}
	return cover
}

func blend(img *image.RGBA, x, y int, v RGB) {
	if !(image.Point{x, y}).In(img.Rect) {
		return
	}
	i := img.PixOffset(x, y)
	img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = uint8(clamp(v.R)), uint8(clamp(v.G)), uint8(clamp(v.B)), 255
}

// MarkPicture is the couple in their disc alone, size pixels square, on the
// panel colour: the banner's image.
func MarkPicture(size int, pose Pose) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = uint8(Panel.R), uint8(Panel.G), uint8(Panel.B), 255
	}
	drawMarkAA(img, 0, 0, float64(size), pose)
	return img
}
