package commands

import (
	"net/url"
	"os/exec"
	"regexp"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/justinallenmarsh/tangotube-cli/internal/api"
	"github.com/justinallenmarsh/tangotube-cli/internal/output"
)

var youtubeID = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

func newOpen(a *App) *cobra.Command {
	return &cobra.Command{
		Use:   "open ID",
		Short: "Open a video, a practice clip, or a dancer's page on tangotube.tv.",
		Long: `Open a video, a practice clip, or a dancer, couple, orchestra, or song page on
tangotube.tv in your browser. Playback stays on YouTube's player; tt never
downloads video.

Given a video id it opens the watch page; given a clip id, the clip, looping.
A YouTube or tangotube.tv link works as well as an id. Piped or under --agent
it opens nothing and writes the URL instead.`,
		Example: `  tt open uGwRPRusbC0
  tt open sacada-1-cuando-el-amor-muere
  tt open noelia-hurtado
  tt open "https://youtu.be/uGwRPRusbC0" --agent --jq .url`,
		Args: exactArgs(1, "tt open uGwRPRusbC0"),
		RunE: func(cmd *cobra.Command, args []string) error {
			target, kind, err := a.resolveOpen(cmd, NormalizeID(args[0]))
			if err != nil {
				return a.Fail(err)
			}
			p := a.Printer()
			if !a.Interactive() {
				return p.Success(output.NewEnvelope(map[string]string{"url": target, "kind": kind}, target), nil)
			}
			if err := a.Browser(target); err != nil {
				p.Line("Open this in a browser: %s", target)
				return nil
			}
			p.Line("Opened %s", p.Style.Gold(target))
			return nil
		},
	}
}

// pages are the site pages tt open can land on, in the order an id is tried
// after videos and clips: the API path and the site path for each.
var pages = []struct{ kind, api, site string }{
	{"dancer", "/api/v1/dancers/", "/dancers/"},
	{"couple", "/api/v1/couples/", "/couples/"},
	{"orchestra", "/api/v1/orchestras/", "/orchestras/"},
	{"song", "/api/v1/songs/", "/songs/"},
}

// resolveOpen asks the API what an id is. A video wins; a clip is next; then
// a dancer, couple, orchestra, or song. With no API to ask, an
// eleven-character id is taken for a YouTube id.
func (a *App) resolveOpen(cmd *cobra.Command, id string) (string, string, error) {
	client := a.Client()
	base := a.BaseURL()
	env, err := client.Get(ctx(cmd), "/api/v1/videos/"+url.PathEscape(id), nil)
	if err == nil {
		v := decode[api.Video](env.Data)
		if v.WatchURL != "" {
			return v.WatchURL, "video", nil
		}
		return base + "/watch?v=" + url.QueryEscape(id), "video", nil
	}
	if apiErr, ok := api.AsError(err); ok && apiErr.Code() == output.CodeNetwork {
		if youtubeID.MatchString(id) {
			return base + "/watch?v=" + url.QueryEscape(id), "video", nil
		}
		return "", "", err
	}
	env, err = client.Get(ctx(cmd), "/api/v1/clips/"+url.PathEscape(id), nil)
	if err == nil {
		c := decode[api.Clip](env.Data)
		if c.ClipURL != "" {
			return c.ClipURL, "clip", nil
		}
		return base + "/clips/" + url.PathEscape(id), "clip", nil
	}
	if !notFound(err) {
		return "", "", err
	}
	for _, page := range pages {
		_, err = client.Get(ctx(cmd), page.api+url.PathEscape(id), nil)
		if err == nil {
			return base + page.site + url.PathEscape(id), page.kind, nil
		}
		if !notFound(err) {
			return "", "", err
		}
	}
	return "", "", api.Fail(output.CodeNotFound,
		"Nothing on TangoTube has that id. tt open takes a video, clip, dancer, couple, orchestra, or song id.",
		`tt search "di sarli"`)
}

func notFound(err error) bool {
	apiErr, ok := api.AsError(err)
	return ok && apiErr.Code() == output.CodeNotFound
}

func openBrowser(target string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", target).Start()
	case "windows":
		// start would read & in a URL as a new command; the URL handler does not.
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", target).Start()
	default:
		return exec.Command("xdg-open", target).Start()
	}
}
