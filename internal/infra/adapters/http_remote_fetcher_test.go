package adapters

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPRemoteFetcher_Fetch_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("hello"))
	}))
	defer srv.Close()

	f := NewHTTPRemoteFetcher(srv.Client(), 1024, true)
	data, ct, err := f.Fetch(context.Background(), srv.URL, "")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Errorf("expected hello, got %s", string(data))
	}
	if ct != "text/plain" {
		t.Errorf("expected text/plain, got %s", ct)
	}
}

func TestHTTPRemoteFetcher_Fetch_ExpectedMediaType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte("data"))
	}))
	defer srv.Close()

	f := NewHTTPRemoteFetcher(srv.Client(), 1024, true)
	_, ct, err := f.Fetch(context.Background(), srv.URL, "image/png")
	if err != nil {
		t.Fatal(err)
	}
	if ct != "image/png" {
		t.Errorf("expected image/png override, got %s", ct)
	}
}

func TestHTTPRemoteFetcher_Fetch_SizeLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("12345678901")) // 11 bytes
	}))
	defer srv.Close()

	f := NewHTTPRemoteFetcher(srv.Client(), 10, true)
	_, _, err := f.Fetch(context.Background(), srv.URL, "")
	if err == nil {
		t.Error("expected size limit error")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("expected exceeds error, got %v", err)
	}
}

func TestHTTPRemoteFetcher_Fetch_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	f := NewHTTPRemoteFetcher(srv.Client(), 1024, true)
	_, _, err := f.Fetch(context.Background(), srv.URL, "")
	if err == nil {
		t.Error("expected error for 404")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("expected 404 in error, got %v", err)
	}
}

func TestHTTPRemoteFetcher_Fetch_NilContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	f := NewHTTPRemoteFetcher(srv.Client(), 1024, true)
	data, _, err := f.Fetch(nil, srv.URL, "")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "ok" {
		t.Errorf("expected ok, got %s", string(data))
	}
}

func TestHTTPRemoteFetcher_Fetch_FallbackContentType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// No Content-Type header
		_, _ = w.Write([]byte("binary"))
	}))
	defer srv.Close()

	f := NewHTTPRemoteFetcher(srv.Client(), 1024, true)
	_, ct, err := f.Fetch(context.Background(), srv.URL, "")
	if err != nil {
		t.Fatal(err)
	}
	// The server might set a default content type. We check it doesn't crash.
	if ct == "" {
		t.Error("expected non-empty content type")
	}
}

func TestNewHTTPRemoteFetcher_NilClient(t *testing.T) {
	f := NewHTTPRemoteFetcher(nil, 1024, false)
	if f.client == nil {
		t.Error("expected default client to be created")
	}
}
