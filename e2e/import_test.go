package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// takeoutExport writes a small Takeout folder: 2,500 watches (so two
// requests), half on TangoTube; two likes; two subscriptions.
func takeoutExport(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "Takeout", "YouTube and YouTube Music")
	for _, dir := range []string{"history", "playlists", "subscriptions"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	var history []map[string]any
	for i := 0; i < 2500; i++ {
		prefix := "t"
		if i%2 == 1 {
			prefix = "x"
		}
		history = append(history, map[string]any{
			"header": "YouTube", "title": "Watched a video", "products": []string{"YouTube"},
			"titleUrl": fmt.Sprintf("https://www.youtube.com/watch?v=%s%010d", prefix, i),
			"time":     fmt.Sprintf("2024-01-01T00:%02d:%02dZ", i/60%60, i%60),
		})
	}
	raw, _ := json.Marshal(history)
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("history/watch-history.json", string(raw))
	write("playlists/Liked videos.csv", "Video ID,Playlist Video Creation Timestamp\ntliked00001,2023-05-05T05:05:05+00:00\nxliked00002,2023-05-06T05:05:05+00:00\n")
	write("subscriptions/subscriptions.csv", "Channel Id,Channel Url,Channel Title\nUCtaaaaaaaaaaaaaaaaaaaaa,,Tango\nUCxbbbbbbbbbbbbbbbbbbbbb,,Cooking\n")
	return filepath.Dir(filepath.Dir(root))
}

func TestImportYouTubeSendsTheExportInChunksAndListsWhatIsMissing(t *testing.T) {
	s := server(t)
	export := takeoutExport(t)
	unknown := filepath.Join(t.TempDir(), "unknown.txt")

	r := tt(t, s, signedIn, "import", "youtube", export, "--unknown", unknown, "--json")
	env := parse(t, r)
	if !env.OK || r.code != 0 {
		t.Fatalf("import: code %d %+v %s", r.code, env, r.stderr)
	}
	var data struct {
		Found   map[string]int `json:"found"`
		Watches struct {
			Sent, Matched, Added int
		} `json:"watches"`
		Likes, Follows  struct{ Added int }
		UnknownVideoIDs []string `json:"unknown_video_ids"`
	}
	json.Unmarshal(env.Data, &data)
	if data.Found["watches"] != 2500 || data.Watches.Sent != 2500 || data.Watches.Added != 1250 {
		t.Errorf("watches: found %v, %+v", data.Found, data.Watches)
	}
	if data.Likes.Added != 1 || data.Follows.Added != 1 || len(data.UnknownVideoIDs) != 1251 {
		t.Errorf("likes %+v, follows %+v, unknown %d", data.Likes, data.Follows, len(data.UnknownVideoIDs))
	}
	if env.Summary != "Added 1,250 watches, 1 like, and 1 follow · 1,251 videos TangoTube doesn't have" {
		t.Errorf("summary = %q", env.Summary)
	}
	lines, _ := os.ReadFile(unknown)
	if got := strings.Count(string(lines), "\n"); got != 1252 || !strings.Contains(string(lines), "https://www.youtube.com/channel/UCxbbbbbbbbbbbbbbbbbbbbb") {
		t.Errorf("unknown file: %d lines", got)
	}
}

func TestImportYouTubeLeavesOutWhatItIsTold(t *testing.T) {
	s := server(t)
	env := parse(t, tt(t, s, signedIn, "import", "youtube", takeoutExport(t), "--no-history", "--no-subscriptions", "--dry-run", "--json"))
	if env.Summary != "Would add 1 like · 1 video TangoTube doesn't have" {
		t.Errorf("summary = %q", env.Summary)
	}
	if len(env.Breadcrumbs) != 1 || !strings.HasPrefix(env.Breadcrumbs[0], "tt import youtube ") {
		t.Errorf("a dry run's next step is the import: %v", env.Breadcrumbs)
	}
}

func TestImportYouTubeExplainsTakeoutWhenThePathHasNothing(t *testing.T) {
	s := server(t)
	r := tt(t, s, signedIn, "import", "youtube", t.TempDir(), "--json")
	if env := parse(t, r); r.code != 1 || env.Error == nil || !strings.Contains(env.Error.Hint, "takeout.google.com") {
		t.Fatalf("empty folder: code %d %+v", r.code, env)
	}
	if r := tt(t, s, nil, "import", "youtube", takeoutExport(t), "--json"); r.code != 3 {
		t.Fatalf("without a token: code %d", r.code)
	}
}
