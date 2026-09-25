package commands

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/justinallenmarsh/tangotube-cli/internal/api"
	"github.com/justinallenmarsh/tangotube-cli/internal/output"
)

func enum(desc string, values ...string) map[string]any {
	return map[string]any{"type": "string", "description": desc, "enum": values}
}

func boolean(desc string) map[string]any {
	return map[string]any{"type": "boolean", "description": desc}
}

func strs(desc string) map[string]any {
	return map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": desc}
}

func arg(args map[string]any, k string) string {
	if v, ok := args[k].(string); ok {
		return v
	}
	return ""
}

func stringList(args map[string]any, k string) []string {
	var out []string
	if raw, ok := args[k].([]any); ok {
		for _, v := range raw {
			out = append(out, fmt.Sprint(v))
		}
	}
	return out
}

// body copies the named arguments that were given into a request body.
func body(args map[string]any, keys ...string) map[string]any {
	b := map[string]any{}
	for _, k := range keys {
		if v, ok := args[k]; ok && v != nil && v != "" {
			b[k] = v
		}
	}
	return b
}

// collectionKeys are the filters the liked and history pages share.
var collectionKeys = []string{"q", "leader", "follower", "orchestra", "genre", "event", "channel", "category", "year", "sort", "limit", "cursor"}

func withCollection(props map[string]any, sorts ...string) map[string]any {
	for k, v := range map[string]any{
		"q": str("A word in the title."), "leader": str("A dancer who leads."), "follower": str("A dancer who follows."),
		"orchestra": str("Orchestra name or slug."), "genre": str("tango, vals, or milonga."), "event": str("Event name or slug."),
		"channel": str("YouTube channel title or id."), "category": str("Kind of video: performance, class…"),
		"year": num("Year filmed."), "sort": enum("Order.", sorts...), "limit": num("Up to 50."), "cursor": str("next_cursor from the last page."),
	} {
		props[k] = v
	}
	return props
}

func need(args map[string]any, keys ...string) error {
	for _, k := range keys {
		if args[k] == nil || args[k] == "" {
			return Usage(k+" is needed for this action", "tools/list names the arguments")
		}
	}
	return nil
}

