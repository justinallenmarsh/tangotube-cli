//go:build !windows

package commands

import (
	"os"

	"golang.org/x/sys/unix"
)

// windowPixels asks the terminal on f for its size, pixels included.
func windowPixels(f *os.File) (winsize, bool) {
	ws, err := unix.IoctlGetWinsize(int(f.Fd()), unix.TIOCGWINSZ)
	if err != nil {
		return winsize{}, false
	}
	return winsize{cols: int(ws.Col), rows: int(ws.Row), xpixel: int(ws.Xpixel), ypixel: int(ws.Ypixel)}, true
}
