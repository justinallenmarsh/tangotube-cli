package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/justinallenmarsh/tangotube-cli/internal/output"
)

// tt admin announcement: the banner across the top of the site.
func newAdminAnnouncement(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "announcement",
		Short: "The banner across the top of the site: list, create, end.",
		Long: `The banner across the top of the site. One shows at a time, the lowest position
first, to everyone, to signed-in people or to signed-out ones, between its start
and its end. Ending one takes it off the site at once; tt admin undo puts it
back.`,
		Example: `  tt admin announcement list
  tt admin announcement create --title "Mundial week" --body "Every final, as it happens." --ends 2026-10-08 --dry-run
  tt admin announcement end 12`,
		Args: cobra.NoArgs,
	}
	cmd.AddCommand(newAnnouncementList(a), newAnnouncementCreate(a), newAnnouncementEnd(a))
	return cmd
}

type announcement struct {
	ID       int    `json:"id"`
	Title    string `json:"title"`
	Body     string `json:"body"`
	Link     string `json:"link"`
	LinkText string `json:"link_text"`
	Style    string `json:"style"`
	Audience string `json:"audience"`
	State    string `json:"state"`
	StartsAt string `json:"starts_at"`
	EndsAt   string `json:"ends_at"`
}

func newAnnouncementList(a *App) *cobra.Command {
	return &cobra.Command{
		Use:         "list",
		Short:       "Every announcement, newest first: live, scheduled or ended.",
		Example:     "  tt admin announcement list\n  tt admin announcement list --jq '.announcements[] | select(.state == \"live\")'",
		Annotations: map[string]string{listsThings: "yes"},
		Args:        cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.adminDo(cmd, http.MethodGet, "/api/v1/admin/announcements", a.listQuery(""), nil, func(p *output.Printer, raw json.RawMessage) {
				d := decode[struct {
					Announcements []announcement `json:"announcements"`
				}](raw)
				rows := make([][]string, 0, len(d.Announcements))
				for _, an := range d.Announcements {
					rows = append(rows, []string{strconv.Itoa(an.ID), an.State, audienceWord(an.Audience), window(an), an.Title})
				}
				p.Table([]output.Column{{Header: "ID", ID: true, Right: true}, {Header: "STATE"}, {Header: "FOR", Dim: true},
					{Header: "WHEN", Dim: true}, {Header: "TITLE", Flex: true}}, rows)
			})
		},
	}
}

func newAnnouncementCreate(a *App) *cobra.Command {
	var w writeFlags
	var title, body, link, linkText, style, audience, starts, ends string
	var position int
	var sticky bool
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Put up an announcement. Recorded; tt admin announcement end takes it down.",
		Long: `Put up an announcement: a title and a line of text, with a link if it leads
somewhere. It shows from --starts (now when left out) to --ends (until ended),
to --audience everyone (the default), signed_in or signed_out. --style is info
(the default), sponsor or feedback. Recorded; tt admin announcement end takes
it down.`,
		Example: `  tt admin announcement create --title "Mundial week" --body "Every final, as it happens." \
      --link /events/mundial-de-tango --link-text "Watch" --ends 2026-10-08 --dry-run`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if title == "" || body == "" {
				return Usage("an announcement needs --title and --body", `tt admin announcement create --title "…" --body "…" --dry-run`)
			}
			req := map[string]any{"title": title, "body": body}
			for key, value := range map[string]string{"link": link, "link_text": linkText, "style": style, "audience": audience, "starts_at": starts, "ends_at": ends} {
				if value != "" {
					req[key] = value
				}
			}
			if cmd.Flags().Changed("position") {
				req["position"] = position
			}
			if cmd.Flags().Changed("no-dismiss") {
				req["dismissible"] = !sticky
			}
			return a.adminDo(cmd, http.MethodPost, "/api/v1/admin/announcements", nil, w.body(req), showAnnouncement)
		},
	}
	f := cmd.Flags()
	f.StringVar(&title, "title", "", "The headline, a `TITLE`.")
	f.StringVar(&body, "body", "", "The line under it, `TEXT`.")
	f.StringVar(&link, "link", "", "Where it leads: a `URL` (https://…) or a path on TangoTube (/events/…).")
	f.StringVar(&linkText, "link-text", "", "The link's words, `TEXT`.")
	f.StringVar(&style, "style", "", "`STYLE`: info, sponsor or feedback.")
	f.StringVar(&audience, "audience", "", "Who sees it, `WHO`: everyone, signed_in or signed_out.")
	f.StringVar(&starts, "starts", "", "When it goes up, a `DATE` or time (2026-10-01, 2026-10-01T18:00). Now by default.")
	f.StringVar(&ends, "ends", "", "When it comes down, a `DATE` or time. Never by default.")
	f.IntVar(&position, "position", 1, "Which shows first when several are live, a `NUMBER`; lowest first.")
	f.BoolVar(&sticky, "no-dismiss", false, "People cannot close it.")
	w.bind(cmd, false)
	return cmd
}

func newAnnouncementEnd(a *App) *cobra.Command {
	var w writeFlags
	cmd := &cobra.Command{
		Use:     "end ID",
		Short:   "Take an announcement off the site now. Recorded; tt admin undo puts it back.",
		Example: "  tt admin announcement end 12 --dry-run",
		Args:    idArg("an announcement's id", "tt admin announcement end 12"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.adminDo(cmd, http.MethodPost, "/api/v1/admin/announcements/"+args[0]+"/end", nil, w.body(nil), showAnnouncement)
		},
	}
	w.bind(cmd, false)
	return cmd
}

func showAnnouncement(p *output.Printer, raw json.RawMessage) {
	d := decode[struct {
		Announcement announcement               `json:"announcement"`
		Would        map[string]json.RawMessage `json:"would"`
		Action       *adminAction               `json:"action"`
	}](raw)
	an := d.Announcement
	p.Field("title", an.Title)
	p.Para("body", an.Body)
	p.Field("link", join(" · ", an.LinkText, an.Link))
	p.Field("shows", join(" · ", an.State, audienceWord(an.Audience), window(an), an.Style))
	// A new announcement is all new: the fields above are the change.
	if _, creating := d.Would["announcement"]; creating || (d.Action != nil && d.Action.Action == "announcement.create") {
		if d.Action != nil {
			p.Field("recorded", fmt.Sprintf("action %d", d.Action.ID))
		}
		if an.ID != 0 {
			p.Field("undo", fmt.Sprintf("tt admin announcement end %d takes it down", an.ID))
		}
		return
	}
	showChange(p, raw)
}

func audienceWord(audience string) string {
	switch audience {
	case "signed_in":
		return "signed in"
	case "signed_out":
		return "signed out"
	}
	return audience
}

// window is "Oct 1 – Oct 8", "from Oct 1", "until Oct 8" or "always".
func window(an announcement) string {
	day := func(iso string) string {
		if len(iso) < 10 {
			return iso
		}
		return iso[:10]
	}
	switch {
	case an.StartsAt != "" && an.EndsAt != "":
		return day(an.StartsAt) + " – " + day(an.EndsAt)
	case an.StartsAt != "":
		return "from " + day(an.StartsAt)
	case an.EndsAt != "":
		return "until " + day(an.EndsAt)
	}
	return "always"
}
