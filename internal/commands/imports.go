package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/justinallenmarsh/tangotube-cli/internal/output"
	"github.com/justinallenmarsh/tangotube-cli/internal/takeout"
)

// importChunk is the most of each kind the API takes in one request.
const importChunk = 2000

const takeoutSteps = "takeout.google.com: deselect all, pick \"YouTube and YouTube Music\", keep history, playlists and subscriptions, and choose JSON for history. Then: tt import youtube ~/Downloads/takeout-….zip"

func newImport(a *App) *cobra.Command {
	cmd := &cobra.Command{Use: "import", Short: "Bring your library over from elsewhere.", Args: cobra.NoArgs}
	cmd.AddCommand(newImportYouTube(a))
	return cmd
}

type importOptions struct {
	dryRun, noHistory, noLikes, noSubscriptions bool
	unknown                                     string
}

func newImportYouTube(a *App) *cobra.Command {
	var o importOptions
	cmd := &cobra.Command{
		Use:   "youtube PATH",
		Short: "Import your YouTube history, likes and subscriptions from a Google Takeout export.",
		Long: `Import your YouTube history, likes and subscriptions from a Google Takeout
export: the watches, likes and follows TangoTube has a video or channel for.

YouTube's API has not given out watch history since 2016, so this starts from
Takeout. Ask for it at takeout.google.com: deselect all, pick "YouTube and
YouTube Music", keep history, playlists and subscriptions, and choose JSON for
history. PATH is the zip Google sends, the folder it unzips to, or one file
from it (watch-history.json or .html, Liked videos.csv, subscriptions.csv).

The export is read on this machine. Only YouTube video ids, the times you
watched or liked them, and channel ids are sent. Watches and likes keep
YouTube's dates. Importing again adds only what is new. Needs a token with
write access (tt auth login).`,
		Example: `  tt import youtube ~/Downloads/takeout-20260924T101500Z-001.zip --dry-run
  tt import youtube ~/Downloads/Takeout
  tt import youtube watch-history.json --no-likes --unknown not-on-tangotube.txt`,
		Args: exactArgs(1, "tt import youtube ~/Downloads/takeout-….zip"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.importYouTube(cmd, args[0], o)
		},
	}
	f := cmd.Flags()
	f.BoolVar(&o.dryRun, "dry-run", false, "Count what would be added, and add nothing.")
	f.BoolVar(&o.noHistory, "no-history", false, "Leave the watch history out.")
	f.BoolVar(&o.noLikes, "no-likes", false, "Leave the liked videos out.")
	f.BoolVar(&o.noSubscriptions, "no-subscriptions", false, "Leave the subscriptions out.")
	f.StringVar(&o.unknown, "unknown", "", "Write the YouTube links TangoTube does not have to `FILE`.")
	return cmd
}

// importTally is the API's counts for one kind, summed over the chunks.
type importTally struct {
	Sent    int `json:"sent"`
	Matched int `json:"matched"`
	Added   int `json:"added"`
	Updated int `json:"updated,omitempty"`
	Already int `json:"already"`
}

func (t *importTally) add(o importTally) {
	t.Sent += o.Sent
	t.Matched += o.Matched
	t.Added += o.Added
	t.Updated += o.Updated
	t.Already += o.Already
}

type importChunkResult struct {
	Watches           importTally `json:"watches"`
	Likes             importTally `json:"likes"`
	Follows           importTally `json:"follows"`
	UnknownVideoIDs   []string    `json:"unknown_video_ids"`
	UnknownChannelIDs []string    `json:"unknown_channel_ids"`
}

// importResult is what tt import youtube reports: what the export held, and
// what the API did with it.
type importResult struct {
	Files   []string        `json:"files"`
	Found   map[string]int  `json:"found"`
	Skipped takeout.Skipped `json:"skipped"`
	importChunkResult
	UnknownFile string `json:"unknown_file,omitempty"`
	DryRun      bool   `json:"dry_run"`
}

