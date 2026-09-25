package commands

import (
	"strings"
	"testing"
	"time"
)

func TestExpiry(t *testing.T) {
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	cases := map[string]string{
		now.Add(90 * 24 * time.Hour).Format(time.RFC3339): "(in 90 days)",
		now.Add(30 * time.Hour).Format(time.RFC3339):      "(tomorrow)",
		now.Add(-time.Hour).Format(time.RFC3339):          "run tt auth login --admin",
		"not a date":                                      "not a date",
	}
	for in, want := range cases {
		if got := expiry(in, now); !strings.Contains(got, want) {
			t.Errorf("expiry(%q) = %q, want it to contain %q", in, got, want)
		}
	}
}
