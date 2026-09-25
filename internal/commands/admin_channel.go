package commands

import (
	"net/http"
	"net/url"

	"github.com/spf13/cobra"
)

// tt admin channel: whether a channel's videos show, and fetching them now.
func newAdminChannel(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "channel",
		Short: "Operator changes to channels: review, activate, deactivate, reject-noise, sync.",
		Args:  cobra.NoArgs,
	}
	for _, verb := range []struct {
		name, short, long string
		confirm           bool
	}{
		{"review", "Mark a channel as tango and trusted; its videos show and it syncs nightly.", "", false},
		{"activate", "Show a deactivated channel's videos again.", "", false},
		{"deactivate", "Take a channel's videos out of every listing. Asks for --yes.",
			"Take a channel's videos out of every listing. They stay reachable by id, and\ntt admin undo puts the channel back. Asks for --yes.", true},
		{"reject-noise", "Reject a channel as not tango, taking all its videos out. Cannot be undone.",
			"Reject a channel as not tango: every video it holds is taken out in bulk, and\nit leaves the review queue. This cannot be undone with tt admin undo (tt admin\nchannel review puts the channel back). Asks for --yes.", true},
		{"sync", "Fetch a channel's new videos now, in the background.",
			"Fetch a channel's new videos now, in the background, instead of waiting for\nthe nightly sync. Only an active, reviewed channel syncs.", false},
	} {
		var w writeFlags
		name := verb.name
		c := &cobra.Command{
			Use:     name + " CHANNEL",
			Short:   verb.short,
			Long:    verb.long,
			Example: "  tt admin channel " + name + " UCtdgMR0bmogczrZNpPaO66Q --dry-run",
			Args:    exactArgs(1, "tt admin channel "+name+" UCtdgMR0bmogczrZNpPaO66Q"),
			RunE: func(cmd *cobra.Command, args []string) error {
				return a.adminWrite(cmd, http.MethodPost, "/api/v1/admin/channels/"+url.PathEscape(args[0])+"/"+name, &w, nil)
			},
		}
		w.bind(c, verb.confirm)
		cmd.AddCommand(c)
	}
	return cmd
}
