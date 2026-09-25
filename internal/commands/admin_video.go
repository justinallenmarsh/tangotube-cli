package commands

import (
	"net/http"
	"net/url"

	"github.com/spf13/cobra"
)

// newAdminVideoVerbs are tt admin video's verbs beyond hide and unhide.
func newAdminVideoVerbs(a *App) []*cobra.Command {
	path := func(id, verb string) string {
		return "/api/v1/admin/videos/" + url.PathEscape(NormalizeID(id)) + "/" + verb
	}
	var cmds []*cobra.Command
	for _, verb := range []struct{ name, short, long string }{
		{"feature", "Feature a video: it leads the featured row and ranks higher.", ""},
		{"unfeature", "Stop featuring a video.", ""},
		{"reidentify", "Re-read a video for dancers, song and event, in the background.",
			"Re-read a video's title and description for dancers, song and event, as an\nimport does, in the background. Recorded; there is nothing to undo."},
	} {
		var w writeFlags
		name := verb.name
		c := &cobra.Command{
			Use: name + " VIDEO", Short: verb.short, Long: verb.long,
			Example: "  tt admin video " + name + " uGwRPRusbC0 --dry-run",
			Args:    exactArgs(1, "tt admin video "+name+" uGwRPRusbC0"),
			RunE: func(cmd *cobra.Command, args []string) error {
				return a.adminWrite(cmd, http.MethodPost, path(args[0], name), &w, nil)
			},
		}
		w.bind(c, false)
		cmds = append(cmds, c)
	}

	var labelW writeFlags
	label := &cobra.Command{
		Use:   "label VIDEO FORM",
		Short: "Label a video's dance form as a person's: tango_family, folklore, other, not_dance.",
		Long: `Label a video's dance form: tango_family, folklore, other or not_dance. A
person's label outranks every keyword and session rule, and a tango label
spreads to the other takes of the same performance. tt admin undo puts the
previous label back.`,
		Example: "  tt admin video label uGwRPRusbC0 folklore --note \"a chacarera\" --dry-run",
		Args:    exactArgs(2, "tt admin video label uGwRPRusbC0 folklore"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.adminWrite(cmd, http.MethodPost, path(args[0], "label"), &labelW, map[string]any{"form": args[1]})
		},
	}
	labelW.bind(label, false)

	var stampW writeFlags
	stamp := &cobra.Command{
		Use:   "stamp VIDEO STATUS",
		Short: "Say why a video has no commercial song: live_music, no_commercial_take, needs_human, none.",
		Long: `Say why a video has no commercial song: live_music (an orchestra played
live), no_commercial_take (the music was never released), needs_human (a
person should look), or none to clear it. A stamp keeps the song matchers
off the video. tt admin undo puts the previous stamp back.`,
		Example: "  tt admin video stamp uGwRPRusbC0 live_music --dry-run",
		Args:    exactArgs(2, "tt admin video stamp uGwRPRusbC0 live_music"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.adminWrite(cmd, http.MethodPost, path(args[0], "stamp"), &stampW, map[string]any{"status": args[1]})
		},
	}
	stampW.bind(stamp, false)

	var importW writeFlags
	imp := &cobra.Command{
		Use:   "import VIDEO...",
		Short: "Add videos to the catalogue by YouTube id or link, in the background.",
		Long: `Add up to 50 videos to the catalogue by YouTube id or link. They are fetched
and identified in the background; ids already in the catalogue are named and
left alone. Recorded; hide a video to take it out again.`,
		Example: "  tt admin video import uGwRPRusbC0 https://youtu.be/EJv04w-mZaM --dry-run",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return Usage("give at least one YouTube id", "tt admin video import uGwRPRusbC0")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ids := make([]string, 0, len(args))
			for _, arg := range args {
				ids = append(ids, NormalizeID(arg))
			}
			return a.adminWrite(cmd, http.MethodPost, "/api/v1/admin/videos/import", &importW, map[string]any{"ids": ids})
		},
	}
	importW.bind(imp, false)
	return append(cmds, label, stamp, imp)
}
