package commands

import "testing"

func TestParseTime(t *testing.T) {
	good := map[string]int{"1:12": 72, "0:05": 5, "72": 72, "72s": 72, "1:02:03": 3723, " 2:00 ": 120}
	for in, want := range good {
		got, err := ParseTime(in)
		if err != nil || got != want {
			t.Errorf("ParseTime(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	for _, in := range []string{"", "1:7", "1:60", "a:bc", "1:2:3:4", "-5", "1.5"} {
		if _, err := ParseTime(in); err == nil {
			t.Errorf("ParseTime(%q) should fail", in)
		}
	}
}

func TestTagName(t *testing.T) {
	if got := tagName(" Media Luna "); got != "media_luna" {
		t.Fatal(got)
	}
	got := splitTags([]string{"sacada,Boleo", "media luna"})
	if len(got) != 3 || got[2] != "media_luna" {
		t.Fatal(got)
	}
}

func TestBaseURLPrecedence(t *testing.T) {
	env := map[string]string{}
	a := &App{Env: func(k string) string { return env[k] }}
	if a.BaseURL() != "https://tangotube.tv" {
		t.Fatal(a.BaseURL())
	}
	env["TANGOTUBE_DEV"] = "1"
	if a.BaseURL() != "http://localhost:3000" {
		t.Fatal(a.BaseURL())
	}
	env["TANGOTUBE_API_URL"] = "http://tangotube.example:3002/"
	if a.BaseURL() != "http://tangotube.example:3002" {
		t.Fatal(a.BaseURL())
	}
	a.Flags.APIURL = "http://flag"
	if a.BaseURL() != "http://flag" {
		t.Fatal(a.BaseURL())
	}
}

func TestInteractiveOnlyAtATerminalWithoutMachineFlags(t *testing.T) {
	a := &App{TTY: true, Env: func(string) string { return "" }}
	if !a.Interactive() {
		t.Fatal("tty should be interactive")
	}
	a.Flags.Agent = true
	if a.Interactive() {
		t.Fatal("--agent must never be interactive")
	}
}
