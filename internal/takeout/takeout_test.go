package takeout

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func at(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func ids(items []Item) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.VideoID
	}
	return out
}

func equal(t *testing.T, name string, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s = %#v, want %#v", name, got, want)
	}
}

func TestReadsAnEnglishTakeoutFolder(t *testing.T) {
	export, err := Read("testdata/en")
	if err != nil {
		t.Fatal(err)
	}

	// Newest first; the repeat keeps its latest time; music, ads, a removed
	// video and the search history are left out.
	equal(t, "watches", export.Watches, []Item{
		{VideoID: "aaaaaaaaaa1", At: at("2024-03-14T20:05:00.123Z")},
		{VideoID: "bbbbbbbbbb2", At: at("2023-07-01T10:00:00Z")},
	})
	equal(t, "skipped", export.Skipped, Skipped{Music: 1, Ads: 1, Removed: 1})

	// The old liked playlist: a details block, then Video Id,Time Added. Watch
	// later is a playlist, not likes.
	equal(t, "likes", export.Likes, []Item{
		{VideoID: "bbbbbbbbbb2", At: at("2020-01-02T03:04:05Z")},
		{VideoID: "cccccccccc3", At: at("2019-05-05T05:05:05Z")},
	})
	equal(t, "subscriptions", export.Subscriptions, []string{"UCaaaaaaaaaaaaaaaaaaaaaa", "UCbbbbbbbbbbbbbbbbbbbbbb"})
	equal(t, "files", export.Files, []string{
		"Takeout/YouTube and YouTube Music/history/watch-history.json",
		"Takeout/YouTube and YouTube Music/playlists/Liked videos.csv",
		"Takeout/YouTube and YouTube Music/subscriptions/subscriptions.csv",
	})
}

func TestReadsASpanishTakeoutFolder(t *testing.T) {
	export, err := Read("testdata/es")
	if err != nil {
		t.Fatal(err)
	}
	equal(t, "likes", export.Likes, []Item{{VideoID: "eeeeeeeeee5", At: at("2022-02-02T02:02:02Z")}})
	equal(t, "subscriptions", export.Subscriptions, []string{"UCcccccccccccccccccccccc"})
	equal(t, "watches", len(export.Watches), 0)
}

func TestReadsTheHTMLHistory(t *testing.T) {
	export, err := Read("testdata/html/watch-history.html")
	if err != nil {
		t.Fatal(err)
	}
	// US and day-first dates are read, a narrow no-break space and all; a
	// date in another language arrives undated rather than guessed.
	equal(t, "watches", export.Watches, []Item{
		{VideoID: "hhhhhhhhhh1", At: at("2024-03-14T20:05:00Z")},
		{VideoID: "hhhhhhhhhh2", At: at("2020-01-02T15:04:05Z")},
		{VideoID: "hhhhhhhhhh3"},
	})
	equal(t, "skipped", export.Skipped, Skipped{Music: 1})
}

func TestReadsTheZipGoogleSends(t *testing.T) {
	path := filepath.Join(t.TempDir(), "takeout-20240314T000000Z-001.zip")
	zipDir(t, "testdata/en", path)

	export, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	equal(t, "watches", ids(export.Watches), []string{"aaaaaaaaaa1", "bbbbbbbbbb2"})
	equal(t, "likes", ids(export.Likes), []string{"bbbbbbbbbb2", "cccccccccc3"})
	equal(t, "subscriptions", len(export.Subscriptions), 2)
}

func TestFindsNothingInSomethingElse(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "notes.csv"), []byte("a,b\n1,2\n"), 0o600)
	os.WriteFile(filepath.Join(dir, "data.json"), []byte(`{"not":"history"}`), 0o600)

	export, err := Read(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !export.Empty() {
		t.Errorf("export = %#v, want empty", export)
	}
	if _, err := Read(filepath.Join(dir, "missing")); err == nil {
		t.Error("reading a missing path: no error")
	}
}

func zipDir(t *testing.T, dir, path string) {
	t.Helper()
	out, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	w := zip.NewWriter(out)
	defer w.Close()
	filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		f, err := w.Create(filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		in, err := os.Open(p)
		if err != nil {
			return err
		}
		defer in.Close()
		_, err = io.Copy(f, in)
		return err
	})
}
