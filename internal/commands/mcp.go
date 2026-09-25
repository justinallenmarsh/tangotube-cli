package commands

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/justinallenmarsh/tangotube-cli/internal/api"
	"github.com/justinallenmarsh/tangotube-cli/internal/output"
)

func newMCP(a *App) *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Serve tt to an MCP client over stdio.",
		Long: `Serve tt as a Model Context Protocol server over stdio, for agents that speak
MCP rather than shell. The tools are the same calls tt makes: search, facets,
resolve a name, the front page, the catalogue's lists, show a video, dancer,
event or song, practice clips, and the signed-in person's own things: likes,
history, playlists, follows and their feed, the practice list, saved
searches and notifications. It uses the token tt already has; there is no
separate login. With an operator's admin token (tt auth login --admin) it also
serves the tt admin tools.`,
		Example: `  claude mcp add tangotube -- tt mcp`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return serveMCP(cmd.Context(), a, a.In, a.Out)
		},
	}
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type mcpTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
	call        func(context.Context, *api.Client, map[string]any) (*output.Envelope, error)
}

func str(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }
func num(desc string) map[string]any { return map[string]any{"type": "integer", "description": desc} }

// schema is a tool's input: these properties and no others, so a caller that
// guesses "query" for "q" hears about it instead of getting an unrelated list.
func schema(required []string, props map[string]any) map[string]any {
	s := map[string]any{"type": "object", "properties": props, "additionalProperties": false}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

func query(args map[string]any, keys ...string) url.Values {
	q := url.Values{}
	for _, k := range keys {
		switch v := args[k].(type) {
		case string:
			if v != "" {
				q.Set(k, v)
			}
		case float64:
			q.Set(k, fmt.Sprint(int(v)))
		case bool:
			q.Set(k, fmt.Sprint(v))
		}
	}
	return q
}

// filterKeys are search's filters, shared by search and facets.
var filterKeys = []string{"dancer", "leader", "follower", "couple", "orchestra", "song", "event", "channel",
	"genre", "kind", "year", "year_from", "year_to", "uploaded", "hd"}

func withFilters(props map[string]any) map[string]any {
	for k, v := range map[string]any{
		"dancer": str("Dancer name or slug."), "leader": str("A dancer who leads."), "follower": str("A dancer who follows."),
		"couple": str("Couple: both names, or the slug."), "orchestra": str("Orchestra name or slug."),
		"song": str("Song title or slug."), "event": str("Event name or slug."), "channel": str("YouTube channel title or id."),
		"genre": str("tango, vals, or milonga."), "kind": str("performance, class, workshop, interview, competition…"),
		"year": num("Year the music was recorded."), "year_from": num("Recorded in or after."), "year_to": num("Recorded in or before."),
		"uploaded": num("Year the video went up."), "hd": map[string]any{"type": "boolean", "description": "Only HD."},
	} {
		props[k] = v
	}
	return props
}

var (
	listKinds = []string{"dancers", "couples", "orchestras", "songs", "events", "channels", "singers", "champions"}
	showKinds = []string{"dancer", "couple", "orchestra", "song", "event", "channel"}
)

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func mcpTools() []mcpTool {
	return append(append(catalogueTools(), libraryTools()...), helpTools()...)
}

func catalogueTools() []mcpTool {
	return []mcpTool{
		{
			Name:        "search",
			Description: "Search Argentine tango performance videos by text, dancer, leader, follower, couple, orchestra, song, event, channel, genre, kind of video, recording year, upload year — or browse the whole catalogue with sort alone. Through practice clips: technique and style. Names or slugs both work.",
			InputSchema: schema(nil, withFilters(map[string]any{
				"q":         str("Free text, as typed into the site's search box."),
				"sort":      map[string]any{"type": "string", "enum": []string{"browse", "trending", "popular", "newest", "oldest", "hidden-gems"}},
				"style":     str("milonguero, salon, nuevo, or stage."),
				"technique": str("A clip tag such as sacada."),
				"limit":     num("Up to 50."), "cursor": str("next_cursor from the previous page."),
			})),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				return cl.Get(c, "/api/v1/search", query(args, append(filterKeys, "q", "sort", "style", "technique", "limit", "cursor")...))
			},
		},
		{
			Name:        "facets",
			Description: "Count what a search holds before reading it: leaders, followers, couples, orchestras, songs, events, channels, genres, kinds of video, and a histogram of the years the music was recorded. Counts are performances. Same filters as search.",
			InputSchema: schema(nil, withFilters(map[string]any{
				"q": str("Free text."), "limit": num("Options per facet, up to 25."),
			})),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				return cl.Get(c, "/api/v1/facets", query(args, append(filterKeys, "q", "limit")...))
			},
		},
		{
			Name:        "resolve",
			Description: "What a typed name means before searching: matching dancers, couples, orchestras, songs, events and channels, and how search would read the words as filters. Offers a spelling when nothing matches.",
			InputSchema: schema([]string{"q"}, map[string]any{"q": str("The words, like \"di sarli noelia\".")}),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				return cl.Get(c, "/api/v1/resolve", query(args, "q"))
			},
		},
		{
			Name:        "home",
			Description: "What the front page of tangotube.tv shows today: trending, new uploads, the featured dancer, orchestra, song, event and channel, classes, hidden gems, the Mundial — each with the search that shows all of it.",
			InputSchema: schema(nil, map[string]any{}),
			call: func(c context.Context, cl *api.Client, _ map[string]any) (*output.Envelope, error) {
				return cl.Get(c, "/api/v1/home", nil)
			},
		},
		{
			Name:        "catalogue_list",
			Description: "List the catalogue a page at a time: dancers, couples, orchestras, songs, events, channels, singers, or Mundial champions. Use it to enumerate instead of guessing names.",
			InputSchema: schema([]string{"kind"}, map[string]any{
				"kind":       map[string]any{"type": "string", "enum": listKinds},
				"q":          str("Only names matching these words."),
				"role":       str("dancers: leader or follower."),
				"champion":   map[string]any{"type": "boolean", "description": "dancers: only Mundial champions."},
				"era":        str("orchestras: golden-age or contemporary."),
				"orchestra":  str("songs, singers: an orchestra's name or slug."),
				"genre":      str("songs: tango, vals, or milonga."),
				"decade":     num("songs: like 1940."),
				"country":    str("events: a country."),
				"continent":  str("events: a continent."),
				"category":   str("events: the kind of event; champions: pista (salon) or escenario."),
				"year":       num("champions: one year's Mundial."),
				"min_videos": num("couples: filmed at least this often."),
				"sort":       str("Each list's orders; popular is the default."),
				"limit":      num("Up to 50."), "cursor": str("next_cursor from the previous page."),
			}),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				kind := fmt.Sprint(args["kind"])
				if !contains(listKinds, kind) {
					return nil, Usage("kind is one of "+strings.Join(listKinds, ", "), "")
				}
				q := query(args, "q", "role", "era", "orchestra", "genre", "decade", "country", "continent", "category", "year", "min_videos", "sort", "limit", "cursor")
				if champion, _ := args["champion"].(bool); champion {
					q.Set("champion", "1")
				}
				return cl.Get(c, "/api/v1/"+kind, q)
			},
		},
		{
			Name:        "video_show",
			Description: "One performance by its YouTube id: dancers, song, orchestra, year, event, clips, related videos. With identity: each fact settled, proposed (and by what), or missing, and any answer waiting for a yes.",
			InputSchema: schema([]string{"id"}, map[string]any{"id": str("The YouTube id."), "identity": boolean("Add what TangoTube knows and how.")}),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				var q url.Values
				if identity, _ := args["identity"].(bool); identity {
					q = url.Values{"include": {"identity"}}
				}
				return cl.Get(c, "/api/v1/videos/"+url.PathEscape(NormalizeID(fmt.Sprint(args["id"]))), q)
			},
		},
		{
			Name:        "entity_show",
			Description: "A dancer, couple, orchestra, song, event or channel by slug, with its performances. A channel's slug is its YouTube id.",
			InputSchema: schema([]string{"kind", "slug"}, map[string]any{
				"kind":    map[string]any{"type": "string", "enum": showKinds},
				"slug":    str("The slug, like noelia-hurtado or carlos-di-sarli."),
				"lyrics":  map[string]any{"type": "boolean", "description": "Songs only: include the lyrics, Spanish and English (en_source says when the English is a machine translation)."},
				"include": str("Dancers only: any of timeline, tour, repertoire, comma-separated."),
				"year":    num("Events only: one edition."),
			}),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				kind := fmt.Sprint(args["kind"])
				if !contains(showKinds, kind) {
					return nil, Usage("kind is one of "+strings.Join(showKinds, ", "), "")
				}
				q := url.Values{}
				if withLyrics, _ := args["lyrics"].(bool); withLyrics && kind == "song" {
					q.Set("lyrics", "1")
				}
				if kind == "dancer" {
					q = query(args, "include")
				}
				if kind == "event" {
					q = query(args, "year")
				}
				return cl.Get(c, "/api/v1/"+kind+"s/"+url.PathEscape(fmt.Sprint(args["slug"])), q)
			},
		},
		{
			Name:        "song_versions",
			Description: "The other recordings of a song's composition: the same tango by other orchestras or singers, most danced first.",
			InputSchema: schema([]string{"slug"}, map[string]any{"slug": str("The song's slug.")}),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				return cl.Get(c, "/api/v1/songs/"+url.PathEscape(fmt.Sprint(args["slug"]))+"/versions", nil)
			},
		},
		{
			Name:        "performance_show",
			Description: "One dance and every video of it (the cameras that filmed it, by channel, length and views), the couple, the song with its credits, the occasion, and the couple's other dances that occasion in order. The id is the YouTube id of any video of the dance, or the performance id video_show returns as performance.",
			InputSchema: schema([]string{"id"}, map[string]any{"id": str("A YouTube id of any video of the dance, or a performance id.")}),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				return cl.Get(c, "/api/v1/performances/"+url.PathEscape(NormalizeID(fmt.Sprint(args["id"]))), nil)
			},
		},
		{
			Name:        "partners",
			Description: "Who a dancer has danced with on camera, most filmed together first, each with the couple's slug. Depth 2 adds who those partners dance with: nodes carry ring (0 the dancer, 1 a partner, 2 a partner's partner), edges are partnerships weighted by videos.",
			InputSchema: schema([]string{"slug"}, map[string]any{
				"slug":  str("The dancer's slug, like noelia-hurtado."),
				"depth": map[string]any{"type": "integer", "enum": []int{1, 2}, "description": "1 (default) for partners, 2 to add their partners."},
			}),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				return cl.Get(c, "/api/v1/dancers/"+url.PathEscape(fmt.Sprint(args["slug"]))+"/partners", query(args, "depth"))
			},
		},
		{
			Name:        "clip_list",
			Description: "Practice clips: on one video, tagged with a technique or style, of a dancer, or the user's own (mine).",
			InputSchema: schema(nil, map[string]any{
				"video": str("YouTube id."), "technique": str("Clip tag."), "style": str("Style tag."),
				"dancer": str("Dancer name or slug."), "mine": map[string]any{"type": "boolean"}, "limit": num("Up to 50."),
			}),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				mine, _ := args["mine"].(bool)
				return cl.Do(c, http.MethodGet, "/api/v1/clips", query(args, "video", "technique", "style", "dancer", "mine", "limit"), nil, mine)
			},
		},
		{
			Name:        "clip_create",
			Description: "Make a practice clip on a video for the signed-in user. Times are m:ss or seconds; tags come from clip_tags.",
			InputSchema: schema([]string{"video", "start", "end"}, map[string]any{
				"video": str("YouTube id."), "start": str("Start, m:ss."), "end": str("End, m:ss."),
				"tags":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"title": str("Optional title."), "visibility": str("private, unlisted, or public."),
			}),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				start, err := ParseTime(fmt.Sprint(args["start"]))
				if err != nil {
					return nil, Usage("start: "+err.Error(), "")
				}
				end, err := ParseTime(fmt.Sprint(args["end"]))
				if err != nil {
					return nil, Usage("end: "+err.Error(), "")
				}
				var tags []string
				if raw, ok := args["tags"].([]any); ok {
					for _, t := range raw {
						tags = append(tags, fmt.Sprint(t))
					}
				}
				body := map[string]any{"start_s": start, "end_s": end, "tags": splitTags(tags)}
				for _, k := range []string{"title", "visibility"} {
					if v, ok := args[k].(string); ok && v != "" {
						body[k] = v
					}
				}
				return cl.Do(c, http.MethodPost, "/api/v1/videos/"+url.PathEscape(NormalizeID(fmt.Sprint(args["video"])))+"/clips", nil, body, true)
			},
		},
		{
			Name:        "clip_tags",
			Description: "The techniques and styles a clip can be tagged with.",
			InputSchema: schema(nil, map[string]any{}),
			call: func(c context.Context, cl *api.Client, _ map[string]any) (*output.Envelope, error) {
				return cl.Get(c, "/api/v1/tags", nil)
			},
		},
	}
}

