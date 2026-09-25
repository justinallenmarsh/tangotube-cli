package brand

import (
	"bytes"
	"image/png"
	"strings"
	"testing"
)

func TestWordmarkPNGMatchesItsMeasurements(t *testing.T) {
	cfg, err := png.DecodeConfig(bytes.NewReader(wordmarkPNG))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Width != wordmarkW || cfg.Height != wordmarkH {
		t.Fatalf("wordmark.png is %d×%d; wordmarkW, wordmarkH say %d×%d", cfg.Width, cfg.Height, wordmarkW, wordmarkH)
	}
}

func TestLockupsAreTheSizeTheySay(t *testing.T) {
	for name, tc := range map[string]struct {
		c          *Canvas
		cols, rows int
	}{
		"poster": {Poster(Still, 1), PosterCols, PosterRows()},
		"banner": {Banner(), BannerCols, BannerRows()},
	} {
		out := tc.c.Render(TrueColor, 0)
		lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
		if len(lines) != tc.rows {
			t.Errorf("%s: %d rows, says %d", name, len(lines), tc.rows)
		}
		if tc.c.W != tc.cols || tc.cols > 78 {
			t.Errorf("%s: %d columns, says %d (and must fit an 80-column terminal)", name, tc.c.W, tc.cols)
		}
	}
}

func TestMonoIsShapesWithoutEscapes(t *testing.T) {
	out := Poster(Still, 1).Render(Mono, 0)
	if strings.Contains(out, "\x1b") {
		t.Fatal("NO_COLOR output carries an escape")
	}
	if !strings.ContainsAny(out, "▀▄█") {
		t.Fatal("no shapes drawn")
	}
}

func TestColourDepths(t *testing.T) {
	c := Banner()
	if out := c.Render(Color256, 0); strings.Contains(out, ";2;") || !strings.Contains(out, "38;5;") {
		t.Error("256-colour output should use only 38;5 and 48;5")
	}
	if out := c.Render(TrueColor, 0); !strings.Contains(out, "38;2;") {
		t.Error("truecolor output should use 38;2")
	}
}

func TestGiroStartsAndEndsAsTheLogo(t *testing.T) {
	poses := Giro(18)
	if poses[0].Turn != 1 || poses[len(poses)-1].Turn < 0.999 {
		t.Fatalf("a giro starts and ends facing us: %v … %v", poses[0], poses[len(poses)-1])
	}
	back := false
	for _, p := range poses {
		if p.Turn < -0.9 {
			back = true
		}
	}
	if !back {
		t.Fatal("the couple never turned their back")
	}
	still := Poster(Still, 1)
	end := Poster(poses[len(poses)-1], 1)
	if still.Render(TrueColor, 0) != end.Render(TrueColor, 0) {
		t.Fatal("the last frame of a giro should be the still logo, or the picture jumps")
	}
}

func TestTheCoupleIsInTheDisc(t *testing.T) {
	c := NewCanvas(40, 40)
	c.Mark(0, 0, 40, Still)
	centre := c.At(20, 23)
	corner := c.At(0, 0)
	if centre.R < 150 {
		t.Errorf("the embrace at the centre should be light, got %v", centre)
	}
	if corner != Panel {
		t.Errorf("the corner outside the disc should be the panel, got %v", corner)
	}
}

// The logo is flat colour. A blend anywhere is the blur this package exists
// to avoid.
func TestEveryPixelIsALogoColour(t *testing.T) {
	allowed := map[RGB]bool{Panel: true, Disc: true, Shade: true, Light: true}
	for name, c := range map[string]*Canvas{"poster": Poster(Still, 1), "banner": Banner(), "mid-giro": Poster(Giro(18)[5], 1)} {
		for i, v := range c.Px {
			if !allowed[v] {
				t.Fatalf("%s: pixel %d is %v, a blend", name, i, v)
			}
		}
	}
}

func TestSmallMarkHasTwoHeads(t *testing.T) {
	c := Banner()
	lit := 0
	for y := 0; y < c.H/3; y++ {
		for x := 0; x < c.W; x++ {
			if c.At(x, y) == Light {
				lit++
			}
		}
	}
	if lit != 8 {
		t.Fatalf("two 2×2 heads in the top third, got %d light pixels", lit)
	}
}
