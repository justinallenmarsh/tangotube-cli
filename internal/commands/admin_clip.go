package commands

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
)

// tt admin clip: anyone's clip, corrected or deleted by an operator.
func newAdminClip(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clip",
		Short: "Operator changes to anyone's clip: edit, delete.",
		Args:  cobra.NoArgs,
	}
	edit := newAdminEdit(a, "clip", "clips", "ID", `  tt admin clip edit sacada-boleo-combo --title "Sacada into boleo" --dry-run
  tt admin clip edit sacada-boleo-combo --start 1:05 --end 1:20`, []editFlag{
		{flag: "title", key: "title", usage: "The clip's `TITLE`."},
		{flag: "description", key: "description", usage: "A `DESCRIPTION`, or none."},
		{flag: "start", key: "start_s", usage: "Where it starts, `SECONDS` or m:ss.", kind: "time"},
		{flag: "end", key: "end_s", usage: "Where it ends, `SECONDS` or m:ss.", kind: "time"},
		{flag: "visibility", key: "visibility", usage: "`private`, unlisted or public."},
		{flag: "tags", key: "tags", usage: "Replace the tags with these `STEPS`, comma-separated (tt clip tags lists them).", kind: "list"},
	})
	var w writeFlags
	del := &cobra.Command{
		Use:   "delete ID",
		Short: "Delete anyone's clip. Asks for --yes; cannot be undone.",
		Long: `Delete anyone's clip. It asks for --yes and cannot be undone; the record
keeps what the clip was.`,
		Example: "  tt admin clip delete sacada-boleo-combo --dry-run",
		Args:    exactArgs(1, "tt admin clip delete sacada-boleo-combo"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.adminWrite(cmd, http.MethodDelete, "/api/v1/admin/clips/"+url.PathEscape(args[0]), &w, nil)
		},
	}
	w.bind(del, true)
	cmd.AddCommand(edit, del)
	return cmd
}

// tt admin championships load: the Mundial roll, from its checked-in file.
func newAdminChampionships(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "championships",
		Short: "The Mundial champions roll: load.",
		Args:  cobra.NoArgs,
	}
	var w writeFlags
	load := &cobra.Command{
		Use:   "load",
		Short: "Reload the Mundial roll from its file. Stops, loading nothing, on an unresolved champion.",
		Long: `Reload the Mundial champions from db/seeds/championships/mundial.yml. It is
safe to run again. --dry-run shows what the file holds, which dancers it
would create, and any champion whose slug matches no dancer: one of those
stops the load, and nothing is loaded.`,
		Example: "  tt admin championships load --dry-run",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.adminWrite(cmd, http.MethodPost, "/api/v1/admin/championships/load", &w, nil)
		},
	}
	w.bind(load, false)
	cmd.AddCommand(load)
	return cmd
}

func splitList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}
