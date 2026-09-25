package commands

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"io"
	"math"
	"os"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/justinallenmarsh/tangotube-cli/internal/brand"
)

// Where the terminal can show real images, the logo is an image: smooth edges
// at the screen's own resolution instead of half-block pixels. Two protocols
// cover the terminals people run tt in:
//
//   - kitty's graphics protocol: kitty, Ghostty, WezTerm, Konsole
//   - iTerm2's inline images: iTerm2, WezTerm
//
// Where someone is watching, the couple turn one giro first, frame by frame.
//
// Everything else (Terminal.app, tmux, a plain xterm) gets the half-block
// logo. TT_IMAGES=kitty, iterm or none overrides the guess.
type graphics int

const (
	noGraphics graphics = iota
	kittyGraphics
	itermGraphics
)

func (a *App) graphics() graphics {
	switch strings.ToLower(a.Env("TT_IMAGES")) {
	case "kitty":
		return kittyGraphics
	case "iterm", "iterm2":
		return itermGraphics
	case "none", "off", "0":
		return noGraphics
	}
	if !a.Interactive() || a.Env("TMUX") != "" || strings.HasPrefix(a.Env("TERM"), "screen") {
		// tmux keeps images to itself unless told to pass them through, and
		// a stray escape is worse than a blocky logo.
		return noGraphics
	}
	term, program := a.Env("TERM"), a.Env("TERM_PROGRAM")
	switch {
	case term == "xterm-kitty" || a.Env("KITTY_WINDOW_ID") != "":
		return kittyGraphics
	case program == "ghostty" || term == "xterm-ghostty":
		return kittyGraphics
	case a.Env("KONSOLE_VERSION") != "":
		return kittyGraphics
	case program == "iTerm.app" || program == "WezTerm":
		return itermGraphics
	}
	return noGraphics
}

// cellPixels is the size of one terminal cell in pixels, from the window size
// the terminal reports. Terminals that report nothing get a typical cell.
func (a *App) cellPixels() (float64, float64) {
	if a.Cell != nil {
		return a.Cell()
	}
	ws, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ)
	if err != nil || ws.Xpixel == 0 || ws.Ypixel == 0 || ws.Col == 0 || ws.Row == 0 {
		return 10, 21
	}
	return float64(ws.Xpixel) / float64(ws.Col), float64(ws.Ypixel) / float64(ws.Row)
}

// The terminal is told only how wide (or tall) an image is, in cells, and
// keeps the image's own proportions for the other side. Cells are not the
// same shape in every font, and a disc stretched to a guessed cell shape comes
// out an oval.

// pictureWidth is how many pixels wide to draw an image cols cells wide:
// twice the reported size, because a Mac reports points and draws pixels,
// and never more than 1600.
func (a *App) pictureWidth(cols int) int {
	cw, _ := a.cellPixels()
	return int(math.Min(1600, math.Round(float64(cols)*cw*2)))
}

// showBannerPicture draws the couple as an image, rows tall, with the words
// set beside it, and leaves the cursor under it.
func (a *App) showBannerPicture(g graphics, words []string, rows int) {
	cw, ch := a.cellPixels()
	// The couple are round, rows tall; the words start a little past where
	// the disc ends even if the font's cells are narrower than reported.
	cols := int(math.Ceil(float64(rows)*ch/cw)) + 2
	px := int(math.Min(800, math.Round(float64(rows)*ch*2)))
	img := brand.MarkPicture(px, brand.Still)
	a.reserve(rows)
	a.placeImage(g, img, size{rows: rows}, 2, true)
	// Back to the image's top-left, wherever the terminal left the cursor.
	fmt.Fprint(a.Out, "\x1b8")
	for i := 0; i < rows; i++ {
		if i < len(words) && words[i] != "" {
			fmt.Fprintf(a.Out, "\x1b[%dG%s", cols+4, words[i])
		}
		fmt.Fprintln(a.Out)
	}
}

// posterCols is the image poster's width in cells, the same footprint as the
// half-block poster. posterAspect is its width over its height.
const (
	posterCols   = 58
	posterAspect = 1.25
)

// posterPixels is the poster image's size in pixels.
func (a *App) posterPixels() (int, int) {
	w := a.pictureWidth(posterCols)
	return w, int(math.Round(float64(w) / posterAspect))
}

// posterRows is about how many rows the poster takes, for deciding whether it
// fits. The terminal decides the real number.
func (a *App) posterRows() int {
	cw, ch := a.cellPixels()
	return int(math.Ceil(float64(posterCols) * cw / posterAspect / ch))
}

// showPicture draws the poster as an image, animated when someone is
// watching. It reports false if it drew nothing, so the caller can fall back.
func (a *App) showPicture(ctx context.Context, g graphics, animate bool) {
	w, h := a.posterPixels()
	if !animate {
		a.placeImage(g, brand.Picture(w, h, brand.Still, 1), size{cols: posterCols}, 1, false)
		return
	}
	a.playPictures(ctx, g, intro(), w, h)
}

