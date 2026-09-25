package api

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func serve(status int, body string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
}

func TestDecodesSuccessAndKeepsTheRawBytes(t *testing.T) {
	body := `{"ok":true,"data":{"id":"2ByaUQXeAeo"},"summary":"s","breadcrumbs":["tt open 2ByaUQXeAeo"]}`
	srv := serve(200, body)
	defer srv.Close()
	env, err := New(srv.URL, "", "test").Get(context.Background(), "/api/v1/videos/2ByaUQXeAeo", nil)
	if err != nil || !env.OK || string(env.Raw) != body || env.Breadcrumbs[0] != "tt open 2ByaUQXeAeo" {
		t.Fatalf("%+v %v", env, err)
	}
}

func TestDecodesTheErrorEnvelope(t *testing.T) {
	srv := serve(404, `{"ok":false,"error":{"code":"not_found","message":"No video with that id","hint":"tt search","retryable":false}}`)
	defer srv.Close()
	_, err := New(srv.URL, "", "test").Get(context.Background(), "/x", nil)
	apiErr, ok := AsError(err)
	if !ok || apiErr.Code() != "not_found" || apiErr.Envelope.Error.Hint != "tt search" {
		t.Fatalf("%v", err)
	}
}

func TestSomethingThatIsNotAnEnvelopeIsAnAPIError(t *testing.T) {
	srv := serve(502, `<html>Bad gateway</html>`)
	defer srv.Close()
	_, err := New(srv.URL, "", "test").Get(context.Background(), "/x", nil)
	if apiErr, ok := AsError(err); !ok || apiErr.Code() != "api" {
		t.Fatalf("%v", err)
	}
	srv429 := serve(429, `slow down`)
	defer srv429.Close()
	_, err = New(srv429.URL, "", "test").Get(context.Background(), "/x", nil)
	if apiErr, _ := AsError(err); apiErr.Code() != "rate_limit" {
		t.Fatalf("%v", err)
	}
}

func TestUnreachableIsANetworkError(t *testing.T) {
	_, err := New("http://127.0.0.1:1", "", "test").Get(context.Background(), "/x", nil)
	if apiErr, ok := AsError(err); !ok || apiErr.Code() != "network" || !apiErr.Envelope.Error.Retryable {
		t.Fatalf("%v", err)
	}
}

func TestWritesRefuseToLeaveWithoutAToken(t *testing.T) {
	_, err := New("http://127.0.0.1:1", "", "test").Do(context.Background(), "POST", "/x", nil, map[string]int{}, true)
	if apiErr, _ := AsError(err); apiErr.Code() != "auth" || apiErr.Envelope.Error.Hint != "tt auth login" {
		t.Fatalf("%v", err)
	}
}

func TestSendsTheBearerToken(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"ok":true,"data":{}}`))
	}))
	defer srv.Close()
	_, _ = New(srv.URL, "tt_live_x", "test").Get(context.Background(), "/x", nil)
	if got != "Bearer tt_live_x" {
		t.Fatal(got)
	}
}

func TestSendsAnUploadAsMultipart(t *testing.T) {
	var kind, name string
	var got []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("not multipart: %v", err)
		}
		kind = r.FormValue("kind")
		f, h, _ := r.FormFile("file")
		name = h.Filename
		got, _ = io.ReadAll(f)
		_, _ = w.Write([]byte(`{"ok":true,"data":{},"summary":"","breadcrumbs":[]}`))
	}))
	defer srv.Close()
	c := New(srv.URL, "tt_live_x", "test")
	_, err := c.Do(context.Background(), http.MethodPost, "/x", nil,
		&Upload{Fields: map[string]string{"kind": "portrait"}, FileField: "file", FileName: "noelia.jpg", File: []byte{0xFF, 0xD8, 0xFF}}, true)
	if err != nil || kind != "portrait" || name != "noelia.jpg" || !bytes.Equal(got, []byte{0xFF, 0xD8, 0xFF}) {
		t.Fatalf("err %v kind %q name %q bytes %v", err, kind, name, got)
	}
}
