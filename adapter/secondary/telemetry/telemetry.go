package telemetry

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/trace"
)

// Counter is a monotonically increasing counter.
type Counter struct {
	val  atomic.Int64
	help string
	name string
}

// Inc increments the counter by 1.
func (c *Counter) Inc() { c.val.Add(1) }

// Add increments the counter by n.
func (c *Counter) Add(n int64) { c.val.Add(n) }

type counterSeries struct {
	labels []string
	val    atomic.Int64
}

// CounterVec is a counter partitioned by a bounded set of labels.
type CounterVec struct {
	mu         sync.Mutex
	help       string
	name       string
	labelNames []string
	series     map[string]*counterSeries
}

// Inc increments the labelled counter by 1.
func (v *CounterVec) Inc(labelValues ...string) { v.Add(1, labelValues...) }

// Add increments the labelled counter by n.
func (v *CounterVec) Add(n int64, labelValues ...string) {
	series := v.get(labelValues...)
	if series == nil {
		return
	}
	series.val.Add(n)
}

func (v *CounterVec) get(labelValues ...string) *counterSeries {
	if v == nil || len(labelValues) != len(v.labelNames) {
		return nil
	}
	key := strings.Join(labelValues, "\xff")
	v.mu.Lock()
	defer v.mu.Unlock()
	if series, ok := v.series[key]; ok {
		return series
	}
	labels := append([]string(nil), labelValues...)
	series := &counterSeries{labels: labels}
	v.series[key] = series
	return series
}

// Gauge is a value that can go up or down.
type Gauge struct {
	val  atomic.Int64
	help string
	name string
}

// Set sets the gauge to v.
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

// Observe records a single observation.
func (h *Histogram) Observe(v float64) {
	h.mu.Lock()
	h.sum += v
	h.count++
	for i, bucket := range h.buckets {
		if v <= bucket {
			h.counts[i]++
		}
	}
	h.mu.Unlock()
}

// ObserveDuration records a duration as seconds.
func (h *Histogram) ObserveDuration(d time.Duration) {
	h.Observe(d.Seconds())
}

type histogramSeries struct {
	labels    []string
	histogram *Histogram
}

// HistogramVec is a histogram partitioned by a bounded set of labels.
type HistogramVec struct {
	mu         sync.Mutex
	help       string
	name       string
	labelNames []string
	series     map[string]*histogramSeries
}

// Observe records a labelled observation.
func (v *HistogramVec) Observe(value float64, labelValues ...string) {
	series := v.get(labelValues...)
	if series == nil {
		return
	}
	series.histogram.Observe(value)
}

// ObserveDuration records a labelled duration as seconds.
func (v *HistogramVec) ObserveDuration(duration time.Duration, labelValues ...string) {
	v.Observe(duration.Seconds(), labelValues...)
}

func (v *HistogramVec) get(labelValues ...string) *histogramSeries {
	if v == nil || len(labelValues) != len(v.labelNames) {
		return nil
	}
	key := strings.Join(labelValues, "\xff")
	v.mu.Lock()
	defer v.mu.Unlock()
	if series, ok := v.series[key]; ok {
		return series
	}
	labels := append([]string(nil), labelValues...)
	series := &histogramSeries{
		labels: labels,
		histogram: &Histogram{
			name:    v.name,
			help:    v.help,
			buckets: defaultHistogramBuckets,
			counts:  make([]int64, len(defaultHistogramBuckets)),
		},
	}
	v.series[key] = series
	return series
}

// Registry holds all application metrics.
type Registry struct {
	mu            sync.RWMutex
	counters      map[string]*Counter
	counterVecs   map[string]*CounterVec
	gauges        map[string]*Gauge
	histograms    map[string]*Histogram
	histogramVecs map[string]*HistogramVec
}

var defaultHistogramBuckets = []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}

const metricHelpFormat = "# HELP %s %s\n"

