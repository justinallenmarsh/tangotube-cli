package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/justinallenmarsh/tangotube-cli/internal/api"
	"github.com/justinallenmarsh/tangotube-cli/internal/output"
)

// imageSubjects are the nouns tt admin image takes. Only a dancer, an
// orchestra or an event has a picture of its own; the server answers the
// other three with what their page shows instead.
var imageSubjects = []string{"dancer", "orchestra", "event", "couple", "channel", "singer"}

var imageLicences = []string{"unknown", "own_work", "permission", "public_page", "public_domain", "cc"}

const maxImageBytes = 10 << 20

// adminImage is one picture as the API returns it.
type adminImage struct {
	ID      int `json:"id"`
	Subject struct {
		Type string `json:"type"`
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"subject"`
	Kind            string `json:"kind"`
	Status          string `json:"status"`
	Shown           bool   `json:"shown"`
	Source          string `json:"source"`
	SourceURL       string `json:"source_url"`
	Licence         string `json:"licence"`
	Credit          string `json:"credit"`
	Width           int    `json:"width"`
	Height          int    `json:"height"`
	ContentType     string `json:"content_type"`
	Bytes           int    `json:"bytes"`
	AddedAt         string `json:"added_at"`
	TakedownReason  string `json:"takedown_reason"`
	RejectionReason string `json:"rejection_reason"`
}

func newAdminImage(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "image",
		Short: "The pictures a page shows: add, list, choose, take down, review.",
		Long: `The pictures a dancer's, orchestra's or event's page shows: a portrait and a
cover. Add one from disk or a URL, list them, choose the one shown, take one
down, and review proposed ones. Couples, channels and singers have no picture
of their own: a couple's page shows its dancers' portraits, a channel's its
YouTube avatar.

Adding is recorded and cannot be undone (take the picture down instead).
Choosing, taking down, accepting and rejecting are undone with tt admin undo.`,
		Example: `  tt admin image list dancer noelia-hurtado
  tt admin image add dancer noelia-hurtado ./noelia.jpg --kind portrait --licence permission --credit "Ana Gómez" --dry-run
  tt admin image primary 57
  tt admin image takedown 57 --reason "asked by the dancer" --yes`,
		Args: cobra.NoArgs,
	}
	cmd.AddCommand(newAdminImageAdd(a), newAdminImageList(a), newAdminImageReview(a))
	for _, verb := range []struct {
		name, short, long string
		confirm, reason   bool
	}{
		{"primary", "Show this picture on its page, in place of the one shown now.",
			"Show this picture on its page for its kind (portrait or cover), in place of\nthe one shown now. tt admin undo switches back.", false, false},
		{"takedown", "Take a picture off its page, keeping it on the record. Asks for --yes.",
			"Take a picture off its page. The row stays on the record, so a takedown\nrequest stays answerable. If it was the one shown, the page falls back to the\nnewest picture of the same kind still up, or shows none; --dry-run says which.\ntt admin undo puts it back. Asks for --yes.", true, true},
		{"accept", "Accept a proposed picture; it is shown if the page shows none.", "", false, false},
		{"reject", "Reject a proposed picture, with a reason.", "", false, true},
	} {
		var w writeFlags
		var reason string
		name := verb.name
		c := &cobra.Command{
			Use:     name + " ID",
			Short:   verb.short,
			Long:    verb.long,
			Example: "  tt admin image " + name + " 57 --dry-run",
			Args:    exactArgs(1, "tt admin image "+name+" 57"),
			RunE: func(cmd *cobra.Command, args []string) error {
				if _, err := strconv.Atoi(args[0]); err != nil {
					return Usage("a picture's id is a number, from tt admin image list", "tt admin image list dancer noelia-hurtado")
				}
				body := map[string]any{}
				if reason != "" {
					body["reason"] = reason
				}
				return a.adminWrite(cmd, http.MethodPost, "/api/v1/admin/images/"+args[0]+"/"+name, &w, body)
			},
		}
		w.bind(c, verb.confirm)
		if verb.reason {
			c.Flags().StringVar(&reason, "reason", "", "Why, a `REASON` kept on the picture (defaults to --note).")
		}
		cmd.AddCommand(c)
	}
	return cmd
}

func newAdminImageAdd(a *App) *cobra.Command {
	var w writeFlags
	var kind, licence, credit string
	var primary bool
	cmd := &cobra.Command{
		Use:   "add TYPE SLUG PATH|URL",
		Short: "Add a picture to a dancer, orchestra or event, from disk or a URL.",
		Long: `Add a picture to a dancer's, orchestra's or event's page. PATH is uploaded;
a URL is fetched by TangoTube, from the public web only.

TangoTube reads the picture itself, not its name: JPEG, PNG or WebP, at most
10 MB, at least 200 pixels on each side. It is shown on the page when you say
--primary or the page shows no picture of that kind yet.

--licence says under what terms it may be shown: unknown, own_work, permission,
public_page, public_domain or cc. --credit names who to credit.

Adding is recorded and cannot be undone; tt admin image takedown takes the
picture off the page and keeps the record.`,
		Example: `  tt admin image add dancer noelia-hurtado ./noelia.jpg --kind portrait --licence permission --credit "Ana Gómez"
  tt admin image add orchestra carlos-di-sarli https://example.org/di-sarli.jpg --kind cover --licence public_domain --dry-run`,
		Args: exactArgs(3, "tt admin image add dancer noelia-hurtado ./noelia.jpg --kind portrait --licence permission"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !contains(imageSubjects, args[0]) {
				return Usage("TYPE is one of "+strings.Join(imageSubjects, ", "), "tt admin image add dancer noelia-hurtado ./noelia.jpg --kind portrait --licence permission")
			}
			if !contains(imageLicences, licence) {
				return Usage("--licence is one of "+strings.Join(imageLicences, ", "), "")
			}
			path := "/api/v1/admin/images/" + args[0] + "/" + url.PathEscape(args[1])
			fields := w.body(map[string]any{"kind": kind, "licence": licence})
			if credit != "" {
				fields["credit"] = credit
			}
			if primary {
				fields["primary"] = true
			}
			if isWebAddress(args[2]) {
				fields["url"] = args[2]
				return a.adminDo(cmd, http.MethodPost, path, nil, fields, showImageAdd)
			}
			upload, err := readUpload(args[2], fields)
			if err != nil {
				return err
			}
			return a.adminDo(cmd, http.MethodPost, path, nil, upload, showImageAdd)
		},
	}
	f := cmd.Flags()
	f.StringVar(&kind, "kind", "portrait", "`KIND`: portrait or cover.")
	f.StringVar(&licence, "licence", "", "Under what terms it may be shown: `LICENCE` (unknown, own_work, permission, public_page, public_domain, cc).")
	f.StringVar(&credit, "credit", "", "Who to `CREDIT`, kept with the picture.")
	f.BoolVar(&primary, "primary", false, "Show it now, in place of the picture shown.")
	w.bind(cmd, false)
	return cmd
}

