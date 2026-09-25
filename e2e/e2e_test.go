package e2e

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/justinallenmarsh/tangotube-cli/e2e/fixtures"

	// Linked so go test's cache notices when the CLI changes; TestMain builds
	// the binary from source, which the cache cannot see on its own.
	_ "github.com/justinallenmarsh/tangotube-cli/internal/commands"
)

var ttBin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "tt-e2e")
	if err != nil {
		panic(err)
	}
	ttBin = filepath.Join(dir, "tt")
	build := exec.Command("go", "build", "-o", ttBin, "../cmd/tt")
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		panic(err)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

type result struct {
	stdout, stderr string
	code           int
}

func tt(t *testing.T, server string, env []string, args ...string) result {
	t.Helper()
	return ttStdin(t, server, env, "", args...)
}

// ttStdin is tt with stdin, for the commands that read "-".
func ttStdin(t *testing.T, server string, env []string, stdin string, args ...string) result {
	t.Helper()
	cmd := exec.Command(ttBin, args...)
	cmd.Stdin = strings.NewReader(stdin)
	home := t.TempDir()
	cmd.Env = append([]string{
		"HOME=" + home, "XDG_CONFIG_HOME=" + filepath.Join(home, ".config"),
		"PATH=" + os.Getenv("PATH"), "TANGOTUBE_NO_KEYRING=1", "TANGOTUBE_API_URL=" + server,
	}, env...)
	cmd.Dir = home
	var out, errOut strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err := cmd.Run()
	code := 0
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		code = exit.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return result{out.String(), errOut.String(), code}
}

type envelope struct {
	OK          bool            `json:"ok"`
	Data        json.RawMessage `json:"data"`
	Summary     string          `json:"summary"`
	Breadcrumbs []string        `json:"breadcrumbs"`
	Error       *struct {
		Code    string `json:"code"`
		Hint    string `json:"hint"`
		Message string `json:"message"`
	} `json:"error"`
}

func parse(t *testing.T, r result) envelope {
	t.Helper()
	var env envelope
	if err := json.Unmarshal([]byte(r.stdout), &env); err != nil {
		t.Fatalf("stdout is not an envelope: %v\n%s\nstderr: %s", err, r.stdout, r.stderr)
	}
	return env
}

func server(t *testing.T) string {
	s := httptest.NewServer(fixtures.Handler())
	t.Cleanup(s.Close)
	return s.URL
}

func TestSearchWritesTheEnvelopeWithBreadcrumbs(t *testing.T) {
	r := tt(t, server(t), nil, "search", "di sarli", "--json")
	env := parse(t, r)
	if r.code != 0 || !env.OK || len(env.Breadcrumbs) == 0 {
		t.Fatalf("code %d, env %+v", r.code, env)
	}
	var data struct {
		Videos []struct{ ID string } `json:"videos"`
	}
	_ = json.Unmarshal(env.Data, &data)
	if len(data.Videos) == 0 {
		t.Fatal("no videos")
	}

	show := parse(t, tt(t, server(t), nil, "video", "show", "2ByaUQXeAeo", "--json"))
	if !show.OK {
		t.Fatalf("video show: %+v", show)
	}
}

func TestTechniqueSearchCarriesClips(t *testing.T) {
	r := tt(t, server(t), nil, "search", "sacada", "--technique", "sacada", "--dancer", "noelia hurtado", "--json", "--jq", ".videos[0].clips[0].tags[0]")
	if r.code != 0 || strings.TrimSpace(r.stdout) != "sacada" {
		t.Fatalf("code %d stdout %q stderr %q", r.code, r.stdout, r.stderr)
	}
}

func TestPipedOutputIsJSONWithoutAsking(t *testing.T) {
	env := parse(t, tt(t, server(t), nil, "search", "noelia"))
	if !env.OK {
		t.Fatal("expected ok")
	}
}

func TestQuietWritesOnlyTheData(t *testing.T) {
	r := tt(t, server(t), nil, "clip", "tags", "--quiet")
	var data map[string][]string
	if err := json.Unmarshal([]byte(r.stdout), &data); err != nil || len(data["techniques"]) == 0 {
		t.Fatalf("quiet output %q: %v", r.stdout, err)
	}
}

func TestExitCodes(t *testing.T) {
	s := server(t)
	cases := []struct {
		name string
		env  []string
		args []string
		code int
		err  string
	}{
		{"usage", nil, []string{"search", "--json"}, 1, "usage"},
		{"usage from the API", nil, []string{"search", "sacada", "--technique", "sacada", "--event", "mundial"}, 1, "usage"},
		{"bad time", nil, []string{"clip", "create", "x", "--start", "1:7", "--end", "1:18", "--token", fixtures.Token}, 1, "usage"},
		{"not found", nil, []string{"video", "show", "nope"}, 2, "not_found"},
		{"no token on a write", nil, []string{"clip", "create", "2ByaUQXeAeo", "--start", "1:12", "--end", "1:18", "--tag", "sacada"}, 3, "auth"},
		{"bad token", []string{"TANGOTUBE_TOKEN=tt_live_wrong"}, []string{"auth", "status"}, 3, "auth"},
		{"network", []string{"TANGOTUBE_API_URL=http://127.0.0.1:1"}, []string{"search", "di sarli"}, 6, "network"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := tt(t, s, c.env, c.args...)
			env := parse(t, r)
			if r.code != c.code || env.Error == nil || env.Error.Code != c.err {
				t.Fatalf("code %d (want %d), env %+v, stderr %s", r.code, c.code, env, r.stderr)
			}
		})
	}
}

