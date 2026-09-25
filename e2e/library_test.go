package e2e

import (
	"strings"
	"testing"

	"github.com/justinallenmarsh/tangotube-cli/e2e/fixtures"
)

var signedIn = []string{"TANGOTUBE_TOKEN=" + fixtures.Token}

func TestLikeListAndUnlike(t *testing.T) {
	s := server(t)
	env := parse(t, tt(t, s, signedIn, "like", "https://www.youtube.com/watch?v=n07s2yjCs-E", "--json"))
	if !env.OK || !strings.HasPrefix(env.Summary, "Liked") {
		t.Fatalf("like: %+v", env)
	}
	if got := jq(t, s, ".videos | length", "likes", "--token", fixtures.Token); got != "3" {
		t.Fatalf("likes = %q", got)
	}
	if env := parse(t, tt(t, s, signedIn, "unlike", "_zDlQzQA3-U", "--json")); env.Data == nil || !strings.HasPrefix(env.Summary, "Unliked") {
		t.Fatalf("unlike: %+v", env)
	}
	if r := tt(t, s, nil, "likes", "--json"); r.code != 3 {
		t.Fatalf("likes without a token: code %d", r.code)
	}
}

func TestHistoryAddsListsAndClearsOnlyWhenSure(t *testing.T) {
	s := server(t)
	if got := jq(t, s, ".watches[0].video.id", "history", "--token", fixtures.Token); got == "" {
		t.Fatal("history lists no video")
	}
	env := parse(t, tt(t, s, signedIn, "history", "add", "n07s2yjCs-E", "--at", "2026-09-20T21:30:00Z", "--json"))
	if !env.OK || !strings.HasPrefix(env.Summary, "Added") {
		t.Fatalf("history add: %+v", env)
	}
	r := tt(t, s, signedIn, "history", "clear", "--json")
	if env := parse(t, r); r.code != 1 || env.Error == nil || env.Error.Hint != "tt history clear --yes" {
		t.Fatalf("clear without --yes: code %d %+v", r.code, env)
	}
	if r := tt(t, s, signedIn, "history", "clear", "--yes", "--json"); r.code != 0 {
		t.Fatalf("clear --yes: code %d %s", r.code, r.stderr)
	}
}
