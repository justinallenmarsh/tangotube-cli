package commands

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/spf13/cobra"

	"github.com/justinallenmarsh/tangotube-cli/internal/output"
)

// tt admin dancer: edit a dancer, manage the spellings the matcher reads as
// them, and fold a duplicate into the real record.
func newAdminDancer(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dancer",
		Short: "Operator changes to dancers: edit, alias, merge.",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(
		newAdminEdit(a, "dancer", "dancers", "SLUG", `  tt admin dancer edit noelia-hurtado --bio "Born in Buenos Aires." --dry-run
  tt admin dancer edit noelia-hurtado --role follower --reviewed true`, []editFlag{
			{flag: "name", key: "name", usage: "The dancer's `NAME` as the site prints it."},
			{flag: "nickname", key: "nickname", usage: "A `NICKNAME`, or none."},
			{flag: "bio", key: "bio", usage: "A short `BIO`, or none."},
			{flag: "gender", key: "gender", usage: "`male`, female or none."},
			{flag: "role", key: "primary_role", usage: "`leader`, follower, both, neither or none."},
			{flag: "reviewed", key: "reviewed", usage: "`true` once a person has checked the record.", kind: "bool"},
		}),
		newAdminDancerAlias(a), newAdminDancerMerge(a),
	)
	return cmd
}

func newAdminDancerAlias(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "alias",
		Short: "The other spellings the matcher reads as a dancer: list, add, rm.",
		Args:  cobra.NoArgs,
	}
	list := &cobra.Command{
		Use:     "list SLUG",
		Short:   "List a dancer's aliases.",
		Example: "  tt admin dancer alias list noelia-hurtado",
		Args:    exactArgs(1, "tt admin dancer alias list noelia-hurtado"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.adminDo(cmd, http.MethodGet, "/api/v1/admin/dancers/"+url.PathEscape(args[0])+"/aliases", nil, nil, func(p *output.Printer, raw json.RawMessage) {
				d := decode[struct {
					Aliases []struct {
						Name   string `json:"name"`
						Source string `json:"source"`
					} `json:"aliases"`
				}](raw)
				rows := make([][]string, 0, len(d.Aliases))
				for _, al := range d.Aliases {
					rows = append(rows, []string{al.Name, al.Source})
				}
				p.Table([]output.Column{{Header: "ALIAS", Flex: true}, {Header: "SOURCE", Dim: true}}, rows)
			})
		},
	}
	var addW writeFlags
	var replay bool
	add := &cobra.Command{
		Use:   "add SLUG SPELLING",
		Short: "Teach the matcher another spelling of a dancer's name.",
		Long: `Teach the matcher another spelling of a dancer's name, so titles that use it
credit this dancer. --replay re-reads the videos whose titles carry the
spelling (up to 500, in the background) so the credits land now.`,
		Example: `  tt admin dancer alias add noelia-hurtado "Noe Hurtado" --dry-run
  tt admin dancer alias add noelia-hurtado "Noe Hurtado" --replay`,
		Args: exactArgs(2, `tt admin dancer alias add noelia-hurtado "Noe Hurtado"`),
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{"name": args[1]}
			if replay {
				body["replay"] = true
			}
			return a.adminWrite(cmd, http.MethodPost, "/api/v1/admin/dancers/"+url.PathEscape(args[0])+"/aliases", &addW, body)
		},
	}
	add.Flags().BoolVar(&replay, "replay", false, "Re-read the videos whose titles carry the spelling.")
	addW.bind(add, false)
	var rmW writeFlags
	rm := &cobra.Command{
		Use:     "rm SLUG SPELLING",
		Short:   "Stop reading a spelling as this dancer.",
		Example: `  tt admin dancer alias rm noelia-hurtado "Noe Hurtado" --dry-run`,
		Args:    exactArgs(2, `tt admin dancer alias rm noelia-hurtado "Noe Hurtado"`),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/api/v1/admin/dancers/" + url.PathEscape(args[0]) + "/aliases/" + url.PathEscape(args[1])
			return a.adminWrite(cmd, http.MethodDelete, path, &rmW, nil)
		},
	}
	rmW.bind(rm, false)
	cmd.AddCommand(list, add, rm)
	return cmd
}

func newAdminDancerMerge(a *App) *cobra.Command {
	var w writeFlags
	cmd := &cobra.Command{
		Use:   "merge LOSER into WINNER",
		Short: "Fold a duplicate dancer into the real one. Cannot be undone.",
		Long: `Fold a duplicate dancer into the real one: its credits, couples, aliases,
titles, followers and pictures move to WINNER, its name becomes an alias, and
LOSER is deleted.

This cannot be undone. Run it with --dry-run first to see exactly what moves;
it only runs with --yes, and the record keeps a full snapshot of LOSER.`,
		Example: `  tt admin dancer merge noelia-hurtadoo into noelia-hurtado --dry-run
  tt admin dancer merge noelia-hurtadoo into noelia-hurtado --yes`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 3 && strings.EqualFold(args[1], "into") || len(args) == 2 {
				return nil
			}
			return Usage("say which dancer folds into which", "tt admin dancer merge noelia-hurtadoo into noelia-hurtado --dry-run")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			winner := args[len(args)-1]
			return a.adminWrite(cmd, http.MethodPost, "/api/v1/admin/dancers/"+url.PathEscape(args[0])+"/merge", &w, map[string]any{"into": winner})
		},
	}
	w.bind(cmd, true)
	return cmd
}