func isWebAddress(s string) bool {
	lower := strings.ToLower(s)
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}

// readUpload reads PATH into a multipart body beside the form fields.
func readUpload(path string, fields map[string]any) (*api.Upload, error) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return nil, Usage("no file at "+path, "give a path to a JPEG, PNG or WebP, or an https:// address")
	}
	if info.Size() > maxImageBytes {
		return nil, Usage(fmt.Sprintf("%s is %s; a picture is at most 10 MB", path, humanSize(int(info.Size()))), "")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, Usage("could not read "+path+": "+err.Error(), "")
	}
	form := make(map[string]string, len(fields))
	for k, v := range fields {
		form[k] = fmt.Sprint(v)
	}
	return &api.Upload{Fields: form, FileField: "file", FileName: filepath.Base(path), File: raw}, nil
}

// showImageAdd renders an add: what was read, whether it is shown, on what
// terms, and the record.
func showImageAdd(p *output.Printer, raw json.RawMessage) {
	d := decode[struct {
		DryRun bool             `json:"dry_run"`
		Image  adminImage       `json:"image"`
		Would  map[string][]any `json:"would"`
		Action *adminAction     `json:"action"`
	}](raw)
	img := d.Image
	id := ""
	if img.ID > 0 {
		id = strconv.Itoa(img.ID)
	}
	p.Field("picture", join(" · ", id, img.Kind, imageSize(img), humanSize(img.Bytes)))
	switch {
	case img.Shown && d.DryRun:
		p.Field("shown", "yes, on the page")
	case img.Shown:
		p.Field("shown", "yes, on the page now")
	case d.DryRun:
		p.Field("shown", "no; the page keeps the picture it shows")
	default:
		p.Field("shown", fmt.Sprintf("no; tt admin image primary %d shows it", img.ID))
	}
	if shown := d.Would["shown_image_id"]; len(shown) == 2 && shown[0] != nil {
		if n, ok := shown[0].(float64); ok {
			p.Field("replaces", fmt.Sprintf("picture %d, which stays up", int(n)))
		} else {
			p.Field("replaces", plainValue(shown[0]))
		}
	}
	p.Field("licence", join(" · ", img.Licence, img.Credit))
	if img.SourceURL != "" {
		p.Field("from", output.Truncate(img.SourceURL, p.Columns()-13))
	}
	if d.Action != nil {
		p.Field("recorded", fmt.Sprintf("action %d", d.Action.ID))
	}
	p.Field("undo", "not undoable; tt admin image takedown takes it off the page")
}

