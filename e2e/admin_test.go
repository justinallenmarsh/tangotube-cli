package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/justinallenmarsh/tangotube-cli/e2e/fixtures"
)

var adminToken = []string{"TANGOTUBE_TOKEN=" + fixtures.AdminToken}

func TestAdminNeedsAToken(t *testing.T) {
	r := tt(t, server(t), nil, "admin", "actions", "--json")
	if r.code != 3 || parse(t, r).Error.Code != "auth" {
		t.Fatalf("code %d stdout %s", r.code, r.stdout)
	}
}

func TestAdminHideDryRunThenUndo(t *testing.T) {
	s := server(t)
	dry := parse(t, tt(t, s, adminToken, "admin", "video", "hide", "EJv04w-mZaM", "--dry-run", "--json"))
	if !dry.OK || !strings.HasPrefix(dry.Summary, "Would hide") {
		t.Fatalf("dry run: %+v", dry)
	}
	hid := parse(t, tt(t, s, adminToken, "admin", "video", "hide", "https://www.youtube.com/watch?v=EJv04w-mZaM", "--note", "reupload of another video", "--json"))
	if !hid.OK || !strings.Contains(hid.Summary, "Undo: tt admin undo") {
		t.Fatalf("hide: %+v", hid)
	}
	undo := parse(t, tt(t, s, adminToken, "admin", "undo", "1", "--json"))
	if !undo.OK || !strings.HasPrefix(undo.Summary, "Undid action 1 (video.hide)") {
		t.Fatalf("undo: %+v", undo)
	}
}

func TestAdminUndoRefusesAndExitsNonZero(t *testing.T) {
	r := tt(t, server(t), adminToken, "admin", "undo", "7", "--json")
	env := parse(t, r)
	if r.code == 0 || env.OK || env.Error.Hint != "tt admin action 1" {
		t.Fatalf("code %d env %+v", r.code, env)
	}
}

func TestAdminActionsListsUndoable(t *testing.T) {
	r := tt(t, server(t), adminToken, "admin", "actions", "--json", "--jq", ".actions[0].undoable")
	if r.code != 0 || strings.TrimSpace(r.stdout) != "true" {
		t.Fatalf("code %d stdout %q", r.code, r.stdout)
	}
}

func TestAdminTurnsAwayADancersToken(t *testing.T) {
	r := tt(t, server(t), []string{"TANGOTUBE_TOKEN=" + fixtures.Token}, "admin", "jobs", "--json")
	env := parse(t, r)
	if r.code == 0 || env.Error.Code != "forbidden" || env.Error.Hint != "tt auth login --admin" {
		t.Fatalf("code %d env %+v", r.code, env)
	}
}

func TestAdminReportsAnswer(t *testing.T) {
	s := server(t)
	for _, args := range [][]string{
		{"dashboard"}, {"pipeline"}, {"intake"}, {"search-quality"}, {"channels"}, {"precision"}, {"jobs"},
		{"coverage", "--pile", "conflict"}, {"desk", "EJv04w-mZaM"}, {"describe", "dancer"},
	} {
		env := parse(t, tt(t, s, adminToken, append(append([]string{"admin"}, args...), "--json")...))
		if !env.OK || env.Summary == "" {
			t.Errorf("tt admin %s: %+v", strings.Join(args, " "), env)
		}
	}
}

func TestAdminQueryAnswersAndRefuses(t *testing.T) {
	s := server(t)
	r := tt(t, s, adminToken, "admin", "query", "SELECT o.name, count(*) FROM videos v JOIN songs s ON s.id = v.song_id JOIN orchestras o ON o.id = s.orchestra_id GROUP BY 1", "--json", "--jq", ".rows[0][0]")
	if r.code != 0 || !strings.Contains(r.stdout, "ARIENZO") {
		t.Fatalf("code %d stdout %q", r.code, r.stdout)
	}
	r = ttStdin(t, s, adminToken, "SELECT email FROM users", "admin", "query", "-", "--json")
	env := parse(t, r)
	if r.code != 1 || env.Error.Code != "usage" {
		t.Fatalf("code %d env %+v", r.code, env)
	}
}

func TestMCPServesAdminToolsOnlyToAnOperator(t *testing.T) {
	s := server(t)
	list := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n"
	if out := ttStdin(t, s, adminToken, list, "mcp").stdout; !strings.Contains(out, `"name":"admin_query"`) {
		t.Fatalf("operator's tools/list lacks admin_query: %s", out)
	}
	if out := ttStdin(t, s, []string{"TANGOTUBE_TOKEN=" + fixtures.Token}, list, "mcp").stdout; strings.Contains(out, `"admin_`) {
		t.Fatal("a dancer's tools/list offers admin tools")
	}
}

