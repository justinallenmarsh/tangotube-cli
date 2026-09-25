// Package takeout reads the parts of a Google Takeout export that tt import
// youtube needs: what was watched and when, what was liked, and which
// channels were subscribed to. It reads a Takeout folder, the .zip Google
// sends, or a single file from one, and never sends anything anywhere; the
// command decides what leaves the machine (only ids and times).
//
// Takeout's folder and file names are translated into the account's
// language, so files are recognised by what is in them as well as by name:
//
//   - watch history: any .json whose entries link to youtube.com/watch?v=
//     (watch-history.json), or any .html of Google's activity cells with such
//     links (watch-history.html). Search history links to searches, not
//     videos, so it never matches.
//   - liked videos: a .csv in a playlists folder whose name says liked, in
//     the languages below (Liked videos.csv, Liked videos-videos.csv,
//     Videos que me gustan.csv, …), with a video id column.
//   - subscriptions: a .csv whose rows carry YouTube channel ids (UC…),
//     outside the playlists folder (subscriptions.csv).
package takeout

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Item is one video and the latest time it was watched or liked. A zero At
// means the export did not say when (or said it in a way tt cannot read).
type Item struct {
	VideoID string
	At      time.Time
}

// Export is what one Takeout export holds for tt.
type Export struct {
	Watches       []Item
	Likes         []Item
	Subscriptions []string
	// Files are the files each part was read from, relative to the export.
	Files []string
	// Skipped counts history entries left out: YouTube Music plays, ads,
	// and videos YouTube no longer names (removed or private).
	Skipped Skipped
}

// Skipped is what the history held that is not a watch of a video.
type Skipped struct {
	Music   int `json:"music"`
	Ads     int `json:"ads"`
	Removed int `json:"removed"`
}

// Empty is true when the export held none of the three.
func (e *Export) Empty() bool {
	return len(e.Watches) == 0 && len(e.Likes) == 0 && len(e.Subscriptions) == 0
}

type file struct {
	name string
	open func() (io.ReadCloser, error)
}

// Read reads a Takeout folder, a Takeout .zip, or one file from an export.
func Read(path string) (*Export, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w", path, err)
	}
	var files []file
	switch {
	case info.IsDir():
		files, err = dirFiles(path)
	case strings.EqualFold(filepath.Ext(path), ".zip"):
		var closer io.Closer
		files, closer, err = zipFiles(path)
		if closer != nil {
			defer closer.Close()
		}
	default:
		files = []file{{name: filepath.Base(path), open: func() (io.ReadCloser, error) { return os.Open(path) }}}
	}
	if err != nil {
		return nil, err
	}
	return readFiles(files)
}

func dirFiles(root string) ([]file, error) {
	var files []file
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		files = append(files, file{name: filepath.ToSlash(rel), open: func() (io.ReadCloser, error) { return os.Open(p) }})
		return nil
	})
	return files, err
}

func zipFiles(path string) ([]file, io.Closer, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot open %s as a zip: %w", path, err)
	}
	var files []file
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		files = append(files, file{name: f.Name, open: f.Open})
	}
	return files, r, nil
}

func readFiles(files []file) (*Export, error) {
	watches := map[string]time.Time{}
	likes := map[string]time.Time{}
	subscriptions := map[string]bool{}
	export := &Export{}

	for _, f := range files {
		ext := strings.ToLower(filepath.Ext(f.name))
		if ext != ".json" && ext != ".html" && ext != ".csv" {
			continue
		}
		raw, err := readAll(f)
		if err != nil {
			return nil, err
		}
		found := false
		switch ext {
		case ".json":
			found = readHistoryJSON(raw, watches, &export.Skipped)
		case ".html":
			found = readHistoryHTML(raw, watches, &export.Skipped)
		case ".csv":
			switch {
			case isLikedPlaylist(f.name):
				found = readLikes(raw, likes)
			case !inPlaylists(f.name):
				found = readSubscriptions(raw, subscriptions)
			}
		}
		if found {
			export.Files = append(export.Files, f.name)
		}
	}

	export.Watches = items(watches)
	export.Likes = items(likes)
	for id := range subscriptions {
		export.Subscriptions = append(export.Subscriptions, id)
	}
	sort.Strings(export.Subscriptions)
	sort.Strings(export.Files)
	return export, nil
}

func readAll(f file) ([]byte, error) {
	r, err := f.open()
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w", f.name, err)
	}
	defer r.Close()
	return io.ReadAll(r)
}

// items turns a set into a list, newest first, ties by id, so output and
// requests are the same every run.
func items(set map[string]time.Time) []Item {
	list := make([]Item, 0, len(set))
	for id, at := range set {
		list = append(list, Item{VideoID: id, At: at})
	}
	sort.Slice(list, func(i, j int) bool {
		if !list[i].At.Equal(list[j].At) {
			return list[i].At.After(list[j].At)
		}
		return list[i].VideoID < list[j].VideoID
	})
	return list
}

