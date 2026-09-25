// Package fixtures serves recorded /api/v1 envelopes, so tt's end-to-end tests
// need no database and no network.
package fixtures

import (
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

//go:embed *.json
var files embed.FS

// Token is the dancer's token the fixture API accepts.
const Token = "tt_live_fixture"

// AdminToken is an operator's admin token: everything Token can do, and
// tt admin.
const AdminToken = "tt_live_fixture_admin"

// Handler answers the routes tt calls with the files in this directory.
func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v1/")
		admin := r.Header.Get("Authorization") == "Bearer "+AdminToken
		authed := admin || r.Header.Get("Authorization") == "Bearer "+Token
		hasToken := r.Header.Get("Authorization") != ""
		q := r.URL.Query()

		switch {
		// Like the API: a token that is sent is checked, whatever the route.
		case hasToken && !authed:
			serve(w, 401, "error_auth.json")
		case r.Method == http.MethodGet && path == "search":
			searched := false
			for key := range q {
				if key != "limit" && key != "cursor" && q.Get(key) != "" {
					searched = true
				}
			}
			switch {
			case !searched:
				serve(w, 400, "error_search_nothing.json")
			case q.Get("sort") == "loudest":
				serve(w, 400, "error_usage_sort.json")
			case q.Get("q") == "" && q.Get("sort") != "":
				serve(w, 200, "search_browse.json")
			case q.Get("technique") != "" && q.Get("event") != "":
				serve(w, 422, "error_usage_technique.json")
			case q.Get("technique") != "" || q.Get("style") != "":
				serve(w, 200, "search_sacada.json")
			case strings.Contains(strings.ToLower(q.Get("q")), "noelia"):
				serve(w, 200, "search_noelia.json")
			case strings.Contains(strings.ToLower(q.Get("q")), "sarli") || q.Get("orchestra") == "carlos-di-sarli":
				serve(w, 200, "search_di_sarli.json")
			default:
				serve(w, 200, "search_empty.json")
			}
		case r.Method == http.MethodGet && lists[path]:
			serve(w, 200, "list_"+path+".json")
		case r.Method == http.MethodGet && path == "facets":
			serve(w, 200, "facets.json")
		case r.Method == http.MethodGet && path == "home":
			serve(w, 200, "home.json")
		case r.Method == http.MethodGet && path == "resolve":
			if q.Get("q") == "" {
				serve(w, 400, "error_resolve_nothing.json")
				return
			}
			serve(w, 200, "resolve.json")
		case r.Method == http.MethodGet && strings.HasPrefix(path, "events/"):
			serveOr404(w, "event_"+strings.TrimPrefix(path, "events/")+".json")
		case r.Method == http.MethodGet && strings.HasPrefix(path, "channels/"):
			serveOr404(w, "channel_"+strings.TrimPrefix(path, "channels/")+".json")
		case r.Method == http.MethodGet && strings.HasPrefix(path, "songs/") && strings.HasSuffix(path, "/versions"):
			serveOr404(w, "song_versions_"+strings.TrimSuffix(strings.TrimPrefix(path, "songs/"), "/versions")+".json")
		case r.Method == http.MethodGet && path == "tags":
			serve(w, 200, "tags.json")
		case r.Method == http.MethodGet && path == "me":
			if !authed {
				serve(w, 401, "error_auth.json")
				return
			}
			if admin {
				serve(w, 200, "me_admin.json")
				return
			}
			serve(w, 200, "me.json")
		case r.Method == http.MethodGet && path == "clips":
			if q.Get("mine") == "true" && !authed {
				serve(w, 401, "error_auth.json")
				return
			}
			serve(w, 200, "clips.json")
		case r.Method == http.MethodPost && strings.HasPrefix(path, "videos/") && strings.HasSuffix(path, "/clips"):
			if !authed {
				serve(w, 401, "error_auth.json")
				return
			}
			serve(w, 201, "clip_created.json")
		// Helping: reports need no token; everything else that writes does.
		case r.Method == http.MethodPost && path == "reports":
			serve(w, 201, "report.json")
		case r.Method == http.MethodGet && path == "identify/queue":
			serve(w, 200, "queue.json")
		case r.Method == http.MethodGet && path == "identify/search":
			serve(w, 200, "identify_search_song.json")
		case r.Method == http.MethodPost && contribution(path) && !authed:
			serve(w, 401, "error_auth.json")
		case r.Method == http.MethodPost && strings.HasSuffix(path, "/suggestions/batch"):
			serve(w, 200, "suggestion_batch.json")
		case r.Method == http.MethodPost && strings.HasSuffix(path, "/agree"):
			serve(w, 400, "agree_own.json")
		case r.Method == http.MethodPost && strings.HasSuffix(path, "/confirmation"):
			serve(w, 201, "confirmation.json")
		case r.Method == http.MethodPost && strings.HasSuffix(path, "/suggestions"):
			body := readBody(r)
			switch {
			case body["agree"] == true:
				serve(w, 201, "suggestion_applied.json")
			case body["dancer"] != nil && body["dancer"] != "roxana-suarez":
				serve(w, 400, "suggestion_near_miss.json")
			default:
				serve(w, 201, "suggestion_queued.json")
			}
		case r.Method == http.MethodPost && strings.HasSuffix(path, "/tag_suggestions"):
			if readBody(r)["name"] == "sacada" {
				serve(w, 400, "tag_suggestion_exists.json")
				return
			}
			serve(w, 201, "tag_suggestion.json")
		// A person's own library: every route needs the token.
		// Operating TangoTube: the fixture token stands in for an admin one.
		case strings.HasPrefix(path, "admin/"):
			operate(w, r, strings.TrimPrefix(path, "admin/"), authed, admin)
		case personal(r.Method, path) && !authed:
			serve(w, 401, "error_auth.json")
		case r.Method == http.MethodPost && path == "likes":
			serve(w, 201, "like_created.json")
		case r.Method == http.MethodDelete && path == "likes":
			serve(w, 200, "unlike.json")
		case r.Method == http.MethodGet && path == "me/likes":
			if q.Get("kind") == "clips" {
				serve(w, 200, "likes_clips.json")
				return
			}
			serve(w, 200, "likes.json")
		case r.Method == http.MethodPost && path == "me/imports/youtube":
			importYouTube(w, readBody(r))
		case r.Method == http.MethodGet && path == "me/history":
			serve(w, 200, "history.json")
		case r.Method == http.MethodPost && path == "me/history":
			serve(w, 201, "history_added.json")
		case r.Method == http.MethodDelete && path == "me/history":
			if q.Get("confirm") != "true" {
				serve(w, 400, "history_clear_usage.json")
				return
			}
			serve(w, 200, "history_removed.json")
		case r.Method == http.MethodDelete && strings.HasPrefix(path, "me/history/"):
			serve(w, 200, "history_removed.json")
		case r.Method == http.MethodGet && path == "playlists":
			serve(w, 200, "playlists.json")
		case r.Method == http.MethodPost && path == "playlists":
			serve(w, 201, "playlist_created.json")
		case r.Method == http.MethodPatch && strings.HasPrefix(path, "playlists/"):
			if q := readBody(r); q["visibility"] != nil && q["visibility"] != "private" && q["visibility"] != "unlisted" && q["visibility"] != "public" {
				serve(w, 422, "playlist_usage.json")
				return
			}
			serve(w, 200, "playlist_renamed.json")
		case r.Method == http.MethodDelete && strings.HasPrefix(path, "playlists/") && strings.Contains(path, "/items/"):
			serve(w, 200, "playlist_removed.json")
		case r.Method == http.MethodDelete && strings.HasPrefix(path, "playlists/"):
			serve(w, 200, "playlist_deleted.json")
		case r.Method == http.MethodPost && strings.HasPrefix(path, "playlists/") && strings.HasSuffix(path, "/items"):
			serve(w, 201, "playlist_added.json")
		case r.Method == http.MethodPut && strings.HasPrefix(path, "playlists/") && strings.HasSuffix(path, "/order"):
			serve(w, 200, "playlist_moved.json")
		case r.Method == http.MethodGet && strings.HasPrefix(path, "playlists/"):
			serveOr404(w, "playlist_"+strings.TrimPrefix(path, "playlists/")+".json")
		case r.Method == http.MethodPost && path == "follows":
			body := readBody(r)
			switch {
			case body["level"] != nil && body["level"] != "personalized" && body["level"] != "all_activity" && body["level"] != "muted":
				serve(w, 400, "follow_usage.json")
			case body["dancer"] == "noelia hurtado":
				serve(w, 200, "follow_again.json")
			default:
				serve(w, 201, "follow_created.json")
			}
		case r.Method == http.MethodDelete && path == "follows":
			serve(w, 200, "unfollow.json")
		case r.Method == http.MethodGet && path == "me/following":
			if q.Get("feed") == "true" {
				serve(w, 200, "following_feed.json")
				return
			}
			serve(w, 200, "following.json")
		case r.Method == http.MethodGet && path == "me/practice":
			serve(w, 200, "practice.json")
		case r.Method == http.MethodPost && path == "me/practice":
			serve(w, 201, "practice_added.json")
		case r.Method == http.MethodDelete && strings.HasPrefix(path, "me/practice/"):
			serve(w, 200, "practice_removed.json")
		case r.Method == http.MethodGet && path == "me/saved_searches":
			serve(w, 200, "saved_searches.json")
		case r.Method == http.MethodPost && path == "me/saved_searches":
			serve(w, 201, "saved_search_created.json")
		case r.Method == http.MethodDelete && strings.HasPrefix(path, "me/saved_searches/"):
			serve(w, 200, "saved_search_deleted.json")
		case r.Method == http.MethodGet && path == "me/notifications":
			if q.Get("unread") == "true" {
				serve(w, 200, "notifications_unread.json")
				return
			}
			serve(w, 200, "notifications.json")
		case r.Method == http.MethodPost && path == "me/notifications/read":
			serve(w, 200, "notifications_read.json")
		case r.Method == http.MethodPatch && strings.HasPrefix(path, "clips/"):
			if !authed {
				serve(w, 401, "error_auth.json")
				return
			}
			serve(w, 200, "clip_edited.json")
		case r.Method == http.MethodDelete && path == "tokens/current":
			if !authed {
				serve(w, 401, "error_auth.json")
				return
			}
			serve(w, 200, "token_revoked.json")
		case r.Method == http.MethodDelete && strings.HasPrefix(path, "clips/"):
			if !authed {
				serve(w, 401, "error_auth.json")
				return
			}
			serve(w, 200, "clip_deleted.json")
		case r.Method == http.MethodGet && strings.HasPrefix(path, "videos/"):
			name := "video_" + strings.TrimPrefix(path, "videos/")
			if q.Get("include") == "identity" {
				name += "_identity"
			}
			serveOr404(w, name+".json")
		case r.Method == http.MethodGet && strings.HasPrefix(path, "performances/"):
			serveOr404(w, "performance_"+strings.TrimPrefix(path, "performances/")+".json")
		case r.Method == http.MethodGet && strings.HasPrefix(path, "clips/"):
			serveOr404(w, "clip_"+strings.TrimPrefix(path, "clips/")+".json")
		case r.Method == http.MethodGet && strings.HasPrefix(path, "dancers/") && strings.HasSuffix(path, "/partners"):
			name := "partners_" + strings.TrimSuffix(strings.TrimPrefix(path, "dancers/"), "/partners")
			switch q.Get("depth") {
			case "", "1":
			case "2":
				name += "_2"
			default:
				serve(w, 422, "error_usage_depth.json")
				return
			}
			serveOr404(w, name+".json")
		case r.Method == http.MethodGet && strings.HasPrefix(path, "dancers/"):
			name := "dancer_" + strings.TrimPrefix(path, "dancers/")
			if q.Get("include") != "" {
				name += "_include"
			}
			serveOr404(w, name+".json")
		case r.Method == http.MethodGet && strings.HasPrefix(path, "couples/"):
			serveOr404(w, "couple_"+strings.TrimPrefix(path, "couples/")+".json")
		case r.Method == http.MethodGet && strings.HasPrefix(path, "songs/"):
			name := "song_" + strings.TrimPrefix(path, "songs/")
			if q.Get("lyrics") != "" {
				name += "_lyrics"
			}
			serveOr404(w, name+".json")
		case r.Method == http.MethodGet && strings.HasPrefix(path, "orchestras/"):
			serveOr404(w, "orchestra_"+strings.TrimPrefix(path, "orchestras/")+".json")
		default:
			serve(w, 404, "error_not_found.json")
		}
	})
}

