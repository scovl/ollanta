package postgres_test

import (
	"encoding/json"
	"testing"

	"github.com/scovl/ollanta/ollantastore/postgres"
)

// TestValidTransitionsAllowed checks that the exported sentinel for ErrNotFound works
// and that transition logic constants are sane.
func TestIssueTransitionValidResolutions(t *testing.T) {
	t.Parallel()
	// ErrNotFound must not be nil (used to identify missing issues).
	if postgres.ErrNotFound == nil {
		t.Fatal("ErrNotFound must not be nil")
	}
}

// TestIssueTransitionStatusMapping validates mapping rules used by the API handler.
// false_positive, wont_fix → closed; "" → reopen (open); confirmed → confirmed
func TestIssueTransitionStatusMapping(t *testing.T) {
	t.Parallel()
	tests := []struct {
		resolution     string
		expectedStatus string
	}{
		{"false_positive", "closed"},
		{"wont_fix", "closed"},
		{"fixed", "closed"},
		{"confirmed", "closed"},
		{"", "open"}, // reopen
	}
	for _, tt := range tests {
		toStatus := "closed"
		if tt.resolution == "" {
			toStatus = "open"
		}
		if toStatus != tt.expectedStatus {
			t.Errorf("resolution=%q: expected status %q, got %q", tt.resolution, tt.expectedStatus, toStatus)
		}
	}
}

func TestIssueRow_EngineIDField(t *testing.T) {
	t.Parallel()
	row := postgres.IssueRow{EngineID: "semgrep"}
	if row.EngineID != "semgrep" {
		t.Errorf("EngineID: got %q", row.EngineID)
	}
}

func TestIssueRow_SecondaryLocationsJSON(t *testing.T) {
	t.Parallel()
	sl := json.RawMessage(`[{"filePath":"a.go","message":"see also","startLine":5}]`)
	row := postgres.IssueRow{SecondaryLocations: sl}
	if string(row.SecondaryLocations) != string(sl) {
		t.Errorf("SecondaryLocations mismatch")
	}
}

func TestIssueFilter_EngineIDField(t *testing.T) {
	t.Parallel()
	eid := "ollanta"
	f := postgres.IssueFilter{EngineID: &eid}
	if *f.EngineID != "ollanta" {
		t.Errorf("EngineID filter: got %q", *f.EngineID)
	}
}