// keep records a video, keeping its latest time.
func keep(set map[string]time.Time, id string, at time.Time) {
	if previous, seen := set[id]; !seen || at.After(previous) {
		set[id] = at
	}
}

var (
	watchLink = regexp.MustCompile(`https?://(?:www\.|m\.)?youtube\.com/watch\?v=([A-Za-z0-9_-]{11})`)
	musicLink = regexp.MustCompile(`https?://music\.youtube\.com/`)
	channelID = regexp.MustCompile(`^UC[A-Za-z0-9_-]{22}$`)
)

// ── watch-history.json ──────────────────────────────────────────────────

type historyEntry struct {
	Header   string   `json:"header"`
	TitleURL string   `json:"titleUrl"`
	Time     string   `json:"time"`
	Products []string `json:"products"`
	Details  []struct {
		Name string `json:"name"`
	} `json:"details"`
}

func readHistoryJSON(raw []byte, watches map[string]time.Time, skipped *Skipped) bool {
	var entries []historyEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return false
	}
	found := false
	for _, e := range entries {
		switch {
		case e.Header == "YouTube Music" || musicLink.MatchString(e.TitleURL):
			skipped.Music++
			continue
		case fromAds(e):
			skipped.Ads++
			continue
		}
		m := watchLink.FindStringSubmatch(e.TitleURL)
		if m == nil {
			if e.TitleURL == "" && strings.Contains(strings.Join(e.Products, " "), "YouTube") && e.Time != "" {
				skipped.Removed++
			}
			continue
		}
		found = true
		at, _ := time.Parse(time.RFC3339Nano, e.Time)
		keep(watches, m[1], at.UTC())
	}
	return found
}

func fromAds(e historyEntry) bool {
	for _, d := range e.Details {
		if strings.Contains(d.Name, "Google Ads") {
			return true
		}
	}
	return false
}

// ── watch-history.html ──────────────────────────────────────────────────

var (
	cellSplit = regexp.MustCompile(`<div class="outer-cell`)
	headerRe  = regexp.MustCompile(`mdl-typography--title">([^<]*)`)
	tagRe     = regexp.MustCompile(`<[^>]+>`)
)

// htmlLayouts are the English and day-first date lines Google writes under
// each entry, after its spaces are made plain. Other languages' month names
// are not read; those watches arrive undated.
var htmlLayouts = []string{
	"Jan 2, 2006, 3:04:05 PM MST",
	"Jan 2, 2006, 3:04:05 PM",
	"Jan 2, 2006, 3:04:05 PM Z07:00",
	"2 Jan 2006, 15:04:05 MST",
	"2 Jan 2006, 15:04:05",
	"2006-01-02 15:04:05 MST",
	"2 Jan 2006 15:04:05 MST",
}

func readHistoryHTML(raw []byte, watches map[string]time.Time, skipped *Skipped) bool {
	if !bytes.Contains(raw, []byte("outer-cell")) {
		return false
	}
	found := false
	for _, cell := range cellSplit.Split(string(raw), -1)[1:] {
		if h := headerRe.FindStringSubmatch(cell); h != nil && strings.TrimSpace(h[1]) == "YouTube Music" {
			skipped.Music++
			continue
		}
		if strings.Contains(cell, "Google Ads") {
			skipped.Ads++
			continue
		}
		m := watchLink.FindStringSubmatch(cell)
		if m == nil {
			continue
		}
		found = true
		keep(watches, m[1], cellTime(cell))
	}
	return found
}

// cellTime reads the date line of one activity cell: the text of the content
// cell after its last link.
func cellTime(cell string) time.Time {
	content := cell
	if i := strings.Index(cell, "content-cell"); i >= 0 {
		content = cell[i:]
		if j := strings.Index(content, "</div>"); j >= 0 {
			content = content[:j]
		}
	}
	if i := strings.LastIndex(content, "</a>"); i >= 0 {
		content = content[i+len("</a>"):]
	}
	for _, part := range strings.Split(content, "<br>") {
		text := plainSpaces(html.UnescapeString(tagRe.ReplaceAllString(part, "")))
		if text == "" {
			continue
		}
		for _, layout := range htmlLayouts {
			if at, err := time.Parse(layout, text); err == nil {
				return at.UTC()
			}
		}
	}
	return time.Time{}
}

// plainSpaces turns Google's no-break and narrow no-break spaces (before PM,
// inside dates) into plain ones.
func plainSpaces(s string) string {
	s = strings.Map(func(r rune) rune {
		if r == ' ' || r == ' ' || r == ' ' {
			return ' '
		}
		return r
	}, s)
	return strings.Join(strings.Fields(s), " ")
}

