package commands

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/justinallenmarsh/tangotube-cli/internal/api"
	"github.com/justinallenmarsh/tangotube-cli/internal/output"
)

// showGroup is "tt NOUN show ID", the shape every lookup shares.
func showGroup(a *App, noun, short, long, example, path string, render func(*output.Printer, json.RawMessage)) *cobra.Command {
	group := &cobra.Command{Use: noun, Short: short, Args: cobra.NoArgs}
	group.AddCommand(&cobra.Command{
		Use:     "show " + strings.ToUpper(argName(noun)),
		Short:   short,
		Long:    long,
		Example: example,
		Args:    exactArgs(1, "tt "+noun+" show "+strings.ToUpper(argName(noun))),
		RunE: func(cmd *cobra.Command, args []string) error {
			env, err := a.Client().Get(ctx(cmd), path+"/"+url.PathEscape(NormalizeID(args[0])), nil)
			return a.ShowDetail(env, err, render)
		},
	})
	return group
}

func argName(noun string) string {
	if noun == "video" {
		return "id"
	}
	return "slug"
}

// exactArgs is cobra.ExactArgs with a hint instead of a stack of usage.
func exactArgs(n int, hint string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) != n {
			return Usage(fmt.Sprintf("%s takes %d argument%s", cmd.CommandPath(), n, plural(n)), hint)
		}
		return nil
	}
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func newVideo(a *App) *cobra.Command {
	group := showGroup(a, "video", "Show one performance: dancers, song, orchestra, event, clips.",
		`Show one performance: who danced (and their world titles), the recording
(singer, composer, lyricist, the day it was recorded), the festival, other
cameras on the same dance, and where to listen: Spotify, YouTube Music,
El Recodo. At a terminal that supports it, every name there is a link.

--identity adds what TangoTube knows about it and how: each fact settled,
proposed (by audio, the YouTube panel, or a person), or not known yet.

The id is the YouTube id, the same one in https://tangotube.tv/watch?v=ID.
A pasted YouTube or tangotube.tv link works too.`,
		`  tt video show uGwRPRusbC0
  tt video show uGwRPRusbC0 --identity
  tt video show "https://www.youtube.com/watch?v=uGwRPRusbC0"
  tt video show uGwRPRusbC0 --json --jq '.dancers[].name'`,
		"/api/v1/videos", renderVideo)

	show := group.Commands()[0]
	var identity bool
	show.Flags().BoolVar(&identity, "identity", false, "Add what TangoTube knows about it and how, and what is waiting for a yes.")
	show.RunE = func(cmd *cobra.Command, args []string) error {
		id := NormalizeID(args[0])
		var q url.Values
		if identity {
			q = url.Values{"include": {"identity"}}
		}
		env, err := a.Client().Get(ctx(cmd), "/api/v1/videos/"+url.PathEscape(id), q)
		return a.ShowDetail(env, err, func(p *output.Printer, raw json.RawMessage) {
			renderVideo(p, raw)
			if identity {
				renderIdentity(p, id, decode[struct {
					Identity []identityRow `json:"identity"`
				}](raw).Identity)
			}
		})
	}
	return group
}

