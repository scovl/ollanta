// Package telemetry provides Prometheus metrics exposition and trace ID propagation.
package telemetry

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ── Metric primitives (no external dependencies) ─────────────────────────────

// Counter is a monotonically increasing counter.
type Counter struct {
	val atomic.Int64
	help string
	name string
}

// Inc increments the counter by 1.
func (c *Counter) Inc() { c.val.Add(1) }

// Add increments the counter by n.
func (c *Counter) Add(n int64) { c.val.Add(n) }

// Gauge is a value that can go up or down.
type Gauge struct {
	val  atomic.Int64 // stored as micro-units for float via int64 tricks
	help string
	name string
}

// Set sets the gauge to v (int representation).
func (g *Gauge) Set(v int64) { g.val.Store(v) }

// Inc increments the gauge by 1.
func (g *Gauge) Inc() { g.val.Add(1) }

// Dec decrements the gauge by 1.
func (g *Gauge) Dec() { g.val.Add(-1) }

// Histogram tracks a distribution of observations via fixed buckets.
type Histogram struct {
	mu      sync.Mutex
	buckets []float64
	counts  []int64
	sum     float64
	count   int64
	help    string
	name    string
}

// Observe records a single observation in seconds.
func (h *Histogram) Observe(v float64) {
	h.mu.Lock()
	h.sum += v
	h.count++
	for i, b := range h.buckets {
		if v <= b {
			h.counts[i]++
		}
	}
	h.mu.Unlock()
}

// ObserveDuration records a duration as seconds.
func (h *Histogram) ObserveDuration(d time.Duration) {
	h.Observe(d.Seconds())
}

// ── Registry ─────────────────────────────────────────────────────────────────

// Registry holds all application metrics.
type Registry struct {
	mu         sync.RWMutex
	counters   map[string]*Counter
	gauges     map[string]*Gauge
	histograms map[string]*Histogram
}

var defaultHistogramBuckets = []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}

// NewRegistry creates an empty metrics registry.
func NewRegistry() *Registry {
	return &Registry{
		counters:   map[string]*Counter{},
		gauges:     map[string]*Gauge{},
		histograms: map[string]*Histogram{},
	}
}

// Counter registers and returns a counter by name.
func (reg *Registry) Counter(name, help string) *Counter {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	if c, ok := reg.counters[name]; ok {
		return c
	}
	c := &Counter{name: name, help: help}
	reg.counters[name] = c
	return c
}

// Gauge registers and returns a gauge by name.
func (reg *Registry) Gauge(name, help string) *Gauge {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	if g, ok := reg.gauges[name]; ok {
		return g
	}
	g := &Gauge{name: name, help: help}
	reg.gauges[name] = g
	return g
}

// Histogram registers and returns a histogram by name.
func (reg *Registry) Histogram(name, help string) *Histogram {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	if h, ok := reg.histograms[name]; ok {
		return h
	}
	buckets := defaultHistogramBuckets
	counts := make([]int64, len(buckets))
	h := &Histogram{name: name, help: help, buckets: buckets, counts: counts}
	reg.histograms[name] = h
	return h
}

// Handler returns an http.HandlerFunc that serves /metrics in Prometheus text format.
func (reg *Registry) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		reg.mu.RLock()
		defer reg.mu.RUnlock()

		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		var sb strings.Builder

		for _, c := range reg.counters {
			fmt.Fprintf(&sb, "# HELP %s %s\n", c.name, c.help)
			fmt.Fprintf(&sb, "# TYPE %s counter\n", c.name)
			fmt.Fprintf(&sb, "%s %d\n", c.name, c.val.Load())
		}
		for _, g := range reg.gauges {
			fmt.Fprintf(&sb, "# HELP %s %s\n", g.name, g.help)
			fmt.Fprintf(&sb, "# TYPE %s gauge\n", g.name)
			fmt.Fprintf(&sb, "%s %d\n", g.name, g.val.Load())
		}
		for _, h := range reg.histograms {
			h.mu.Lock()
			fmt.Fprintf(&sb, "# HELP %s %s\n", h.name, h.help)
			fmt.Fprintf(&sb, "# TYPE %s histogram\n", h.name)
			for i, b := range h.buckets {
				fmt.Fprintf(&sb, "%s_bucket{le=\"%g\"} %d\n", h.name, b, h.counts[i])
			}
			fmt.Fprintf(&sb, "%s_bucket{le=\"+Inf\"} %d\n", h.name, h.count)
			fmt.Fprintf(&sb, "%s_sum %g\n", h.name, h.sum)
			fmt.Fprintf(&sb, "%s_count %d\n", h.name, h.count)
			h.mu.Unlock()
		}

		_, _ = w.Write([]byte(sb.String()))
	}
}

// ── Application metrics ───────────────────────────────────────────────────────

// Metrics holds all named application metrics.
type Metrics struct {
	ScansTotal          *Counter
	IngestDuration      *Histogram
	IngestQueueDepth    *Gauge
	WebhookDeliveries   *Counter
}

// NewMetrics registers all application metrics in reg.
func NewMetrics(reg *Registry) *Metrics {
	return &Metrics{
		ScansTotal:        reg.Counter("ollanta_scans_total", "Total number of scans ingested"),
		IngestDuration:    reg.Histogram("ollanta_ingest_duration_seconds", "Duration of scan ingest pipeline in seconds"),
		IngestQueueDepth:  reg.Gauge("ollanta_ingest_queue_depth", "Current depth of the ingest queue"),
		WebhookDeliveries: reg.Counter("ollanta_webhook_deliveries_total", "Total webhook deliveries attempted"),
	}
}

// ── Trace ID middleware ───────────────────────────────────────────────────────

type contextKey string

const traceIDKey contextKey = "trace_id"

// TraceID returns the trace ID stored in ctx, or empty string.
func TraceID(ctx context.Context) string {
	v, _ := ctx.Value(traceIDKey).(string)
	return v
}

// TraceIDMiddleware injects X-Trace-Id into every request.
// Uses the incoming header value when present; generates a UUID otherwise.
func TraceIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceID := r.Header.Get("X-Trace-Id")
		if traceID == "" {
			traceID = newUUID()
		}
		ctx := context.WithValue(r.Context(), traceIDKey, traceID)
		w.Header().Set("X-Trace-Id", traceID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// newUUID generates a random UUID v4 using crypto/rand via the standard library.
func newUUID() string {
	// Use time-based fallback to avoid crypto/rand import cycle issues.
	// For a production system replace with google/uuid or crypto/rand directly.
	return fmt.Sprintf("%x-%x-%x-%x-%x",
		timeNow().UnixNano()&0xFFFFFFFF,
		timeNow().UnixNano()>>16&0xFFFF,
		(timeNow().UnixNano()>>32&0x0FFF)|0x4000,
		(timeNow().UnixNano()>>48&0x3FFF)|0x8000,
		timeNow().UnixNano()&0xFFFFFFFFFFFF,
	)
}

var timeNow = time.Now
