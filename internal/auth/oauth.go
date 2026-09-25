package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ClientID is the one OAuth client TangoTube knows: this CLI.
const ClientID = "tt"

const deviceGrant = "urn:ietf:params:oauth:grant-type:device_code"

// pollInterval is RFC 8628's default wait between polls, in seconds.
var pollInterval = 5

// OAuth mints a personal access token through the site's small OAuth server.
type OAuth struct {
	BaseURL   string
	Scope     string // "read", "write" or "admin"
	TokenName string // "tt on <hostname>"
	HTTP      *http.Client
}

// TokenResponse is RFC 6749 §5.1. The access token is the PAT itself.
type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	Error       string `json:"error"`
	Description string `json:"error_description"`
}

// Verifier and challenge for PKCE (RFC 7636, S256).
func pkcePair() (verifier, challenge string) {
	verifier = randomString(32)
	sum := sha256.Sum256([]byte(verifier))
	return verifier, base64.RawURLEncoding.EncodeToString(sum[:])
}

func randomString(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// Loopback runs the browser flow: listen on 127.0.0.1, send the browser to
// /oauth/authorize, catch the code on /callback, trade it for a token.
// open is handed the authorize URL; it should open a browser and say so.
func (o *OAuth) Loopback(ctx context.Context, open func(authorizeURL string)) (*TokenResponse, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("could not listen on 127.0.0.1: %w", err)
	}
	defer listener.Close()
	redirect := fmt.Sprintf("http://127.0.0.1:%d/callback", listener.Addr().(*net.TCPAddr).Port)
	verifier, challenge := pkcePair()
	state := randomString(16)

	q := url.Values{
		"response_type":         {"code"},
		"client_id":             {ClientID},
		"redirect_uri":          {redirect},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
		"scope":                 {o.Scope},
		"state":                 {state},
		"token_name":            {o.TokenName},
	}
	authorize := strings.TrimRight(o.BaseURL, "/") + "/oauth/authorize?" + q.Encode()

	type result struct {
		code string
		err  error
	}
	results := make(chan result, 1)
	server := &http.Server{ReadHeaderTimeout: 10 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/callback" {
			http.NotFound(w, r)
			return
		}
		params := r.URL.Query()
		switch {
		case params.Get("state") != state:
			page(w, "That answer did not come from this login. Run tt auth login again.")
			results <- result{err: errors.New("the callback state did not match; nothing was saved")}
		case params.Get("error") != "":
			page(w, "No token was made. You can close this tab.")
			results <- result{err: fmt.Errorf("the site said %s", params.Get("error"))}
		default:
			page(w, "tt is signed in. You can close this tab and go back to the terminal.")
			results <- result{code: params.Get("code")}
		}
	})}
	go func() { _ = server.Serve(listener) }()
	defer server.Close()

	open(authorize)

	select {
	case <-ctx.Done():
		return nil, errors.New("gave up waiting for the browser")
	case r := <-results:
		if r.err != nil {
			return nil, r.err
		}
		return o.exchange(ctx, url.Values{
			"grant_type":    {"authorization_code"},
			"code":          {r.code},
			"redirect_uri":  {redirect},
			"client_id":     {ClientID},
			"code_verifier": {verifier},
		})
	}
}

// DeviceCode is RFC 8628 §3.2.
type DeviceCode struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
	Error                   string `json:"error"`
}

// StartDevice asks for a user code to type into /device on another machine.
func (o *OAuth) StartDevice(ctx context.Context) (*DeviceCode, error) {
	var dc DeviceCode
	err := o.post(ctx, "/oauth/device_authorization", url.Values{
		"client_id": {ClientID}, "scope": {o.Scope}, "token_name": {o.TokenName},
	}, &dc)
	if err != nil {
		return nil, err
	}
	if dc.Error != "" || dc.DeviceCode == "" {
		return nil, fmt.Errorf("the site would not start a device login (%s)", dc.Error)
	}
	return &dc, nil
}

// PollDevice waits until the code is approved, denied, or expires.
func (o *OAuth) PollDevice(ctx context.Context, dc *DeviceCode) (*TokenResponse, error) {
	interval := time.Duration(max(dc.Interval, pollInterval)) * time.Second
	deadline := time.Now().Add(time.Duration(max(dc.ExpiresIn, 60)) * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}
		tr, err := o.exchange(ctx, url.Values{
			"grant_type": {deviceGrant}, "device_code": {dc.DeviceCode}, "client_id": {ClientID},
		})
		var oe *OAuthError
		switch {
		case err == nil:
			return tr, nil
		case errors.As(err, &oe) && oe.Code == "authorization_pending":
		case errors.As(err, &oe) && oe.Code == "slow_down":
			interval += 5 * time.Second
		default:
			return nil, err
		}
	}
	return nil, errors.New("the code expired before anyone approved it")
}

// OAuthError is an RFC 6749 §5.2 error.
type OAuthError struct{ Code, Description string }

func (e *OAuthError) Error() string {
	switch e.Code {
	case "access_denied":
		return "the request was denied on the site"
	case "expired_token":
		return "the code expired before anyone approved it"
	}
	if e.Description != "" {
		return e.Description
	}
	return "the site said " + e.Code
}

func (o *OAuth) exchange(ctx context.Context, form url.Values) (*TokenResponse, error) {
	var tr TokenResponse
	if err := o.post(ctx, "/oauth/token", form, &tr); err != nil {
		return nil, err
	}
	if tr.Error != "" {
		return nil, &OAuthError{Code: tr.Error, Description: tr.Description}
	}
	if tr.AccessToken == "" {
		return nil, errors.New("the site answered without a token")
	}
	return &tr, nil
}

func (o *OAuth) post(ctx context.Context, path string, form url.Values, into any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(o.BaseURL, "/")+path, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	client := o.HTTP
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("could not reach %s: %w", o.BaseURL, err)
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(into); err != nil {
		return fmt.Errorf("%s%s answered HTTP %d without JSON", o.BaseURL, path, resp.StatusCode)
	}
	return nil
}

func page(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!doctype html><title>TangoTube CLI</title>
<body style="font:16px/1.5 system-ui;background:#151311;color:#F4EFE6;display:grid;place-items:center;height:100vh;margin:0">
<main style="max-width:28rem;text-align:center"><p style="color:#C41E3A;font-weight:700;letter-spacing:.1em">TANGOTUBE</p><p>%s</p></main>`,
		html.EscapeString(message))
}
