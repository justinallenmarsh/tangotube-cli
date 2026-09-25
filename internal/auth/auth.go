// Package auth finds, keeps, and mints the personal access token tt sends as
// "Authorization: Bearer tt_live_…".
package auth

import (
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/zalando/go-keyring"
)

// Service is the OS keyring service name.
const Service = "tangotube"

// Where a token came from.
const (
	SourceFlag    = "--token"
	SourceEnv     = "TANGOTUBE_TOKEN"
	SourceKeyring = "keyring"
	SourceFile    = "file"
)

// Keyring is the slice of an OS keyring tt uses.
type Keyring interface {
	Get(service, user string) (string, error)
	Set(service, user, secret string) error
	Delete(service, user string) error
}

// Store resolves and persists tokens for one API host.
type Store struct {
	Host      string // "tangotube.tv", "localhost:3000"
	Env       func(string) string
	Keyring   Keyring
	ConfigDir string // ~/.config/tangotube
}

// NewStore builds a store for baseURL with the real keyring and environment.
func NewStore(baseURL string) *Store {
	host := baseURL
	if u, err := url.Parse(baseURL); err == nil && u.Host != "" {
		host = u.Host
	}
	return &Store{Host: host, Env: os.Getenv, Keyring: timeoutKeyring{}, ConfigDir: ConfigDir()}
}

// ConfigDir is $XDG_CONFIG_HOME/tangotube, or ~/.config/tangotube, or on
// Windows %AppData%\tangotube.
func ConfigDir() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "tangotube")
	}
	if runtime.GOOS == "windows" {
		if dir, err := os.UserConfigDir(); err == nil {
			return filepath.Join(dir, "tangotube")
		}
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "tangotube")
}

// Resolve finds the token by precedence: --token, TANGOTUBE_TOKEN, the OS
// keyring, then the token file. It returns "" when there is none.
func (s *Store) Resolve(flag string) (token, source string) {
	if t := strings.TrimSpace(flag); t != "" {
		return t, SourceFlag
	}
	if t := strings.TrimSpace(s.Env("TANGOTUBE_TOKEN")); t != "" {
		return t, SourceEnv
	}
	return s.Stored()
}

// Stored is the token tt keeps itself — keyring, then file — ignoring the
// flag and the environment.
func (s *Store) Stored() (token, source string) {
	if s.useKeyring() {
		if t, err := s.Keyring.Get(Service, s.Host); err == nil && strings.TrimSpace(t) != "" {
			return strings.TrimSpace(t), SourceKeyring
		}
	}
	if raw, err := os.ReadFile(s.FilePath()); err == nil {
		if t := strings.TrimSpace(string(raw)); t != "" {
			return t, SourceFile
		}
	}
	return "", ""
}

// Save keeps a token in the keyring, or in the token file when the keyring is
// off or unavailable. It returns where the token went.
func (s *Store) Save(token string) (string, error) {
	if s.useKeyring() {
		if err := s.Keyring.Set(Service, s.Host, token); err == nil {
			_ = os.Remove(s.FilePath())
			return SourceKeyring, nil
		}
	}
	// On Windows the modes only keep the file writable; what keeps it private
	// is that %AppData% belongs to the user.
	if err := os.MkdirAll(filepath.Dir(s.FilePath()), 0o700); err != nil {
		return "", err
	}
	if err := os.WriteFile(s.FilePath(), []byte(token+"\n"), 0o600); err != nil {
		return "", err
	}
	return SourceFile, os.Chmod(s.FilePath(), 0o600)
}

// Delete wipes the token from the keyring and the file. Environment variables
// are the caller's to unset.
func (s *Store) Delete() error {
	if s.useKeyring() {
		_ = s.Keyring.Delete(Service, s.Host)
	}
	if err := os.Remove(s.FilePath()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// FilePath is the token file for this host: "token" for the live site,
// "token-<host>" for anything else, so a dev token never meets production.
func (s *Store) FilePath() string {
	name := "token"
	if s.Host != "" && s.Host != "tangotube.tv" {
		name = "token-" + strings.NewReplacer(":", "_", "/", "_").Replace(s.Host)
	}
	return filepath.Join(s.ConfigDir, name)
}

// Describe says where a source lives, for tt auth status.
func (s *Store) Describe(source string) string {
	switch source {
	case SourceFile:
		return s.FilePath()
	case SourceKeyring:
		return "OS keyring (service " + Service + ", account " + s.Host + ")"
	case SourceEnv:
		return "TANGOTUBE_TOKEN environment variable"
	case SourceFlag:
		return "--token flag"
	}
	return "nowhere"
}

func (s *Store) useKeyring() bool {
	return s.Keyring != nil && s.Env("TANGOTUBE_NO_KEYRING") == ""
}

// timeoutKeyring wraps the OS keyring so a headless box with a D-Bus that
// never answers costs two seconds, not a hang.
type timeoutKeyring struct{}

func (timeoutKeyring) Get(service, user string) (string, error) {
	var out string
	err := withTimeout(func() error {
		var e error
		out, e = keyring.Get(service, user)
		return e
	})
	return out, err
}

func (timeoutKeyring) Set(service, user, secret string) error {
	return withTimeout(func() error { return keyring.Set(service, user, secret) })
}

func (timeoutKeyring) Delete(service, user string) error {
	return withTimeout(func() error { return keyring.Delete(service, user) })
}

func withTimeout(fn func() error) error {
	done := make(chan error, 1)
	go func() { done <- fn() }()
	select {
	case err := <-done:
		return err
	case <-time.After(2 * time.Second):
		return errors.New("keyring did not answer")
	}
}
