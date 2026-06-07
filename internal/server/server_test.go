package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServer_Health(t *testing.T) {
	srv := New(Config{})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("health status = %d, want 200", resp.StatusCode)
	}
}
