package commands

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/justinallenmarsh/tangotube-cli/internal/api"
	"github.com/justinallenmarsh/tangotube-cli/internal/output"
)

// adminTools are tt admin as MCP tools. tt mcp lists them only when its token
// is an admin token held by an operator, so an agent never sees tools it
// cannot use; the API checks again on every call either way.
func adminTools() []mcpTool {
	get := func(path string, keys ...string) func(context.Context, *api.Client, map[string]any) (*output.Envelope, error) {
		return func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
			return cl.Do(c, http.MethodGet, path, query(args, keys...), nil, true)
		}
	}
	reports := []string{"dashboard", "pipeline", "intake", "search-quality", "channels", "precision", "jobs"}
	return []mcpTool{
		{
			Name: "admin_report",
			Description: "Operator only. One admin screen as JSON: dashboard (catalogue size, imports, people, search, audio), pipeline (imports by day, channel syncs, audio), " +
				"intake (this week's arrivals and how they were classified), search-quality (clicks, rank, zero-result queries), channels (scorecard), " +
				"precision (how right each matching method is), jobs (queues, cron schedule and what is paused, failures by id and error; admin_job and admin_cron change them). Read-only.",
			InputSchema: schema([]string{"report"}, map[string]any{
				"report": enum("Which screen.", reports...),
				"limit":  num("channels only: how many, up to 200."),
			}),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				report := arg(args, "report")
				if !contains(reports, report) {
					return nil, Usage("report is one of "+strings.Join(reports, ", "), "")
				}
				return cl.Do(c, http.MethodGet, "/api/v1/admin/"+report, query(args, "limit"), nil, true)
			},
		},
		{
			Name:        "admin_coverage",
			Description: "Operator only. The coverage desk: without pile, the heartbeat (how much is named, every alarm); with pile, that pile of work, deepest first. Read-only.",
			InputSchema: schema(nil, map[string]any{
				"pile":   enum("A pile of work.", "holes", "blank", "panel", "conflict", "miss", "label", "print", "payloads"),
				"window": enum("Only the last…", "24h", "7d", "all"),
				"clock":  enum("Count from when a video was…", "matched", "arrived"),
				"q":      str("Narrow the pile (3+ characters)."),
			}),
			call: get("/api/v1/admin/coverage", "pile", "window", "clock", "q"),
		},
		{
			Name:        "admin_desk",
			Description: "Operator only. Everything the record holds about one video: what each source said, what the ledger did, fact checks. Read-only.",
			InputSchema: schema([]string{"video"}, map[string]any{"video": str("YouTube id.")}),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				return cl.Do(c, http.MethodGet, "/api/v1/admin/desk/"+url.PathEscape(NormalizeID(arg(args, "video"))), nil, nil, true)
			},
		},
		{
			Name:        "admin_describe",
			Description: "Operator only. A kind of record: its table and columns (for admin_query), enum values, links, and the admin verbs that change it. Without type, the kinds there are.",
			InputSchema: schema(nil, map[string]any{
				"type": enum("Kind of record.", "video", "dancer", "couple", "orchestra", "singer", "song", "event", "channel", "performance"),
			}),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				path := "/api/v1/admin/describe"
				if t := arg(args, "type"); t != "" {
					path += "/" + url.PathEscape(t)
				}
				return cl.Do(c, http.MethodGet, path, nil, nil, true)
			},
		},
		{
			Name: "admin_query",
			Description: "Operator only. Ask the catalogue a question in SQL: one SELECT, read-only, 5 second limit, 200 rows by default (limit up to 2000). " +
				"Catalogue tables only (videos, dancers, couples, songs, orchestras, events, channels, performances, pipeline tables); never accounts, tokens, sessions, likes, watches or playlists. " +
				"Aggregates, text, number, date, array and JSON functions only. admin_describe lists columns. A refusal says why; fix the SQL rather than retrying it.",
			InputSchema: schema([]string{"sql"}, map[string]any{"sql": str("One SELECT."), "limit": num("Rows, up to 2000.")}),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				return cl.Do(c, http.MethodPost, "/api/v1/admin/query", nil, body(args, "sql", "limit"), true)
			},
		},
		{
			Name: "admin_actions",
			Description: "Operator only. The audit log of operator changes, newest first: what changed, before and after, by whom, and whether admin_undo can put it back. " +
				"kinds: true lists instead every verb the log holds, how often, and whether it undoes.",
			InputSchema: schema(nil, map[string]any{
				"since": str("A date (2026-09-01) or an age (24h, 7d, 2w)."), "kind": str("A verb (video.hide) or family (video, payload)."),
				"mine": boolean("Only the caller's changes."), "limit": num("Up to 50."), "cursor": str("next_cursor from the last page."),
				"kinds": boolean("List the verbs the log holds instead."),
			}),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				if kinds, _ := args["kinds"].(bool); kinds {
					return cl.Do(c, http.MethodGet, "/api/v1/admin/actions/kinds", nil, nil, true)
				}
				return get("/api/v1/admin/actions", "since", "kind", "mine", "limit", "cursor")(c, cl, args)
			},
		},
		{
			Name: "admin_undo",
			Description: "Operator only. Put a recorded change back. Runs only while nothing has changed the same field since; otherwise the error names the field. " +
				"Try dry_run first and tell the person what it would restore. Undoing another operator's change needs confirm, which only the person may give.",
			InputSchema: schema([]string{"action"}, map[string]any{
				"action": num("The action's id, from admin_actions."), "dry_run": boolean("Show what would be restored."),
				"confirm": boolean("The person agreed to undo another operator's change."),
			}),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				return cl.Do(c, http.MethodPost, fmt.Sprintf("/api/v1/admin/actions/%v/undo", args["action"]), nil, body(args, "dry_run", "confirm"), true)
			},
		},
		{
			Name: "admin_video",
			Description: "Operator only. hide (out of search and every list, still reachable by id), unhide, feature (leads the featured row), unfeature, " +
				"label (dance form as a person's: form tango_family|folklore|other|not_dance), stamp (why no commercial song: status live_music|no_commercial_take|needs_human|none), " +
				"reidentify (re-read for dancers/song/event in the background; not undoable). Recorded; all but reidentify undo with admin_undo. Use dry_run to preview; give a note.",
			InputSchema: schema([]string{"video", "verb"}, map[string]any{
				"video": str("YouTube id."), "verb": enum("What to do.", "hide", "unhide", "feature", "unfeature", "label", "stamp", "reidentify"),
				"form": str("label only: the dance form."), "status": str("stamp only: the status."),
				"note": str("Why, kept with the record."), "dry_run": boolean("Show what would change."),
			}),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				verb := arg(args, "verb")
				if !contains([]string{"hide", "unhide", "feature", "unfeature", "label", "stamp", "reidentify"}, verb) {
					return nil, Usage("verb is hide, unhide, feature, unfeature, label, stamp or reidentify", "")
				}
				path := "/api/v1/admin/videos/" + url.PathEscape(NormalizeID(arg(args, "video"))) + "/" + verb
				return cl.Do(c, http.MethodPost, path, nil, body(args, "form", "status", "note", "dry_run"), true)
			},
		},
		{
			Name:        "admin_video_import",
			Description: "Operator only. Add up to 50 videos by YouTube id, fetched and identified in the background; ids already in the catalogue are named and left alone. Recorded. dry_run previews.",
			InputSchema: schema([]string{"ids"}, map[string]any{
				"ids":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "YouTube ids."},
				"note": str("Why."), "dry_run": boolean("Show what would be imported."),
			}),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				payload := body(args, "note", "dry_run")
				payload["ids"] = args["ids"]
				return cl.Do(c, http.MethodPost, "/api/v1/admin/videos/import", nil, payload, true)
			},
		},
	}
}

