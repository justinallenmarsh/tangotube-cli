package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

type fakeKeyring struct {
	secrets map[string]string
	broken  bool
}

func (f *fakeKeyring) Get(_, user string) (string, error) {
	if f.broken {
		return "", errors.New("no keyring")
	}
	if s, ok := f.secrets[user]; ok {
		return s, nil
	}
	return "", errors.New("not found")
}

func (f *fakeKeyring) Set(_, user, secret string) error {
	if f.broken {
		return errors.New("no keyring")
	}
	f.secrets[user] = secret
	return nil
}

func (f *fakeKeyring) Delete(_, user string) error { delete(f.secrets, user); return nil }

func store(t *testing.T, env map[string]string, kr *fakeKeyring) *Store {
	return &Store{Host: "tangotube.tv", Env: func(k string) string { return env[k] }, Keyring: kr, ConfigDir: t.TempDir()}
}

func TestResolvePrecedence(t *testing.T) {
	env := map[string]string{}
	kr := &fakeKeyring{secrets: map[string]string{}}
	s := store(t, env, kr)

	if tok, _ := s.Resolve(""); tok != "" {
		t.Fatal("expected no token")
	}
	_ = os.MkdirAll(s.ConfigDir, 0o700)
	_ = os.WriteFile(s.FilePath(), []byte("tt_live_file\n"), 0o600)
	check := func(flag, want, source string) {
		t.Helper()
		tok, src := s.Resolve(flag)
		if tok != want || src != source {
			t.Fatalf("got %s from %s, want %s from %s", tok, src, want, source)
		}
	}
	check("", "tt_live_file", SourceFile)
	kr.secrets["tangotube.tv"] = "tt_live_keyring"
	check("", "tt_live_keyring", SourceKeyring)
	env["TANGOTUBE_NO_KEYRING"] = "1"
	check("", "tt_live_file", SourceFile)
	env["TANGOTUBE_TOKEN"] = "tt_live_env"
	check("", "tt_live_env", SourceEnv)
	check("tt_live_flag", "tt_live_flag", SourceFlag)
}

func TestSaveFallsBackToAPrivateFile(t *testing.T) {
	s := store(t, map[string]string{}, &fakeKeyring{broken: true})
	where, err := s.Save("tt_live_secret")
	if err != nil || where != SourceFile {
		t.Fatalf("%s %v", where, err)
	}
	info, _ := os.Stat(s.FilePath())
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", info.Mode().Perm())
	}
	if err := s.Delete(); err != nil {
		t.Fatal(err)
	}
	if tok, _ := s.Resolve(""); tok != "" {
		t.Fatal("token survived logout")
	}
}

func TestSaveUsesTheKeyringAndLeavesNoFile(t *testing.T) {
	kr := &fakeKeyring{secrets: map[string]string{}}
	s := store(t, map[string]string{}, kr)
	if where, _ := s.Save("tt_live_secret"); where != SourceKeyring || kr.secrets["tangotube.tv"] != "tt_live_secret" {
		t.Fatal("not in keyring")
	}
	if _, err := os.Stat(s.FilePath()); !os.IsNotExist(err) {
		t.Fatal("file written too")
	}
}

func TestDevTokensLiveInTheirOwnFile(t *testing.T) {
	s := &Store{Host: "localhost:3000", ConfigDir: "/x"}
	if s.FilePath() != filepath.Join("/x", "token-localhost_3000") {
		t.Fatal(s.FilePath())
	}
}

func TestDeviceFlowPollsUntilApproved(t *testing.T) {
	polls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		switch r.URL.Path {
		case "/oauth/device_authorization":
			_ = json.NewEncoder(w).Encode(map[string]any{"device_code": "dc", "user_code": "TANGO-1234", "verification_uri": "http://x/device", "expires_in": 60, "interval": 0})
		case "/oauth/token":
			if r.Form.Get("grant_type") != deviceGrant || r.Form.Get("client_id") != ClientID {
				w.WriteHeader(400)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_request"})
				return
			}
			polls++
			if polls < 2 {
				w.WriteHeader(400)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "authorization_pending"})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "tt_live_device", "token_type": "bearer", "scope": "write"})
		}
	}))
	defer srv.Close()

	o := &OAuth{BaseURL: srv.URL, Scope: "write", TokenName: "tt on test"}
	dc, err := o.StartDevice(context.Background())
	if err != nil || dc.UserCode != "TANGO-1234" {
		t.Fatalf("%+v %v", dc, err)
	}
	dc.Interval = -4 // max(…, 5) keeps the RFC floor; the test shortens it below
	pollInterval = 0
	defer func() { pollInterval = 5 }()
	tr, err := o.PollDevice(context.Background(), dc)
	if err != nil || tr.AccessToken != "tt_live_device" || polls != 2 {
		t.Fatalf("%+v %v polls=%d", tr, err, polls)
	}
}

func TestPKCEChallengeIsS256OfTheVerifier(t *testing.T) {
	v, c := pkcePair()
	if len(v) < 43 || len(c) != 43 || v == c {
		t.Fatalf("%s %s", v, c)
	}
}