// NewRegistry creates an empty metrics registry.
func NewRegistry() *Registry {
	return &Registry{
		counters:      map[string]*Counter{},
		counterVecs:   map[string]*CounterVec{},
		gauges:        map[string]*Gauge{},
		histograms:    map[string]*Histogram{},
		histogramVecs: map[string]*HistogramVec{},
	}
}

// Counter registers and returns a counter by name.
func (reg *Registry) Counter(name, help string) *Counter {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	if counter, ok := reg.counters[name]; ok {
		return counter
	}
	counter := &Counter{name: name, help: help}
	reg.counters[name] = counter
	return counter
}

// CounterVec registers and returns a labelled counter by name.
func (reg *Registry) CounterVec(name, help string, labelNames ...string) *CounterVec {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	if counter, ok := reg.counterVecs[name]; ok {
		return counter
	}
	counter := &CounterVec{name: name, help: help, labelNames: append([]string(nil), labelNames...), series: map[string]*counterSeries{}}
	reg.counterVecs[name] = counter
	return counter
}

// Gauge registers and returns a gauge by name.
func (reg *Registry) Gauge(name, help string) *Gauge {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	if gauge, ok := reg.gauges[name]; ok {
		return gauge
	}
	gauge := &Gauge{name: name, help: help}
	reg.gauges[name] = gauge
	return gauge
}

// Histogram registers and returns a histogram by name.
func (reg *Registry) Histogram(name, help string) *Histogram {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	if histogram, ok := reg.histograms[name]; ok {
		return histogram
	}
	counts := make([]int64, len(defaultHistogramBuckets))
	histogram := &Histogram{name: name, help: help, buckets: defaultHistogramBuckets, counts: counts}
	reg.histograms[name] = histogram
	return histogram
}

// HistogramVec registers and returns a labelled histogram by name.
func (reg *Registry) HistogramVec(name, help string, labelNames ...string) *HistogramVec {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	if histogram, ok := reg.histogramVecs[name]; ok {
		return histogram
	}
	histogram := &HistogramVec{name: name, help: help, labelNames: append([]string(nil), labelNames...), series: map[string]*histogramSeries{}}
	reg.histogramVecs[name] = histogram
	return histogram
}

// Handler returns an http.HandlerFunc that serves /metrics in Prometheus text format.
func (reg *Registry) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		reg.mu.RLock()
		defer reg.mu.RUnlock()

		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		var sb strings.Builder
		reg.writeMetrics(&sb)

		_, _ = w.Write([]byte(sb.String()))
	}
}

func (reg *Registry) writeMetrics(sb *strings.Builder) {
	writeCounters(sb, reg.counters)
	writeCounterVecs(sb, reg.counterVecs)
	writeGauges(sb, reg.gauges)
	writeHistograms(sb, reg.histograms)
	writeHistogramVecs(sb, reg.histogramVecs)
}

func writeCounters(sb *strings.Builder, counters map[string]*Counter) {
	for _, counter := range counters {
		fmt.Fprintf(sb, metricHelpFormat, counter.name, counter.help)
		fmt.Fprintf(sb, "# TYPE %s counter\n", counter.name)
		fmt.Fprintf(sb, "%s %d\n", counter.name, counter.val.Load())
	}
}

func writeCounterVecs(sb *strings.Builder, counters map[string]*CounterVec) {
	for _, counter := range counters {
		fmt.Fprintf(sb, metricHelpFormat, counter.name, counter.help)
		fmt.Fprintf(sb, "# TYPE %s counter\n", counter.name)
		for _, series := range counter.sortedSeries() {
			fmt.Fprintf(sb, "%s%s %d\n", counter.name, metricLabels(counter.labelNames, series.labels, "", ""), series.val.Load())
		}
	}
}

