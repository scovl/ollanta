package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReadinessReturnsNotReadyWhenPostgresFails(t *testing.T) {
	t.Cleanup(func() { deps = nil })
	deps = &healthDeps{db: fakeHealth{err: errors.New("db down")}, indexer: fakeHealth{}}

	rec := httptest.NewRecorder()
	Readiness(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"status":"not_ready"`) {
		t.Fatalf("body = %s, want not_ready", rec.Body.String())
	}
}

func TestReadinessReturnsDegradedWhenOnlySearchFails(t *testing.T) {
	t.Cleanup(func() { deps = nil })
	deps = &healthDeps{db: fakeHealth{}, indexer: fakeHealth{err: errors.New("search down")}}

	rec := httptest.NewRecorder()
	Readiness(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"status":"degraded"`) {
		t.Fatalf("body = %s, want degraded", rec.Body.String())
	}
}

type fakeHealth struct {
	err error
}

func (f fakeHealth) Health(context.Context) error {
	return f.err
}
