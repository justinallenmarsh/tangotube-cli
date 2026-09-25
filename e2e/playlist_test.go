package e2e

import (
	"strings"
	"testing"

	"github.com/justinallenmarsh/tangotube-cli/e2e/fixtures"
)

func TestPlaylistMakeFillAndOrder(t *testing.T) {
	s := server(t)
	if got := jq(t, s, ".id", "playlist", "create", "Di Sarli for Sunday", "--visibility", "unlisted", "--token", fixtures.Token); got != "di-sarli-for-sunday" {
		t.Fatalf("create id = %q", got)
	}
	added := parse(t, tt(t, s, signedIn, "playlist", "add", "di-sarli-for-sunday", "JxNPcHQcHDI", "https://youtu.be/b2GrLTAoyDU", "--json"))
	if !strings.Contains(added.Summary, "Torsion, Pivot") || !strings.Contains(added.Summary, "Istanbul Tango Fiesta") {
		t.Fatalf("adding two should name both: %q", added.Summary)
	}
	if got := jq(t, s, ".videos[0].id", "playlist", "move", "di-sarli-for-sunday", "b2GrLTAoyDU", "--to", "1", "--token", fixtures.Token); got != "b2GrLTAoyDU" {
		t.Fatalf("after move, first = %q", got)
	}
	if r := tt(t, s, signedIn, "playlist", "move", "di-sarli-for-sunday", "b2GrLTAoyDU", "--json"); r.code != 1 {
		t.Fatalf("move without --to: code %d", r.code)
	}
}

func TestPlaylistReadsWithoutATokenButChangesOnlyWithOne(t *testing.T) {
	s := server(t)
	if got := jq(t, s, ".videos | length", "playlist", "show", "https://tangotube.tv/playlists/di-sarli-for-sunday"); got != "3" {
		t.Fatalf("show = %q", got)
	}
	if r := tt(t, s, nil, "playlist", "delete", "di-sarli-for-sunday", "--json"); r.code != 3 {
		t.Fatalf("delete without a token: code %d", r.code)
	}
	env := parse(t, tt(t, s, signedIn, "playlist", "edit", "di-sarli-for-sunday", "--visibility", "friends", "--json"))
	if env.Error == nil || env.Error.Code != "usage" {
		t.Fatalf("bad visibility: %+v", env)
	}
	if r := tt(t, s, signedIn, "playlist", "edit", "di-sarli-for-sunday", "--json"); r.code != 1 {
		t.Fatalf("edit with nothing to change: code %d", r.code)
	}
	for _, args := range [][]string{{"rename", "scratch", "Scratch pad"}, {"remove", "scratch", "4p7S-qW0-xk"}, {"delete", "scratch", "--yes"}, {"list"}} {
		env := parse(t, tt(t, s, signedIn, append(append([]string{"playlist"}, args...), "--json")...))
		if !env.OK || env.Summary == "" {
			t.Fatalf("playlist %v: %+v", args, env)
		}
	}
}

func TestPlaylistDeleteWantsYes(t *testing.T) {
	s := server(t)
	r := tt(t, s, signedIn, "playlist", "delete", "scratch", "--json")
	env := parse(t, r)
	if r.code != 1 || env.Error == nil || env.Error.Code != "usage" || env.Error.Hint != "tt playlist delete scratch --yes" {
		t.Fatalf("delete without --yes: code %d, %+v", r.code, env)
	}
	if !strings.Contains(env.Error.Message, "“Scratch pad” (1 video)") {
		t.Fatalf("the refusal should say what would go: %q", env.Error.Message)
	}
}