func writeGauges(sb *strings.Builder, gauges map[string]*Gauge) {
	for _, gauge := range gauges {
		fmt.Fprintf(sb, metricHelpFormat, gauge.name, gauge.help)
		fmt.Fprintf(sb, "# TYPE %s gauge\n", gauge.name)
		fmt.Fprintf(sb, "%s %d\n", gauge.name, gauge.val.Load())
	}
}

func writeHistograms(sb *strings.Builder, histograms map[string]*Histogram) {
	for _, histogram := range histograms {
		writeHistogram(sb, histogram)
	}
}

func writeHistogram(sb *strings.Builder, histogram *Histogram) {
	histogram.mu.Lock()
	defer histogram.mu.Unlock()

	fmt.Fprintf(sb, metricHelpFormat, histogram.name, histogram.help)
	fmt.Fprintf(sb, "# TYPE %s histogram\n", histogram.name)
	for i, bucket := range histogram.buckets {
		fmt.Fprintf(sb, "%s_bucket{le=\"%g\"} %d\n", histogram.name, bucket, histogram.counts[i])
	}
	fmt.Fprintf(sb, "%s_bucket{le=\"+Inf\"} %d\n", histogram.name, histogram.count)
	fmt.Fprintf(sb, "%s_sum %g\n", histogram.name, histogram.sum)
	fmt.Fprintf(sb, "%s_count %d\n", histogram.name, histogram.count)
}

func writeHistogramVecs(sb *strings.Builder, histograms map[string]*HistogramVec) {
	for _, histogram := range histograms {
		fmt.Fprintf(sb, metricHelpFormat, histogram.name, histogram.help)
		fmt.Fprintf(sb, "# TYPE %s histogram\n", histogram.name)
		for _, series := range histogram.sortedSeries() {
			writeHistogramSeries(sb, histogram, series)
		}
	}
}

func writeHistogramSeries(sb *strings.Builder, vec *HistogramVec, series *histogramSeries) {
	series.histogram.mu.Lock()
	defer series.histogram.mu.Unlock()

	for i, bucket := range series.histogram.buckets {
		labels := metricLabels(vec.labelNames, series.labels, "le", fmt.Sprintf("%g", bucket))
		fmt.Fprintf(sb, "%s_bucket%s %d\n", vec.name, labels, series.histogram.counts[i])
	}
	fmt.Fprintf(sb, "%s_bucket%s %d\n", vec.name, metricLabels(vec.labelNames, series.labels, "le", "+Inf"), series.histogram.count)
	fmt.Fprintf(sb, "%s_sum%s %g\n", vec.name, metricLabels(vec.labelNames, series.labels, "", ""), series.histogram.sum)
	fmt.Fprintf(sb, "%s_count%s %d\n", vec.name, metricLabels(vec.labelNames, series.labels, "", ""), series.histogram.count)
}

