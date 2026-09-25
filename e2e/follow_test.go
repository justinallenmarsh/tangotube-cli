package e2e

import (
	"strings"
	"testing"
)

func TestFollowOnceAtALevelAndUnfollow(t *testing.T) {
	s := server(t)
	env := parse(t, tt(t, s, signedIn, "follow", "dancer", "leandro", "oliver", "--json"))
	if !env.OK || !strings.HasPrefix(env.Summary, "Following Leandro Oliver") {
		t.Fatalf("follow: %+v", env)
	}
	if env := parse(t, tt(t, s, signedIn, "follow", "dancer", "noelia hurtado", "--json")); !strings.HasPrefix(env.Summary, "Already following") {
		t.Fatalf("follow again: %+v", env)
	}
	r := tt(t, s, signedIn, "follow", "dancer", "x", "--level", "loud", "--json")
	if env := parse(t, r); r.code != 1 || env.Error == nil || env.Error.Code != "usage" {
		t.Fatalf("bad level: code %d %+v", r.code, env)
	}
	if r := tt(t, s, signedIn, "follow", "orchestra", "di sarli", "--json"); r.code != 1 {
		t.Fatalf("an orchestra cannot be followed: code %d", r.code)
	}
	if env := parse(t, tt(t, s, signedIn, "unfollow", "dancer", "noelia hurtado", "--json")); !strings.HasPrefix(env.Summary, "Unfollowed") {
		t.Fatalf("unfollow: %+v", env)
	}
}

func TestFollowingAndItsFeed(t *testing.T) {
	s := server(t)
	if got := jq(t, s, "[.follows[].kind] | sort | unique | join(\",\")", "following", "--token", "tt_live_fixture"); got != "channel,dancer,event" {
		t.Fatalf("kinds = %q", got)
	}
	if got := jq(t, s, ".groups | all(.count >= (.videos | length))", "following", "--feed", "--token", "tt_live_fixture"); got != "true" {
		t.Fatal("each group's count covers the videos it shows")
	}
}
