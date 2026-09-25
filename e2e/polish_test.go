package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/justinallenmarsh/tangotube-cli/e2e/fixtures"
)

// inHome runs tt with a HOME that outlives the call, so a stored token does.
func inHome(t *testing.T, s, home, stdin string, args ...string) result {
	t.Helper()
	cmd := exec.Command(ttBin, args...)
	cmd.Env = []string{
		"HOME=" + home, "XDG_CONFIG_HOME=" + filepath.Join(home, ".config"),
		"PATH=" + os.Getenv("PATH"), "TANGOTUBE_NO_KEYRING=1", "TANGOTUBE_API_URL=" + s,
	}
	cmd.Stdin = strings.NewReader(stdin)
	var out, errOut strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err := cmd.Run()
	code := 0
	if exit, ok := err.(*exec.ExitError); ok {
		code = exit.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return result{out.String(), errOut.String(), code}
}

func TestARevokedStoredTokenDoesNotBreakSearch(t *testing.T) {
	s := server(t)
	home := t.TempDir()
	dir := filepath.Join(home, ".config", "tangotube")
	login := inHome(t, s, home, fixtures.Token+"\n", "auth", "login", "--with-token", "--json")
	if login.code != 0 {
		t.Fatalf("login: %+v", login)
	}
	stored, _ := filepath.Glob(filepath.Join(dir, "token*"))
	if len(stored) != 1 {
		t.Fatalf("token files: %v", stored)
	}
	_ = os.WriteFile(stored[0], []byte("tt_live_revoked\n"), 0o600)

	r := inHome(t, s, home, "", "search", "di sarli", "--json")
	if r.code != 0 || !parse(t, r).OK {
		t.Fatalf("search with a revoked stored token: %+v", r)
	}
	if !strings.Contains(r.stderr, "tt auth login") {
		t.Fatalf("no warning on stderr: %q", r.stderr)
	}
}

func TestLogoutRevokesTheStoredToken(t *testing.T) {
	s := server(t)
	home := t.TempDir()
	if r := inHome(t, s, home, fixtures.Token+"\n", "auth", "login", "--with-token", "--json"); r.code != 0 {
		t.Fatalf("login: %+v", r)
	}
	out := parse(t, inHome(t, s, home, "", "auth", "logout", "--json"))
	if out.Summary != "Signed out. The token is revoked." {
		t.Fatalf("logout: %+v", out)
	}
	again := parse(t, inHome(t, s, home, "", "auth", "logout", "--json"))
	if again.Summary != "tt was not signed in." {
		t.Fatalf("second logout: %+v", again)
	}
}

func TestPastedLinksWorkAsIDs(t *testing.T) {
	s := server(t)
	for _, link := range []string{
		"https://www.youtube.com/watch?v=2ByaUQXeAeo",
		"https://youtu.be/2ByaUQXeAeo",
		"https://tangotube.tv/watch?v=2ByaUQXeAeo&t=30",
	} {
		if r := tt(t, s, nil, "video", "show", link, "--json"); r.code != 0 {
			t.Errorf("video show %s: %+v", link, r)
		}
	}
	r := tt(t, s, nil, "open", "https://tangotube.tv/clips/sacada-1-cuando-el-amor-muere", "--agent", "--jq", ".url")
	if strings.TrimSpace(r.stdout) != "https://tangotube.tv/clips/sacada-1-cuando-el-amor-muere" {
		t.Fatalf("open clip link: %+v", r)
	}
}

func TestOpenFindsADancerPage(t *testing.T) {
	s := server(t)
	r := tt(t, s, nil, "open", "noelia-hurtado", "--agent", "--jq", ".url")
	if strings.TrimSpace(r.stdout) != s+"/dancers/noelia-hurtado" {
		t.Fatalf("%+v", r)
	}
}

func TestFlagMistakesAreUsageErrorsInWords(t *testing.T) {
	s := server(t)
	for _, args := range [][]string{
		{"search", "x", "--limit", "0"},
		{"search", "x", "--limit", "51"},
		{"search", "x", "--limit", "abc"},
		{"clip", "create", "2ByaUQXeAeo", "--start", "1:12"},
	} {
		r := tt(t, s, nil, append(args, "--json")...)
		env := parse(t, r)
		if r.code != 1 || env.Error == nil || env.Error.Code != "usage" || strings.Contains(r.stdout, "strconv") {
			t.Errorf("%v: %+v", args, r)
		}
	}
}

func TestBreadcrumbsNeverInventAClip(t *testing.T) {
	s := server(t)
	for _, args := range [][]string{{"search", "di sarli"}, {"video", "show", "2ByaUQXeAeo"}, {"search", "sacada", "--technique", "sacada"}} {
		for _, crumb := range parse(t, tt(t, s, nil, append(args, "--json")...)).Breadcrumbs {
			if strings.Contains(crumb, "clip create") {
				t.Errorf("%v suggests %q", args, crumb)
			}
		}
	}
}

func TestMCPRefusesArgumentsItDoesNotKnow(t *testing.T) {
	s := server(t)
	cmd := exec.Command(ttBin, "mcp")
	cmd.Env = []string{"HOME=" + t.TempDir(), "PATH=" + os.Getenv("PATH"), "TANGOTUBE_NO_KEYRING=1", "TANGOTUBE_API_URL=" + s}
	cmd.Stdin = strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"search","arguments":{"query":"di sarli"}}}` + "\n")
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"isError":true`) || !strings.Contains(string(out), "query") {
		t.Fatalf("%s", out)
	}
}

func TestMCPReachesThePersonsOwnLibrary(t *testing.T) {
	s := server(t)
	cmd := exec.Command(ttBin, "mcp")
	cmd.Env = []string{"HOME=" + t.TempDir(), "PATH=" + os.Getenv("PATH"), "TANGOTUBE_NO_KEYRING=1", "TANGOTUBE_API_URL=" + s, "TANGOTUBE_TOKEN=tt_live_fixture"}
	cmd.Stdin = strings.NewReader(strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"playlist_edit","arguments":{"action":"move","id":"di-sarli-for-sunday","video":"b2GrLTAoyDU","position":1}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"history_edit","arguments":{"action":"clear"}}}`,
	}, "\n") + "\n")
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, tool := range []string{"like", "history_edit", "playlist_edit", "follow", "following", "practice", "saved_searches", "notifications", "clip_edit"} {
		if !strings.Contains(lines[0], `"name":"`+tool+`"`) {
			t.Errorf("tools/list lacks %s", tool)
		}
	}
	if !strings.Contains(lines[1], `"isError":false`) || !strings.Contains(lines[1], "Reordered") {
		t.Fatalf("move: %s", lines[1])
	}
	if !strings.Contains(lines[2], `"isError":true`) || !strings.Contains(lines[2], "--yes") {
		t.Fatalf("clear without confirm: %s", lines[2])
	}
}