func newAdminImageList(a *App) *cobra.Command {
	return &cobra.Command{
		Use:   "list TYPE SLUG",
		Short: "A dancer's, orchestra's or event's pictures: shown, up, proposed, taken down.",
		Example: `  tt admin image list dancer noelia-hurtado
  tt admin image list event mundial-de-tango-2024 --jq '.images[] | select(.shown) | .id'`,
		Annotations: map[string]string{listsThings: "yes"},
		Args:        exactArgs(2, "tt admin image list dancer noelia-hurtado"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !contains(imageSubjects, args[0]) {
				return Usage("TYPE is one of "+strings.Join(imageSubjects, ", "), "tt admin image list dancer noelia-hurtado")
			}
			path := "/api/v1/admin/images/" + args[0] + "/" + url.PathEscape(args[1])
			return a.adminDo(cmd, http.MethodGet, path, nil, nil, func(p *output.Printer, raw json.RawMessage) {
				showImageTable(p, decode[struct {
					Images []adminImage `json:"images"`
				}](raw).Images, false)
			})
		},
	}
}

func newAdminImageReview(a *App) *cobra.Command {
	return &cobra.Command{
		Use:         "review",
		Short:       "Proposed pictures waiting for accept or reject, oldest first.",
		Example:     "  tt admin image review\n  tt admin image accept 61 --dry-run",
		Annotations: map[string]string{listsThings: "yes"},
		Args:        cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			q := url.Values{}
			if a.Flags.Limit > 0 {
				q.Set("limit", strconv.Itoa(a.Flags.Limit))
			}
			return a.adminDo(cmd, http.MethodGet, "/api/v1/admin/images/review", q, nil, func(p *output.Printer, raw json.RawMessage) {
				showImageTable(p, decode[struct {
					Images []adminImage `json:"images"`
				}](raw).Images, true)
			})
		},
	}
}

// showImageTable lists pictures; withSubject adds whose they are (review
// spans every page).
func showImageTable(p *output.Printer, images []adminImage, withSubject bool) {
	now := time.Now()
	rows := make([][]string, 0, len(images))
	for _, img := range images {
		state := strings.ReplaceAll(img.Status, "_", " ")
		if img.Shown {
			state = "shown"
		} else if img.Status == "applied" {
			state = "up"
		}
		row := []string{strconv.Itoa(img.ID), img.Kind, state, imageSize(img), img.Source,
			join(" · ", img.Licence, img.Credit), ago(img.AddedAt, now)}
		if withSubject {
			row = append([]string{row[0], img.Subject.Name}, row[1:]...)
		}
		rows = append(rows, row)
	}
	cols := []output.Column{
		{Header: "ID", ID: true, Right: true}, {Header: "KIND"}, {Header: "STATE"}, {Header: "SIZE", Dim: true},
		{Header: "SOURCE", Dim: true}, {Header: "LICENCE", Flex: true}, {Header: "ADDED", Dim: true},
	}
	if withSubject {
		cols = append([]output.Column{cols[0], {Header: "OF", Flex: true}}, cols[1:]...)
	}
	p.Table(cols, rows)
}

func imageSize(img adminImage) string {
	if img.Width == 0 || img.Height == 0 {
		return ""
	}
	format := strings.TrimPrefix(img.ContentType, "image/")
	return strings.TrimSpace(fmt.Sprintf("%d×%d %s", img.Width, img.Height, format))
}

func humanSize(n int) string {
	switch {
	case n <= 0:
		return ""
	case n < 1<<10:
		return fmt.Sprintf("%d B", n)
	case n < 1<<20:
		return fmt.Sprintf("%.0f KB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	}
}
