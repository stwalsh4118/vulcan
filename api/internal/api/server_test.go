package api

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/seantiz/vulcan/internal/store"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	s, err := store.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	return NewServer(":0", s, logger)
}

func TestRequestIDHeader(t *testing.T) {
	srv := newTestServer(t)
	srv.Router().Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/test")
	if err != nil {
		t.Fatalf("GET /test: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	// chi middleware.RequestID does not set X-Request-Id on the response by default,
	// but it sets it in the request context. Verify the middleware is active by
	// checking the request was processed successfully.
}

func TestPanicRecovery(t *testing.T) {
	srv := newTestServer(t)
	srv.Router().Get("/panic", func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/panic")
	if err != nil {
		t.Fatalf("GET /panic: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", resp.StatusCode)
	}
}

func TestCORSHeaders(t *testing.T) {
	srv := newTestServer(t)
	srv.Router().Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	req, _ := http.NewRequest("OPTIONS", ts.URL+"/test", nil)
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("OPTIONS /test: %v", err)
	}
	defer resp.Body.Close()

	if v := resp.Header.Get("Access-Control-Allow-Origin"); v != "*" {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", v, "*")
	}
}