func (a *App) importYouTube(cmd *cobra.Command, path string, o importOptions) error {
	export, err := takeout.Read(path)
	if err != nil {
		return a.Fail(Usage(err.Error(), "tt import youtube ~/Downloads/takeout-….zip"))
	}
	if o.noHistory {
		export.Watches = nil
	}
	if o.noLikes {
		export.Likes = nil
	}
	if o.noSubscriptions {
		export.Subscriptions = nil
	}
	if export.Empty() {
		return a.Fail(Usage("No YouTube watch history, liked videos or subscriptions in "+path, takeoutSteps))
	}

	result := importResult{
		Files: export.Files,
		Found: map[string]int{
			"watches": len(export.Watches), "likes": len(export.Likes), "subscriptions": len(export.Subscriptions),
		},
		Skipped: export.Skipped,
		DryRun:  o.dryRun,
	}
	result.UnknownVideoIDs = []string{}
	result.UnknownChannelIDs = []string{}

	p := a.Printer()
	human := p.Mode == output.Human && p.JQ == ""
	if human {
		fmt.Fprintln(a.Err, p.Style.Dim(fmt.Sprintf("Read %s from %s",
			sentenceList(nouns(len(export.Watches), "watch", "watches"), nouns(len(export.Likes), "like", "likes"),
				nouns(len(export.Subscriptions), "subscription", "subscriptions")), path)))
	}

	chunks := chunkCount(len(export.Watches), len(export.Likes), len(export.Subscriptions))
	seen := map[string]bool{}
	for i := 0; i < chunks; i++ {
		if human && chunks > 1 {
			fmt.Fprintf(a.Err, "\r%s", p.Style.Dim(fmt.Sprintf("Sending %d of %d…", i+1, chunks)))
		}
		body := map[string]any{
			"watches":       itemsBody(slice(export.Watches, i), "watched_at"),
			"likes":         itemsBody(slice(export.Likes, i), "liked_at"),
			"subscriptions": slice(export.Subscriptions, i),
			"dry_run":       o.dryRun,
		}
		env, err := a.Client().Do(ctx(cmd), http.MethodPost, "/api/v1/me/imports/youtube", nil, body, true)
		if err != nil {
			if human && i > 0 {
				fmt.Fprintf(a.Err, "\r%d of %d parts went in before this. Importing again is safe: it adds only what is new.\n", i, chunks)
			}
			return a.Fail(err)
		}
		var part importChunkResult
		if err := json.Unmarshal(env.Data, &part); err != nil {
			return a.Fail(err)
		}
		result.Watches.add(part.Watches)
		result.Likes.add(part.Likes)
		result.Follows.add(part.Follows)
		for _, id := range part.UnknownVideoIDs {
			// A video both watched and liked comes back from two chunks.
			if !seen[id] {
				seen[id] = true
				result.UnknownVideoIDs = append(result.UnknownVideoIDs, id)
			}
		}
		result.UnknownChannelIDs = append(result.UnknownChannelIDs, part.UnknownChannelIDs...)
	}
	if human && chunks > 1 {
		fmt.Fprint(a.Err, "\r\033[K")
	}

	if o.unknown != "" {
		if err := writeUnknown(o.unknown, result); err != nil {
			return a.Fail(Usage("Cannot write "+o.unknown+": "+err.Error(), "--unknown ~/not-on-tangotube.txt"))
		}
		result.UnknownFile = o.unknown
	}

	crumbs := []string{"tt history", "tt likes", "tt following"}
	if o.dryRun {
		crumbs = []string{"tt import youtube " + shellWord(path)}
	}
	return a.Show(output.NewEnvelope(result, importSummary(result), crumbs...), nil, renderImport)
}

func chunkCount(lengths ...int) int {
	most := 0
	for _, n := range lengths {
		most = max(most, n)
	}
	return max(1, (most+importChunk-1)/importChunk)
}

// slice is the i-th chunk of a list, empty once the list has run out.
func slice[T any](list []T, i int) []T {
	start := i * importChunk
	if start >= len(list) {
		return []T{}
	}
	return list[start:min(start+importChunk, len(list))]
}

func itemsBody(items []takeout.Item, key string) []map[string]string {
	body := make([]map[string]string, len(items))
	for i, it := range items {
		body[i] = map[string]string{"video_id": it.VideoID}
		if !it.At.IsZero() {
			body[i][key] = it.At.UTC().Format(time.RFC3339)
		}
	}
	return body
}

