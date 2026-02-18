package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthzEndpoint(t *testing.T) {
	srv := newTestServer(t)

	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	var body healthResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.Status != "ok" {
		t.Errorf("status = %q, want %q", body.Status, "ok")
	}
}

func TestMetricsEndpoint(t *testing.T) {
	srv := newTestServer(t)

	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	// Make a request to generate metrics.
	http.Get(ts.URL + "/healthz")

	resp, err := http.Get(ts.URL + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/plain") && !strings.Contains(contentType, "text/openmetrics") {
		t.Errorf("Content-Type = %q, expected prometheus format", contentType)
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	body := string(bodyBytes)

	if !strings.Contains(body, "vulcan_http_requests_total") {
		t.Error("metrics output missing vulcan_http_requests_total")
	}
	if !strings.Contains(body, "vulcan_http_request_duration_seconds") {
		t.Error("metrics output missing vulcan_http_request_duration_seconds")
	}
}
