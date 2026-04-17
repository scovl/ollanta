package tracking_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/scovl/ollanta/ollantacore/domain"
	"github.com/scovl/ollanta/ollantaengine/tracking"
)

// ── helpers ────────────────────────────────────────────────────────────────

func openIssue(rule, path, hash string, line int) *domain.Issue {
	return &domain.Issue{
		RuleKey:       rule,
		ComponentPath: path,
		Line:          line,
		LineHash:      hash,
		Status:        domain.StatusOpen,
	}
}

func closedIssue(rule, path, hash string, line int) *domain.Issue {
	iss := openIssue(rule, path, hash, line)
	iss.Status = domain.StatusClosed
	return iss
}

// ── TrackingResult counters ────────────────────────────────────────────────

func TestTrackingResult_Counts(t *testing.T) {
	r := &tracking.TrackingResult{
		New:       []*domain.Issue{{}, {}},
		Unchanged: []tracking.IssuePair{{}, {}},
		Closed:    []*domain.Issue{{}},
		Reopened:  []tracking.IssuePair{{}},
	}
	if r.NewCount() != 2 {
		t.Errorf("NewCount: got %d", r.NewCount())
	}
	if r.ClosedCount() != 1 {
		t.Errorf("ClosedCount: got %d", r.ClosedCount())
	}
	if r.ReopenedCount() != 1 {
		t.Errorf("ReopenedCount: got %d", r.ReopenedCount())
	}
	if r.TotalOpen() != 2+2+1 {
		t.Errorf("TotalOpen: got %d, want 5", r.TotalOpen())
	}
}

// ── Track ──────────────────────────────────────────────────────────────────

func TestTrack_NoPrevious_AllNew(t *testing.T) {
	current := []*domain.Issue{
		openIssue("r1", "a.go", "h1", 1),
		openIssue("r2", "a.go", "h2", 2),
	}
	r := tracking.Track(current, nil)
	if r.NewCount() != 2 {
		t.Errorf("expected 2 new, got %d", r.NewCount())
	}
	if r.TotalOpen() != 2 {
		t.Errorf("TotalOpen: got %d", r.TotalOpen())
	}
}

func TestTrack_NoCurrent_AllClosed(t *testing.T) {
	previous := []*domain.Issue{
		openIssue("r1", "a.go", "h1", 1),
		openIssue("r2", "a.go", "h2", 2),
	}
	r := tracking.Track(nil, previous)
	if r.ClosedCount() != 2 {
		t.Errorf("expected 2 closed, got %d", r.ClosedCount())
	}
}

func TestTrack_MatchedByRuleAndHash(t *testing.T) {
	prev := []*domain.Issue{openIssue("rule:a", "f.go", "hash1", 10)}
	cur := []*domain.Issue{openIssue("rule:a", "f.go", "hash1", 15)} // line changed
	r := tracking.Track(cur, prev)
	if len(r.Unchanged) != 1 {
		t.Fatalf("expected 1 unchanged, got %d", len(r.Unchanged))
	}
	if r.Unchanged[0].Current.Line != 15 {
		t.Error("current issue line should be 15")
	}
}

func TestTrack_DifferentRuleNoMatch(t *testing.T) {
	prev := []*domain.Issue{openIssue("rule:a", "f.go", "hash1", 10)}
	cur := []*domain.Issue{openIssue("rule:b", "f.go", "hash1", 10)}
	r := tracking.Track(cur, prev)
	if r.NewCount() != 1 {
		t.Errorf("expected 1 new (different rule), got %d", r.NewCount())
	}
	if r.ClosedCount() != 1 {
		t.Errorf("expected 1 closed (unmatched prev), got %d", r.ClosedCount())
	}
}

func TestTrack_DifferentHashNoMatch(t *testing.T) {
	prev := []*domain.Issue{openIssue("rule:a", "f.go", "hash1", 10)}
	cur := []*domain.Issue{openIssue("rule:a", "f.go", "hash2", 10)}
	r := tracking.Track(cur, prev)
	if r.NewCount() != 1 {
		t.Errorf("expected 1 new (different hash)")
	}
}

func TestTrack_Reopened(t *testing.T) {
	prev := []*domain.Issue{closedIssue("rule:a", "f.go", "hash1", 10)}
	cur := []*domain.Issue{openIssue("rule:a", "f.go", "hash1", 10)}
	r := tracking.Track(cur, prev)
	if r.ReopenedCount() != 1 {
		t.Errorf("expected 1 reopened, got %d", r.ReopenedCount())
	}
	if r.NewCount() != 0 {
		t.Errorf("expected 0 new for reopened issue")
	}
}