// personal routes are about the person holding the token.
func personal(method, path string) bool {
	if (path == "playlists" || strings.HasPrefix(path, "playlists/")) && !(method == http.MethodGet && path != "playlists") {
		return true
	}
	if strings.HasPrefix(path, "me/") {
		return true
	}
	return path == "follows" || path == "me/following" || path == "likes" || path == "me/likes" || path == "me/history" || strings.HasPrefix(path, "me/history/")
}

// contribution routes tell TangoTube what a video or clip is.
func contribution(path string) bool {
	return strings.HasSuffix(path, "/suggestions") || strings.HasSuffix(path, "/suggestions/batch") ||
		strings.HasSuffix(path, "/agree") || strings.HasSuffix(path, "/confirmation") || strings.HasSuffix(path, "/tag_suggestions")
}

// importYouTube answers an import the way the API does, from the body: a
// video or channel id starting with "t" (or "UCt") is one TangoTube has.
func importYouTube(w http.ResponseWriter, body map[string]any) {
	result := map[string]any{"unknown_video_ids": []string{}, "unknown_channel_ids": []string{}, "dry_run": body["dry_run"] == true}
	unknown := map[string]bool{}
	for kind, key := range map[string]string{"watches": "watches", "likes": "likes", "subscriptions": "follows"} {
		list, _ := body[kind].([]any)
		if len(list) > 2000 {
			serve(w, 400, "error_usage_sort.json")
			return
		}
		matched := 0
		for _, item := range list {
			id, _ := item.(string)
			if m, ok := item.(map[string]any); ok {
				id, _ = m["video_id"].(string)
			}
			known := strings.HasPrefix(id, "t") || strings.HasPrefix(id, "UCt")
			switch {
			case known:
				matched++
			case kind == "subscriptions":
				result["unknown_channel_ids"] = append(result["unknown_channel_ids"].([]string), id)
			case !unknown[id]:
				unknown[id] = true
				result["unknown_video_ids"] = append(result["unknown_video_ids"].([]string), id)
			}
		}
		result[key] = map[string]int{"sent": len(list), "matched": matched, "added": matched, "already": 0}
	}
	raw, _ := json.Marshal(map[string]any{"ok": true, "data": result, "summary": "", "breadcrumbs": []string{}})
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(raw)
}

