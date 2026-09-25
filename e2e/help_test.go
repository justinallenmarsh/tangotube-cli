package e2e

import (
	"strings"
	"testing"
)

func TestSuggestSaysWhetherItAppliedOrWaits(t *testing.T) {
	s := server(t)
	env := parse(t, tt(t, s, signedIn, "suggest", "KICHdm0zuCQ", "--dancer", "roxana-suarez", "--role", "follower", "--json"))
	if !env.OK || !strings.HasPrefix(env.Summary, "Sent for review") {
		t.Fatalf("queued: %+v", env)
	}
	if got := jq(t, s, ".outcome + \" \" + .reason", "suggest", "p6owMk_o_8A", "--song", "milonga-criolla-francisco-canaro", "--agree", "--token", "tt_live_fixture"); got != "applied agreement" {
		t.Fatalf("agreement = %q", got)
	}
	r := tt(t, s, signedIn, "suggest", "KICHdm0zuCQ", "--dancer", "Sebastian Achavall", "--json")
	if env := parse(t, r); r.code != 1 || env.Error == nil || !strings.Contains(env.Error.Hint, "tt identify search dancer") {
		t.Fatalf("near miss: code %d %+v", r.code, env)
	}
}

func TestSuggestNamesOneFactOrABatch(t *testing.T) {
	s := server(t)
	if r := tt(t, s, signedIn, "suggest", "KICHdm0zuCQ", "--json"); r.code != 1 {
		t.Fatalf("nothing named: code %d", r.code)
	}
	if r := tt(t, s, signedIn, "suggest", "KICHdm0zuCQ", "--song", "a", "--event", "b", "--json"); r.code != 1 {
		t.Fatalf("two facts: code %d", r.code)
	}
	if r := tt(t, s, nil, "suggest", "KICHdm0zuCQ", "--song", "a", "--json"); r.code != 3 {
		t.Fatalf("no token: code %d", r.code)
	}
	cmd := []string{"suggest", "k-shh3fWESA", "--batch", "-", "--json"}
	r := ttStdin(t, s, signedIn, `[{"fact":"kind","op":"replace","kind":"performance"}]`, cmd...)
	if env := parse(t, r); !env.OK || !strings.Contains(env.Summary, "waiting for somebody to agree") {
		t.Fatalf("batch: %+v", env)
	}
	if r := ttStdin(t, s, signedIn, `{"fact": "song"}`, cmd...); r.code != 1 {
		t.Fatalf("not a list: code %d", r.code)
	}
}

func TestConfirmAgreeAndReport(t *testing.T) {
	s := server(t)
	if env := parse(t, tt(t, s, signedIn, "confirm", "k-shh3fWESA", "--json")); !env.OK || !strings.HasPrefix(env.Summary, "Confirmed") {
		t.Fatalf("confirm: %+v", env)
	}
	if r := tt(t, s, signedIn, "agree", "k-shh3fWESA", "1", "--json"); r.code != 1 {
		t.Fatalf("agreeing with your own: code %d", r.code)
	}
	if got := jq(t, s, ".report.weight", "report", "SkTQuLhQG9A", "--kind", "not_tango"); got != "1" {
		t.Fatalf("anonymous report weight = %q", got)
	}
	if r := tt(t, s, nil, "report", "SkTQuLhQG9A", "--json"); r.code != 1 {
		t.Fatalf("report without a kind: code %d", r.code)
	}
}

func TestClipTagSuggestPointsAtAnExistingStep(t *testing.T) {
	s := server(t)
	if env := parse(t, tt(t, s, signedIn, "clip", "tag-suggest", "parallel-cross-sacada-turn-linear-exit", "cross system", "--json")); !env.OK {
		t.Fatalf("suggest: %+v", env)
	}
	r := tt(t, s, signedIn, "clip", "tag-suggest", "parallel-cross-sacada-turn-linear-exit", "sacada", "--json")
	if env := parse(t, r); r.code != 1 || env.Error == nil || !strings.Contains(env.Error.Hint, "--add-tag sacada") {
		t.Fatalf("existing step: code %d %+v", r.code, env)
	}
}

func TestQueueIdentityAndSearchHandBackRunnableCommands(t *testing.T) {
	s := server(t)
	env := parse(t, tt(t, s, nil, "queue", "--proposed", "--json"))
	if !env.OK || len(env.Breadcrumbs) < 2 || !strings.HasSuffix(env.Breadcrumbs[1], "--agree") {
		t.Fatalf("queue: %+v", env)
	}
	if got := jq(t, s, "[.identity[].fact] | join(\",\")", "video", "show", "k-shh3fWESA", "--identity"); got != "song,dancers,event,kind" {
		t.Fatalf("identity = %q", got)
	}
	if got := jq(t, s, ".candidates[0].slug", "identify", "search", "song", "la", "mulateada"); got != "la-mulateada-carlos-di-sarli" {
		t.Fatalf("first candidate = %q", got)
	}
}

func TestMCPCanHelpAndSaysWhatHappened(t *testing.T) {
	s := server(t)
	r := ttStdin(t, s, signedIn, strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"suggest","arguments":{"video":"p6owMk_o_8A","fact":"song","song":"milonga-criolla-francisco-canaro","agree":true}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"report","arguments":{"video":"SkTQuLhQG9A","kind":"not_tango"}}}`,
	}, "\n")+"\n", "mcp")
	lines := strings.Split(strings.TrimSpace(r.stdout), "\n")
	if len(lines) != 3 {
		t.Fatalf("mcp answered %d lines:\n%s\n%s", len(lines), r.stdout, r.stderr)
	}
	for _, tool := range []string{"suggest", "confirm", "agree", "report", "tag_suggest", "queue", "identify_search"} {
		if !strings.Contains(lines[0], `"name":"`+tool+`"`) {
			t.Errorf("tools/list lacks %s", tool)
		}
	}
	if !strings.Contains(lines[1], `"isError":false`) || !strings.Contains(lines[1], "two sources agreeing") {
		t.Fatalf("suggest: %s", lines[1])
	}
	if !strings.Contains(lines[2], `"isError":false`) {
		t.Fatalf("report: %s", lines[2])
	}
}

// --limit is 50 for most lists, and more where the command says so: help
// names each command's own ceiling, and the check before the call keeps it.
func TestLimitHelpIsEachCommandsOwn(t *testing.T) {
	s := server(t)
	if out := tt(t, s, nil, "admin", "payload", "export", "--help").stdout; !strings.Contains(out, "up to 3000 (500 by default)") {
		t.Fatalf("payload export help:\n%s", out)
	}
	if out := tt(t, s, nil, "search", "--help").stdout; !strings.Contains(out, "up to 50.") {
		t.Fatalf("search help:\n%s", out)
	}
	if env := parse(t, tt(t, s, adminToken, "admin", "payload", "export", "gender", "--limit", "3001", "--json")); env.Error == nil || !strings.Contains(env.Error.Message, "from 1 to 3000") {
		t.Fatalf("3001: %+v", env)
	}
	if env := parse(t, tt(t, s, adminToken, "admin", "payload", "export", "gender", "--limit", "500", "--json")); env.Error != nil && strings.Contains(env.Error.Message, "--limit") {
		t.Fatalf("500 is allowed: %+v", env)
	}
}
