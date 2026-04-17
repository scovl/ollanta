package newcode_test

import (
	"context"
	"testing"
	"time"

	"github.com/scovl/ollanta/ollantaengine/newcode"
)

// mockLister is a test implementation of ScanLister.
type mockLister struct {
	scans []newcode.Scan
}

func (m *mockLister) ListByProject(ctx context.Context, projectID int64) ([]newcode.Scan, error) {
	return m.scans, nil
}

func makeScan(id int64, version, branch string, analysisDate time.Time) newcode.Scan {
	return newcode.Scan{ID: id, Version: version, Branch: branch, AnalysisDate: analysisDate}
}

func TestResolveNumberOfDays(t *testing.T) {
	t.Parallel()
	now := time.Now()
	lister := &mockLister{scans: []newcode.Scan{
		makeScan(3, "", "main", now),
		makeScan(2, "", "main", now.Add(-20*24*time.Hour)),
		makeScan(1, "", "main", now.Add(-40*24*time.Hour)),
	}}
	startID, err := newcode.Resolve(context.Background(), newcode.StrategyNumberOfDays, "30", 1, lister)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Oldest scan within 30 days of latest (scan 3) → scan 2
	if startID != 2 {
		t.Errorf("expected scan 2 as start, got %d", startID)
	}
}

func TestResolveSpecificAnalysis(t *testing.T) {
	t.Parallel()
	lister := &mockLister{scans: []newcode.Scan{
		makeScan(42, "v1.0.0", "main", time.Now()),
	}}
	startID, err := newcode.Resolve(context.Background(), newcode.StrategySpecificAnalysis, "42", 1, lister)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if startID != 42 {
		t.Errorf("expected scan 42, got %d", startID)
	}
}

func TestResolveReferenceBranch(t *testing.T) {
	t.Parallel()
	lister := &mockLister{scans: []newcode.Scan{
		makeScan(11, "", "release", time.Now()),
		makeScan(10, "", "release", time.Now().Add(-10*time.Hour)),
	}}
	startID, err := newcode.Resolve(context.Background(), newcode.StrategyReferenceBranch, "release", 1, lister)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if startID != 11 {
		t.Errorf("expected latest scan on branch (11), got %d", startID)
	}
}

func TestResolvePreviousVersion(t *testing.T) {
	t.Parallel()
	lister := &mockLister{scans: []newcode.Scan{
		makeScan(3, "v1.2.0", "main", time.Now()),
		makeScan(2, "v1.1.0", "main", time.Now().Add(-10*24*time.Hour)),
		makeScan(1, "v1.0.0", "main", time.Now().Add(-30*24*time.Hour)),
	}}
	startID, err := newcode.Resolve(context.Background(), newcode.StrategyPreviousVersion, "", 1, lister)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if startID != 2 {
		t.Errorf("expected second-to-latest versioned scan (2), got %d", startID)
	}
}

func TestResolveAutoFallback30Days(t *testing.T) {
	t.Parallel()
	now := time.Now()
	// No semver versions — auto should fall back to 30 days
	lister := &mockLister{scans: []newcode.Scan{
		makeScan(3, "", "main", now),
		makeScan(2, "", "main", now.Add(-25*24*time.Hour)),
		makeScan(1, "", "main", now.Add(-40*24*time.Hour)),
	}}
	startID, err := newcode.Resolve(context.Background(), newcode.StrategyAuto, "", 1, lister)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Oldest within 30 days of latest: scan 2
	if startID != 2 {
		t.Errorf("expected scan 2 for auto fallback 30 days, got %d", startID)
	}
}
