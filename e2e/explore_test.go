package e2e

import (
	"strings"
	"testing"
)

func TestPerformanceShowsEveryVideoOfOneDance(t *testing.T) {
	s := server(t)
	if got := jq(t, s, ".dance.videos | length", "performance", "show", "cPJ3MjWDUVY"); got != "3" {
		t.Fatalf("videos = %q", got)
	}
	if got := jq(t, s, ".session | map(.videos_count) | map(tostring) | join(\",\")", "performance", "show", "80786"); got != "2,2,1" {
		t.Fatalf("session = %q", got)
	}
	// A pasted watch link works like the id.
	if got := jq(t, s, ".id", "performance", "show", "https://www.youtube.com/watch?v=cPJ3MjWDUVY"); got != "76192" {
		t.Fatalf("id = %q", got)
	}
	if r := tt(t, s, nil, "performance", "show", "nowhere00000", "--json"); r.code != 2 {
		t.Fatalf("an unknown performance exits 2, got %d", r.code)
	}
}

func TestPartnersWalkTheNetworkOneOrTwoRingsOut(t *testing.T) {
	s := server(t)
	if got := jq(t, s, ".nodes | map(select(.ring == 1)) | .[0].couple", "partners", "noelia-hurtado"); got != "carlitos-espinoza-noelia-hurtado" {
		t.Fatalf("first partner's couple = %q", got)
	}
	if got := jq(t, s, ".nodes | map(.ring) | max", "partners", "sebastian-achaval", "--depth", "2"); got != "2" {
		t.Fatalf("depth 2 reaches ring %q", got)
	}
	if r := tt(t, s, nil, "partners", "noelia-hurtado", "--depth", "3", "--json"); r.code != 1 {
		t.Fatalf("depth 3 is usage: code %d", r.code)
	}
}

func TestMCPExploresPerformancesAndPartners(t *testing.T) {
	s := server(t)
	r := ttStdin(t, s, nil, strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"performance_show","arguments":{"id":"cPJ3MjWDUVY"}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"partners","arguments":{"slug":"sebastian-achaval","depth":2}}}`,
	}, "\n")+"\n", "mcp")
	lines := strings.Split(strings.TrimSpace(r.stdout), "\n")
	if len(lines) != 2 {
		t.Fatalf("mcp answered %d lines:\n%s\n%s", len(lines), r.stdout, r.stderr)
	}
	if !strings.Contains(lines[0], `"isError":false`) || !strings.Contains(lines[0], "AiresDeMilonga") {
		t.Fatalf("performance_show: %.300s", lines[0])
	}
	if !strings.Contains(lines[1], `"isError":false`) || !strings.Contains(lines[1], "roxana-suarez") {
		t.Fatalf("partners: %.300s", lines[1])
	}
}