func TestAdminDancerEditPreviews(t *testing.T) {
	env := parse(t, tt(t, server(t), adminToken, "admin", "dancer", "edit", "noelia-hurtado", "--bio", "Born in Buenos Aires.", "--dry-run", "--json"))
	if !env.OK || env.Summary != "Would change bio on Noelia Hurtado." {
		t.Fatalf("%+v", env)
	}
}

func TestAdminDancerEditWantsAField(t *testing.T) {
	r := tt(t, server(t), adminToken, "admin", "dancer", "edit", "noelia-hurtado", "--json")
	if r.code != 1 || parse(t, r).Error.Code != "usage" {
		t.Fatalf("code %d stdout %s", r.code, r.stdout)
	}
}

func TestAdminDancerMergePreviewsThenWantsYes(t *testing.T) {
	s := server(t)
	dry := parse(t, tt(t, s, adminToken, "admin", "dancer", "merge", "bailaron-neolia-hurtado", "into", "noelia-hurtado", "--dry-run", "--json"))
	if !dry.OK || !strings.Contains(dry.Summary, "cannot be undone") {
		t.Fatalf("dry run: %+v", dry)
	}
	r := tt(t, s, adminToken, "admin", "dancer", "merge", "bailaron-neolia-hurtado", "into", "noelia-hurtado", "--json")
	env := parse(t, r)
	if r.code != 1 || env.Error.Hint != "tt admin dancer merge bailaron-neolia-hurtado into noelia-hurtado --yes" {
		t.Fatalf("code %d env %+v", r.code, env)
	}
}

func TestAdminCatalogueEditsPreview(t *testing.T) {
	s := server(t)
	for _, args := range [][]string{
		{"song", "edit", "volver-a-sonar-carlos-di-sarli", "--composer", "Carlos Di Sarli", "--recorded", "1941-01-01"},
		{"event", "edit", "planetango", "--city", "Buenos Aires", "--lat", "-34.6"},
		{"orchestra", "edit", "carlos-di-sarli", "--bio", "El Señor del Tango."},
	} {
		env := parse(t, tt(t, s, adminToken, append(append([]string{"admin"}, args...), "--dry-run", "--json")...))
		if !env.OK || !strings.HasPrefix(env.Summary, "Would change") {
			t.Errorf("tt admin %s: %+v", strings.Join(args, " "), env)
		}
	}
}

func TestAdminSongLyricsReadsStdin(t *testing.T) {
	r := ttStdin(t, server(t), adminToken, "To dream again, a translation.\n", "admin", "song", "lyrics", "volver-a-sonar-carlos-di-sarli", "--en", "-", "--dry-run", "--json")
	env := parse(t, r)
	if !env.OK || env.Summary != "Would change lyrics_en and lyrics_en_source of Volver a Soñar." {
		t.Fatalf("code %d env %+v", r.code, env)
	}
}

func TestAdminEditRejectsANonNumber(t *testing.T) {
	r := tt(t, server(t), adminToken, "admin", "event", "edit", "planetango", "--lat", "north", "--json")
	if r.code != 1 || parse(t, r).Error.Code != "usage" {
		t.Fatalf("code %d stdout %s", r.code, r.stdout)
	}
}

func TestAdminChannelDeactivateWantsYes(t *testing.T) {
	s := server(t)
	r := tt(t, s, adminToken, "admin", "channel", "deactivate", "UCCoOxQMnmwZ-jhezbLfSUgQ", "--json")
	env := parse(t, r)
	if r.code != 1 || env.Error.Hint != "tt admin channel deactivate UCCoOxQMnmwZ-jhezbLfSUgQ --yes" {
		t.Fatalf("code %d env %+v", r.code, env)
	}
	dry := parse(t, tt(t, s, adminToken, "admin", "channel", "deactivate", "UCCoOxQMnmwZ-jhezbLfSUgQ", "--dry-run", "--json"))
	if !dry.OK || dry.Summary != "Would deactivate Prischepov TV." {
		t.Fatalf("dry run: %+v", dry)
	}
}

func TestAdminVideoLabelStampImport(t *testing.T) {
	s := server(t)
	label := parse(t, tt(t, s, adminToken, "admin", "video", "label", "EJv04w-mZaM", "folklore", "--dry-run", "--json"))
	if !label.OK || !strings.HasPrefix(label.Summary, "Would label") {
		t.Fatalf("label: %+v", label)
	}
	r := tt(t, s, adminToken, "admin", "video", "stamp", "EJv04w-mZaM", "circus", "--json")
	if r.code != 1 || !strings.Contains(r.stdout, "or none") {
		t.Fatalf("stamp: code %d %s", r.code, r.stdout)
	}
	imp := parse(t, tt(t, s, adminToken, "admin", "video", "import", "EJv04w-mZaM", "https://youtu.be/aaaaaaaaaaa", "--dry-run", "--json"))
	if !imp.OK || !strings.HasPrefix(imp.Summary, "Would import 1 video") {
		t.Fatalf("import: %+v", imp)
	}
}