func TestTrack_Mixed(t *testing.T) {
	prev := []*domain.Issue{
		openIssue("rule:a", "f.go", "hA", 1),   // will be unchanged
		openIssue("rule:b", "f.go", "hB", 2),   // will be closed
		closedIssue("rule:c", "f.go", "hC", 3), // will be reopened
	}
	cur := []*domain.Issue{
		openIssue("rule:a", "f.go", "hA", 1),  // unchanged
		openIssue("rule:c", "f.go", "hC", 3),  // reopened
		openIssue("rule:d", "f.go", "hD", 99), // new
	}
	r := tracking.Track(cur, prev)
	if len(r.Unchanged) != 1 {
		t.Errorf("unchanged: got %d", len(r.Unchanged))
	}
	if r.ClosedCount() != 1 {
		t.Errorf("closed: got %d", r.ClosedCount())
	}
	if r.ReopenedCount() != 1 {
		t.Errorf("reopened: got %d", r.ReopenedCount())
	}
	if r.NewCount() != 1 {
		t.Errorf("new: got %d", r.NewCount())
	}
}

func TestTrack_BothEmpty(t *testing.T) {
	r := tracking.Track(nil, nil)
	if r.TotalOpen() != 0 || r.ClosedCount() != 0 {
		t.Error("both empty → all zero counts")
	}
}

func TestTrack_FallbackToPathLine(t *testing.T) {
	// No line hash — fallback matching uses component_path + line
	prev := []*domain.Issue{openIssue("rule:a", "f.go", "", 10)}
	cur := []*domain.Issue{openIssue("rule:a", "f.go", "", 10)}
	r := tracking.Track(cur, prev)
	if len(r.Unchanged) != 1 {
		t.Errorf("expected 1 unchanged via fallback, got %d unchanged / %d new", len(r.Unchanged), r.NewCount())
	}
}

func TestTrack_SameRuleDifferentHash(t *testing.T) {
	prev := []*domain.Issue{
		openIssue("rule:a", "f.go", "h1", 1),
		openIssue("rule:a", "f.go", "h2", 2),
	}
	cur := []*domain.Issue{
		openIssue("rule:a", "f.go", "h1", 1),
		openIssue("rule:a", "f.go", "h2", 2),
	}
	r := tracking.Track(cur, prev)
	if len(r.Unchanged) != 2 {
		t.Errorf("expected 2 unchanged, got %d", len(r.Unchanged))
	}
}

func TestTrack_LargeSet(t *testing.T) {
	const n = 200
	prev := make([]*domain.Issue, n)
	cur := make([]*domain.Issue, n)
	for i := range prev {
		h := fmt.Sprintf("hash%d", i)
		prev[i] = openIssue("rule:x", "f.go", h, i)
		cur[i] = openIssue("rule:x", "f.go", h, i)
	}
	r := tracking.Track(cur, prev)
	if len(r.Unchanged) != n {
		t.Errorf("large set: expected %d unchanged, got %d", n, len(r.Unchanged))
	}
}

func TestTrack_ReopenedPreservesPreviousMetadata(t *testing.T) {
	prev := closedIssue("rule:a", "f.go", "hash1", 10)
	prev.Message = "original message"
	cur := openIssue("rule:a", "f.go", "hash1", 10)
	r := tracking.Track([]*domain.Issue{cur}, []*domain.Issue{prev})
	if r.ReopenedCount() != 1 {
		t.Fatal("expected 1 reopened")
	}
	if r.Reopened[0].Previous.Message != "original message" {
		t.Error("previous metadata should be preserved")
	}
}

func TestTrack_Concurrent(t *testing.T) {
	prev := []*domain.Issue{openIssue("rule:a", "f.go", "h1", 1)}
	cur := []*domain.Issue{openIssue("rule:a", "f.go", "h1", 1)}
	const n = 20
	results := make([]*tracking.TrackingResult, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx] = tracking.Track(cur, prev)
		}(i)
	}
	wg.Wait()
	for i, r := range results {
		if len(r.Unchanged) != 1 {
			t.Errorf("goroutine %d: expected 1 unchanged, got %d", i, len(r.Unchanged))
		}
	}
}
