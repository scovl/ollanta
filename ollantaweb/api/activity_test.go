package api

import (
	"testing"
	"time"

	"github.com/scovl/ollanta/ollantastore/postgres"
)

func TestBuildActivityEntriesAddsDeltasAndEvents(t *testing.T) {
	now := time.Now().UTC()
	scans := []*postgres.Scan{
		{
			ID:                   2,
			AnalysisDate:         now,
			GateStatus:           "ERROR",
			Version:              "1.1.0",
			TotalNcloc:           120,
			TotalIssues:          18,
			TotalBugs:            3,
			TotalCodeSmells:      13,
			TotalVulnerabilities: 2,
			NewIssues:            9,
		},
		{
			ID:                   1,
			AnalysisDate:         now.Add(-time.Hour),
			GateStatus:           "OK",
			Version:              "1.0.0",
			TotalNcloc:           100,
			TotalIssues:          10,
			TotalBugs:            1,
			TotalCodeSmells:      8,
			TotalVulnerabilities: 1,
		},
	}

	entries := buildActivityEntries(scans, 2, 0, 1)
	if len(entries) != 1 {
		t.Fatalf("entries len = %d, want 1", len(entries))
	}
	entry := entries[0]
	if entry.Delta.Issues != 8 || entry.Delta.Bugs != 2 || entry.Delta.CodeSmells != 5 || entry.Delta.Vulnerabilities != 1 || entry.Delta.Ncloc != 20 {
		t.Fatalf("Delta = %+v, want scan-to-scan differences", entry.Delta)
	}
	if len(entry.Events) != 3 {
		t.Fatalf("Events = %#v, want gate, version, and issue spike events", entry.Events)
	}
	if entry.Events[0].Category != "QUALITY_GATE" || entry.Events[1].Category != "VERSION" || entry.Events[2].Category != "ISSUE_SPIKE" {
		t.Fatalf("Events = %#v, want ordered notable events", entry.Events)
	}
}

func TestBuildActivityEntriesMarksFirstAnalysis(t *testing.T) {
	now := time.Now().UTC()
	scans := []*postgres.Scan{{ID: 1, AnalysisDate: now, GateStatus: "OK"}}

	entries := buildActivityEntries(scans, 1, 0, 20)
	if len(entries) != 1 {
		t.Fatalf("entries len = %d, want 1", len(entries))
	}
	if len(entries[0].Events) != 1 || entries[0].Events[0].Category != "FIRST_ANALYSIS" {
		t.Fatalf("Events = %#v, want first analysis event", entries[0].Events)
	}
}
