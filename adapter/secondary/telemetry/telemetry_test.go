package telemetry

import (
	"context"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel/trace"
)

func TestParseLogLevel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  slog.Level
	}{
		{name: "debug", input: "debug", want: slog.LevelDebug},
		{name: "warn", input: "warn", want: slog.LevelWarn},
		{name: "warning", input: "warning", want: slog.LevelWarn},
		{name: "error", input: "error", want: slog.LevelError},
		{name: "default", input: "bogus", want: slog.LevelInfo},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := parseLogLevel(tt.input); got != tt.want {
				t.Fatalf("parseLogLevel(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestWithTraceAttrsIncludesTraceFields(t *testing.T) {
	t.Parallel()

	spanContext := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{0xde, 0xad, 0xbe, 0xef, 0xde, 0xad, 0xbe, 0xef, 0xde, 0xad, 0xbe, 0xef, 0xde, 0xad, 0xbe, 0xef},
		SpanID:     trace.SpanID{0xde, 0xad, 0xbe, 0xef, 0xde, 0xad, 0xbe, 0xef},
		TraceFlags: trace.FlagsSampled,
		Remote:     true,
	})
	ctx := trace.ContextWithRemoteSpanContext(context.Background(), spanContext)

	attrs := WithTraceAttrs(ctx, "service", "ollanta")
	if len(attrs) != 6 {
		t.Fatalf("len(attrs) = %d, want 6", len(attrs))
	}
	if attrs[2] != "trace_id" || attrs[3] != spanContext.TraceID().String() {
		t.Fatalf("unexpected trace attrs: %+v", attrs)
	}
	if attrs[4] != "span_id" || attrs[5] != spanContext.SpanID().String() {
		t.Fatalf("unexpected span attrs: %+v", attrs)
	}
}

func TestRegistryHandlerExportsMetrics(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	metrics := NewMetrics(registry)
	metrics.ObserveHTTPRequest(25)

	rec := httptest.NewRecorder()
	registry.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	body := rec.Body.String()
	if !strings.Contains(body, "ollanta_http_requests_total 1") {
		t.Fatalf("metrics output missing request counter: %s", body)
	}
	if !strings.Contains(body, "ollanta_http_request_duration_seconds_count 1") {
		t.Fatalf("metrics output missing duration histogram count: %s", body)
	}
}

func TestRegistryHandlerExportsLabelledMetrics(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	metrics := NewMetrics(registry)
	metrics.ObserveIngestStep("process", "success", 150*time.Millisecond)

	rec := httptest.NewRecorder()
	registry.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	body := rec.Body.String()
	if !strings.Contains(body, `ollanta_ingest_step_total{step="process",outcome="success"} 1`) {
		t.Fatalf("metrics output missing labelled counter: %s", body)
	}
	if !strings.Contains(body, `ollanta_ingest_step_duration_seconds_count{step="process",outcome="success"} 1`) {
		t.Fatalf("metrics output missing labelled histogram count: %s", body)
	}
}
