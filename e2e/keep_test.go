package e2e

import (
	"strings"
	"testing"
)

func TestPracticeSaveListAndTakeOff(t *testing.T) {
	s := server(t)
	env := parse(t, tt(t, s, signedIn, "practice", "add", "https://tangotube.tv/clips/sacada-1-cuando-el-amor-muere", "--json"))
	if !env.OK || !strings.HasPrefix(env.Summary, "Saved") {
		t.Fatalf("practice add: %+v", env)
	}
	if got := jq(t, s, ".clips | length", "practice", "--token", "tt_live_fixture"); got == "0" {
		t.Fatal("practice lists nothing")
	}
	if env := parse(t, tt(t, s, signedIn, "practice", "rm", "sacada-3-volver-a-sonar", "--json")); !strings.HasPrefix(env.Summary, "Took") {
		t.Fatalf("practice rm: %+v", env)
	}
}

func TestSavedSearchHandsBackItsCommandAndRuns(t *testing.T) {
	s := server(t)
	if got := jq(t, s, ".command", "saved-search", "add", "Di Sarli vals", "--orchestra", "di sarli", "--genre", "vals", "--token", "tt_live_fixture"); !strings.HasPrefix(got, "tt search --orchestra") {
		t.Fatalf("command = %q", got)
	}
	if got := jq(t, s, ".videos | length > 0", "saved-search", "run", "di sarli vals", "--token", "tt_live_fixture"); got != "true" {
		t.Fatal("run finds nothing")
	}
	r := tt(t, s, signedIn, "saved-search", "run", "nothing like it", "--json")
	if env := parse(t, r); r.code == 0 || env.Error == nil || env.Error.Code != "not_found" {
		t.Fatalf("run unknown: code %d %+v", r.code, env)
	}
}

func TestNotificationsSayWhatHappenedAndMarkRead(t *testing.T) {
	s := server(t)
	if got := jq(t, s, "[.notifications[] | select(.message | contains(\"liked your clip\"))] | length", "notifications", "--token", "tt_live_fixture"); got != "1" {
		t.Fatalf("liked notices = %q", got)
	}
	if got := jq(t, s, "[.notifications[].read] | unique", "notifications", "--unread", "--token", "tt_live_fixture"); got != "[\n  false\n]" && got != "[false]" {
		t.Fatalf("unread = %q", got)
	}
	if env := parse(t, tt(t, s, signedIn, "notifications", "read", "--json")); !env.OK || !strings.HasPrefix(env.Summary, "Marked") {
		t.Fatalf("read: %+v", env)
	}
}

func TestClipEditChangesOnlyWhatItIsTold(t *testing.T) {
	s := server(t)
	if got := jq(t, s, ".tags | join(\",\")", "clip", "edit", "sacada-into-the-cross", "--add-tag", "boleo", "--end", "1:12", "--token", "tt_live_fixture"); got != "sacada,boleo" {
		t.Fatalf("tags = %q", got)
	}
	if r := tt(t, s, signedIn, "clip", "edit", "sacada-into-the-cross", "--json"); r.code != 1 {
		t.Fatalf("nothing to change: code %d", r.code)
	}
	if r := tt(t, s, signedIn, "clip", "edit", "sacada-into-the-cross", "--end", "soon", "--json"); r.code != 1 {
		t.Fatalf("bad time: code %d", r.code)
	}
	if r := tt(t, s, nil, "clip", "edit", "sacada-into-the-cross", "--title", "x", "--json"); r.code != 3 {
		t.Fatalf("no token: code %d", r.code)
	}
}