// lists are the catalogue's index routes, each recorded once.
var lists = map[string]bool{"dancers": true, "couples": true, "orchestras": true, "songs": true,
	"events": true, "channels": true, "singers": true, "champions": true}

// readBody is a JSON request body, for the few routes that answer by it.
func readBody(r *http.Request) map[string]any {
	body := map[string]any{}
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		if r.ParseMultipartForm(16<<20) == nil {
			for k, v := range r.MultipartForm.Value {
				body[k] = v[0]
				if v[0] == "true" {
					body[k] = true
				}
			}
			if _, ok := r.MultipartForm.File["file"]; ok {
				body["file"] = true
			}
		}
		return body
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	return body
}

func serveOr404(w http.ResponseWriter, name string) {
	if _, err := files.Open(name); err != nil || strings.Contains(name, "/") {
		serve(w, 404, "error_not_found.json")
		return
	}
	serve(w, 200, name)
}

func serve(w http.ResponseWriter, status int, name string) {
	raw, err := files.ReadFile(name)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(raw)
}

// operate answers tt admin's routes: auth without a token, forbidden with a
// dancer's, and the recorded operator's answers with an admin one.
func operate(w http.ResponseWriter, r *http.Request, path string, authed, admin bool) {
	if !authed {
		serve(w, 401, "error_auth.json")
		return
	}
	if !admin {
		serve(w, 403, "error_forbidden_admin.json")
		return
	}
	body := map[string]any{}
	if r.Method != http.MethodGet {
		body = readBody(r)
	}
	switch {
	case r.Method == http.MethodGet && path == "actions":
		serve(w, 200, "admin_actions.json")
	case r.Method == http.MethodGet && path == "actions/kinds":
		serve(w, 200, "admin_action_kinds.json")
	case r.Method == http.MethodGet && path == "payloads/export":
		serve(w, 200, "admin_payload_export.json")
	case r.Method == http.MethodGet && path == "payloads" && r.URL.Query().Get("counts") == "true":
		serve(w, 200, "admin_payloads_counted.json")
	case r.Method == http.MethodGet && path == "payloads":
		serve(w, 200, "admin_payloads.json")
	case r.Method == http.MethodGet && path == "jobs/runs":
		serve(w, 200, "admin_job_runs.json")
	case r.Method == http.MethodPost && (strings.HasPrefix(path, "jobs/run/") || strings.HasPrefix(path, "rebuilds/")):
		name := "admin_job_run"
		if strings.HasPrefix(path, "rebuilds/") {
			name = "admin_rebuild"
		}
		args, _ := body["args"].(map[string]any)
		expensive := name == "admin_rebuild" || args["mode"] == "full"
		switch {
		case body["dry_run"] == true:
			serve(w, 200, name+"_dry.json")
		case expensive && body["confirm"] != true:
			serve(w, 400, name+"_refused.json")
		default:
			serve(w, 200, "admin_job_run.json")
		}
	case r.Method == http.MethodPost && strings.HasPrefix(path, "cron/") && body["dry_run"] == true:
		serve(w, 200, "admin_cron_pause_dry.json")
	case r.Method == http.MethodGet && path == "announcements":
		serve(w, 200, "admin_announcements.json")
	case r.Method == http.MethodPost && path == "announcements" && body["dry_run"] == true:
		serve(w, 200, "admin_announcement_create_dry.json")
	case r.Method == http.MethodPost && path == "payloads":
		if body["dry_run"] == true {
			serve(w, 200, "admin_payload_import_dry.json")
			return
		}
		serve(w, 400, "admin_payload_import_refused.json")
	case r.Method == http.MethodPost && strings.HasPrefix(path, "payloads/") && strings.HasSuffix(path, "/revert") && body["confirm"] != true:
		serve(w, 400, "admin_payload_revert_refused.json")
	case r.Method == http.MethodGet && reports[path]:
		serve(w, 200, "admin_"+strings.ReplaceAll(path, "-", "_")+".json")
	case r.Method == http.MethodGet && path == "coverage":
		serve(w, 200, "admin_coverage_conflict.json")
	case r.Method == http.MethodGet && path == "describe/dancer":
		serve(w, 200, "admin_describe_dancer.json")
	case r.Method == http.MethodGet && strings.HasPrefix(path, "desk/"):
		serve(w, 200, "admin_desk.json")
	case r.Method == http.MethodPost && path == "query":
		if strings.Contains(strings.ToLower(fmt.Sprint(body["sql"])), "users") {
			serve(w, 400, "admin_query_refused.json")
			return
		}
		serve(w, 200, "admin_query.json")
	case r.Method == http.MethodPost && strings.HasPrefix(path, "videos/") && strings.HasSuffix(path, "/hide"):
		if body["dry_run"] == true {
			serve(w, 200, "admin_video_hide_dry.json")
			return
		}
		serve(w, 200, "admin_video_hide.json")
	case r.Method == http.MethodGet && path == "reviews":
		serve(w, 200, "admin_reviews.json")
	case r.Method == http.MethodGet && strings.HasPrefix(path, "users/"):
		serve(w, 200, "admin_user.json")
	case r.Method == http.MethodGet && path == "images/review":
		serve(w, 200, "admin_image_review.json")
	case r.Method == http.MethodGet && strings.HasPrefix(path, "images/dancer/"):
		serve(w, 200, "admin_image_list.json")
	case r.Method == http.MethodPost && strings.HasPrefix(path, "images/dancer/"):
		switch {
		case strings.Contains(fmt.Sprint(body["url"]), "169.254."):
			serve(w, 400, "admin_image_add_refused.json")
		case body["dry_run"] == true:
			serve(w, 200, "admin_image_add_dry.json")
		default:
			serve(w, 200, "admin_image_add.json")
		}
	case recordWrite(w, r.Method, path, body):
		return
	case r.Method == http.MethodPost && path == "actions/1/undo":
		if body["dry_run"] == true {
			serve(w, 200, "admin_undo_dry.json")
			return
		}
		serve(w, 200, "admin_undo.json")
	case r.Method == http.MethodPost && strings.HasPrefix(path, "actions/") && strings.HasSuffix(path, "/undo"):
		serve(w, 409, "admin_undo_conflict.json")
	default:
		serve(w, 404, "error_not_found.json")
	}
}

