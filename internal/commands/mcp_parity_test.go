package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/justinallenmarsh/tangotube-cli/internal/auth"
)

// manifestTool is one tool as TangoTube's own MCP server declares it
// (Mcp::Registry in the Rails app). bin/rails mcp:manifest writes the file.
type manifestTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
	Scope       *string        `json:"scope"`
	Destructive bool           `json:"destructive"`
}

func loadManifest(t *testing.T) []manifestTool {
	t.Helper()
	raw, err := os.ReadFile("testdata/mcp_tools.json")
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Tools []manifestTool `json:"tools"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	return m.Tools
}

// tt mcp serves exactly the tools the remote MCP at tangotube.tv/mcp serves:
// the same names in the same order, descriptions, schemas, the same admin
// set and the same destructive ones. When this fails, the Rails registry
// changed: make tt mcp match it.
func TestMCPToolsMatchTheServersRegistry(t *testing.T) {
	manifest := loadManifest(t)
	ours := append(mcpTools(), allAdminTools()...)
	admin := map[string]bool{}
	for _, tool := range allAdminTools() {
		admin[tool.Name] = true
	}
	if len(ours) != len(manifest) {
		t.Errorf("tt mcp has %d tools, the registry %d", len(ours), len(manifest))
	}
	for i, want := range manifest {
		if i >= len(ours) {
			t.Errorf("tt mcp lacks %s", want.Name)
			continue
		}
		got := ours[i]
		if got.Name != want.Name {
			t.Errorf("tool %d: tt mcp has %s, the registry %s", i, got.Name, want.Name)
			continue
		}
		if got.Description != want.Description {
			t.Errorf("%s: description differs\n tt: %s\nweb: %s", got.Name, got.Description, want.Description)
		}
		if !reflect.DeepEqual(normalized(t, got.InputSchema), normalized(t, want.InputSchema)) {
			a, _ := json.Marshal(got.InputSchema)
			b, _ := json.Marshal(want.InputSchema)
			t.Errorf("%s: schema differs\n tt: %s\nweb: %s", got.Name, a, b)
		}
		wantAdmin := want.Scope != nil && *want.Scope == "admin"
		if admin[got.Name] != wantAdmin {
			t.Errorf("%s: admin in tt mcp %v, in the registry %v", got.Name, admin[got.Name], wantAdmin)
		}
		if destructiveTools[got.Name] != want.Destructive {
			t.Errorf("%s: destructive in tt mcp %v, in the registry %v", got.Name, destructiveTools[got.Name], want.Destructive)
		}
	}
}

func normalized(t *testing.T, v any) any {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out any
	_ = json.Unmarshal(raw, &out)
	return out
}

// A destructive tool without confirm only previews, as on the remote MCP:
// the request goes out with dry_run, and the answer says what to pass.
func TestDestructiveToolWithoutConfirmOnlyPreviews(t *testing.T) {
	var sent map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sent = map[string]any{}
		_ = json.NewDecoder(r.Body).Decode(&sent)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"data":{"dry_run":true},"summary":"Would change hidden on Noelia y Carlitos.","breadcrumbs":[]}`))
	}))
	defer srv.Close()
	a := &App{Out: &bytes.Buffer{}, Err: &bytes.Buffer{}, Env: func(string) string { return "" }, Home: t.TempDir(), NewStore: auth.NewStore}
	a.Flags.APIURL = srv.URL
	a.Flags.Token = "tt_live_x"

	result := callTool(context.Background(), a, allAdminTools(), "admin_video", map[string]any{"video": "abc", "verb": "hide"})
	if sent["dry_run"] != true {
		t.Errorf("want dry_run sent, got %v", sent)
	}
	text := result["content"].([]map[string]any)[0]["text"].(string)
	if !strings.Contains(text, "Pass confirm: true") {
		t.Errorf("want the answer to say how to apply it, got %s", text)
	}

	callTool(context.Background(), a, allAdminTools(), "admin_video", map[string]any{"video": "abc", "verb": "hide", "confirm": true})
	if _, ok := sent["dry_run"]; ok {
		t.Errorf("with confirm, want no dry_run, got %v", sent)
	}
}
