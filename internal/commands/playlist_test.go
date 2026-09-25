package commands

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/justinallenmarsh/tangotube-cli/internal/output"
)

// A playlist names who made it, "you" for your own, and no "by" line at all
// when its maker gave TangoTube no name.
func TestPlaylistHeadingSaysWhoMadeIt(t *testing.T) {
	for _, tc := range []struct {
		raw, want string
	}{
		{`{"title":"Di Sarli for Sunday","owner":"Justin Marsh"}`, "Justin Marsh"},
		{`{"title":"Di Sarli for Sunday","owner":"Justin Marsh","mine":true}`, "you"},
		{`{"title":"Di Sarli for Sunday","owner":null}`, ""},
	} {
		var out bytes.Buffer
		renderPlaylist(&output.Printer{Out: &out, Width: 100}, json.RawMessage(tc.raw), 0, true)
		by := ""
		for _, line := range strings.Split(out.String(), "\n") {
			if f := strings.Fields(line); len(f) > 1 && f[0] == "by" {
				by = strings.Join(f[1:], " ")
			}
		}
		if by != tc.want {
			t.Errorf("%s: by %q, want %q\n%s", tc.raw, by, tc.want, out.String())
		}
	}
}
