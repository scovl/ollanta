package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestWaitForServerJobCompleted(t *testing.T) {
	t.Parallel()

	jobChecks := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/scan-jobs/7":
			jobChecks++
			w.Header().Set("Content-Type", "application/json")
			if jobChecks == 1 {
				_, _ = w.Write([]byte(`{"id":7,"status":"accepted"}`))
				return
			}
			_, _ = w.Write([]byte(`{"id":7,"status":"completed","scan_id":42}`))
		case "/api/v1/scans/42":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":42,"status":"completed","gate_status":"OK","new_issues":5,"closed_issues":2}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	result, err := waitForServerJob(server.URL, "token", 7, 2*time.Second, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("waitForServerJob() error = %v", err)
	}
	if result.GateStatus != "OK" || result.NewIssues != 5 || result.ClosedIssues != 2 {
		t.Fatalf("unexpected final scan result: %+v", result)
	}
}

func TestWaitForServerJobFailed(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/scan-jobs/9" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":9,"status":"failed","last_error":"boom"}`))
	}))
	defer server.Close()

	_, err := waitForServerJob(server.URL, "token", 9, time.Second, 10*time.Millisecond)
	if err == nil {
		t.Fatal("expected waitForServerJob() to fail")
	}
	if got := err.Error(); got != "scan job 9 failed: boom" {
		t.Fatalf("unexpected error: %s", got)
	}
}

func TestAuthorizedRequestAddsBearerToken(t *testing.T) {
	t.Parallel()

	req, err := authorizedRequest(http.MethodGet, "http://example.com", "secret", nil)
	if err != nil {
		t.Fatalf("authorizedRequest() error = %v", err)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer secret" {
		t.Fatalf("Authorization = %q, want %q", got, "Bearer secret")
	}
	if got := fmt.Sprint(req.Body != nil); got != "true" {
		t.Fatalf("expected request body reader to be initialized, got %s", got)
	}
}
