package api

import (
	"fmt"
	"strings"
)

// Ref is a named thing with a slug: a dancer, an orchestra, a couple.
type Ref struct {
	Name        string `json:"name"`
	Title       string `json:"title,omitempty"`
	Slug        string `json:"slug"`
	Role        string `json:"role,omitempty"`
	Genre       string `json:"genre,omitempty"`
	City        string `json:"city,omitempty"`
	Country     string `json:"country,omitempty"`
	VideosCount int    `json:"videos_count,omitempty"`
	Year        *int   `json:"year,omitempty"`
	Partner     *Ref   `json:"partner,omitempty"` // on a dancer's couples: the other dancer
	Also        []Ref  `json:"also,omitempty"`    // on search filters: names that also matched
	FromQuery   bool   `json:"from_query,omitempty"`
	// On a video's dancers, on one video's page: world titles, newest first.
	Titles []string `json:"titles,omitempty"`
	// On events.
	StartDate string `json:"start_date,omitempty"`
	EndDate   string `json:"end_date,omitempty"`
	// On top orchestras and singers: performances.
	Count int `json:"count,omitempty"`
}

// Song is a recording: who wrote it, who sang it, when, and where to listen.
type Song struct {
	Title      string    `json:"title"`
	Slug       string    `json:"slug"`
	Genre      string    `json:"genre"`
	RecordedOn string    `json:"recorded_on"`
	Singer     string    `json:"singer"`
	Composer   string    `json:"composer"`
	Lyricist   string    `json:"lyricist"`
	HasLyrics  bool      `json:"has_lyrics"`
	Links      SongLinks `json:"links"`
}

// SongLinks are the places to hear a recording, and its TangoTube page.
type SongLinks struct {
	TangoTube    string `json:"tangotube"`
	Spotify      string `json:"spotify"`
	YoutubeMusic string `json:"youtube_music"`
	ElRecodo     string `json:"el_recodo"`
}

// Label is the title, nil-safe.
func (s *Song) Label() string {
	if s == nil {
		return ""
	}
	return s.Title
}

// Years is a span, like the years a couple has been filmed.
type Years struct {
	From int `json:"from"`
	To   int `json:"to"`
}

// Text is "2011–2026", or one year when they are the same.
func (y *Years) Text() string {
	if y == nil || y.From == 0 {
		return ""
	}
	if y.From == y.To || y.To == 0 {
		return fmt.Sprint(y.From)
	}
	return fmt.Sprintf("%d–%d", y.From, y.To)
}

// VideoLinks are a video's pages: TangoTube, YouTube, and its channel.
type VideoLinks struct {
	TangoTube string `json:"tangotube"`
	YouTube   string `json:"youtube"`
	Channel   string `json:"channel"`
}

// Angle is another camera on the same dance.
type Angle struct {
	ID       string `json:"id"`
	Channel  string `json:"channel"`
	WatchURL string `json:"watch_url"`
}

// Label is the name, or the title for songs and events.
func (r *Ref) Label() string {
	if r == nil {
		return ""
	}
	if r.Name != "" {
		return r.Name
	}
	return r.Title
}

// Video is one filmed performance.
type Video struct {
	ID              string     `json:"id"`
	YoutubeID       string     `json:"youtube_id"`
	Title           string     `json:"title"`
	WatchURL        string     `json:"watch_url"`
	DurationS       *int       `json:"duration_s"`
	Dancers         []Ref      `json:"dancers"`
	Couple          *Ref       `json:"couple"`
	Orchestra       *Ref       `json:"orchestra"`
	Song            *Song      `json:"song"`
	Year            *int       `json:"year"`
	Event           *Ref       `json:"event"`
	Channel         string     `json:"channel"`
	UploadedOn      string     `json:"uploaded_on"`
	Style           []string   `json:"style"`
	Techniques      []string   `json:"techniques"`
	ClipsCount      int        `json:"clips_count"`
	RelatedVideoIDs []string   `json:"related_video_ids"`
	Clips           []Clip     `json:"clips"`
	YoutubeViews    *int64     `json:"youtube_views"`
	Links           VideoLinks `json:"links"`
	OtherAngles     []Angle    `json:"other_angles"`
	// On one video's page: the other dances of the same couple, the same
	// occasion.
	Session *Session `json:"performance_session"`
}

// Session is one couple's dances on one occasion: a Mundial round, a
// festival's performance of two or three songs.
type Session struct {
	Occasion string  `json:"occasion"`
	Position int     `json:"position"`
	Total    int     `json:"total"`
	Dances   []Dance `json:"dances"`
}

// Dance is one of a session's dances.
type Dance struct {
	ID        string `json:"id"`
	Song      string `json:"song"`
	Orchestra string `json:"orchestra"`
	WatchURL  string `json:"watch_url"`
	Current   bool   `json:"current"`
}

// DancerNames joins the dancers the way a dancer would say them.
func (v Video) DancerNames() string {
	var names []string
	for _, d := range v.Dancers {
		names = append(names, d.Name)
	}
	return strings.Join(names, " & ")
}

// YearText is the recording year, or blank.
func (v Video) YearText() string { return intText(v.Year) }

// Clip is a practice loop on a video.
type Clip struct {
	ID          string   `json:"id"`
	VideoID     string   `json:"video_id"`
	StartS      int      `json:"start_s"`
	EndS        int      `json:"end_s"`
	Tags        []string `json:"tags"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Visibility  string   `json:"visibility"`
	WatchURL    string   `json:"watch_url"`
	ClipURL     string   `json:"clip_url"`
	Mine        bool     `json:"mine"`
}

// Range is "1:12–1:18".
func (c Clip) Range() string { return Clock(c.StartS) + "–" + Clock(c.EndS) }

// Clock formats seconds as m:ss, or h:mm:ss past the hour.
func Clock(s int) string {
	if s >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", s/3600, s%3600/60, s%60)
	}
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

func intText(n *int) string {
	if n == nil || *n == 0 {
		return ""
	}
	return fmt.Sprint(*n)
}