// recordWrite answers the recorded operator changes to records: a dry run
// with its preview, a destructive verb without confirm with its refusal, and
// otherwise the change as it was recorded. False when path is not one.
func recordWrite(w http.ResponseWriter, method, path string, body map[string]any) bool {
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return false
	}
	name := ""
	switch {
	case method == http.MethodPatch && len(parts) == 2:
		name = "admin_" + strings.TrimSuffix(parts[0], "s") + "_edit"
	case method == http.MethodPost && len(parts) == 3 && parts[0] == "dancers" && parts[2] == "merge":
		name = "admin_dancer_merge"
	case method == http.MethodPost && path == "championships/load":
		name = "admin_championships_load"
	case method == http.MethodDelete && len(parts) == 2 && parts[0] == "clips":
		name = "admin_clip_delete"
	case method == http.MethodPost && path == "videos/import":
		name = "admin_video_import"
	case method == http.MethodPost && len(parts) == 3 && parts[0] == "videos" && parts[2] == "stamp" && body["status"] == "circus":
		serve(w, 400, "admin_video_stamp_refused.json")
		return true
	case method == http.MethodPost && len(parts) == 3 && parts[0] == "videos" && (parts[2] == "label" || parts[2] == "stamp"):
		name = "admin_video_" + parts[2]
	case method == http.MethodPost && len(parts) == 3 && parts[0] == "channels":
		name = "admin_channel_" + strings.ReplaceAll(parts[2], "-", "_")
	case method == http.MethodPut && len(parts) == 3 && parts[2] == "lyrics":
		name = "admin_song_lyrics"
	case method == http.MethodPost && len(parts) == 3 && parts[0] == "images":
		name = "admin_image_" + parts[2]
	case method == http.MethodPost && len(parts) == 3 && (parts[0] == "suggestions" || parts[0] == "tags"):
		name = "admin_" + strings.TrimSuffix(parts[0], "s") + "_" + parts[2]
	case method == http.MethodPost && len(parts) == 3 && parts[0] == "performances" && parts[2] == "recredit":
		name = "admin_performance_recredit"
	default:
		return false
	}
	switch {
	case body["dry_run"] == true:
		serve(w, 200, name+"_dry.json")
	case destructive[name] && body["confirm"] != true:
		serve(w, 400, name+"_refused.json")
	default:
		serve(w, 200, name+".json")
	}
	return true
}

// destructive verbs refuse without confirm.
var destructive = map[string]bool{"admin_dancer_merge": true, "admin_channel_deactivate": true, "admin_channel_reject_noise": true, "admin_clip_delete": true, "admin_image_takedown": true, "admin_tag_block": true}

// reports are the admin screens served as they were recorded.
var reports = map[string]bool{
	"dashboard": true, "pipeline": true, "intake": true, "search-quality": true,
	"channels": true, "precision": true, "jobs": true,
}