// renderVideo reads the way a person takes a performance in: who danced, to
// what music, where, the video itself, then where to listen and watch.
func renderVideo(p *output.Printer, raw json.RawMessage) {
	v := decode[api.Video](raw)
	fmt.Fprintln(p.Out, p.Style.Bold(v.Title))
	p.Field("id", p.Style.ID("", v.ID))

	// Who
	p.Field("dancers", v.DancerNames())
	var titles []string
	for _, d := range v.Dancers {
		if len(d.Titles) > 0 {
			titles = append(titles, d.Name+": "+strings.Join(d.Titles, "; "))
		}
	}
	for i, t := range titles {
		label := ""
		if i == 0 {
			label = "titles"
		}
		p.Para(label, t)
	}

	// Music
	p.Field("song", join(" · ", v.Song.Label(), v.Orchestra.Label(), v.YearText(), songGenre(v.Song)))
	if v.Song != nil {
		p.Items("credits", credits(v.Song))
		p.Field("recorded", longDate(v.Song.RecordedOn))
	}

	// Where
	if v.Event != nil {
		p.Field("event", join(" · ", join(", ", v.Event.Label(), v.Event.City, v.Event.Country), dateSpan(v.Event.StartDate, v.Event.EndDate)))
	}

	// The video
	var length string
	if v.DurationS != nil {
		length = api.Clock(*v.DurationS)
	}
	var views string
	if v.YoutubeViews != nil && *v.YoutubeViews > 0 {
		views = compactCount(*v.YoutubeViews) + " views on YouTube"
	}
	p.Field("video", join(" · ", length, uploaded(v.UploadedOn), views))
	if n := len(v.OtherAngles); n > 0 {
		ids := make([]string, 0, n)
		for _, a := range v.OtherAngles {
			ids = append(ids, p.Style.ID(a.WatchURL, a.ID))
		}
		// Other cameras on the same dance: same couple, same night, same song.
		p.Items("angles", ids)
	}
	session(p, v.Session)
	p.Field("steps", strings.Join(v.Techniques, " "))
	p.Field("style", strings.Join(v.Style, " "))
	if v.ClipsCount > 0 {
		p.Field("clips", fmt.Sprintf("%d practice clip%s", v.ClipsCount, plural(v.ClipsCount)))
	}

	// Listen and watch
	if v.Song != nil {
		listen(p, v.Song.Links)
	}
	watch := v.Links.TangoTube
	if watch == "" {
		watch = v.WatchURL
	}
	p.FieldLink("watch", watch, watch)
	yt := v.Links.YouTube
	if yt == "" && v.YoutubeID != "" {
		yt = "https://www.youtube.com/watch?v=" + v.YoutubeID
	}
	p.FieldLink("youtube", yt, yt)
	channelField(p, v.Channel, v.Links.Channel)
}

// session lists the couple's other dances that night, this one marked: a
// performance is rarely one song.
func session(p *output.Printer, s *api.Session) {
	if s == nil || s.Total < 2 {
		return
	}
	p.Field("session", join(" · ", fmt.Sprintf("dance %d of %d", s.Position, s.Total), s.Occasion))
	for i, d := range s.Dances {
		music := join(" · ", d.Song, d.Orchestra)
		if music == "" {
			music = "song not yet known"
		}
		n := fmt.Sprint(i + 1)
		if d.Current {
			p.Field("", p.Style.Bold(n)+"  "+p.Style.ID("", d.ID)+"  "+p.Style.Bold(music)+p.Style.Dim("  ← this one"))
			continue
		}
		p.Field("", p.Style.Dim(n)+"  "+p.Style.ID(d.WatchURL, d.ID)+"  "+music)
	}
}

// credits is "sung by X", "music Y", "words Z": the people a dancer asks about.
func credits(s *api.Song) []string {
	var out []string
	if s.Singer != "" {
		out = append(out, "sung by "+s.Singer)
	}
	if s.Composer != "" {
		out = append(out, "music "+s.Composer)
	}
	if s.Lyricist != "" {
		out = append(out, "words "+s.Lyricist)
	}
	return out
}

// listen offers the recording where people listen to music. With links on,
// one line of names to click; with links off, every address in full, so
// nothing is hidden behind a word that cannot be clicked.
func listen(p *output.Printer, l api.SongLinks) {
	places := []struct{ label, name, url string }{
		{"spotify", "Spotify", l.Spotify},
		{"yt music", "YouTube Music", l.YoutubeMusic},
		{"el recodo", "El Recodo", l.ElRecodo},
	}
	if p.Style.Links {
		var names []string
		for _, pl := range places {
			if pl.url != "" {
				names = append(names, p.Style.Link(pl.url, pl.name))
			}
		}
		p.Items("listen", names)
		return
	}
	for _, pl := range places {
		p.Field(pl.label, pl.url)
	}
}

