package commands

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/justinallenmarsh/tangotube-cli/internal/api"
	"github.com/justinallenmarsh/tangotube-cli/internal/output"
)

// videoPath is /api/v1/videos/ID plus the rest.
func videoPath(args map[string]any, rest string) string {
	return "/api/v1/videos/" + url.PathEscape(NormalizeID(arg(args, "video"))) + rest
}

// helpTools are how a person, through their agent, tells TangoTube what a
// video is: the site's rules decide whether it applies, and every answer says.
func helpTools() []mcpTool {
	return []mcpTool{
		{
			Name: "suggest",
			Description: "Name a video's dancer, song, event, or kind, as the watch page asks. Only suggest what the person told you or confirmed. " +
				"The answer says outcome: applied (on the video now) or queued (waiting for review); tell them which. " +
				"For a dancer, reach says where it shows: occasion (their dancer page lists it now) or placing (once the video is placed, within 15 minutes); pass the summary on as written. " +
				"Records by slug or exact name (identify_search finds them); a near miss is an error listing candidates. " +
				"agree: true with song says it is the song the video already proposes, which applies at once. " +
				"changes: several at once, all or nothing, each {fact: song|dancers|event|kind, op: add|replace|remove|withdraw, song|dancer|event|kind|name}.",
			InputSchema: schema([]string{"video"}, map[string]any{
				"video": str("YouTube id."), "fact": enum("What is being named.", "dancer", "song", "event", "kind"),
				"dancer": str("Dancer slug or exact name."), "song": str("Song slug or exact title."),
				"event": str("Event slug or exact name."), "kind": str("performance, class, practice…"),
				"role": enum("With dancer.", "leader", "follower"), "precision": enum("With song: the tune and orchestra, or that recording.", "rendition", "take"),
				"note": str("How the person knows, for the reviewer."), "agree": boolean("With song: the one the video proposes."),
				"new":     boolean("With dancer: somebody TangoTube does not have yet."),
				"changes": map[string]any{"type": "array", "items": map[string]any{"type": "object"}, "description": "A batch instead of one fact."},
			}),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				if changes, ok := args["changes"].([]any); ok {
					return cl.Do(c, http.MethodPost, videoPath(args, "/suggestions/batch"), nil, map[string]any{"changes": changes}, true)
				}
				if err := need(args, "fact"); err != nil {
					return nil, err
				}
				return cl.Do(c, http.MethodPost, videoPath(args, "/suggestions"), nil,
					body(args, "fact", "dancer", "song", "event", "kind", "role", "precision", "note", "agree", "new"), true)
			},
		},
		{
			Name:        "confirm",
			Description: "Say the dancers credited on a video are right. Once per person; enough people confirming verifies them.",
			InputSchema: schema([]string{"video"}, map[string]any{"video": str("YouTube id.")}),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				return cl.Do(c, http.MethodPost, videoPath(args, "/confirmation"), nil, map[string]any{}, true)
			},
		},
		{
			Name:        "agree",
			Description: "Agree with somebody else's answer waiting on a video (its suggestion id is in video_show with identity). Three people agreeing applies it.",
			InputSchema: schema([]string{"video", "suggestion"}, map[string]any{"video": str("YouTube id."), "suggestion": num("The suggestion's id.")}),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				return cl.Do(c, http.MethodPost, videoPath(args, fmt.Sprintf("/suggestions/%v/agree", args["suggestion"])), nil, map[string]any{}, true)
			},
		},
		{
			Name:        "report",
			Description: "Say something on a video is wrong, as the report button does. Needs no account. dancer reports that one credit.",
			InputSchema: schema([]string{"video", "kind"}, map[string]any{
				"video":  str("YouTube id."),
				"kind":   enum("What is wrong.", "wrong_dancer", "wrong_role", "wrong_song", "wrong_event", "wrong_kind", "not_tango", "duplicate", "other"),
				"dancer": str("The credited dancer who is wrong."), "note": str("For whoever looks at it."),
			}),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				b := body(args, "kind", "dancer", "note")
				b["video_id"] = NormalizeID(arg(args, "video"))
				return cl.Do(c, http.MethodPost, "/api/v1/reports", nil, b, false)
			},
		},
		{
			Name:        "tag_suggest",
			Description: "Suggest a step name the clip tags do not have yet; an admin adds it. Three a day. A step that exists is added with clip_edit instead.",
			InputSchema: schema([]string{"clip", "name"}, map[string]any{"clip": str("Clip slug."), "name": str("The step, e.g. calesita.")}),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				return cl.Do(c, http.MethodPost, "/api/v1/clips/"+url.PathEscape(arg(args, "clip"))+"/tag_suggestions", nil, body(args, "name"), true)
			},
		},
		{
			Name:        "queue",
			Description: "Videos with something still to name (no song, dancers, or event), most watched first, with what is open on each and any proposal waiting for a yes. proposed: only those.",
			InputSchema: schema(nil, map[string]any{
				"fact": enum("Only videos missing this.", "song", "dancers", "event"), "dancer": str("Dancer name or slug."),
				"orchestra": str("Orchestra name or slug; a songless video counts by its proposed song."), "event": str("Event name or slug."),
				"channel": str("Channel title or YouTube id."), "proposed": boolean("Only videos with an answer waiting for a yes."),
				"limit": num("Up to 25."), "cursor": str("next_cursor."),
			}),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				return cl.Get(c, "/api/v1/identify/queue", query(args, "fact", "dancer", "orchestra", "event", "channel", "proposed", "limit", "cursor"))
			},
		},
		{
			Name:        "identify_search",
			Description: "Find the song, dancer, or event to name a video with, as the watch page's picker does, and its slug. Songs come with orchestra and each recording's year.",
			InputSchema: schema([]string{"kind", "q"}, map[string]any{
				"kind": enum("What to find.", "song", "dancer", "event"), "q": str("At least two letters."), "limit": num("Up to 25."),
			}),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				return cl.Get(c, "/api/v1/identify/search", query(args, "kind", "q", "limit"))
			},
		},
	}
}
