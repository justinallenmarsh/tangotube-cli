package commands

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/justinallenmarsh/tangotube-cli/internal/output"
)

func TestDatesReadTheWayPeopleSayThem(t *testing.T) {
	cases := map[[2]string]string{
		{"2019-05-09", "2019-05-14"}: "9–14 May 2019",
		{"2019-04-30", "2019-05-02"}: "30 April – 2 May 2019",
		{"2019-12-30", "2020-01-02"}: "30 December 2019 – 2 January 2020",
		{"2019-05-09", ""}:           "9 May 2019",
		{"", ""}:                     "",
	}
	for in, want := range cases {
		if got := dateSpan(in[0], in[1]); got != want {
			t.Errorf("dateSpan(%v) = %q, want %q", in, got, want)
		}
	}
	if got := longDate("1940-10-08"); got != "8 October 1940" {
		t.Errorf("longDate = %q", got)
	}
	if got := longDate("1940"); got != "1940" {
		t.Errorf("a bare year stays a year, got %q", got)
	}
}

func TestCompactCount(t *testing.T) {
	cases := map[int64]string{812: "812", 9_999: "9,999", 23_486: "23K", 1_234_567: "1.2M", 3_000_000: "3M"}
	for n, want := range cases {
		if got := compactCount(n); got != want {
			t.Errorf("compactCount(%d) = %q, want %q", n, got, want)
		}
	}
}

const enrichedVideo = `{
  "id": "uGwRPRusbC0", "youtube_id": "uGwRPRusbC0", "title": "Lyon 2019",
  "watch_url": "https://tangotube.tv/watch?v=uGwRPRusbC0", "duration_s": 219,
  "dancers": [{"name": "Sebastián Achaval", "slug": "sebastian-achaval", "role": "leader",
               "titles": ["World Champion · Tango de Salón 2005"]}],
  "song": {"title": "Volver a soñar", "slug": "volver-a-sonar", "genre": "tango",
           "recorded_on": "1940-10-08", "singer": "Roberto Rufino", "composer": "Andrés Fraga",
           "lyricist": "Francisco García Jiménez", "has_lyrics": true,
           "links": {"tangotube": "https://tangotube.tv/songs/volver-a-sonar",
                     "spotify": "https://open.spotify.com/track/abc", "youtube_music": null,
                     "el_recodo": "https://www.el-recodo.com/music?T=volver"}},
  "event": {"title": "Lyon Tango Festival", "slug": "lyon", "city": "Lyon", "country": "France",
            "start_date": "2019-05-09", "end_date": "2019-05-14"},
  "channel": "Armelle tango", "uploaded_on": "2019-05-11", "youtube_views": 23486,
  "links": {"tangotube": "https://tangotube.tv/watch?v=uGwRPRusbC0",
            "youtube": "https://www.youtube.com/watch?v=uGwRPRusbC0",
            "channel": "https://www.youtube.com/channel/UCx"},
  "other_angles": [{"id": "0c8mDJPaM4c", "channel": "Another", "watch_url": "https://tangotube.tv/watch?v=0c8mDJPaM4c"}]
}`

func render(t *testing.T, links bool, fn func(*output.Printer, json.RawMessage), raw string) string {
	t.Helper()
	var out bytes.Buffer
	p := &output.Printer{Out: &out, Width: 100, Style: output.Style{Links: links}}
	fn(p, json.RawMessage(raw))
	return out.String()
}

