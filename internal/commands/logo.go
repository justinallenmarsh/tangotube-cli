package commands

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/justinallenmarsh/tangotube-cli/internal/brand"
	"github.com/justinallenmarsh/tangotube-cli/internal/output"
)

const tagline = "tango videos, for you and your agent"

// logoDepth is how many colours the logo may use: whatever the text uses, and
// the shapes alone under NO_COLOR.
func logoDepth(s output.Style) brand.Depth {
	switch {
	case s.Enabled && s.TrueColor:
		return brand.TrueColor
	case s.Enabled:
		return brand.Color256
	default:
		return brand.Mono
	}
}

// fits reports whether a lockup cols wide and rows tall fits the terminal.
// An unknown size (0) counts as fitting: a terminal that will not say how
// wide it is is usually a wide one.
func (a *App) fits(cols, rows int) bool {
	return (a.Width == 0 || a.Width >= cols+2) && (a.Height == 0 || a.Height >= rows)
}

// canAnimate is true when a person is watching a real terminal and has not
// asked for stillness. TT_NO_ANIMATION=1 turns the motion off for good.
func (a *App) canAnimate(p *output.Printer) bool {
	return a.Interactive() && a.StdinTTY && p.Style.Enabled &&
		a.Env("TT_NO_ANIMATION") == "" && a.Env("CI") == "" && a.Env("TERM") != "dumb"
}

// masthead is what tt with nothing after it shows: the poster when there is
// room, the banner when there is less. When someone is watching, the couple
// turn one giro and the wordmark walks in; it takes a little over a second.
func (a *App) masthead(p *output.Printer) {
	if g := a.graphics(); g != noGraphics && a.fits(posterCols, a.posterRows()+8) {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		a.showPicture(ctx, g, a.canAnimate(p))
		fmt.Fprintln(a.Out, p.Style.Text(tagline))
		fmt.Fprintln(a.Out, p.Style.Dim("https://tangotube.tv"))
		return
	}
	if !a.fits(brand.PosterCols, brand.PosterRows()+8) {
		a.banner(p)
		return
	}
	depth := logoDepth(p.Style)
	if a.canAnimate(p) {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		a.play(ctx, intro(), depth, brand.PosterRows())
	} else {
		fmt.Fprint(a.Out, brand.Poster(brand.Still, 1).Render(depth, 0))
	}
	fmt.Fprintln(a.Out)
	fmt.Fprintln(a.Out, p.Style.Text(tagline))
	fmt.Fprintln(a.Out, p.Style.Dim("https://tangotube.tv"))
}

// banner is the small lockup: the couple in their disc, and beside them the
// name, the tagline, and the address, set in words. Where even the disc will
// not fit, the words stand alone.
func (a *App) banner(p *output.Printer) {
	words := []string{
		"",
		"",
		"",
		"",
		p.Style.Bold("TangoTube"),
		p.Style.Text(tagline),
		p.Style.Dim("https://tangotube.tv"),
	}
	if !a.fits(brand.BannerCols+3+len(tagline), brand.BannerRows()) {
		for _, w := range words[4:] {
			fmt.Fprintln(a.Out, w)
		}
		return
	}
	if g := a.graphics(); g != noGraphics {
		a.showBannerPicture(g, []string{"", words[4], words[5], words[6]}, 5)
		return
	}
	rows := strings.Split(strings.TrimRight(brand.Banner().Render(logoDepth(p.Style), 0), "\n"), "\n")
	for i, row := range rows {
		if pad := brand.BannerCols - output.DisplayWidth(output.StripEscapes(row)); pad > 0 {
			row += strings.Repeat(" ", pad)
		}
		if i < len(words) && words[i] != "" {
			row += "   " + words[i]
		}
		fmt.Fprintln(a.Out, strings.TrimRight(row, " "))
	}
}

// frame is one picture of the poster and how long it stays up.
type frame struct {
	pose   brand.Pose
	reveal float64
	hold   time.Duration
}

// intro is the couple turning one giro in the empty poster, then the wordmark
// walking in under them.
func intro() []frame {
	var fs []frame
	fs = append(fs, frame{brand.Still, 0, 120 * time.Millisecond})
	for _, pose := range brand.Giro(18) {
		fs = append(fs, frame{pose, 0, 40 * time.Millisecond})
	}
	for i := 1; i <= 10; i++ {
		fs = append(fs, frame{brand.Still, float64(i) / 10, 30 * time.Millisecond})
	}
	return fs
}

// phrase is one phrase of the dance: rock, turn, rock, turn.
func phrase() []frame {
	var fs []frame
	rock := func() {
		for _, pose := range brand.Cadencia(16, 0.1) {
			fs = append(fs, frame{pose, 1, 45 * time.Millisecond})
		}
	}
	turn := func() {
		for _, pose := range brand.Giro(20) {
			fs = append(fs, frame{pose, 1, 40 * time.Millisecond})
		}
	}
	rock()
	turn()
	rock()
	turn()
	return fs
}

// play draws the frames over one another in place. The cursor hides while it
// runs and comes back however it ends, Ctrl-C included.
func (a *App) play(ctx context.Context, frames []frame, depth brand.Depth, rows int) bool {
	fmt.Fprint(a.Out, "\x1b[?25l")
	defer fmt.Fprint(a.Out, "\x1b[?25h")
	for i, f := range frames {
		if i > 0 {
			fmt.Fprintf(a.Out, "\x1b[%dA", rows)
		}
		fmt.Fprint(a.Out, brand.Poster(f.pose, f.reveal).Render(depth, 0))
		select {
		case <-ctx.Done():
			return false
		case <-time.After(f.hold):
		}
	}
	return true
}

func newDanceCmd(a *App) *cobra.Command {
	return &cobra.Command{
		Use:    "dance",
		Short:  "Watch the couple dance one tango. Ctrl-C says gracias.",
		Hidden: true,
		Long: `Watch the couple from the TangoTube logo dance one tango, about three
minutes, in the terminal. In a milonga, "gracias" means the tanda is over for
you; here Ctrl-C says it.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p := a.Printer()
			if !a.Interactive() || !p.Style.Enabled {
				return Usage("The couple only dance at a terminal with colour.", "tt")
			}
			if !a.fits(brand.PosterCols, brand.PosterRows()+3) {
				return Usage(fmt.Sprintf("The couple need a terminal %d columns by %d rows to dance.", brand.PosterCols+2, brand.PosterRows()+3), "tt")
			}
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
			defer stop()
			ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
			defer cancel()
			fmt.Fprintln(a.Out)
			if g := a.graphics(); g != noGraphics && a.fits(posterCols, a.posterRows()+3) {
				w, h := a.posterPixels()
				for a.playPictures(ctx, g, phrase(), w, h) {
					// Back to the top of the picture for the next phrase.
					fmt.Fprint(a.Out, "\x1b8")
				}
			} else {
				depth := logoDepth(p.Style)
				for a.play(ctx, phrase(), depth, brand.PosterRows()) {
					fmt.Fprintf(a.Out, "\x1b[%dA", brand.PosterRows())
				}
			}
			fmt.Fprintln(a.Out)
			fmt.Fprintln(a.Out, p.Style.Text("Gracias."))
			return nil
		},
	}
}
