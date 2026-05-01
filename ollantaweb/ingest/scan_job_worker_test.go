package ingest

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	telemetry "github.com/scovl/ollanta/adapter/secondary/telemetry"
)

func TestScanJobWorkerRefreshQueueMetricsSetsGauge(t *testing.T) {
	t.Parallel()

	reg := telemetry.NewRegistry()
	metrics := telemetry.NewMetrics(reg)
	worker := &ScanJobWorker{
		processor: fakeScanJobProcessor{counts: map[string]int{"accepted": 2}},
		metrics:   metrics,
	}

	worker.refreshQueueMetrics(context.Background())

	body := readMetricsBody(t, reg)
	if !strings.Contains(body, "ollanta_ingest_queue_depth 2") {
		t.Fatalf("expected ingest queue depth in metrics output, got: %s", body)
	}
}

type fakeScanJobProcessor struct {
	counts map[string]int
}

func (p fakeScanJobProcessor) ProcessNext(context.Context) (*ScanJob, error) {
	return nil, nil
}

func (p fakeScanJobProcessor) CountByStatus(_ context.Context, status string) (int, error) {
	return p.counts[status], nil
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
