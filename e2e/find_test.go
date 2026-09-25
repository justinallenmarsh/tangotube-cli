package e2e

import (
	"strings"
	"testing"
)

// jq runs tt with --jq and returns what it printed, failing on a non-zero exit.
func jq(t *testing.T, s string, expr string, args ...string) string {
	t.Helper()
	r := tt(t, s, nil, append(args, "--jq", expr)...)
	if r.code != 0 {
		t.Fatalf("tt %s: code %d stdout %q stderr %q", strings.Join(args, " "), r.code, r.stdout, r.stderr)
	}
	return strings.TrimSpace(r.stdout)
}

func TestEveryCatalogueListAnswers(t *testing.T) {
	s := server(t)
	for _, list := range []string{"dancers", "couples", "orchestras", "songs", "events", "channels", "singers", "champions"} {
		env := parse(t, tt(t, s, nil, list, "--json"))
		if !env.OK || env.Summary == "" {
			t.Errorf("tt %s: %+v", list, env)
		}
	}
}

func TestListsHandBackSlugsAndTheNextPage(t *testing.T) {
	s := server(t)
	if got := jq(t, s, ".dancers[0].champion", "dancers", "--champion"); got != "true" {
		t.Fatalf("champion = %q", got)
	}
	env := parse(t, tt(t, s, nil, "dancers", "--champion", "--json"))
	if !strings.HasPrefix(env.Breadcrumbs[0], "tt dancer show ") || !strings.Contains(env.Breadcrumbs[1], "--cursor 2") {
		t.Fatalf("breadcrumbs %v", env.Breadcrumbs)
	}
	if got := jq(t, s, ".champions[0].year", "champions", "--year", "2005"); got != "2005" {
		t.Fatalf("champions year = %q", got)
	}
}

func TestFacetsCountTheSearch(t *testing.T) {
	s := server(t)
	if got := jq(t, s, ".facets.recorded | length > 0", "facets", "--orchestra", "di sarli"); got != "true" {
		t.Fatalf("recorded histogram: %q", got)
	}
	if got := jq(t, s, ".facets | has(\"orchestra\")", "facets", "--orchestra", "di sarli"); got != "false" {
		t.Fatal("a facet already filtered on is not counted")
	}
}

func TestResolveReadsANameIntoFilters(t *testing.T) {
	s := server(t)
	if got := jq(t, s, ".reading[0].value", "resolve", "di sarli noelia"); got != "carlos-di-sarli" {
		t.Fatalf("reading = %q", got)
	}
	if r := tt(t, s, nil, "resolve", "", "--json"); r.code != 1 {
		t.Fatalf("an empty name is usage: code %d", r.code)
	}
}

func TestSearchBrowsesWithASortAlone(t *testing.T) {
	s := server(t)
	if got := jq(t, s, ".filters.sort", "search", "--sort", "hidden-gems", "--kind", "class"); got != "hidden_gems" {
		t.Fatalf("sort = %q", got)
	}
	r := tt(t, s, nil, "search", "--sort", "loudest", "--json")
	if env := parse(t, r); r.code != 1 || env.Error == nil || env.Error.Code != "usage" {
		t.Fatalf("unknown sort: code %d %+v", r.code, env)
	}
}

func TestEventChannelVersionsAndHome(t *testing.T) {
	s := server(t)
	if got := jq(t, s, ".years | length > 3", "event", "show", "planetango"); got != "true" {
		t.Fatal("event editions")
	}
	if got := jq(t, s, ".trending | length > 0", "channel", "show", "UCCoOxQMnmwZ-jhezbLfSUgQ"); got != "true" {
		t.Fatal("channel trending")
	}
	if got := jq(t, s, ".versions[0].slug", "song", "versions", "todo-es-amor-fulvio-salamanca"); got != "todo-es-amor-rodolfo-biagi" {
		t.Fatalf("versions = %q", got)
	}
	if got := jq(t, s, ".sections[0].search", "home"); !strings.HasPrefix(got, "tt search") {
		t.Fatalf("home section search = %q", got)
	}
	if r := tt(t, s, nil, "event", "show", "nowhere", "--json"); r.code != 2 {
		t.Fatalf("an unknown event exits 2, got %d", r.code)
	}
}

func TestDancerShowAddsTimelineAndTour(t *testing.T) {
	s := server(t)
	if got := jq(t, s, ".timeline | length > 5", "dancer", "show", "sebastian-achaval", "--timeline", "--tour"); got != "true" {
		t.Fatal("timeline")
	}
	if got := jq(t, s, ".tour.performances_count > 20", "dancer", "show", "sebastian-achaval", "--tour"); got != "true" {
		t.Fatal("tour count")
	}
}

func TestVideoShowNamesTheOtherDancesOfTheSession(t *testing.T) {
	s := server(t)
	if got := jq(t, s, ".performance_session | [.position, .total] | map(tostring) | join(\"/\")", "video", "show", "bUFFLZVvttk"); got != "1/3" {
		t.Fatalf("session = %q", got)
	}
}

func TestLyricsSayWhenTheEnglishIsAMachineTranslation(t *testing.T) {
	if got := jq(t, server(t), ".lyrics.en_source", "song", "show", "volver-a-sonar-carlos-di-sarli", "--lyrics"); got != "machine" {
		t.Fatalf("en_source = %q", got)
	}
}