// libraryTools are the signed-in person's own things: likes, history,
// playlists, follows, practice, saved searches, notifications, and edits to
// their clips. Each needs the token tt already has.
func libraryTools() []mcpTool {
	return []mcpTool{
		{
			Name:        "like",
			Description: "Like or unlike a video (by YouTube id) or a practice clip (by slug). Liking twice is liking once; liking someone's clip tells them.",
			InputSchema: schema(nil, map[string]any{
				"video": str("YouTube id."), "clip": str("Clip slug."), "unlike": boolean("Take the like back instead."),
			}),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				q := url.Values{}
				switch {
				case arg(args, "clip") != "":
					q.Set("clip_id", NormalizeID(arg(args, "clip")))
				case arg(args, "video") != "":
					q.Set("video_id", NormalizeID(arg(args, "video")))
				default:
					return nil, Usage("Say what to like: video or clip", "")
				}
				if unlike, _ := args["unlike"].(bool); unlike {
					return cl.Do(c, http.MethodDelete, "/api/v1/likes", q, nil, true)
				}
				return cl.Do(c, http.MethodPost, "/api/v1/likes", nil, valuesBody(q), true)
			},
		},
		{
			Name:        "likes",
			Description: "What the signed-in person has liked: videos (filterable like the site's liked page) or clips.",
			InputSchema: schema(nil, withCollection(map[string]any{"kind": enum("What to list.", "videos", "clips")}, "recent", "newest", "popular")),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				return cl.Do(c, http.MethodGet, "/api/v1/me/likes", query(args, append(collectionKeys, "kind")...), nil, true)
			},
		},
		{
			Name:        "history",
			Description: "Watch history, newest first, with the site's history filters. Each entry: watched_at, progress_s, completed, video.",
			InputSchema: schema(nil, withCollection(map[string]any{}, "recent", "oldest", "popular")),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				return cl.Do(c, http.MethodGet, "/api/v1/me/history", query(args, collectionKeys...), nil, true)
			},
		},
		{
			Name:        "history_edit",
			Description: "Change watch history: add a watch (at backdates it, ISO 8601), remove one video, or clear all (confirm must be true).",
			InputSchema: schema([]string{"action"}, map[string]any{
				"action": enum("What to do.", "add", "remove", "clear"), "video": str("YouTube id, for add and remove."),
				"at": str("When it was watched, for add: 2024-05-02T21:30:00Z."), "confirm": boolean("Must be true to clear."),
			}),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				switch arg(args, "action") {
				case "add":
					if err := need(args, "video"); err != nil {
						return nil, err
					}
					return cl.Do(c, http.MethodPost, "/api/v1/me/history", nil,
						body(map[string]any{"video_id": NormalizeID(arg(args, "video")), "at": args["at"]}, "video_id", "at"), true)
				case "remove":
					if err := need(args, "video"); err != nil {
						return nil, err
					}
					return cl.Do(c, http.MethodDelete, "/api/v1/me/history/"+url.PathEscape(NormalizeID(arg(args, "video"))), nil, nil, true)
				default:
					return cl.Do(c, http.MethodDelete, "/api/v1/me/history", query(args, "confirm"), nil, true)
				}
			},
		},
		{
			Name:        "playlists",
			Description: "The signed-in person's playlists; or with id, one playlist (anyone's public or unlisted, or your own) and its videos in order.",
			InputSchema: schema(nil, map[string]any{
				"id": str("A playlist's slug, to show that one."), "q": str("A word in the title."),
				"sort": enum("Order of the list.", "recent", "oldest", "alphabetical", "largest"), "limit": num("Up to 50."), "cursor": str("next_cursor."),
			}),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				if id := arg(args, "id"); id != "" {
					return cl.Get(c, playlistPath(id), query(args, "limit", "cursor"))
				}
				return cl.Do(c, http.MethodGet, "/api/v1/playlists", query(args, "q", "sort", "limit", "cursor"), nil, true)
			},
		},
		{
			Name: "playlist_edit",
			Description: "Make and change the signed-in person's playlists: create (title), update (title, description, visibility), delete, " +
				"add or remove a video, or move a video to a position (1 is first). Adding a video already there leaves it put.",
			InputSchema: schema([]string{"action"}, map[string]any{
				"action": enum("What to do.", "create", "update", "delete", "add", "remove", "move"),
				"id":     str("The playlist's slug; every action but create."), "title": str("Title, for create and update."),
				"description": str("For create and update."), "visibility": enum("Who can see it.", "private", "unlisted", "public"),
				"video": str("YouTube id: for add, remove, move; optional first video for create."), "position": num("For move: 1 is first."),
			}),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				action := arg(args, "action")
				if action != "create" {
					if err := need(args, "id"); err != nil {
						return nil, err
					}
				}
				id, video := arg(args, "id"), NormalizeID(arg(args, "video"))
				switch action {
				case "create":
					if err := need(args, "title"); err != nil {
						return nil, err
					}
					b := body(args, "title", "description", "visibility")
					if video != "" {
						b["video_id"] = video
					}
					return cl.Do(c, http.MethodPost, "/api/v1/playlists", nil, b, true)
				case "update":
					return cl.Do(c, http.MethodPatch, playlistPath(id), nil, body(args, "title", "description", "visibility"), true)
				case "delete":
					return cl.Do(c, http.MethodDelete, playlistPath(id), nil, nil, true)
				case "add":
					if err := need(args, "video"); err != nil {
						return nil, err
					}
					return cl.Do(c, http.MethodPost, playlistPath(id, "/items"), nil, map[string]any{"video_id": video}, true)
				case "remove":
					if err := need(args, "video"); err != nil {
						return nil, err
					}
					return cl.Do(c, http.MethodDelete, playlistPath(id, "/items/", url.PathEscape(video)), nil, nil, true)
				default:
					if err := need(args, "video", "position"); err != nil {
						return nil, err
					}
					return cl.Do(c, http.MethodPut, playlistPath(id, "/order"), nil, map[string]any{"video_id": video, "position": args["position"]}, true)
				}
			},
		},
		{
			Name: "follow",
			Description: "Follow or unfollow a dancer, YouTube channel, or event, by name or slug; level sets how much the person hears. " +
				"Following twice is following once; a first follow turns on the weekly email, as on the site.",
			InputSchema: schema([]string{"kind", "name"}, map[string]any{
				"kind": enum("What to follow.", "dancer", "channel", "event"), "name": str("Name, slug, or channel id."),
				"level": enum("How much to hear.", "personalized", "all_activity", "muted"), "unfollow": boolean("Stop following instead."),
			}),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				q := url.Values{arg(args, "kind"): {arg(args, "name")}}
				if unfollow, _ := args["unfollow"].(bool); unfollow {
					return cl.Do(c, http.MethodDelete, "/api/v1/follows", q, nil, true)
				}
				if level := arg(args, "level"); level != "" {
					q.Set("level", level)
				}
				return cl.Do(c, http.MethodPost, "/api/v1/follows", nil, valuesBody(q), true)
			},
		},
		{
			Name:        "following",
			Description: "Who the signed-in person follows; or with feed, what those dancers, channels and events posted in the last 30 days, a few videos from each (limit is per group).",
			InputSchema: schema(nil, map[string]any{
				"feed": boolean("Their new videos instead of who they are."), "kind": enum("Only one kind.", "dancers", "channels", "events"),
				"limit": num("Up to 50; with feed, videos per group."), "cursor": str("next_cursor."),
			}),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				return cl.Do(c, http.MethodGet, "/api/v1/me/following", query(args, "feed", "kind", "limit", "cursor"), nil, true)
			},
		},
		{
			Name:        "practice",
			Description: "The practice list: clips the signed-in person saved to loop. action add or remove changes it; without one, it lists (filter by technique, style, genre, dancer, orchestra).",
			InputSchema: schema(nil, map[string]any{
				"action": enum("Change the list instead of listing it.", "add", "remove"), "clip": str("Clip slug, for add and remove."),
				"q": str("A word in the title."), "technique": str("Clip tag, like sacada."), "style": str("Style tag."),
				"genre": str("tango, vals, or milonga."), "dancer": str("Dancer name or slug."), "orchestra": str("Orchestra name or slug."),
				"sort": enum("Order.", "recent", "oldest", "popular"), "limit": num("Up to 50."), "cursor": str("next_cursor."),
			}),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				clip := NormalizeID(arg(args, "clip"))
				switch arg(args, "action") {
				case "add":
					if err := need(args, "clip"); err != nil {
						return nil, err
					}
					return cl.Do(c, http.MethodPost, "/api/v1/me/practice", nil, map[string]any{"clip_id": clip}, true)
				case "remove":
					if err := need(args, "clip"); err != nil {
						return nil, err
					}
					return cl.Do(c, http.MethodDelete, "/api/v1/me/practice/"+url.PathEscape(clip), nil, nil, true)
				}
				return cl.Do(c, http.MethodGet, "/api/v1/me/practice",
					query(args, "q", "technique", "style", "genre", "dancer", "orchestra", "sort", "limit", "cursor"), nil, true)
			},
		},
		{
			Name: "saved_searches",
			Description: "Saved searches, each with the tt search command and site link that run it. action save stores search's filters under name " +
				"(a name already used is replaced); remove deletes by name or id.",
			InputSchema: schema(nil, withFilters(map[string]any{
				"action": enum("Change them instead of listing.", "save", "remove"), "name": str("The saved search's name (or id, for remove)."),
				"q": str("Free text, for save."), "sort": str("Search order, for save."),
			})),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				switch arg(args, "action") {
				case "save":
					if err := need(args, "name"); err != nil {
						return nil, err
					}
					return cl.Do(c, http.MethodPost, "/api/v1/me/saved_searches", nil, body(args, append(filterKeys, "name", "q", "sort")...), true)
				case "remove":
					if err := need(args, "name"); err != nil {
						return nil, err
					}
					return cl.Do(c, http.MethodDelete, "/api/v1/me/saved_searches/"+url.PathEscape(arg(args, "name")), nil, nil, true)
				}
				return cl.Do(c, http.MethodGet, "/api/v1/me/saved_searches", url.Values{"limit": {"50"}}, nil, true)
			},
		},
		{
			Name:        "notifications",
			Description: "Notifications, newest first, each a sentence with the page it is about. mark_read marks them all read.",
			InputSchema: schema(nil, map[string]any{
				"unread": boolean("Only unread ones."), "mark_read": boolean("Mark every notification read."),
				"limit": num("Up to 50."), "cursor": str("next_cursor."),
			}),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				if mark, _ := args["mark_read"].(bool); mark {
					return cl.Do(c, http.MethodPost, "/api/v1/me/notifications/read", nil, nil, true)
				}
				return cl.Do(c, http.MethodGet, "/api/v1/me/notifications", query(args, "unread", "limit", "cursor"), nil, true)
			},
		},
		{
			Name:        "clip_edit",
			Description: "Edit one of the signed-in person's clips. What is not given stays. tags replaces the tags; add_tags and remove_tags change them. Times are m:ss or seconds.",
			InputSchema: schema([]string{"id"}, map[string]any{
				"id": str("Clip slug."), "title": str("New title."), "description": str("What to watch for; empty clears it."),
				"start": str("New start, m:ss."), "end": str("New end, m:ss."), "visibility": enum("Who can see it.", "private", "unlisted", "public"),
				"tags": strs("Replace the tags."), "add_tags": strs("Tags to add."), "remove_tags": strs("Tags to take off."),
			}),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				b := body(args, "title", "visibility")
				if d, ok := args["description"].(string); ok {
					b["description"] = d
				}
				for name, key := range map[string]string{"start": "start_s", "end": "end_s"} {
					if v := arg(args, name); v != "" {
						s, err := ParseTime(v)
						if err != nil {
							return nil, Usage(name+": "+err.Error(), "")
						}
						b[key] = s
					}
				}
				for _, k := range []string{"tags", "add_tags", "remove_tags"} {
					if _, ok := args[k]; ok {
						b[k] = splitTags(stringList(args, k))
					}
				}
				return cl.Do(c, http.MethodPatch, "/api/v1/clips/"+url.PathEscape(NormalizeID(arg(args, "id"))), nil, b, true)
			},
		},
	}
}
