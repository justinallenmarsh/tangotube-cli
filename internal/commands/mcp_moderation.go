package commands

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/justinallenmarsh/tangotube-cli/internal/api"
	"github.com/justinallenmarsh/tangotube-cli/internal/output"
)

// adminModerationTools are the review queues as MCP tools, listed with the
// other admin tools only for an operator's admin token.
func adminModerationTools() []mcpTool {
	post := func(c context.Context, cl *api.Client, path string, args map[string]any, keys ...string) (*output.Envelope, error) {
		return cl.Do(c, http.MethodPost, path, nil, body(args, append(keys, "dry_run", "confirm", "note")...), true)
	}
	return []mcpTool{
		{
			Name: "admin_review",
			Description: "Operator only. The enrichment review piles. list: pile conflict (a confident panel disagrees with the stored song) or panel (a panel named a song the matcher would not apply); " +
				"each bill carries its verbs as tt commands. accept (take the proposal; NOT undoable; a dancer proposal needs dancer or create_dancer), " +
				"keep (close it and stamp the stored song manual), reject (close it; the video unchanged); keep and reject undo with admin_undo. dry_run first.",
			InputSchema: schema([]string{"verb"}, map[string]any{
				"verb": enum("What to do.", "list", "accept", "keep", "reject"), "id": num("The proposal's id, from list."),
				"pile": enum("list: which pile.", "conflict", "panel"), "q": str("list: narrow to a title or YouTube id."),
				"dancer": str("accept, dancer proposals: the dancer's slug."), "create_dancer": boolean("accept, dancer proposals: create the dancer named."),
				"limit": num("list: up to 50."), "cursor": str("list: next_cursor."), "dry_run": boolean("Show what would change."), "note": str("Why."),
			}),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				verb := arg(args, "verb")
				switch verb {
				case "list":
					return cl.Do(c, http.MethodGet, "/api/v1/admin/reviews", query(args, "pile", "q", "limit", "cursor"), nil, true)
				case "accept", "keep", "reject":
					return post(c, cl, fmt.Sprintf("/api/v1/admin/reviews/%v/%s", args["id"], verb), args, "dancer", "create_dancer")
				}
				return nil, Usage("verb is list, accept, keep or reject", "")
			},
		},
		{
			Name: "admin_audio",
			Description: "Operator only. Audio match review. matches: the latest match per video, by default matched and not yet judged (status, judged). " +
				"verdict: right, wrong or unsure, a fact check; wrong also rejects the proposal and takes audio's song back, and with song names the right one (rendition when the take is unknown). " +
				"rerun: match again in the background (refingerprint fetches the audio first). Recorded; not undoable with admin_undo. dry_run first.",
			InputSchema: schema([]string{"verb"}, map[string]any{
				"verb": enum("What to do.", "matches", "verdict", "rerun"), "id": num("The match's id."),
				"verdict": enum("verdict: what you heard.", "right", "wrong", "unsure"), "song": str("verdict wrong: the right song's slug."),
				"rendition": boolean("verdict wrong: the tune and orchestra are known, not the take."), "refingerprint": boolean("rerun: fetch the audio again."),
				"status": enum("matches: which.", "matched", "weak", "no_match", "conflict", "error"), "judged": boolean("matches: include judged ones."),
				"limit": num("matches: up to 50."), "cursor": str("matches: next_cursor."), "dry_run": boolean("Show what would change."), "note": str("Why."),
			}),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				switch verb := arg(args, "verb"); verb {
				case "matches":
					return cl.Do(c, http.MethodGet, "/api/v1/admin/audio/matches", query(args, "status", "judged", "limit", "cursor"), nil, true)
				case "verdict":
					return post(c, cl, fmt.Sprintf("/api/v1/admin/audio/matches/%v/verdict", args["id"]), args, "verdict", "song", "rendition")
				case "rerun":
					return post(c, cl, fmt.Sprintf("/api/v1/admin/audio/matches/%v/rerun", args["id"]), args, "refingerprint")
				}
				return nil, Usage("verb is matches, verdict or rerun", "")
			},
		},
		{
			Name: "admin_suggestion",
			Description: "Operator only. Identification suggestions. list (status pending by default, or accepted, rejected, all). " +
				"accept: applies it as a trusted author's applies (the credit or song through the ledger, the author's trust score up a point, the author told), " +
				"and recredits a dancer onto the performance so their dancer page lists it. reject: the author's score down a point, the author told. " +
				"Neither is undoable; run dry_run first and tell the person what would change.",
			InputSchema: schema([]string{"verb"}, map[string]any{
				"verb": enum("What to do.", "list", "accept", "reject"), "id": num("The suggestion's id."),
				"status": enum("list: which.", "pending", "accepted", "rejected", "all"),
				"limit":  num("list: up to 50."), "cursor": str("list: next_cursor."), "dry_run": boolean("Show what would change."), "note": str("Why."),
			}),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				switch verb := arg(args, "verb"); verb {
				case "list":
					return cl.Do(c, http.MethodGet, "/api/v1/admin/suggestions", query(args, "status", "limit", "cursor"), nil, true)
				case "accept", "reject":
					return post(c, cl, fmt.Sprintf("/api/v1/admin/suggestions/%v/%s", args["id"], verb), args)
				}
				return nil, Usage("verb is list, accept or reject", "")
			},
		},
		{
			Name: "admin_inbox",
			Description: "Operator only. The report inbox: what people flagged as wrong. list (status open by default, or resolved, dismissed, all). " +
				"resolve (note says what was done) or dismiss (note says why not); note is required. A fact hidden by reports shows again once they are closed. " +
				"Recorded; admin_undo reopens a report.",
			InputSchema: schema([]string{"verb"}, map[string]any{
				"verb": enum("What to do.", "list", "resolve", "dismiss"), "id": num("The report's id."),
				"status": enum("list: which.", "open", "resolved", "dismissed", "all"), "note": str("resolve or dismiss: what was done, or why not."),
				"limit": num("list: up to 50."), "cursor": str("list: next_cursor."), "dry_run": boolean("Show what would change."),
			}),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				switch verb := arg(args, "verb"); verb {
				case "list":
					return cl.Do(c, http.MethodGet, "/api/v1/admin/reports", query(args, "status", "limit", "cursor"), nil, true)
				case "resolve", "dismiss":
					return post(c, cl, fmt.Sprintf("/api/v1/admin/reports/%v/%s", args["id"], verb), args)
				}
				return nil, Usage("verb is list, resolve or dismiss", "")
			},
		},
		{
			Name: "admin_tag",
			Description: "Operator only. Clip tags people suggested. pending: by tag, most suggested first, with the blocked tags. accept (into the vocabulary and onto every clip it was suggested for; " +
				"the suggesters' trust up a point), reject (close its suggestions), block (off every clip, never suggested again; needs confirm), unblock (may be suggested again; clips are not re-tagged). " +
				"Recorded; none undoes with admin_undo. dry_run first; confirm only after the person agrees.",
			InputSchema: schema([]string{"verb"}, map[string]any{
				"verb": enum("What to do.", "pending", "accept", "reject", "block", "unblock"), "name": str("The tag."),
				"limit": num("pending: up to 50."), "cursor": str("pending: next_cursor."), "dry_run": boolean("Show what would change."),
				"confirm": boolean("block: the person agreed."), "note": str("Why."),
			}),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				switch verb := arg(args, "verb"); verb {
				case "pending":
					return cl.Do(c, http.MethodGet, "/api/v1/admin/tags/pending", query(args, "limit", "cursor"), nil, true)
				case "accept", "reject", "block", "unblock":
					return post(c, cl, "/api/v1/admin/tags/"+url.PathEscape(arg(args, "name"))+"/"+verb, args)
				}
				return nil, Usage("verb is pending, accept, reject, block or unblock", "")
			},
		},
		{
			Name: "admin_user",
			Description: "Operator only. A person's standing, by their email address: show (name, trust tier and score, supporter, counts of what they sent; never their address, role, password or tokens), " +
				"trust (trusted: true trusts them whatever their score; score: 0 or more), supporter (true or false). Tiers: pending below 5, auto_apply from 5, trusted from 20. " +
				"trust and supporter are recorded and undo with admin_undo. dry_run first. Roles and passwords cannot be changed here.",
			InputSchema: schema([]string{"verb", "email"}, map[string]any{
				"verb": enum("What to do.", "show", "trust", "supporter"), "email": str("Their email address."),
				"trusted": boolean("trust: always trusted."), "score": num("trust: the trust score."), "supporter": boolean("supporter: on or off."),
				"dry_run": boolean("Show what would change."), "note": str("Why."),
			}),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				path := "/api/v1/admin/users/" + url.PathEscape(arg(args, "email"))
				switch verb := arg(args, "verb"); verb {
				case "show":
					return cl.Do(c, http.MethodGet, path, nil, nil, true)
				case "trust":
					return cl.Do(c, http.MethodPatch, path+"/trust", nil, body(args, "trusted", "score", "dry_run", "note"), true)
				case "supporter":
					return cl.Do(c, http.MethodPatch, path+"/supporter", nil, body(args, "supporter", "dry_run", "note"), true)
				}
				return nil, Usage("verb is show, trust or supporter", "")
			},
		},
		{
			Name: "admin_performance_recredit",
			Description: "Operator only. Bring a performance's credits up to date with its videos', so a dancer named on a dance that already had credits reaches their dancer page, pairings and search. " +
				"Only adds; a credit no video carries is named (unsupported) and left. id is a performance id or any of its videos' YouTube ids. Recorded; not undoable. dry_run first.",
			InputSchema: schema([]string{"id"}, map[string]any{
				"id": str("Performance id or YouTube id."), "dry_run": boolean("Show the credits it would add."), "note": str("Why."),
			}),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				return post(c, cl, "/api/v1/admin/performances/"+url.PathEscape(NormalizeID(arg(args, "id")))+"/recredit", args)
			},
		},
	}
}
