package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCORSAllowsConfiguredOrigin(t *testing.T) {
	handler := CORS([]string{"https://app.example.com"}, false)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://app.example.com")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want configured origin", got)
	}
}

func TestCORSOmitsDisallowedOrigin(t *testing.T) {
	handler := CORS([]string{"https://app.example.com"}, false)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://other.example.com")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want empty", got)
	}
}

func TestCORSAllowsUnsafeWildcardWhenOptedIn(t *testing.T) {
	handler := CORS([]string{"*"}, true)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Origin", "https://app.example.com")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if got := rr.Code; got != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", got, http.StatusNoContent)
	}
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want *", got)
	}
}

func TestScansIngestOversizedBodyReturnsJSON413(t *testing.T) {
	handler := MaxBody(8)(http.HandlerFunc((&ScansHandler{}).Ingest))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/scans", strings.NewReader(`{"metadata":{"project_key":"demo"}}`))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if got := rr.Code; got != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", got, http.StatusRequestEntityTooLarge)
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
	if body := rr.Body.String(); !strings.Contains(body, `"error"`) {
		t.Fatalf("body = %q, want JSON error", body)
	}
}