// channelField is the channel's name, a link to it on YouTube; the address
// follows it when links are off.
func channelField(p *output.Printer, name, url string) {
	switch {
	case name == "":
		p.Field("channel", url)
	case url == "":
		p.Field("channel", name)
	case p.Style.Links:
		p.FieldLink("channel", name, url)
	default:
		p.Field("channel", name+"  "+p.Style.Dim(url))
	}
}

func songGenre(song *api.Song) string {
	if song == nil {
		return ""
	}
	return song.Genre
}

var months = [...]string{"January", "February", "March", "April", "May", "June", "July",
	"August", "September", "October", "November", "December"}

// longDate writes 1940-10-08 as "8 October 1940"; a bare year stays a year.
func longDate(iso string) string {
	t, err := time.Parse("2006-01-02", iso)
	if err != nil {
		return iso
	}
	return fmt.Sprintf("%d %s %d", t.Day(), months[t.Month()-1], t.Year())
}

// dateSpan writes a festival's dates the short way: "9–14 May 2019",
// "30 April – 2 May 2019", or one day.
func dateSpan(start, end string) string {
	s, err := time.Parse("2006-01-02", start)
	if err != nil {
		return ""
	}
	e, err := time.Parse("2006-01-02", end)
	switch {
	case err != nil || !e.After(s):
		return longDate(start)
	case s.Year() != e.Year():
		return longDate(start) + " – " + longDate(end)
	case s.Month() != e.Month():
		return fmt.Sprintf("%d %s – %s", s.Day(), months[s.Month()-1], longDate(end))
	default:
		return fmt.Sprintf("%d–%s", s.Day(), longDate(end))
	}
}

// compactCount writes 23486 as "23K" and 1234567 as "1.2M"; small numbers
// in full.
func compactCount(n int64) string {
	switch {
	case n >= 1_000_000:
		return strings.TrimSuffix(fmt.Sprintf("%.1f", float64(n)/1_000_000), ".0") + "M"
	case n >= 10_000:
		return fmt.Sprintf("%dK", n/1000)
	default:
		return thousands(int(n))
	}
}

func uploaded(date string) string {
	if date == "" {
		return ""
	}
	return "uploaded " + date
}

// join skips the blanks.
func join(sep string, parts ...string) string {
	var kept []string
	for _, s := range parts {
		if strings.TrimSpace(s) != "" {
			kept = append(kept, s)
		}
	}
	return strings.Join(kept, sep)
}

type person struct {
	Slug          string     `json:"slug"`
	Name          string     `json:"name"`
	Title         string     `json:"title"`
	Nickname      string     `json:"nickname"`
	Role          string     `json:"role"`
	VideosCount   int        `json:"videos_count"`
	SongsCount    int        `json:"songs_count"`
	Titles        []string   `json:"titles"`
	Active        *api.Years `json:"active"`
	Recorded      *api.Years `json:"recorded"`
	TopOrchestras []api.Ref  `json:"top_orchestras"`
	TopSingers    []api.Ref  `json:"top_singers"`
	Couples       []api.Ref  `json:"couples"`
	Dancers       []api.Ref  `json:"dancers"`
	TopSongs      []api.Ref  `json:"top_songs"`
	Links         struct {
		TangoTube string `json:"tangotube"`
	} `json:"links"`
	Videos []api.Video `json:"videos"`
}

func renderEntity(p *output.Printer, raw json.RawMessage) {
	e := entityFacts(p, raw)
	entityVideos(p, e)
}

func entityVideos(p *output.Printer, e person) {
	if len(e.Videos) > 0 {
		fmt.Fprintln(p.Out)
		videoTable(p, e.Videos)
	}
}