// ── liked videos ────────────────────────────────────────────────────────

// likedNames are the words the liked-videos playlist is called by, across
// the languages Takeout names it in, lower-cased and without accents.
var likedNames = []string{
	"liked", "me gusta", "gostei", "j'aime", "jaime", "mag ich", "piaciut",
	"gefallen", "polubion", "begendi", "vind ik leuk", "gillade", "synes godt om",
	"tykkaamani", "нравится", "いいね", "좋아요", "赞过", "喜歡", "喜欢",
}

func isLikedPlaylist(name string) bool {
	if !inPlaylists(name) {
		base := fold(filepath.Base(name))
		return strings.HasPrefix(base, "liked videos")
	}
	base := fold(filepath.Base(name))
	for _, word := range likedNames {
		if strings.Contains(base, word) {
			return true
		}
	}
	return false
}

// inPlaylists is true for a file in Takeout's playlists folder, in any of
// its names.
func inPlaylists(name string) bool {
	dir := fold(filepath.ToSlash(filepath.Dir(name)))
	for _, word := range []string{"playlist", "lista", "liste", "afspeellijst", "spellist"} {
		if strings.Contains(dir, word) {
			return true
		}
	}
	return false
}

// accents folds the accented letters Takeout's Latin-script names use, so
// "Vídeos" and "videos" meet.
var accents = strings.NewReplacer(
	"á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a", "å", "a",
	"é", "e", "è", "e", "ê", "e", "ë", "e",
	"í", "i", "ì", "i", "î", "i", "ï", "i",
	"ó", "o", "ò", "o", "ô", "o", "õ", "o", "ö", "o",
	"ú", "u", "ù", "u", "û", "u", "ü", "u",
	"ç", "c", "ñ", "n", "ł", "l", "ś", "s", "ż", "z", "ź", "z", "ğ", "g", "ı", "i", "ş", "s",
)

// fold lower-cases and strips accents.
func fold(s string) string {
	return accents.Replace(strings.ToLower(s))
}

// likeLayouts are the time formats the liked playlist has used.
var likeLayouts = []string{time.RFC3339Nano, "2006-01-02 15:04:05 MST", "2006-01-02 15:04:05"}

// readLikes reads a liked-videos CSV, old or new: find the row naming a
// video id column, then read the ids (and the time column beside, if any)
// under it. Older exports carry a playlist-details block first.
func readLikes(raw []byte, likes map[string]time.Time) bool {
	rows := readCSV(raw)
	idCol, timeCol := -1, -1
	found := false
	for _, row := range rows {
		if idCol < 0 {
			idCol, timeCol = likeColumns(row)
			continue
		}
		if idCol >= len(row) {
			continue
		}
		id := strings.TrimSpace(row[idCol])
		if len(id) != 11 || strings.ContainsAny(id, " /") {
			continue
		}
		var at time.Time
		if timeCol >= 0 && timeCol < len(row) {
			for _, layout := range likeLayouts {
				if t, err := time.Parse(layout, strings.TrimSpace(row[timeCol])); err == nil {
					at = t.UTC()
					break
				}
			}
		}
		keep(likes, id, at)
		found = true
	}
	return found
}

// likeColumns finds the video id and time columns in a header row, or -1.
func likeColumns(row []string) (idCol, timeCol int) {
	idCol, timeCol = -1, -1
	for i, cell := range row {
		c := fold(strings.TrimSpace(cell))
		switch {
		case strings.Contains(c, "video id") || strings.HasPrefix(c, "id del video") || strings.HasPrefix(c, "id de la video") || strings.HasPrefix(c, "id do video") || c == "video-id":
			idCol = i
		case timeCol < 0 && containsAny(c, "time", "timestamp", "tiempo", "fecha", "date", "data", "zeit", "horodatage", "tempo"):
			timeCol = i
		}
	}
	if idCol < 0 {
		return -1, -1
	}
	return idCol, timeCol
}

func containsAny(s string, words ...string) bool {
	for _, w := range words {
		if strings.Contains(s, w) {
			return true
		}
	}
	return false
}

// ── subscriptions ───────────────────────────────────────────────────────

// readSubscriptions takes every channel id (UC…) in the first column that
// holds them. A CSV with none is not the subscriptions file.
func readSubscriptions(raw []byte, subscriptions map[string]bool) bool {
	found := false
	for _, row := range readCSV(raw) {
		for _, cell := range row {
			if id := strings.TrimSpace(cell); channelID.MatchString(id) {
				subscriptions[id] = true
				found = true
				break
			}
		}
	}
	return found
}

func readCSV(raw []byte) [][]string {
	r := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(raw, []byte("\xef\xbb\xbf"))))
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	var rows [][]string
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		rows = append(rows, row)
	}
	return rows
}