func TestVideoShowTellsTheWholeStory(t *testing.T) {
	out := render(t, true, renderVideo, enrichedVideo)
	for _, want := range []string{
		"Sebastián Achaval: World Champion · Tango de Salón 2005",
		"sung by Roberto Rufino", "music Andrés Fraga", "words Francisco García Jiménez",
		"8 October 1940", "9–14 May 2019", "23K views on YouTube", "0c8mDJPaM4c",
	} {
		if !strings.Contains(output.StripEscapes(out), want) {
			t.Errorf("missing %q in:\n%s", want, output.StripEscapes(out))
		}
	}
	// Who, then music, then where, then the video, then where to listen.
	plain := output.StripEscapes(out)
	order := []string{"dancers", "titles", "song", "credits", "recorded", "event", "video", "angles", "listen", "watch", "youtube", "channel"}
	last := -1
	for _, label := range order {
		i := strings.Index(plain, "  "+label+" ")
		if i < last {
			t.Fatalf("%q is out of order in:\n%s", label, plain)
		}
		last = i
	}
	// With links on, the listen line names the places and links them.
	if !strings.Contains(out, "\x1b]8;;https://open.spotify.com/track/abc\x1b\\Spotify") {
		t.Error("Spotify is not a link")
	}
	if strings.Contains(plain, "YouTube Music") {
		t.Error("listed YouTube Music with no address for it")
	}
}

func TestVideoShowSpellsOutAddressesWithoutLinks(t *testing.T) {
	out := render(t, false, renderVideo, enrichedVideo)
	if strings.Contains(out, "\x1b]8;;") {
		t.Fatal("links off, yet an OSC 8 link was written")
	}
	for _, want := range []string{"spotify    https://open.spotify.com/track/abc", "el recodo  https://www.el-recodo.com/music?T=volver", "https://www.youtube.com/channel/UCx"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "listen") {
		t.Error("the listen line only makes sense when its names are links")
	}
}

func TestSongLyrics(t *testing.T) {
	withLyrics := `{"title": "Volver a soñar", "slug": "volver-a-sonar", "has_lyrics": true,
	  "links": {"tangotube": "https://tangotube.tv/songs/volver-a-sonar", "el_recodo": "https://www.el-recodo.com/music?T=volver"},
	  "lyrics": {"es": "No sé si fue mi mano\nO fue la tuya\n\nNo quiero ni saber", "en": "I don't know if it was my hand"}}`
	out := output.StripEscapes(render(t, false, renderSong, withLyrics))
	for _, want := range []string{"Letra", "No sé si fue mi mano", "O fue la tuya\n\n  No quiero ni saber", "In English", "I don't know if it was my hand"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.HasSuffix(out, "\n\n") {
		t.Error("a blank line after the last verse doubles up with the next: lines")
	}

	none := `{"title": "Nochero soy", "slug": "nochero-soy", "links": {}, "lyrics": {"es": null, "en": null}}`
	if out := render(t, false, renderSong, none); !strings.Contains(out, "No lyrics for this recording yet.") {
		t.Fatalf("got:\n%s", out)
	}

	offer := `{"title": "Volver a soñar", "slug": "volver-a-sonar", "has_lyrics": true, "links": {}}`
	if out := render(t, false, renderSong, offer); !strings.Contains(out, "tt song show volver-a-sonar --lyrics") {
		t.Fatalf("a song with lyrics should say how to read them:\n%s", out)
	}
}

func TestLyricLinesWrapWithinTheColumn(t *testing.T) {
	long := strings.Repeat("palabra ", 20)
	for _, line := range wrap(long, 40) {
		if output.DisplayWidth(line) > 40 {
			t.Fatalf("%q is wider than 40", line)
		}
	}
}

func TestEntityFacts(t *testing.T) {
	raw := `{"slug": "sebastian-achaval", "name": "Sebastián Achaval", "videos_count": 1855,
	  "titles": ["World Champion · Tango de Salón 2005"], "active": {"from": 2006, "to": 2026},
	  "top_orchestras": [{"name": "Juan D'Arienzo", "slug": "juan-d-arienzo", "count": 215}],
	  "links": {"tangotube": "https://tangotube.tv/dancers/sebastian-achaval"}}`
	out := output.StripEscapes(render(t, false, renderEntity, raw))
	for _, want := range []string{"titles     World Champion · Tango de Salón 2005", "filmed     2006–2026", "dances to  Juan D'Arienzo (215)", "page       https://tangotube.tv/dancers/sebastian-achaval"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}