// entityFacts is the heading and facts of a dancer, couple or orchestra.
func entityFacts(p *output.Printer, raw json.RawMessage) person {
	e := decode[person](raw)
	name := e.Name
	if name == "" {
		name = e.Title
	}
	fmt.Fprintln(p.Out, p.Style.Bold(name)+"  "+p.Style.ID("", e.Slug))
	p.Field("known as", e.Nickname)
	p.Field("role", e.Role)
	for i, t := range e.Titles {
		label := ""
		if i == 0 {
			label = "titles"
		}
		p.Field(label, t)
	}
	p.Field("videos", thousands(e.VideosCount))
	p.Field("filmed", e.Active.Text())
	p.Field("songs", thousands(e.SongsCount))
	p.Field("recorded", e.Recorded.Text())
	p.Items("dances to", counted(p, e.TopOrchestras))
	p.Items("voices", counted(p, e.TopSingers))
	for i, c := range e.Couples {
		if i == 5 {
			p.Field("", p.Style.Dim(fmt.Sprintf("and %d more (--jq '.couples[]')", len(e.Couples)-5)))
			break
		}
		label := ""
		if i == 0 {
			label = "partners"
		}
		name := c.Label()
		if c.Partner != nil && c.Partner.Name != "" {
			name = c.Partner.Name
		}
		p.Field(label, fmt.Sprintf("%s %s", name, p.Style.Dim(fmt.Sprintf("%s · %s videos", c.Slug, thousands(c.VideosCount)))))
	}
	for i, s := range e.TopSongs {
		if i == 5 {
			p.Field("", p.Style.Dim(fmt.Sprintf("and %d more (--jq '.top_songs[]')", len(e.TopSongs)-5)))
			break
		}
		label := ""
		if i == 0 {
			label = "top songs"
		}
		p.Field(label, fmt.Sprintf("%s %s", s.Label(), p.Style.Dim(fmt.Sprintf("%s · %s videos", s.Slug, thousands(s.VideosCount)))))
	}
	p.FieldLink("page", e.Links.TangoTube, e.Links.TangoTube)
	return e
}

// counted is "Juan D'Arienzo (215)": a name and, dimmed, how many performances.
func counted(p *output.Printer, refs []api.Ref) []string {
	out := make([]string, 0, len(refs))
	for _, r := range refs {
		out = append(out, r.Label()+" "+p.Style.Dim("("+thousands(r.Count)+")"))
	}
	return out
}

// song is GET /api/v1/songs/:slug: the recording's facts, and its lyrics
// when asked for.
type song struct {
	api.Song
	Orchestra   *api.Ref    `json:"orchestra"`
	Year        *int        `json:"year"`
	VideosCount int         `json:"videos_count"`
	Lyrics      *lyrics     `json:"lyrics"`
	Videos      []api.Video `json:"videos"`
}

type lyrics struct {
	ES       string `json:"es"`
	EN       string `json:"en"`
	ENSource string `json:"en_source"`
}

func renderSong(p *output.Printer, raw json.RawMessage) {
	s := decode[song](raw)
	fmt.Fprintln(p.Out, p.Style.Bold(s.Title)+"  "+p.Style.ID("", s.Slug))
	p.Field("orchestra", join(" · ", s.Orchestra.Label(), s.Genre))
	p.Items("credits", credits(&s.Song))
	p.Field("recorded", longDate(s.RecordedOn))
	if s.VideosCount > 0 {
		p.Field("danced in", fmt.Sprintf("%s performance%s", thousands(s.VideosCount), plural(s.VideosCount)))
	}
	if s.HasLyrics && s.Lyrics == nil {
		p.Field("lyrics", p.Style.Gold("tt song show "+s.Slug+" --lyrics"))
	}
	listen(p, s.Links)
	p.FieldLink("page", s.Links.TangoTube, s.Links.TangoTube)

	if s.Lyrics != nil {
		fmt.Fprintln(p.Out)
		if s.Lyrics.ES == "" && s.Lyrics.EN == "" {
			p.Line("No lyrics for this recording yet.")
			return
		}
		printed := verse(p, "Letra", s.Lyrics.ES, false)
		english := "In English"
		if s.Lyrics.ENSource == "machine" {
			english += " · machine translation"
		}
		verse(p, english, s.Lyrics.EN, printed)
		return
	}
	if len(s.Videos) > 0 {
		fmt.Fprintln(p.Out)
		videoTable(p, s.Videos)
	}
}

