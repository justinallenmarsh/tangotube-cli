package commands

// winsize is the terminal window as the terminal reports it: its size in
// cells and, where the terminal says, in pixels.
type winsize struct {
	cols, rows     int
	xpixel, ypixel int
}
