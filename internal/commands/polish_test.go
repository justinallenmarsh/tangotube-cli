package commands

import (
	"testing"
	"time"
)

func TestNormalizeID(t *testing.T) {
	cases := map[string]string{
		"uGwRPRusbC0":   "uGwRPRusbC0",
		" uGwRPRusbC0 ": "uGwRPRusbC0",
		"https://www.youtube.com/watch?v=uGwRPRusbC0&t=30s":  "uGwRPRusbC0",
		"youtube.com/watch?v=uGwRPRusbC0":                    "uGwRPRusbC0",
		"https://youtu.be/uGwRPRusbC0?si=abc":                "uGwRPRusbC0",
		"https://www.youtube.com/shorts/uGwRPRusbC0":         "uGwRPRusbC0",
		"https://tangotube.tv/watch?v=uGwRPRusbC0":           "uGwRPRusbC0",
		"https://tangotube.tv/clips/sacada-1-volver-a-sonar": "sacada-1-volver-a-sonar",
		"https://tangotube.tv/dancers/noelia-hurtado/":       "noelia-hurtado",
		"sacada-at-1-12": "sacada-at-1-12",
	}
	for in, want := range cases {
		if got := NormalizeID(in); got != want {
			t.Errorf("NormalizeID(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAgo(t *testing.T) {
	now := time.Date(2026, 9, 24, 15, 0, 0, 0, time.UTC)
	cases := map[string]string{
		"2026-09-24T14:59:40Z":      "just now",
		"2026-09-24T14:58:00Z":      "2 minutes ago",
		"2026-09-24T16:00:00+02:00": "1 hour ago",
		"2026-09-21T15:00:00Z":      "3 days ago",
		"2026-01-02T15:00:00Z":      "2 January 2026",
		"not a time":                "not a time",
	}
	for in, want := range cases {
		if got := ago(in, now); got != want {
			t.Errorf("ago(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPlanUpgradeFollowsHowTTWasInstalled(t *testing.T) {
	old := Version
	defer func() { Version = old }()
	Version = "v0.1.0"
	env := func(k string) string { return map[string]string{"GOPATH": "/home/d/gopath"}[k] }

	cases := map[string]string{
		"/home/d/.local/share/mise/installs/github-justinallenmarsh-tangotube-cli/0.1.0/tt": "mise",
		"/home/d/gopath/bin/tt": "go",
		"/home/d/go/bin/tt":     "go",
		"/home/d/.local/bin/tt": "script",
	}
	for self, want := range cases {
		if got := planUpgrade(self, "/home/d", env).Method; got != want {
			t.Errorf("%s: %s, want %s", self, got, want)
		}
	}
	Version = "dev"
	if got := planUpgrade("/home/d/.local/bin/tt", "/home/d", env).Method; got != "source" {
		t.Errorf("a dev build upgrades from source, got %s", got)
	}
}