// verse prints lyrics under a dim heading: each line wrapped to at most 72
// columns (or the terminal), a blank line between stanzas as written, and one
// before the heading when another verse came first. It reports whether it
// printed anything.
func verse(p *output.Printer, heading, text string, after bool) bool {
	if strings.TrimSpace(text) == "" {
		return false
	}
	if after {
		fmt.Fprintln(p.Out)
	}
	width := p.Columns() - 4
	if width > 72 {
		width = 72
	}
	fmt.Fprintln(p.Out, "  "+p.Style.Dim(heading))
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			fmt.Fprintln(p.Out)
			continue
		}
		for _, part := range wrap(line, width) {
			fmt.Fprintln(p.Out, "  "+p.Style.Text(part))
		}
	}
	return true
}

// wrap breaks a line at spaces so no piece is wider than width.
func wrap(line string, width int) []string {
	var out []string
	cur := ""
	for _, w := range strings.Fields(line) {
		switch {
		case cur == "":
			cur = w
		case output.DisplayWidth(cur)+1+output.DisplayWidth(w) > width:
			out = append(out, cur)
			cur = "  " + w // a turned line sits in from the verse
		default:
			cur += " " + w
		}
	}
	return append(out, cur)
}

// thousands writes 2719 as 2,719, and 0 as nothing.
func thousands(n int) string {
	if n == 0 {
		return ""
	}
	s := fmt.Sprint(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

func intText(n *int) string {
	if n == nil || *n == 0 {
		return ""
	}
	return fmt.Sprint(*n)
}

func newCouple(a *App) *cobra.Command {
	return showGroup(a, "couple", "Show a couple and the body of work they filmed together.",
		`Show a couple: the years they have been filmed together, the orchestras
they dance to most, and their performances.

Couple slugs name both dancers, like the ones in /couples/SLUG on the site.`,
		`  tt couple show carlitos-espinoza-noelia-hurtado
  tt dancer show noelia-hurtado --jq '.couples[].slug'`, "/api/v1/couples", renderEntity)
}

func newOrchestra(a *App) *cobra.Command {
	return showGroup(a, "orchestra", "Show an orchestra: its most-danced songs and performances.",
		`Show an orchestra: the years its danced recordings come from, the voices
dancers choose most with it, the recordings they choose most, and the
performances danced to them.`,
		`  tt orchestra show carlos-di-sarli`, "/api/v1/orchestras", renderEntity)
}

func newSong(a *App) *cobra.Command {
	var withLyrics bool
	cmd := &cobra.Command{
		Use:   "song",
		Short: "Show a recording, who wrote and sang it, and where to listen.",
		Args:  cobra.NoArgs,
	}
	show := &cobra.Command{
		Use:   "show SLUG",
		Short: "Show a recording, who wrote and sang it, and where to listen.",
		Long: `Show a recording: the orchestra, the singer, who wrote the music and the
words, when it was recorded, where to listen (Spotify, YouTube Music,
El Recodo), and the performances danced to it.

--lyrics prints the words instead, in Spanish and in English where there is a
translation: for the dancer who wants to know what the singer is saying.`,
		Example: `  tt song show volver-a-sonar-carlos-di-sarli
  tt song show volver-a-sonar-carlos-di-sarli --lyrics
  tt orchestra show carlos-di-sarli --jq '.top_songs[].slug'`,
		Args: exactArgs(1, "tt song show SLUG"),
		RunE: func(cmd *cobra.Command, args []string) error {
			var q url.Values
			if withLyrics {
				q = url.Values{"lyrics": {"1"}}
			}
			env, err := a.Client().Get(ctx(cmd), "/api/v1/songs/"+url.PathEscape(NormalizeID(args[0])), q)
			return a.ShowDetail(env, err, renderSong)
		},
	}
	show.Flags().BoolVar(&withLyrics, "lyrics", false, "Print the lyrics, in Spanish and in English.")
	cmd.AddCommand(show, newSongVersions(a))
	return cmd
}
