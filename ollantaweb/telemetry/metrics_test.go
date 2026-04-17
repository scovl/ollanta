package telemetry_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/scovl/ollanta/ollantaweb/telemetry"
)

func TestCounterIncrements(t *testing.T) {
	t.Parallel()
	reg := telemetry.NewRegistry()
	c := reg.Counter("test_total", "A test counter")
	c.Inc()
	c.Add(4)
	// Just verifying it doesn't panic; value inspection requires Prometheus scrape.
}

func TestGaugeSetAndGet(t *testing.T) {
	t.Parallel()
	reg := telemetry.NewRegistry()
	g := reg.Gauge("test_gauge", "A test gauge")
	g.Set(42)
	_ = g // no Value() exported; just verify it doesn't panic.
}

func TestMetricsEndpointFormat(t *testing.T) {
	t.Parallel()
	reg := telemetry.NewRegistry()
	c := reg.Counter("http_requests_total", "HTTP requests")
	c.Add(3)

	handler := reg.Handler()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	body, _ := io.ReadAll(rec.Body)
	resp := string(body)

	if !strings.Contains(resp, "http_requests_total") {
		t.Errorf("expected metrics output to contain 'http_requests_total', got: %s", resp)
	}
}

func TestTraceIDMiddlewareInjectsHeader(t *testing.T) {
	t.Parallel()
	handler := telemetry.TraceIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := telemetry.TraceID(r.Context())
		if id == "" {
			t.Error("trace ID must be set in context")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Header().Get("X-Trace-Id") == "" {
		t.Error("response must include X-Trace-Id header")
	}
}

func TestTraceIDMiddlewarePropagatesIncomingID(t *testing.T) {
	t.Parallel()
	const incomingID = "my-custom-trace-id"
	handler := telemetry.TraceIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if telemetry.TraceID(r.Context()) != incomingID {
			t.Errorf("expected trace ID %q, got %q", incomingID, telemetry.TraceID(r.Context()))
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Trace-Id", incomingID)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
}

func TestTraceIDFromEmptyContext(t *testing.T) {
	t.Parallel()
	id := telemetry.TraceID(context.Background())
	if id != "" {
		t.Errorf("expected empty trace ID from empty context, got %q", id)
	}
}