func serveMCP(c context.Context, a *App, in io.Reader, out io.Writer) error {
	tools := mcpTools()
	if a.operatorToken(c) {
		tools = append(tools, allAdminTools()...)
	}
	enc := json.NewEncoder(out)
	reply := func(id json.RawMessage, result any, rpcErr map[string]any) {
		msg := map[string]any{"jsonrpc": "2.0", "id": id}
		if rpcErr != nil {
			msg["error"] = rpcErr
		} else {
			msg["result"] = result
		}
		_ = enc.Encode(msg)
	}

	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	for scanner.Scan() {
		var req rpcRequest
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			reply(json.RawMessage("null"), nil, map[string]any{"code": -32700, "message": "parse error"})
			continue
		}
		if len(req.ID) == 0 {
			continue // a notification wants no answer
		}
		switch req.Method {
		case "initialize":
			var params struct {
				ProtocolVersion string `json:"protocolVersion"`
			}
			_ = json.Unmarshal(req.Params, &params)
			if params.ProtocolVersion == "" {
				params.ProtocolVersion = "2025-06-18"
			}
			reply(req.ID, map[string]any{
				"protocolVersion": params.ProtocolVersion,
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo":      map[string]any{"name": "tangotube", "version": Version},
				"instructions":    "Search and browse Argentine tango videos and make practice clips. Resolve a name before searching with it; list the catalogue instead of guessing slugs. Every result is a TangoTube envelope: ok, data, summary, breadcrumbs.",
			}, nil)
		case "ping":
			reply(req.ID, map[string]any{}, nil)
		case "tools/list":
			reply(req.ID, map[string]any{"tools": tools}, nil)
		case "tools/call":
			var params struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
			}
			_ = json.Unmarshal(req.Params, &params)
			reply(req.ID, callTool(c, a, tools, params.Name, params.Arguments), nil)
		default:
			reply(req.ID, nil, map[string]any{"code": -32601, "message": "method not found: " + req.Method})
		}
	}
	return scanner.Err()
}