// writeUnknown lists what TangoTube does not have as YouTube links, one a
// line, so they can be opened, or suggested to TangoTube.
func writeUnknown(path string, r importResult) error {
	var b strings.Builder
	for _, id := range r.UnknownVideoIDs {
		b.WriteString("https://www.youtube.com/watch?v=" + id + "\n")
	}
	for _, id := range r.UnknownChannelIDs {
		b.WriteString("https://www.youtube.com/channel/" + id + "\n")
	}
	return os.WriteFile(path, []byte(b.String()), 0o600)
}

func importSummary(r importResult) string {
	var parts []string
	if r.Watches.Added > 0 {
		parts = append(parts, nouns(r.Watches.Added, "watch", "watches"))
	}
	if r.Watches.Updated > 0 {
		parts = append(parts, nouns(r.Watches.Updated, "watch", "watches")+" moved later")
	}
	if r.Likes.Added > 0 {
		parts = append(parts, nouns(r.Likes.Added, "like", "likes"))
	}
	if r.Follows.Added > 0 {
		parts = append(parts, nouns(r.Follows.Added, "follow", "follows"))
	}
	line := "Nothing new to add"
	if len(parts) > 0 {
		verb := "Added "
		if r.DryRun {
			verb = "Would add "
		}
		line = verb + sentenceList(parts...)
	}
	if n := len(r.UnknownVideoIDs); n > 0 {
		line += " · " + nouns(n, "video", "videos") + " TangoTube doesn't have"
	}
	return line
}

func renderImport(p *output.Printer, raw json.RawMessage) {
	r := decode[importResult](raw)
	p.Line("")
	if r.Found["watches"] > 0 {
		p.Field("Watches", tallyText(r.Watches, r.Found["watches"], r.DryRun))
	}
	if r.Found["likes"] > 0 {
		p.Field("Likes", tallyText(r.Likes, r.Found["likes"], r.DryRun))
	}
	if r.Found["subscriptions"] > 0 {
		p.Field("Follows", tallyText(r.Follows, r.Found["subscriptions"], r.DryRun))
	}
	if skipped := sentenceList(
		nouns(r.Skipped.Music, "YouTube Music play", "YouTube Music plays"),
		nouns(r.Skipped.Ads, "ad", "ads"),
		nouns(r.Skipped.Removed, "removed video", "removed videos")); skipped != "" {
		p.Field("Left out", skipped)
	}
	switch unknown := len(r.UnknownVideoIDs) + len(r.UnknownChannelIDs); {
	case r.UnknownFile != "":
		p.Field("Not here", "listed in "+r.UnknownFile)
	case unknown > 0:
		p.Field("Not here", "--unknown FILE lists the "+thousands(unknown)+" YouTube links")
	}
}

// tallyText is one kind's line: what was added, what was here already, and
// what TangoTube has no video (or channel) for.
func tallyText(t importTally, found int, dryRun bool) string {
	var parts []string
	switch {
	case t.Added > 0 && dryRun:
		parts = append(parts, thousands(t.Added)+" to add")
	case t.Added > 0:
		parts = append(parts, thousands(t.Added)+" added")
	case t.Updated == 0 && t.Already == 0:
		parts = append(parts, "none to add")
	}
	if t.Updated > 0 {
		parts = append(parts, thousands(t.Updated)+" moved later")
	}
	if t.Already > 0 {
		parts = append(parts, thousands(t.Already)+" already here")
	}
	if missing := found - t.Matched; missing > 0 {
		parts = append(parts, thousands(missing)+" not on TangoTube")
	}
	return strings.Join(parts, " · ")
}

func nouns(n int, one, many string) string {
	switch n {
	case 0:
		return ""
	case 1:
		return "1 " + one
	}
	return thousands(n) + " " + many
}

// sentenceList joins the non-empty parts as a sentence would: "a, b, and c".
func sentenceList(parts ...string) string {
	var kept []string
	for _, s := range parts {
		if s != "" {
			kept = append(kept, s)
		}
	}
	switch len(kept) {
	case 0:
		return ""
	case 1:
		return kept[0]
	case 2:
		return kept[0] + " and " + kept[1]
	}
	return strings.Join(kept[:len(kept)-1], ", ") + ", and " + kept[len(kept)-1]
}

// shellWord quotes a path for a next-step line when the shell would split it.
func shellWord(s string) string {
	if s != "" && !strings.ContainsAny(s, " '\"$`\\!*?&;|<>()[]{}#~") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