func (v *CounterVec) sortedSeries() []*counterSeries {
	v.mu.Lock()
	defer v.mu.Unlock()
	keys := make([]string, 0, len(v.series))
	for key := range v.series {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	series := make([]*counterSeries, 0, len(keys))
	for _, key := range keys {
		series = append(series, v.series[key])
	}
	return series
}

func (v *HistogramVec) sortedSeries() []*histogramSeries {
	v.mu.Lock()
	defer v.mu.Unlock()
	keys := make([]string, 0, len(v.series))
	for key := range v.series {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	series := make([]*histogramSeries, 0, len(keys))
	for _, key := range keys {
		series = append(series, v.series[key])
	}
	return series
}

func metricLabels(labelNames, labelValues []string, extraName, extraValue string) string {
	if len(labelNames) == 0 && extraName == "" {
		return ""
	}
	parts := make([]string, 0, len(labelNames)+1)
	for i, name := range labelNames {
		parts = append(parts, fmt.Sprintf("%s=\"%s\"", name, escapeLabelValue(labelValues[i])))
	}
	if extraName != "" {
		parts = append(parts, fmt.Sprintf("%s=\"%s\"", extraName, escapeLabelValue(extraValue)))
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func escapeLabelValue(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "\n", "\\n")
	return strings.ReplaceAll(value, "\"", "\\\"")
}

// Metrics holds named application metrics.
type Metrics struct {
	HTTPRequestsTotal           *Counter
	HTTPRequestDuration         *Histogram
	ScansTotal                  *Counter
	ScanJobsProcessed           *Counter
	ScanJobsFailed              *Counter
	IngestDuration              *Histogram
	IngestStepDuration          *HistogramVec
	IngestStepOutcomes          *CounterVec
	IngestQueueDepth            *Gauge
	IndexQueueDepth             *Gauge
	IndexJobsProcessed          *Counter
	IndexJobRetries             *Counter
	ScanJobsRecovered           *Counter
	ScanJobsFailedByRecovery    *Counter
	IndexJobsRecovered          *Counter
	IndexJobsFailedByRecovery   *Counter
	WebhookQueueDepth           *Gauge
	WebhookDeliveries           *Counter
	WebhookJobsRecovered        *Counter
	WebhookJobsFailedByRecovery *Counter
}

// NewMetrics registers all application metrics in reg.
func NewMetrics(reg *Registry) *Metrics {
	return &Metrics{
		HTTPRequestsTotal:           reg.Counter("ollanta_http_requests_total", "Total number of HTTP requests handled"),
		HTTPRequestDuration:         reg.Histogram("ollanta_http_request_duration_seconds", "Duration of HTTP requests in seconds"),
		ScansTotal:                  reg.Counter("ollanta_scans_total", "Total number of scans ingested"),
		ScanJobsProcessed:           reg.Counter("ollanta_scan_jobs_processed_total", "Total scan jobs processed successfully"),
		ScanJobsFailed:              reg.Counter("ollanta_scan_jobs_failed_total", "Total scan jobs that failed during processing"),
		IngestDuration:              reg.Histogram("ollanta_ingest_duration_seconds", "Duration of scan ingest pipeline in seconds"),
		IngestStepDuration:          reg.HistogramVec("ollanta_ingest_step_duration_seconds", "Duration of scan ingest steps in seconds", "step", "outcome"),
		IngestStepOutcomes:          reg.CounterVec("ollanta_ingest_step_total", "Total scan ingest step outcomes", "step", "outcome"),
		IngestQueueDepth:            reg.Gauge("ollanta_ingest_queue_depth", "Current depth of the ingest queue"),
		IndexQueueDepth:             reg.Gauge("ollanta_index_queue_depth", "Current depth of the durable index queue"),
		IndexJobsProcessed:          reg.Counter("ollanta_index_jobs_total", "Total number of index jobs processed successfully"),
		IndexJobRetries:             reg.Counter("ollanta_index_job_retries_total", "Total number of index job retries scheduled"),
		ScanJobsRecovered:           reg.Counter("ollanta_scan_jobs_recovered_total", "Total stale scan jobs requeued by recovery"),
		ScanJobsFailedByRecovery:    reg.Counter("ollanta_scan_jobs_failed_by_recovery_total", "Total stale scan jobs failed by recovery"),
		IndexJobsRecovered:          reg.Counter("ollanta_index_jobs_recovered_total", "Total stale index jobs requeued by recovery"),
		IndexJobsFailedByRecovery:   reg.Counter("ollanta_index_jobs_failed_by_recovery_total", "Total stale index jobs failed by recovery"),
		WebhookQueueDepth:           reg.Gauge("ollanta_webhook_queue_depth", "Current depth of the durable webhook queue"),
		WebhookDeliveries:           reg.Counter("ollanta_webhook_deliveries_total", "Total webhook deliveries attempted"),
		WebhookJobsRecovered:        reg.Counter("ollanta_webhook_jobs_recovered_total", "Total stale webhook jobs requeued by recovery"),
		WebhookJobsFailedByRecovery: reg.Counter("ollanta_webhook_jobs_failed_by_recovery_total", "Total stale webhook jobs failed by recovery"),
	}
}

// ObserveIngestStep records duration and outcome for a bounded ingest step.
func (m *Metrics) ObserveIngestStep(step, outcome string, duration time.Duration) {
	if m == nil {
		return
	}
	step = boundedIngestStep(step)
	outcome = boundedIngestOutcome(outcome)
	m.IngestStepOutcomes.Inc(step, outcome)
	m.IngestStepDuration.ObserveDuration(duration, step, outcome)
}

func boundedIngestStep(step string) string {
	switch step {
	case "process", "decode", "persist", "side_effects":
		return step
	default:
		return "other"
	}
}

func boundedIngestOutcome(outcome string) string {
	switch outcome {
	case "success", "failure", "skipped":
		return outcome
	default:
		return "unknown"
	}
}

// ObserveJobRecovery records recovery outcomes using bounded per-job-type counters.
func (m *Metrics) ObserveJobRecovery(jobType string, requeued, failed int64) {
	if m == nil {
		return
	}
	switch jobType {
	case "scan":
		m.ScanJobsRecovered.Add(requeued)
		m.ScanJobsFailedByRecovery.Add(failed)
	case "index":
		m.IndexJobsRecovered.Add(requeued)
		m.IndexJobsFailedByRecovery.Add(failed)
	case "webhook":
		m.WebhookJobsRecovered.Add(requeued)
		m.WebhookJobsFailedByRecovery.Add(failed)
	}
}

// ObserveHTTPRequest records a handled HTTP request.
func (m *Metrics) ObserveHTTPRequest(d time.Duration) {
	if m == nil {
		return
	}
	m.HTTPRequestsTotal.Inc()
	m.HTTPRequestDuration.ObserveDuration(d)
}

// SetupLogger creates a structured logger for runtime services.
func SetupLogger(level string, attrs ...any) *slog.Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: parseLogLevel(level)})
	return slog.New(handler).With(attrs...)
}

func parseLogLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// WithTraceAttrs appends trace correlation fields when they exist in ctx.
func WithTraceAttrs(ctx context.Context, attrs ...any) []any {
	traceID := TraceID(ctx)
	spanID := SpanID(ctx)
	if traceID == "" && spanID == "" {
		return attrs
	}
	out := make([]any, 0, len(attrs)+4)
	out = append(out, attrs...)
	if traceID != "" {
		out = append(out, "trace_id", traceID)
	}
	if spanID != "" {
		out = append(out, "span_id", spanID)
	}
	return out
}

type contextKey string

const traceIDKey contextKey = "trace_id"

// TraceID returns the trace ID stored in ctx, or empty string.
func TraceID(ctx context.Context) string {
	spanContext := trace.SpanContextFromContext(ctx)
	if spanContext.IsValid() {
		return spanContext.TraceID().String()
	}
	value, _ := ctx.Value(traceIDKey).(string)
	return value
}

// TraceIDMiddleware injects X-Trace-Id into every request.
func TraceIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceID := r.Header.Get("X-Trace-Id")
		if currentTraceID := TraceID(r.Context()); currentTraceID != "" {
			traceID = currentTraceID
		}
		if traceID == "" {
			traceID = newUUID()
		}
		ctx := context.WithValue(r.Context(), traceIDKey, traceID)
		w.Header().Set("X-Trace-Id", traceID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func newUUID() string {
	return fmt.Sprintf("%x-%x-%x-%x-%x",
		timeNow().UnixNano()&0xFFFFFFFF,
		timeNow().UnixNano()>>16&0xFFFF,
		(timeNow().UnixNano()>>32&0x0FFF)|0x4000,
		(timeNow().UnixNano()>>48&0x3FFF)|0x8000,
		timeNow().UnixNano()&0xFFFFFFFFFFFF,
	)
}

var timeNow = time.Now
