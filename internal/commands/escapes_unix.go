//go:build !windows

package commands

import "os"

// enableEscapes is a no-op where every terminal reads escape sequences.
func enableEscapes(*os.File) bool { return true }
