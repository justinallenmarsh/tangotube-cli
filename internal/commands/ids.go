package commands

import (
	"net/url"
	"strings"
)

// NormalizeID takes what people paste and returns the id tt needs: a YouTube
// or tangotube.tv watch URL gives its v=, youtu.be/ID and /shorts/ID give the
// ID, and a tangotube.tv page (/clips/SLUG, /dancers/SLUG) gives its slug.
// Anything that is not a URL comes back trimmed and otherwise untouched.
func NormalizeID(s string) string {
	s = strings.TrimSpace(s)
	if !strings.Contains(s, "/") && !strings.Contains(s, "?") {
		return s
	}
	raw := s
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return s
	}
	if v := u.Query().Get("v"); v != "" {
		return v
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		return s
	}
	host := strings.TrimPrefix(u.Hostname(), "www.")
	switch {
	case host == "youtu.be":
		return parts[0]
	case len(parts) >= 2 && (parts[0] == "shorts" || parts[0] == "embed" || parts[0] == "live"):
		return parts[1]
	}
	return parts[len(parts)-1]
}
