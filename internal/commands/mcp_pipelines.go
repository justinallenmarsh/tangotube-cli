package commands

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/justinallenmarsh/tangotube-cli/internal/api"
	"github.com/justinallenmarsh/tangotube-cli/internal/output"
)

// adminPipelineTools are the background work and the site's announcements as
// MCP tools, listed with the other admin tools only for an operator's admin
// token. Reading the queues is admin_report's jobs.
func adminPipelineTools() []mcpTool {
	post := func(c context.Context, cl *api.Client, path string, args map[string]any, keys ...string) (*output.Envelope, error) {
		return cl.Do(c, http.MethodPost, path, nil, body(args, append(keys, "dry_run", "confirm", "note")...), true)
	}
	return []mcpTool{
		{
			Name: "admin_job",
			Description: "Operator only. Background jobs (admin_report jobs shows the queues, schedule and failures). runs: the allowlist, each with what it does and its args. " +
				"run: start one (name channel-sync|place-new-videos|reindex|refresh-couples|link-singers|harvest-descriptions|harvest-panels|audio-sweep|fingerprint|sitemap|availability-check; " +
				"args like {\"mode\":\"full\"}); the answer states its cost. Every channel, a full placement walk, a full or listings reindex and the audio sweep need confirm, which only the person may give. " +
				"retry: run a failed job again (id from jobs failures). discard: take a waiting job off the queue (needs confirm). Recorded; not undoable. dry_run first.",
			InputSchema: schema([]string{"verb"}, map[string]any{
				"verb": enum("What to do.", "runs", "run", "retry", "discard"), "name": str("run: the job's name."),
				"args": map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "string"}, "description": "run: the job's arguments."},
				"id":   str("retry, discard: the job's id (a UUID)."), "dry_run": boolean("Say what would happen, start nothing."),
				"confirm": boolean("run (expensive ones), discard: the person agreed."), "note": str("Why."),
			}),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				switch verb := arg(args, "verb"); verb {
				case "runs":
					return cl.Do(c, http.MethodGet, "/api/v1/admin/jobs/runs", nil, nil, true)
				case "run":
					return post(c, cl, "/api/v1/admin/jobs/run/"+url.PathEscape(arg(args, "name")), args, "args")
				case "retry", "discard":
					return post(c, cl, "/api/v1/admin/jobs/"+url.PathEscape(arg(args, "id"))+"/"+verb, args)
				}
				return nil, Usage("verb is runs, run, retry or discard", "")
			},
		},
		{
			Name: "admin_cron",
			Description: "Operator only. Pause or resume a scheduled job by its key (from admin_report jobs), with GoodJob's own switch. " +
				"A paused job does not run until resumed. Recorded; admin_undo puts it back. dry_run first.",
			InputSchema: schema([]string{"verb", "key"}, map[string]any{
				"verb": enum("What to do.", "pause", "resume"), "key": str("The schedule's key, like generate_sitemap."),
				"dry_run": boolean("Say what would change."), "note": str("Why."),
			}),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				verb := arg(args, "verb")
				if verb != "pause" && verb != "resume" {
					return nil, Usage("verb is pause or resume", "")
				}
				return post(c, cl, "/api/v1/admin/cron/"+url.PathEscape(arg(args, "key"))+"/"+verb, args)
			},
		},
		{
			Name: "admin_rebuild",
			Description: "Operator only. One rebuild step, run in the background: occasions, editions, appearances, views, dances, links, pairings, establish, partnerships, " +
				"refresh-music, refresh-families or refresh-roles. Each walks a whole table: dry_run states the cost, and it runs only with confirm, which only the person may give. " +
				"A dancer merge already rebuilds the kept dancer's partnerships. Recorded; not undoable.",
			InputSchema: schema([]string{"step"}, map[string]any{
				"step": str("The step."), "dry_run": boolean("State the cost, start nothing."), "confirm": boolean("The person agreed."), "note": str("Why."),
			}),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				return post(c, cl, "/api/v1/admin/rebuilds/"+url.PathEscape(arg(args, "step")), args)
			},
		},
		{
			Name: "admin_announcement",
			Description: "Operator only. The banner across the top of the site. list: every one, live, scheduled or ended. " +
				"create: title and body required; link (https://… or /path), link_text, style info|sponsor|feedback, audience everyone|signed_in|signed_out, starts_at, ends_at (dates), position; " +
				"dry_run first and show the person the wording. end: take one down now; admin_undo puts it back.",
			InputSchema: schema([]string{"verb"}, map[string]any{
				"verb": enum("What to do.", "list", "create", "end"), "id": num("end: the announcement's id."),
				"title": str("create: the headline."), "body": str("create: the line under it."), "link": str("create: where it leads."),
				"link_text": str("create: the link's words."), "style": enum("create: how it looks.", "info", "sponsor", "feedback"),
				"audience":  enum("create: who sees it.", "everyone", "signed_in", "signed_out"),
				"starts_at": str("create: when it goes up (2026-10-01); now by default."), "ends_at": str("create: when it comes down; never by default."),
				"position": num("create: lowest shows first."), "dry_run": boolean("Say what would change."), "note": str("Why."),
			}),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				switch verb := arg(args, "verb"); verb {
				case "list":
					return cl.Do(c, http.MethodGet, "/api/v1/admin/announcements", nil, nil, true)
				case "create":
					return post(c, cl, "/api/v1/admin/announcements", args, "title", "body", "link", "link_text", "style", "audience", "starts_at", "ends_at", "position")
				case "end":
					return post(c, cl, fmt.Sprintf("/api/v1/admin/announcements/%v/end", args["id"]), args)
				}
				return nil, Usage("verb is list, create or end", "")
			},
		},
	}
}

// destructiveTools change the catalogue as an operator. Without confirm:
// true they only preview: callTool sends dry_run, and the answer says what to
// pass. The remote MCP (Mcp::Registry) marks the same ones.
var destructiveTools = map[string]bool{
	"admin_undo": true, "admin_video": true, "admin_video_import": true, "admin_edit": true, "admin_channel": true,
	"admin_clip_delete": true, "admin_championships_load": true, "admin_song_lyrics": true, "admin_dancer_alias": true,
	"admin_dancer_merge": true, "admin_image_add": true, "admin_image": true, "admin_review": true, "admin_audio": true,
	"admin_suggestion": true, "admin_inbox": true, "admin_tag": true, "admin_user": true, "admin_performance_recredit": true,
	"admin_payload": true, "admin_job": true, "admin_cron": true, "admin_rebuild": true, "admin_announcement": true,
	"playlist_delete": true,
}

const confirmChange = "The person agreed to this change. Without it nothing changes: the answer is the dry_run preview."

// previewOnly is what a destructive tool's answer adds when it ran without confirm.
const previewOnly = " Nothing changed: this is a preview. Pass confirm: true, once the person agrees, to apply it."

// allAdminTools is every admin tool, in the order tools/list gives them.
func allAdminTools() []mcpTool {
	var tools []mcpTool
	for _, group := range [][]mcpTool{adminTools(), adminRecordTools(), adminModerationTools(), adminLedgerTools(), adminPipelineTools()} {
		tools = append(tools, group...)
	}
	for _, tool := range tools {
		if destructiveTools[tool.Name] {
			tool.InputSchema["properties"].(map[string]any)["confirm"] = boolean(confirmChange)
		}
	}
	return tools
}
