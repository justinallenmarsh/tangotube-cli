//go:build windows

package commands

import (
	"os"

	"golang.org/x/sys/windows"
)

// enableEscapes turns on the console's escape-sequence processing, which
// Windows Terminal has on already and the old console has off. It reports
// false when the console cannot do it, and tt then writes plain text.
func enableEscapes(f *os.File) bool {
	h := windows.Handle(f.Fd())
	var mode uint32
	if err := windows.GetConsoleMode(h, &mode); err != nil {
		return false
	}
	if mode&windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING != 0 {
		return true
	}
	return windows.SetConsoleMode(h, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING) == nil
}
