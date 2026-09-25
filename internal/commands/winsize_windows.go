//go:build windows

package commands

import "os"

// windowPixels reports nothing on Windows: the console knows its size in
// cells but not in pixels, so cells get the typical shape.
func windowPixels(*os.File) (winsize, bool) {
	return winsize{}, false
}
