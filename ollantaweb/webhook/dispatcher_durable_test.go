package webhook

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	telemetry "github.com/scovl/ollanta/adapter/secondary/telemetry"
	"github.com/scovl/ollanta/ollantacore/tracectx"
	"github.com/scovl/ollanta/ollantastore/postgres"
	"go.opentelemetry.io/otel/trace"
)

func TestDispatcherDispatchCreatesWebhookJobs(t *testing.T) {
	t.Parallel()

	repo := &fakeWebhookStore{
		hooks: []*postgres.Webhook{{ID: 7, Name: "hook", URL: "http://example.com", Enabled: true}},
	}
	jobs := &fakeWebhookJobStore{}
	d := &Dispatcher{repo: repo, jobs: jobs, client: &http.Client{Timeout: time.Second}, workerID: "worker-1", pollDelay: time.Millisecond}
	ctx, _ := tracedWebhookContext()

	d.Dispatch(ctx, 99, EventScanCompleted, map[string]any{"ok": true})

	if len(jobs.created) != 1 {
		t.Fatalf("created jobs = %d, want 1", len(jobs.created))
	}
	if jobs.created[0].WebhookID != 7 || jobs.created[0].Status != "accepted" {
		t.Fatalf("unexpected created job: %+v", jobs.created[0])
	}
	if jobs.created[0].TraceParent == "" {
		t.Fatal("expected durable webhook job to persist traceparent")
	}
}

func TestDispatcherDispatchReturnsExistingActiveWebhookJob(t *testing.T) {
	t.Parallel()

	repo := &fakeWebhookStore{
		hooks: []*postgres.Webhook{{ID: 7, Name: "hook", URL: "http://example.com", Enabled: true}},
	}
	jobs := &fakeWebhookJobStore{active: &postgres.WebhookJob{ID: 42, WebhookID: 7, Event: EventScanCompleted, Status: "accepted"}}
	d := &Dispatcher{repo: repo, jobs: jobs, client: &http.Client{Timeout: time.Second}, workerID: "worker-1", pollDelay: time.Millisecond}

	d.Dispatch(context.Background(), 99, EventScanCompleted, map[string]any{"ok": true})

	if len(jobs.created) != 0 {
		t.Fatalf("created jobs = %d, want no duplicate create", len(jobs.created))
	}
}

func TestDispatcherProcessNextMarksCompleted(t *testing.T) {
	t.Parallel()

	var seenTraceParent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenTraceParent = r.Header.Get("traceparent")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`ok`))
	}))
	defer server.Close()

	repo := &fakeWebhookStore{
		webhookByID: map[int64]*postgres.Webhook{1: {ID: 1, URL: server.URL, Secret: "secret", Enabled: true}},
	}
	jobs := &fakeWebhookJobStore{
		next: &postgres.WebhookJob{ID: 10, WebhookID: 1, Event: EventScanCompleted, Payload: []byte(`{"ok":true}`), Status: "running", Attempts: 1},
	}
	jobCtx, _ := tracedWebhookContext()
	jobs.next.TraceParent, jobs.next.TraceState = tracectx.Inject(jobCtx)
	d := &Dispatcher{repo: repo, jobs: jobs, client: server.Client(), workerID: "worker-1", pollDelay: time.Millisecond}

	processed, err := d.processNext(context.Background())
	if err != nil {
		t.Fatalf("processNext() error = %v", err)
	}
	if !processed {
		t.Fatal("expected a job to be processed")
	}
	if jobs.completedID != 10 {
		t.Fatalf("completedID = %d, want 10", jobs.completedID)
	}
	if len(repo.deliveries) != 1 || !repo.deliveries[0].Success {
		t.Fatalf("unexpected deliveries: %+v", repo.deliveries)
	}
	if seenTraceParent != jobs.next.TraceParent {
		t.Fatalf("forwarded traceparent = %q, want %q", seenTraceParent, jobs.next.TraceParent)
	}
}

func TestDispatcherRefreshQueueMetricsSetsGauge(t *testing.T) {
	t.Parallel()

	reg := telemetry.NewRegistry()
	metrics := telemetry.NewMetrics(reg)
	d := &Dispatcher{
		jobs:    &fakeWebhookJobStore{counts: map[string]int{"accepted": 4}},
		metrics: metrics,
	}

	d.refreshQueueMetrics(context.Background())

	body := readMetricsBody(t, reg)
	if !strings.Contains(body, "ollanta_webhook_queue_depth 4") {
		t.Fatalf("expected webhook queue depth in metrics output, got: %s", body)
	}
}

func tracedWebhookContext() (context.Context, string) {
	spanContext := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef, 0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef},
		SpanID:     trace.SpanID{0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef},
		TraceFlags: trace.FlagsSampled,
		Remote:     true,
	})
	ctx := trace.ContextWithRemoteSpanContext(context.Background(), spanContext)
	return ctx, spanContext.TraceID().String()
}

type fakeWebhookStore struct {
	hooks       []*postgres.Webhook
	webhookByID map[int64]*postgres.Webhook
	deliveries  []*postgres.WebhookDelivery
}

func (s *fakeWebhookStore) ForEvent(context.Context, int64, string) ([]*postgres.Webhook, error) {
	return s.hooks, nil
}

func (s *fakeWebhookStore) GetByID(_ context.Context, id int64) (*postgres.Webhook, error) {
	if wh, ok := s.webhookByID[id]; ok {
		return wh, nil
	}
	return nil, postgres.ErrNotFound
}

func (s *fakeWebhookStore) CreateDelivery(_ context.Context, d *postgres.WebhookDelivery) error {
	s.deliveries = append(s.deliveries, d)
	return nil
}

type fakeWebhookJobStore struct {
	created       []*postgres.WebhookJob
	active        *postgres.WebhookJob
	next          *postgres.WebhookJob
	counts        map[string]int
	completedID   int64
	failedID      int64
	rescheduledID int64
}

func (s *fakeWebhookJobStore) Create(_ context.Context, job *postgres.WebhookJob) error {
	s.created = append(s.created, job)
	return nil
}

func (s *fakeWebhookJobStore) GetActiveByIdentity(_ context.Context, _ int64, _, _ string) (*postgres.WebhookJob, error) {
	if s.active == nil {
		return nil, postgres.ErrNotFound
	}
	return s.active, nil
}

func (s *fakeWebhookJobStore) ClaimNext(_ context.Context, _ string) (*postgres.WebhookJob, error) {
	if s.next == nil {
		return nil, postgres.ErrNotFound
	}
	return s.next, nil
}

func (s *fakeWebhookJobStore) CountByStatus(_ context.Context, status string) (int, error) {
	return s.counts[status], nil
}

func (s *fakeWebhookJobStore) Reschedule(_ context.Context, id int64, _ string, _ time.Time, _ *int, _ *string) error {
	s.rescheduledID = id
	return nil
}

func (s *fakeWebhookJobStore) MarkCompleted(_ context.Context, id int64, _ *int, _ *string) error {
	s.completedID = id
	return nil
}

func (s *fakeWebhookJobStore) MarkFailed(_ context.Context, id int64, _ string, _ *int, _ *string) error {
	s.failedID = id
	return nil
}

func readMetricsBody(t *testing.T, reg *telemetry.Registry) string {
	t.Helper()

	rec := httptest.NewRecorder()
	reg.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("io.ReadAll() error = %v", err)
	}
	return string(body)
}
