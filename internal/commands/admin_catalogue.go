package commands

import (
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/spf13/cobra"
)

// tt admin orchestra|singer|song|event: the catalogue's facts, edited by an
// operator. Each edit is recorded and tt admin undo puts it back.
func newAdminCatalogue(a *App) []*cobra.Command {
	group := func(noun, short string, subs ...*cobra.Command) *cobra.Command {
		cmd := &cobra.Command{Use: noun, Short: short, Args: cobra.NoArgs}
		cmd.AddCommand(subs...)
		return cmd
	}
	return []*cobra.Command{
		group("orchestra", "Operator changes to orchestras: edit.",
			newAdminEdit(a, "orchestra", "orchestras", "SLUG", `  tt admin orchestra edit juan-darienzo --bio "El Rey del Compás." --dry-run`, []editFlag{
				{flag: "name", key: "name", usage: "The orchestra's `NAME`."},
				{flag: "bio", key: "bio", usage: "A short `BIO`, or none."},
			})),
		group("singer", "Operator changes to singers: edit.",
			newAdminEdit(a, "singer", "singers", "SLUG", `  tt admin singer edit alberto-echague --name "Alberto Echagüe" --dry-run`, []editFlag{
				{flag: "name", key: "name", usage: "The singer's `NAME`."},
				{flag: "reviewed", key: "reviewed", usage: "`true` once a person has checked the record.", kind: "bool"},
			})),
		group("song", "Operator changes to songs: edit, lyrics.",
			newAdminEdit(a, "song", "songs", "SLUG", `  tt admin song edit violetas-aberto-castillo --composer "Rodolfo Sciammarella" --dry-run
  tt admin song edit violetas-aberto-castillo --singer alberto-castillo --recorded 1944-06-21`, []editFlag{
				{flag: "title", key: "title", usage: "The song's `TITLE`."},
				{flag: "singer", key: "singer", usage: "The singer's `SLUG`, or none."},
				{flag: "composer", key: "composer", usage: "Who wrote the music, a `NAME`, or none."},
				{flag: "lyricist", key: "author", usage: "Who wrote the words, a `NAME`, or none."},
				{flag: "recorded", key: "date", usage: "When it was recorded, a `DATE` (1944-06-21), or none."},
				{flag: "spotify", key: "spotify_track_id", usage: "Spotify track `ID`, or none."},
				{flag: "el-recodo", key: "el_recodo_song_id", usage: "El Recodo song `ID`, or none.", kind: "number"},
				{flag: "youtube-music", key: "youtube_music_video_id", usage: "YouTube Music video `ID`, or none."},
			}),
			newAdminSongLyrics(a)),
		group("event", "Operator changes to events: edit.",
			newAdminEdit(a, "event", "events", "SLUG", `  tt admin event edit planetango --city "Buenos Aires" --start 2026-11-20 --end 2026-11-24 --dry-run`, []editFlag{
				{flag: "title", key: "title", usage: "The event's `TITLE`."},
				{flag: "city", key: "city", usage: "Its `CITY`."},
				{flag: "country", key: "country", usage: "Its `COUNTRY`."},
				{flag: "start", key: "start_date", usage: "First day, a `DATE` (2026-11-20), or none."},
				{flag: "end", key: "end_date", usage: "Last day, a `DATE`, or none."},
				{flag: "lat", key: "latitude", usage: "`LATITUDE`, or none.", kind: "number"},
				{flag: "lng", key: "longitude", usage: "`LONGITUDE`, or none.", kind: "number"},
			})),
	}
}

func newAdminSongLyrics(a *App) *cobra.Command {
	var w writeFlags
	var es, en, source string
	cmd := &cobra.Command{
		Use:   "lyrics SLUG --es FILE --en FILE",
		Short: "Set a song's lyrics and English translation from files.",
		Long: `Set a song's lyrics (--es) and English translation (--en) from text files; "-"
reads standard input. An English translation given here is marked as a
person's (--source human) unless you say --source machine. Recorded, and
tt admin undo puts the old text back.`,
		Example: `  tt admin song lyrics violetas-aberto-castillo --es violetas.txt --en violets.txt --dry-run`,
		Args:    exactArgs(1, "tt admin song lyrics violetas-aberto-castillo --es violetas.txt"),
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{}
			for key, path := range map[string]string{"lyrics": es, "lyrics_en": en} {
				if path == "" {
					continue
				}
				text, err := a.readText(path)
				if err != nil {
					return Usage("could not read "+path+": "+err.Error(), "")
				}
				body[key] = text
			}
			if len(body) == 0 {
				return Usage("give --es FILE, --en FILE, or both", "tt admin song lyrics violetas-aberto-castillo --es violetas.txt")
			}
			if source != "" {
				body["lyrics_en_source"] = source
			}
			return a.adminWrite(cmd, http.MethodPut, "/api/v1/admin/songs/"+url.PathEscape(args[0])+"/lyrics", &w, body)
		},
	}
	f := cmd.Flags()
	f.StringVar(&es, "es", "", "Spanish lyrics from a `FILE` (- for standard input).")
	f.StringVar(&en, "en", "", "English translation from a `FILE`.")
	f.StringVar(&source, "source", "", "Who translated: `human` (the default) or machine.")
	w.bind(cmd, false)
	return cmd
}

// readText reads a file, or standard input for "-".
func (a *App) readText(path string) (string, error) {
	if path == "-" {
		raw, err := io.ReadAll(a.In)
		return string(raw), err
	}
	raw, err := os.ReadFile(path)
	return string(raw), err
}