func TestAdminClipDeleteWantsYes(t *testing.T) {
	r := tt(t, server(t), adminToken, "admin", "clip", "delete", "parallel-cross-sacada-turn-linear-exit", "--json")
	env := parse(t, r)
	if r.code != 1 || env.Error.Hint != "tt admin clip delete parallel-cross-sacada-turn-linear-exit --yes" {
		t.Fatalf("code %d env %+v", r.code, env)
	}
}

func TestAdminChampionshipsLoadPreviews(t *testing.T) {
	env := parse(t, tt(t, server(t), adminToken, "admin", "championships", "load", "--dry-run", "--json"))
	if !env.OK || !strings.Contains(env.Summary, "unresolved champion") {
		t.Fatalf("%+v", env)
	}
}

func TestAdminClipEditReadsMinutes(t *testing.T) {
	r := tt(t, server(t), adminToken, "admin", "clip", "edit", "sacada-boleo-combo", "--start", "soon", "--json")
	if r.code != 1 || !strings.Contains(r.stdout, "seconds or m:ss") {
		t.Fatalf("code %d stdout %s", r.code, r.stdout)
	}
}

func TestAdminImageAddUploadsTheFileAndPreviews(t *testing.T) {
	s := server(t)
	path := filepath.Join(t.TempDir(), "noelia.jpg")
	if err := os.WriteFile(path, []byte{0xFF, 0xD8, 0xFF, 0xE0}, 0o600); err != nil {
		t.Fatal(err)
	}
	dry := parse(t, tt(t, s, adminToken, "admin", "image", "add", "dancer", "noelia-hurtado", path,
		"--licence", "permission", "--credit", "Photo: Ana Gómez", "--primary", "--dry-run", "--json"))
	if !dry.OK || !strings.HasPrefix(dry.Summary, "Would add a 640×800 jpg portrait") {
		t.Fatalf("dry run: %+v", dry)
	}
	added := parse(t, tt(t, s, adminToken, "admin", "image", "add", "dancer", "noelia-hurtado", path, "--licence", "permission", "--json"))
	if !added.OK || !strings.Contains(added.Summary, "cannot be undone") {
		t.Fatalf("add: %+v", added)
	}
}

func TestAdminImageAddRefusesBeforeSending(t *testing.T) {
	s := server(t)
	for _, args := range [][]string{
		{"dancer", "noelia-hurtado", "/no/such/file.jpg", "--licence", "permission"},
		{"dancer", "noelia-hurtado", "https://example.org/a.jpg"},
		{"tanda", "noelia-hurtado", "https://example.org/a.jpg", "--licence", "permission"},
	} {
		r := tt(t, s, adminToken, append([]string{"admin", "image", "add"}, append(args, "--json")...)...)
		if env := parse(t, r); r.code != 1 || env.Error.Code != "usage" {
			t.Errorf("%v: code %d env %+v", args, r.code, env)
		}
	}
}

func TestAdminImageAddSaysWhyAURLIsRefused(t *testing.T) {
	r := tt(t, server(t), adminToken, "admin", "image", "add", "dancer", "noelia-hurtado",
		"http://169.254.169.254/latest/meta-data/", "--licence", "public_page", "--json")
	env := parse(t, r)
	if r.code != 1 || !strings.Contains(env.Error.Message, "not a public address") {
		t.Fatalf("code %d env %+v", r.code, env)
	}
}

func TestAdminImageTakedownAsksForYes(t *testing.T) {
	s := server(t)
	r := tt(t, s, adminToken, "admin", "image", "takedown", "2", "--json")
	env := parse(t, r)
	if r.code == 0 || env.Error.Hint != "tt admin image takedown 2 --yes" || !strings.Contains(env.Error.Message, "picture 1 is shown instead") {
		t.Fatalf("code %d env %+v", r.code, env)
	}
	dry := parse(t, tt(t, s, adminToken, "admin", "image", "takedown", "2", "--dry-run", "--json"))
	if !dry.OK || !strings.HasPrefix(dry.Summary, "Would take down picture 2") {
		t.Fatalf("dry run: %+v", dry)
	}
}

func TestAdminImageListAndPrimary(t *testing.T) {
	s := server(t)
	r := tt(t, s, adminToken, "admin", "image", "list", "dancer", "noelia-hurtado", "--jq", ".showing.portrait")
	if r.code != 0 || strings.TrimSpace(r.stdout) != "2" {
		t.Fatalf("list: code %d stdout %q", r.code, r.stdout)
	}
	p := parse(t, tt(t, s, adminToken, "admin", "image", "primary", "1", "--json"))
	if !p.OK || !strings.Contains(p.Summary, "Undo: tt admin undo 41") {
		t.Fatalf("primary: %+v", p)
	}
	if r := tt(t, s, adminToken, "admin", "image", "primary", "noelia", "--json"); r.code != 1 {
		t.Fatalf("a non-numeric id: code %d", r.code)
	}
}
