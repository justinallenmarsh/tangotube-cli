// Command brandgif draws the README's animated logo from the same pixels tt
// paints in a terminal: the couple turns one giro, then the wordmark walks in.
//
//	go run ./tools/brandgif -o docs/images/tt.gif
//
// With -ansi it prints the lockups to the terminal instead, for a quick look.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"log"
	"math"
	"os"
	"strings"

	"github.com/justinallenmarsh/tangotube-cli/internal/brand"
)

func main() {
	out := flag.String("o", "docs/images/tt.gif", "where to write the GIF")
	ansi := flag.Bool("ansi", false, "print the lockups as ANSI instead")
	strip := flag.Bool("strip", false, "print the couple through one giro as ANSI, side by side")
	sizes := flag.Bool("sizes", false, "print the stacked lockup at a few sizes as ANSI")
	flag.Parse()

	if *sizes {
		for _, sz := range [][2]int{{36, 76}, {40, 76}} {
			mark, word := sz[0], sz[1]
			wh := brand.WordmarkHeight(word)
			c := brand.NewCanvas(word+6, 2+mark+2+wh+2)
			c.Mark((word+6-mark)/2, 2, mark, brand.Still)
			c.Wordmark(3, 2+mark+2, word, 1)
			fmt.Printf("mark %d, wordmark %d: %d×%d cells\n", mark, word, c.W, c.H/2)
			fmt.Print(c.Render(brand.TrueColor, 0))
			fmt.Println()
		}
		for _, m := range []int{20, 24, 28} {
			c := brand.NewCanvas(m, m)
			c.Mark(0, 0, m, brand.Still)
			fmt.Printf("banner mark %d\n", m)
			fmt.Print(c.Render(brand.TrueColor, 0))
			fmt.Println()
		}
		return
	}

	if *strip {
		poses := brand.Giro(18)
		var cells []*brand.Canvas
		for i := 0; i < len(poses); i += 2 {
			c := brand.NewCanvas(28, 28)
			c.Mark(0, 0, 28, poses[i])
			cells = append(cells, c)
		}
		rows := make([]string, 14)
		for _, c := range cells {
			for i, line := range strings.Split(strings.TrimRight(c.Render(brand.TrueColor, 0), "\n"), "\n") {
				rows[i] += line + " "
			}
		}
		fmt.Println(strings.Join(rows, "\n"))
		return
	}

	if *ansi {
		fmt.Print(brand.Poster(brand.Still, 1).Render(brand.TrueColor, 2))
		fmt.Println()
		fmt.Print(brand.Banner().Render(brand.TrueColor, 2))
		fmt.Println()
		fmt.Print(brand.Poster(brand.Still, 1).Render(brand.Color256, 2))
		fmt.Println()
		fmt.Print(brand.Poster(brand.Still, 1).Render(brand.Mono, 2))
		return
	}

	// The README shows what a terminal with image support shows: the logo as
	// a real image, the couple turning one giro and the wordmark walking in.
	const w, h = 580, 464
	type still struct {
		pose   brand.Pose
		reveal float64
		cs     int
	}
	var frames []still
	frames = append(frames, still{brand.Still, 0, 60})
	for _, p := range brand.Giro(18) {
		frames = append(frames, still{p, 0, 5})
	}
	for i := 1; i <= 10; i++ {
		frames = append(frames, still{brand.Still, float64(i) / 10, 4})
	}
	for _, p := range brand.Cadencia(12, 0.09) {
		frames = append(frames, still{p, 1, 6})
	}
	frames = append(frames, still{brand.Still, 1, 300})

	g := &gif.GIF{LoopCount: 0}
	for _, f := range frames {
		g.Image = append(g.Image, greys(brand.Picture(w, h, f.pose, f.reveal)))
		g.Delay = append(g.Delay, f.cs)
	}
	f, err := os.Create(*out)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	if err := gif.EncodeAll(f, g); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("wrote %s: %d frames\n", *out, len(frames))
}

// greys maps an image onto 256 steps from the disc's near-black to ivory,
// the whole of the logo's range, by brightness.
func greys(src *image.RGBA) *image.Paletted {
	lo, hi := brand.Disc, brand.Light
	pal := make(color.Palette, 256)
	for i := range pal {
		t := float64(i) / 255
		pal[i] = color.RGBA{R: uint8(lo.R + (hi.R-lo.R)*t), G: uint8(lo.G + (hi.G-lo.G)*t), B: uint8(lo.B + (hi.B-lo.B)*t), A: 255}
	}
	dst := image.NewPaletted(src.Rect, pal)
	span := (hi.R - lo.R) + (hi.G - lo.G) + (hi.B - lo.B)
	for i := range dst.Pix {
		v := (float64(src.Pix[i*4]) - lo.R + float64(src.Pix[i*4+1]) - lo.G + float64(src.Pix[i*4+2]) - lo.B) / span
		dst.Pix[i] = uint8(math.Max(0, math.Min(255, math.Round(v*255))))
	}
	return dst
}
