// Package api talks to the TangoTube /api/v1 JSON API.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/justinallenmarsh/tangotube-cli/internal/output"
)

// DefaultBaseURL is the live site.
const DefaultBaseURL = "https://tangotube.tv"

// DevBaseURL is where TANGOTUBE_DEV=1 points.
const DevBaseURL = "http://localhost:3000"

// Client calls the API and hands back envelopes.
type Client struct {
	BaseURL   string
	Token     string
	UserAgent string
	HTTP      *http.Client
	Debug     io.Writer // --verbose: one line per request

	// StoredToken says Token came from the keyring or token file, not from
	// --token or TANGOTUBE_TOKEN. A stored token the server turns down must
	// not break a search that never needed it: the read is retried without
	// it, and Warn hears about it once.
	StoredToken bool
	Warn        io.Writer
	warned      bool
}

// Upload is a multipart body: form fields and one file. Pass it to Do as the
// body to send a file (tt admin image add PATH) instead of JSON.
type Upload struct {
	Fields    map[string]string
	FileField string
	FileName  string
	File      []byte
}

func (u *Upload) encode() (io.Reader, string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range u.Fields {
		if err := w.WriteField(k, v); err != nil {
			return nil, "", err
		}
	}
	part, err := w.CreateFormFile(u.FileField, u.FileName)
	if err != nil {
		return nil, "", err
	}
	if _, err := part.Write(u.File); err != nil {
		return nil, "", err
	}
	if err := w.Close(); err != nil {
		return nil, "", err
	}
	return &buf, w.FormDataContentType(), nil
}

// Error is an API failure carrying the envelope that describes it.
type Error struct{ Envelope *output.Envelope }

func (e *Error) Error() string { return e.Envelope.Error.Message }

// Code is the envelope's error code.
func (e *Error) Code() string { return e.Envelope.Error.Code }

// AsError unwraps an *Error.
func AsError(err error) (*Error, bool) {
	var apiErr *Error
	ok := errors.As(err, &apiErr)
	return apiErr, ok
}

// New builds a client for baseURL.
func New(baseURL, token, version string) *Client {
	return &Client{
		BaseURL:   strings.TrimRight(baseURL, "/"),
		Token:     token,
		UserAgent: "tt/" + version,
		HTTP:      &http.Client{Timeout: 30 * time.Second},
	}
}

// Get calls a public GET endpoint. The token, if any, rides along.
func (c *Client) Get(ctx context.Context, path string, query url.Values) (*output.Envelope, error) {
	return c.Do(ctx, http.MethodGet, path, query, nil, false)
}

// Do sends one request. needsToken refuses to leave home without a token.
func (c *Client) Do(ctx context.Context, method, path string, query url.Values, body any, needsToken bool) (*output.Envelope, error) {
	if needsToken && c.Token == "" {
		return nil, fail(output.CodeAuth, "This needs a TangoTube token, and none is set.", "tt auth login")
	}
	env, err := c.send(ctx, method, path, query, body)
	if apiErr, ok := AsError(err); ok && apiErr.Code() == output.CodeAuth &&
		method == http.MethodGet && !needsToken && c.StoredToken && c.Token != "" {
		c.Token = ""
		if c.Warn != nil && !c.warned {
			c.warned = true
			fmt.Fprintln(c.Warn, "Your saved token was turned down, so tt is reading without it. Run: tt auth login")
		}
		return c.send(ctx, method, path, query, body)
	}
	return env, err
}

func (c *Client) send(ctx context.Context, method, path string, query url.Values, body any) (*output.Envelope, error) {

	u := c.BaseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	var reader io.Reader
	contentType := "application/json"
	if upload, ok := body.(*Upload); ok {
		var err error
		if reader, contentType, err = upload.encode(); err != nil {
			return nil, fail(output.CodeUsage, err.Error(), "")
		}
	} else if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, fail(output.CodeUsage, err.Error(), "")
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, reader)
	if err != nil {
		return nil, fail(output.CodeUsage, err.Error(), "")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.UserAgent)
	if body != nil {
		req.Header.Set("Content-Type", contentType)
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	start := time.Now()
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fail(output.CodeNetwork, "Could not reach "+c.host()+".", "tt doctor")
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if c.Debug != nil {
		fmt.Fprintf(c.Debug, "%s %s → %d in %s\n", method, u, resp.StatusCode, time.Since(start).Round(time.Millisecond))
	}
	if err != nil {
		return nil, fail(output.CodeNetwork, "The connection to "+c.host()+" dropped mid-answer.", "tt doctor")
	}
	return decode(resp, raw)
}

func decode(resp *http.Response, raw []byte) (*output.Envelope, error) {
	var env output.Envelope
	if err := json.Unmarshal(raw, &env); err != nil || (!env.OK && env.Error == nil) {
		return nil, fail(statusCode(resp.StatusCode),
			fmt.Sprintf("%s answered HTTP %d, not a TangoTube envelope.", resp.Request.URL.Host, resp.StatusCode),
			"tt doctor")
	}
	env.Raw = raw
	if !env.OK {
		if env.Error.Code == "" {
			env.Error.Code = statusCode(resp.StatusCode)
		}
		return &env, &Error{Envelope: &env}
	}
	return &env, nil
}

// statusCode names an HTTP status the way the envelope would have.
func statusCode(status int) string {
	switch {
	case status == 401:
		return output.CodeAuth
	case status == 403:
		return output.CodeForbidden
	case status == 404:
		return output.CodeNotFound
	case status == 429:
		return output.CodeRateLimit
	case status >= 400 && status < 500:
		return output.CodeUsage
	default:
		return output.CodeAPI
	}
}

func (c *Client) host() string {
	if u, err := url.Parse(c.BaseURL); err == nil && u.Host != "" {
		return u.Host
	}
	return c.BaseURL
}

func fail(code, message, hint string) error {
	return &Error{Envelope: output.Fail(code, message, hint)}
}

// Fail builds an *Error for a problem found before any request was sent.
func Fail(code, message, hint string) error { return fail(code, message, hint) }