func callTool(c context.Context, a *App, tools []mcpTool, name string, args map[string]any) map[string]any {
	for _, t := range tools {
		if t.Name != name {
			continue
		}
		var env *output.Envelope
		var err error
		preview := destructiveTools[name] && args["confirm"] != true
		if unknown := unknownArgs(t, args); len(unknown) > 0 {
			err = Usage("Unknown argument "+strings.Join(unknown, ", ")+" for "+name, "tools/list names the arguments")
		} else {
			if preview {
				args = withDryRun(args)
			}
			env, err = t.call(c, a.Client(), args)
		}
		if err != nil {
			if apiErr, ok := api.AsError(err); ok {
				env = apiErr.Envelope
			} else {
				env = output.Fail(output.CodeUsage, err.Error(), "")
			}
		}
		if preview && env.OK && isDryRun(env.Data) {
			env.Summary += previewOnly
			env.Raw = nil
		}
		raw := env.Raw
		if len(raw) == 0 {
			raw, _ = json.Marshal(env)
		}
		return map[string]any{"content": []map[string]any{{"type": "text", "text": string(raw)}}, "isError": !env.OK}
	}
	return map[string]any{"content": []map[string]any{{"type": "text", "text": "no tool named " + name}}, "isError": true}
}

func unknownArgs(t mcpTool, args map[string]any) []string {
	props, _ := t.InputSchema["properties"].(map[string]any)
	var unknown []string
	for k := range args {
		if _, ok := props[k]; !ok {
			unknown = append(unknown, k)
		}
	}
	sort.Strings(unknown)
	return unknown
}

// withDryRun is args with dry_run set, leaving the caller's map alone.
func withDryRun(args map[string]any) map[string]any {
	out := map[string]any{"dry_run": true}
	for k, v := range args {
		if k != "dry_run" {
			out[k] = v
		}
	}
	return out
}

func isDryRun(data json.RawMessage) bool {
	var d struct {
		DryRun bool `json:"dry_run"`
	}
	return json.Unmarshal(data, &d) == nil && d.DryRun
}
