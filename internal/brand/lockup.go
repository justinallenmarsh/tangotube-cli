package brand

import "math"

// Two lockups, both on their own panel.
//
// The poster stacks the couple over the wordmark: 78 columns by 30 rows, for
// tt with nothing after it; it fits an 80-column terminal. The banner is the
// couple alone, 24 columns by 12 rows, for places that should not take over the screen, like tt doctor; the
// name goes beside it in words, because at that size the wordmark's hairlines
// are gone.
const (
	PosterCols = 78
	posterMark = 40
	posterWord = 76

	BannerCols = 24
)

// PosterRows and BannerRows are the terminal rows each lockup takes. They are
// arithmetic, so asking costs nothing; drawing is what takes time.
func PosterRows() int { return (1 + posterMark + 2 + WordmarkHeight(posterWord) + 1 + 1) / 2 }
func BannerRows() int { return (BannerCols + 1) / 2 }

// Poster is the stacked lockup, with the couple in the given pose and the
// wordmark revealed from the left up to reveal (0–1).
func Poster(pose Pose, reveal float64) *Canvas { return poster(pose, reveal) }

func poster(pose Pose, reveal float64) *Canvas {
	wordH := WordmarkHeight(posterWord)
	c := NewCanvas(PosterCols, 1+posterMark+2+wordH+1)
	c.Mark((PosterCols-posterMark)/2, 1, posterMark, pose)
	c.Wordmark((PosterCols-posterWord)/2, 1+posterMark+2, posterWord, reveal)
	return c
}

// Banner is the couple in their disc, small.
func Banner() *Canvas {
	c := NewCanvas(BannerCols, BannerCols)
	c.Mark(0, 0, BannerCols, Still)
	return c
}

// Giro is one turn of the couple, frames long, starting and ending as the
// logo. The turn eases: slow out of the embrace, quick through the back, slow
// into the finish.
func Giro(frames int) []Pose {
	poses := make([]Pose, frames)
	for i := range poses {
		t := float64(i) / float64(frames-1)
		eased := (1 - math.Cos(math.Pi*t)) / 2
		poses[i] = Pose{Turn: math.Cos(2 * math.Pi * eased)}
	}
	return poses
}

// Cadencia is the couple rocking in place over one beat, frames long.
func Cadencia(frames int, lean float64) []Pose {
	poses := make([]Pose, frames)
	for i := range poses {
		poses[i] = Pose{Turn: 1, Lean: lean * math.Sin(2*math.Pi*float64(i)/float64(frames))}
	}
	return poses
}