// adminEditTypes are the records admin_edit changes, and the fields each takes.
var adminEditTypes = map[string]string{
	"dancer":    "name, nickname, bio, gender (male|female), primary_role (leader|follower|both|neither), reviewed (bool)",
	"orchestra": "name, bio",
	"singer":    "name, reviewed (bool)",
	"song":      "title, singer (a singer slug), composer, author (lyricist), date (recorded, YYYY-MM-DD), spotify_track_id, el_recodo_song_id, youtube_music_video_id",
	"event":     "title, city, country, start_date, end_date (YYYY-MM-DD), latitude, longitude",
	"clip":      "title, description, start_s, end_s (seconds), visibility (private|unlisted|public), tags (array of steps)",
}

func adminRecordTools() []mcpTool {
	var kinds, fields []string
	for k, f := range adminEditTypes {
		kinds = append(kinds, k)
		fields = append(fields, k+": "+f)
	}
	sort.Strings(kinds)
	sort.Strings(fields)
	return []mcpTool{
		{
			Name: "admin_edit",
			Description: "Operator only. Change a record's facts; send only the fields to change (null clears one). Recorded, and undoable with admin_undo. " +
				"Use dry_run first and tell the person what would change. Fields by type — " + strings.Join(fields, "; ") + ".",
			InputSchema: schema([]string{"type", "id", "fields"}, map[string]any{
				"type": enum("Kind of record.", kinds...), "id": str("Its slug (a clip's id for clips)."),
				"fields": map[string]any{"type": "object", "description": "Field → new value."},
				"note":   str("Why, kept with the record."), "dry_run": boolean("Show what would change."),
			}),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				kind := arg(args, "type")
				if _, ok := adminEditTypes[kind]; !ok {
					return nil, Usage("type is one of "+strings.Join(kinds, ", "), "")
				}
				payload, _ := args["fields"].(map[string]any)
				if len(payload) == 0 {
					return nil, Usage("fields names at least one field to change", "")
				}
				for k, v := range body(args, "note", "dry_run") {
					payload[k] = v
				}
				return cl.Do(c, http.MethodPatch, "/api/v1/admin/"+kind+"s/"+url.PathEscape(arg(args, "id")), nil, payload, true)
			},
		},
		{
			Name: "admin_channel",
			Description: "Operator only. review (tango and trusted: shows and syncs), activate (show again), deactivate (out of every listing; needs confirm), " +
				"reject-noise (not tango: every video taken out in bulk; needs confirm; CANNOT be undone), sync (fetch new videos now). " +
				"State changes are recorded; review, activate and deactivate undo with admin_undo. Use dry_run first; confirm only after the person agrees.",
			InputSchema: schema([]string{"channel", "verb"}, map[string]any{
				"channel": str("YouTube channel id (UC...)."), "verb": enum("What to do.", "review", "activate", "deactivate", "reject-noise", "sync"),
				"dry_run": boolean("Show what would change."), "confirm": boolean("The person agreed (deactivate, reject-noise)."), "note": str("Why."),
			}),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				verb := arg(args, "verb")
				if !contains([]string{"review", "activate", "deactivate", "reject-noise", "sync"}, verb) {
					return nil, Usage("verb is review, activate, deactivate, reject-noise or sync", "")
				}
				return cl.Do(c, http.MethodPost, "/api/v1/admin/channels/"+url.PathEscape(arg(args, "channel"))+"/"+verb, nil, body(args, "dry_run", "confirm", "note"), true)
			},
		},
		{
			Name:        "admin_clip_delete",
			Description: "Operator only. Delete anyone's clip. CANNOT be undone (the record keeps what it was). dry_run first; confirm only after the person agrees.",
			InputSchema: schema([]string{"clip"}, map[string]any{
				"clip": str("The clip's id."), "dry_run": boolean("Show what would be deleted."),
				"confirm": boolean("The person agreed."), "note": str("Why."),
			}),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				return cl.Do(c, http.MethodDelete, "/api/v1/admin/clips/"+url.PathEscape(arg(args, "clip")), nil, body(args, "dry_run", "confirm", "note"), true)
			},
		},
		{
			Name: "admin_championships_load",
			Description: "Operator only. Reload the Mundial champions roll from its checked-in file (idempotent). dry_run shows the editions, new titles, dancers it would create, " +
				"and champions whose slug matches no dancer, any one of which stops the load with nothing loaded. Recorded.",
			InputSchema: schema(nil, map[string]any{"dry_run": boolean("Show what a load would do."), "note": str("Why.")}),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				return cl.Do(c, http.MethodPost, "/api/v1/admin/championships/load", nil, body(args, "dry_run", "note"), true)
			},
		},
		{
			Name: "admin_song_lyrics",
			Description: "Operator only. Set a song's lyrics (Spanish) and/or English translation. English given here is marked as a person's (lyrics_en_source human) unless machine is said. " +
				"Recorded and undoable with admin_undo; dry_run shows the change by length.",
			InputSchema: schema([]string{"song"}, map[string]any{
				"song": str("Song slug."), "lyrics": str("Spanish lyrics."), "lyrics_en": str("English translation."),
				"lyrics_en_source": enum("Who translated.", "human", "machine"), "note": str("Why."), "dry_run": boolean("Show what would change."),
			}),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				return cl.Do(c, http.MethodPut, "/api/v1/admin/songs/"+url.PathEscape(arg(args, "song"))+"/lyrics", nil,
					body(args, "lyrics", "lyrics_en", "lyrics_en_source", "note", "dry_run"), true)
			},
		},
		{
			Name: "admin_dancer_alias",
			Description: "Operator only. The other spellings the matcher reads as a dancer: list, add (replay re-reads up to 500 videos whose titles carry it) or rm. " +
				"Recorded and undoable with admin_undo. dry_run previews.",
			InputSchema: schema([]string{"dancer", "verb"}, map[string]any{
				"dancer": str("Dancer slug."), "verb": enum("What to do.", "list", "add", "rm"), "spelling": str("The spelling, for add and rm."),
				"replay": boolean("add only: re-read the videos whose titles carry it."), "note": str("Why."), "dry_run": boolean("Show what would change."),
			}),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				path := "/api/v1/admin/dancers/" + url.PathEscape(arg(args, "dancer")) + "/aliases"
				switch arg(args, "verb") {
				case "list":
					return cl.Do(c, http.MethodGet, path, nil, nil, true)
				case "add":
					payload := body(args, "replay", "note", "dry_run")
					payload["name"] = arg(args, "spelling")
					return cl.Do(c, http.MethodPost, path, nil, payload, true)
				case "rm":
					return cl.Do(c, http.MethodDelete, path+"/"+url.PathEscape(arg(args, "spelling")), nil, body(args, "note", "dry_run"), true)
				}
				return nil, Usage("verb is list, add or rm", "")
			},
		},
		{
			Name: "admin_dancer_merge",
			Description: "Operator only. Fold a duplicate dancer (loser) into the real one (into): credits, couples, aliases, titles, followers and pictures move; the loser is deleted. " +
				"CANNOT be undone. Always run dry_run first and show the person exactly what moves; confirm only after they say yes. The record keeps a full snapshot of the loser.",
			InputSchema: schema([]string{"loser", "into"}, map[string]any{
				"loser": str("Slug of the duplicate, which is deleted."), "into": str("Slug of the dancer that stays."),
				"dry_run": boolean("Show what would move."), "confirm": boolean("The person agreed; required to apply."), "note": str("Why."),
			}),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				payload := body(args, "dry_run", "confirm", "note")
				payload["into"] = arg(args, "into")
				return cl.Do(c, http.MethodPost, "/api/v1/admin/dancers/"+url.PathEscape(arg(args, "loser"))+"/merge", nil, payload, true)
			},
		},
		{
			Name: "admin_image_add",
			Description: "Operator only. Add a picture to a dancer, orchestra or event: a portrait or a cover. source is an https address (TangoTube fetches it, public web only) " +
				"or a file path on this machine. Judged by its bytes: JPEG, PNG or WebP, at most 10 MB, at least 200 px a side; a refusal says why. " +
				"licence is required (unknown, own_work, permission, public_page, public_domain, cc); give credit when there is one. It is shown when primary is true or the page shows none. " +
				"Couples, channels and singers have no picture of their own. Adding CANNOT be undone (admin_image takedown takes it off). Run dry_run first and tell the person what it read.",
			InputSchema: schema([]string{"type", "slug", "source", "licence"}, map[string]any{
				"type": enum("Whose picture.", "dancer", "orchestra", "event"), "slug": str("Their slug."),
				"source": str("An https:// address, or a local file path."), "kind": enum("Which picture (default portrait).", "portrait", "cover"),
				"licence": enum("Under what terms it may be shown.", imageLicences...), "credit": str("Who to credit."),
				"primary": boolean("Show it now in place of the current one."), "note": str("Why."), "dry_run": boolean("Show what would be added."),
			}),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				kind := arg(args, "kind")
				if kind == "" {
					kind = "portrait"
				}
				fields := body(args, "licence", "credit", "primary", "note", "dry_run")
				fields["kind"] = kind
				path := "/api/v1/admin/images/" + url.PathEscape(arg(args, "type")) + "/" + url.PathEscape(arg(args, "slug"))
				source := arg(args, "source")
				if isWebAddress(source) {
					fields["url"] = source
					return cl.Do(c, http.MethodPost, path, nil, fields, true)
				}
				upload, err := readUpload(source, fields)
				if err != nil {
					return nil, err
				}
				return cl.Do(c, http.MethodPost, path, nil, upload, true)
			},
		},
		{
			Name:        "admin_images",
			Description: "Operator only. A dancer's, orchestra's or event's pictures (which one each kind shows, and those proposed, rejected or taken down), or with review, every proposed picture. Read-only.",
			InputSchema: schema(nil, map[string]any{
				"type": enum("Whose pictures.", "dancer", "orchestra", "event"), "slug": str("Their slug."),
				"review": boolean("List proposed pictures instead."), "limit": num("review only: up to 50."),
			}),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				if args["review"] == true {
					return cl.Do(c, http.MethodGet, "/api/v1/admin/images/review", query(args, "limit"), nil, true)
				}
				if arg(args, "type") == "" || arg(args, "slug") == "" {
					return nil, Usage("give type and slug, or review", "")
				}
				return cl.Do(c, http.MethodGet, "/api/v1/admin/images/"+url.PathEscape(arg(args, "type"))+"/"+url.PathEscape(arg(args, "slug")), nil, nil, true)
			},
		},
		{
			Name: "admin_image",
			Description: "Operator only. primary (show this picture for its kind), takedown (off the page, kept on the record; the page falls back to the newest picture still up or shows none — dry_run says which; needs confirm), " +
				"accept or reject (a proposed picture; accept shows it when the page shows none). All four are recorded and undo with admin_undo. dry_run first; confirm only after the person agrees.",
			InputSchema: schema([]string{"id", "verb"}, map[string]any{
				"id": num("The picture's id, from admin_images."), "verb": enum("What to do.", "primary", "takedown", "accept", "reject"),
				"reason": str("takedown and reject: why."), "dry_run": boolean("Show what would change."),
				"confirm": boolean("The person agreed (takedown)."), "note": str("Why."),
			}),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				verb := arg(args, "verb")
				if !contains([]string{"primary", "takedown", "accept", "reject"}, verb) {
					return nil, Usage("verb is primary, takedown, accept or reject", "")
				}
				return cl.Do(c, http.MethodPost, fmt.Sprintf("/api/v1/admin/images/%v/%s", args["id"], verb), nil,
					body(args, "reason", "dry_run", "confirm", "note"), true)
			},
		},
	}
}

// operatorToken asks the API whether tt's token is an operator's admin token.
// No token, or any trouble asking, is no.
func (a *App) operatorToken(c context.Context) bool {
	cl := a.Client()
	if cl.Token == "" {
		return false
	}
	env, err := cl.Do(c, http.MethodGet, "/api/v1/me", nil, nil, true)
	if err != nil {
		return false
	}
	return decode[meData](env.Data).Token.Admin
}