func TestClipCreateAndDeleteWithAToken(t *testing.T) {
	s := server(t)
	env := []string{"TANGOTUBE_TOKEN=" + fixtures.Token}
	r := tt(t, s, env, "clip", "create", "2ByaUQXeAeo", "--start", "1:12", "--end", "1:18", "--tag", "sacada", "--jq", ".id")
	if r.code != 0 || strings.TrimSpace(r.stdout) != "sacada-at-1-12" {
		t.Fatalf("create: %+v", r)
	}
	if r := tt(t, s, env, "clip", "delete", "sacada-at-1-12", "--json"); r.code != 0 {
		t.Fatalf("delete: %+v", r)
	}
}

func TestLoginWithTokenStoresItInTheFile(t *testing.T) {
	s := server(t)
	home := t.TempDir()
	env := []string{"HOME=" + home, "XDG_CONFIG_HOME=" + filepath.Join(home, ".config")}
	cmd := exec.Command(ttBin, "auth", "login", "--with-token", "--json")
	cmd.Env = append([]string{"PATH=" + os.Getenv("PATH"), "TANGOTUBE_NO_KEYRING=1", "TANGOTUBE_API_URL=" + s}, env...)
	cmd.Stdin = strings.NewReader(fixtures.Token + "\n")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("login: %v %s", err, out)
	}
	files, _ := filepath.Glob(filepath.Join(home, ".config", "tangotube", "token*"))
	if len(files) != 1 {
		t.Fatalf("token files: %v", files)
	}
	info, _ := os.Stat(files[0])
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("token file mode %v", info.Mode().Perm())
	}
	if strings.Contains(string(out), fixtures.Token) {
		t.Fatal("login printed the secret")
	}
}

func TestOpenUnderAgentPrintsTheURL(t *testing.T) {
	r := tt(t, server(t), nil, "open", "sacada-1-cuando-el-amor-muere", "--agent", "--jq", ".url")
	if strings.TrimSpace(r.stdout) != "https://tangotube.tv/clips/sacada-1-cuando-el-amor-muere" {
		t.Fatalf("%+v", r)
	}
}

func TestCommandsSkillAndDoctor(t *testing.T) {
	s := server(t)
	var cmds struct {
		Commands []struct{ Path string } `json:"commands"`
	}
	env := parse(t, tt(t, s, nil, "commands", "--json"))
	_ = json.Unmarshal(env.Data, &cmds)
	if len(cmds.Commands) < 15 {
		t.Fatalf("only %d commands", len(cmds.Commands))
	}

	if r := tt(t, s, nil, "skill"); !strings.HasPrefix(r.stdout, "---\nname: tangotube") {
		t.Fatalf("skill: %q", r.stdout[:min(80, len(r.stdout))])
	}

	r := tt(t, s, nil, "doctor", "--json")
	doc := parse(t, r)
	if r.code != 0 || !doc.OK {
		t.Fatalf("doctor: %+v %s", doc, r.stderr)
	}
}

func TestSetupInstallsTheSkill(t *testing.T) {
	r := tt(t, server(t), nil, "setup", "claude", "--json")
	var data struct{ Installed []string }
	_ = json.Unmarshal(parse(t, r).Data, &data)
	if r.code != 0 || len(data.Installed) != 1 || !strings.HasSuffix(data.Installed[0], ".claude/skills/tangotube/SKILL.md") {
		t.Fatalf("%+v", r)
	}
}

func TestSongShowCarriesCreditsLinksAndLyricsOnRequest(t *testing.T) {
	r := tt(t, server(t), nil, "song", "show", "volver-a-sonar-carlos-di-sarli", "--json", "--jq", ".links.el_recodo")
	if r.code != 0 || !strings.HasPrefix(strings.TrimSpace(r.stdout), "https://www.el-recodo.com/music?") {
		t.Fatalf("code %d stdout %q stderr %q", r.code, r.stdout, r.stderr)
	}
	r = tt(t, server(t), nil, "song", "show", "volver-a-sonar-carlos-di-sarli", "--lyrics", "--json", "--jq", ".lyrics.es")
	if r.code != 0 || !strings.Contains(r.stdout, "No sé si fue mi mano") {
		t.Fatalf("lyrics: code %d stdout %q", r.code, r.stdout)
	}
	r = tt(t, server(t), nil, "song", "show", "nochero-soy-osvaldo-pugliese", "--lyrics", "--json", "--jq", ".lyrics.es")
	if r.code != 0 || strings.TrimSpace(r.stdout) != "null" {
		t.Fatalf("no lyrics: code %d stdout %q", r.code, r.stdout)
	}
}

func TestVideoShowCarriesTheRecording(t *testing.T) {
	r := tt(t, server(t), nil, "video", "show", "2ByaUQXeAeo", "--json", "--jq", ".song.recorded_on")
	if r.code != 0 || !strings.HasPrefix(strings.Trim(strings.TrimSpace(r.stdout), `"`), "19") {
		t.Fatalf("code %d stdout %q", r.code, r.stdout)
	}
}