// playPictures draws frames in place, then the logo at rest. Frames in motion
// are drawn at half size, which the eye cannot tell at that speed and which
// keeps each one inside its 40 milliseconds; the resting frame is full size.
// Each frame goes to the same spot: kitty replaces the image with the same
// id, iTerm2 draws over it. It reports whether the frames ran to the end.
func (a *App) playPictures(ctx context.Context, g graphics, frames []frame, w, h int) bool {
	a.reserve(a.posterRows())
	next := make(chan []byte, 2)
	go func() {
		defer close(next)
		for _, f := range frames {
			var buf bytes.Buffer
			_ = fastPNG.Encode(&buf, brand.Picture(w/2, h/2, f.pose, f.reveal))
			select {
			case next <- buf.Bytes():
			case <-ctx.Done():
				return
			}
		}
	}()
	shown, finished := 0, true
	for data := range next {
		f := frames[shown]
		shown++
		fmt.Fprint(a.Out, "\x1b8")
		a.sendImage(g, data, size{cols: posterCols}, 1, true)
		select {
		case <-ctx.Done():
			finished = false
		case <-time.After(f.hold):
		}
		if !finished {
			break
		}
	}
	fmt.Fprint(a.Out, "\x1b8")
	a.placeImage(g, brand.Picture(w, h, brand.Still, 1), size{cols: posterCols}, 1, false)
	return finished && shown == len(frames)
}

// size is an image's size in cells. Only one of the two is set; the terminal
// works out the other from the image.
type size struct{ cols, rows int }

// placeImage writes img into the terminal at the cursor. With hold, the
// cursor stays at the image's top-left (kitty only); otherwise it ends on the
// line under the image.
func (a *App) placeImage(g graphics, img image.Image, sz size, id int, hold bool) {
	var buf bytes.Buffer
	_ = fastPNG.Encode(&buf, img)
	a.sendImage(g, buf.Bytes(), sz, id, hold)
}

// sendImage writes an encoded PNG. With hold the cursor is left for the
// caller to put back; kitty keeps it still, iTerm2 moves it and the caller
// restores it.
func (a *App) sendImage(g graphics, data []byte, sz size, id int, hold bool) {
	switch g {
	case kittyGraphics:
		writeKitty(a.Out, data, sz, id, hold)
	case itermGraphics:
		writeITerm(a.Out, data, sz)
	}
	if !hold {
		fmt.Fprintln(a.Out)
	}
}

// fastPNG trades a little size for speed: a frame of the intro has to be
// encoded in the time it is on screen.
var fastPNG = png.Encoder{CompressionLevel: png.BestSpeed}

// reserve makes room for an image rows tall under the cursor and remembers
// where it starts. Printing the room first means a terminal already at its
// bottom line scrolls now, not halfway through the image, so the remembered
// spot stays true.
func (a *App) reserve(rows int) {
	fmt.Fprint(a.Out, strings.Repeat("\n", rows))
	fmt.Fprintf(a.Out, "\x1b[%dA\x1b7", rows)
}

// writeKitty sends a PNG with kitty's graphics protocol: base64 in chunks of
// at most 4096 bytes, the first carrying the keys. a=T transmits and places,
// f=100 is PNG, c or r sizes it in cells (kitty keeps the aspect for the
// other), C=1 holds the cursor still, q=2 keeps the terminal from answering.
func writeKitty(w io.Writer, data []byte, sz size, id int, hold bool) {
	keys := fmt.Sprintf("a=T,f=100,i=%d,p=1,q=2", id)
	if sz.cols > 0 {
		keys += fmt.Sprintf(",c=%d", sz.cols)
	}
	if sz.rows > 0 {
		keys += fmt.Sprintf(",r=%d", sz.rows)
	}
	if hold {
		keys += ",C=1"
	}
	enc := base64.StdEncoding.EncodeToString(data)
	for i := 0; i < len(enc); i += 4096 {
		end := min(i+4096, len(enc))
		more := 0
		if end < len(enc) {
			more = 1
		}
		if i == 0 {
			fmt.Fprintf(w, "\x1b_G%s,m=%d;%s\x1b\\", keys, more, enc[i:end])
		} else {
			fmt.Fprintf(w, "\x1b_Gm=%d;%s\x1b\\", more, enc[i:end])
		}
	}
}

// writeITerm sends an image with iTerm2's inline image protocol, sized in
// cells one way and left to keep its proportions the other.
func writeITerm(w io.Writer, data []byte, sz size) {
	dims := ""
	if sz.cols > 0 {
		dims += fmt.Sprintf(";width=%d", sz.cols)
	}
	if sz.rows > 0 {
		dims += fmt.Sprintf(";height=%d", sz.rows)
	}
	fmt.Fprintf(w, "\x1b]1337;File=inline=1;size=%d%s;preserveAspectRatio=1:%s\a",
		len(data), dims, base64.StdEncoding.EncodeToString(data))
}
