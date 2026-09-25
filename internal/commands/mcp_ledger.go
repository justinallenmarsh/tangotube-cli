package commands

import (
	"context"
	"fmt"
	"net/http"

	"github.com/justinallenmarsh/tangotube-cli/internal/api"
	"github.com/justinallenmarsh/tangotube-cli/internal/output"
)

// adminLedgerTools are the payload ledger as MCP tools, listed with the other
// admin tools only for an operator's admin token.
func adminLedgerTools() []mcpTool {
	return []mcpTool{
		{
			Name: "admin_payload",
			Description: "Operator only. The payload ledger, the jumpstart loop over the API. " +
				"export: a queue's work list (kind gender|family|song|credit|probe|withdrawal; limit up to 3000, 500 by default), each row with ref, context and decide, the shape of the answer. " +
				"import: land decided rows (kind, agent claude|grok|human, rows: objects each with ref plus the decide keys; up to 5000) through the idempotent importer; a malformed row refuses the lot, named by its line. Always dry_run first and tell the person what it would apply. " +
				"status: the payloads landed; counts true also counts the rows waiting per queue (a few seconds). show: one payload's rows (state applied|skipped|reverted). " +
				"revert: put back what a payload, or one row (row: id or ref), overwrote; needs confirm, which only the person may give, and a reverted row is never re-applied. " +
				"probe, credit and withdrawal refs name appearances: export and import against the same database. Not undoable with admin_undo.",
			InputSchema: schema([]string{"verb"}, map[string]any{
				"verb": enum("What to do.", "export", "import", "status", "show", "revert"),
				"kind": enum("export, import, status: the queue.", payloadKinds...), "id": num("show, revert: the payload's id."),
				"agent": enum("import: who decided the rows.", "claude", "grok", "human"),
				"rows":  map[string]any{"type": "array", "items": map[string]any{"type": "object"}, "description": "import: the decided rows."},
				"row":   str("revert: one row's id or ref."), "state": enum("show: which rows.", "applied", "skipped", "reverted"),
				"limit": num("export: up to 3000; status, show: up to 50."), "cursor": str("status, show: next_cursor."),
				"counts":  boolean("status: also count each queue's waiting rows."),
				"dry_run": boolean("import, revert: say what would happen, write nothing."), "confirm": boolean("revert: the person agreed."),
				"note": str("revert: why."),
			}),
			call: func(c context.Context, cl *api.Client, args map[string]any) (*output.Envelope, error) {
				switch verb := arg(args, "verb"); verb {
				case "export":
					return cl.Do(c, http.MethodGet, "/api/v1/admin/payloads/export", query(args, "kind", "limit"), nil, true)
				case "import":
					payload := body(args, "kind", "agent", "dry_run")
					payload["rows"] = args["rows"]
					payload["source"] = "mcp"
					return cl.Do(c, http.MethodPost, "/api/v1/admin/payloads", nil, payload, true)
				case "status":
					return cl.Do(c, http.MethodGet, "/api/v1/admin/payloads", query(args, "kind", "counts", "limit", "cursor"), nil, true)
				case "show":
					return cl.Do(c, http.MethodGet, fmt.Sprintf("/api/v1/admin/payloads/%v", args["id"]), query(args, "state", "limit", "cursor"), nil, true)
				case "revert":
					payload := body(args, "dry_run", "confirm", "note")
					if row := arg(args, "row"); row != "" {
						payload["row_id"] = row
					} else if n, ok := args["row"].(float64); ok {
						payload["row_id"] = fmt.Sprint(int(n))
					}
					return cl.Do(c, http.MethodPost, fmt.Sprintf("/api/v1/admin/payloads/%v/revert", args["id"]), nil, payload, true)
				}
				return nil, Usage("verb is export, import, status, show or revert", "")
			},
		},
	}
}
