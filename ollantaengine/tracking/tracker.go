// Package tracking implements the 2-tier issue matching algorithm that tracks issue
// identity across successive scans. Inspired by the genealogy/clone tracking from
// OpenStaticAnalyzer and the "leak period" concept from SonarQube.
package tracking

import (
	"fmt"

	"github.com/scovl/ollanta/ollantacore/domain"
)

// IssuePair links a current issue to its matched previous counterpart.
type IssuePair struct {
	Current  *domain.Issue `json:"current"`
	Previous *domain.Issue `json:"previous"`
}

// TrackingResult holds the categorised outcome of a Track() call.
type TrackingResult struct {
	New       []*domain.Issue `json:"new"`
	Unchanged []IssuePair     `json:"unchanged"`
	Closed    []*domain.Issue `json:"closed"`
	Reopened  []IssuePair     `json:"reopened"`
}

// NewCount returns the number of genuinely new issues.
func (r *TrackingResult) NewCount() int { return len(r.New) }

// ClosedCount returns the number of issues no longer present.
func (r *TrackingResult) ClosedCount() int { return len(r.Closed) }

// ReopenedCount returns the number of previously closed issues that reappeared.
func (r *TrackingResult) ReopenedCount() int { return len(r.Reopened) }

// TotalOpen returns the number of open issues (new + unchanged + reopened).
func (r *TrackingResult) TotalOpen() int {
	return len(r.New) + len(r.Unchanged) + len(r.Reopened)
}

// Track performs the 2-tier matching algorithm:
//
//  1. Match current issues against previously OPEN issues by {rule_key, line_hash}.
//  2. Remaining unmatched current issues are tried against previously CLOSED issues.
//     If found → REOPENED; otherwise → NEW.
//
// Previous open issues not matched by any current issue → CLOSED.
func Track(current, previous []*domain.Issue) *TrackingResult {
	result := &TrackingResult{}

	if len(previous) == 0 {
		result.New = append(result.New, current...)
		return result
	}
	if len(current) == 0 {
		for _, p := range previous {
			if p.Status == domain.StatusOpen || p.Status == domain.StatusConfirmed {
				result.Closed = append(result.Closed, p)
			}
		}
		return result
	}

	// Build tier-1 index: open/confirmed previous issues.
	openIdx := map[string]*domain.Issue{}
	for _, p := range previous {
		if p.Status == domain.StatusOpen || p.Status == domain.StatusConfirmed {
			openIdx[matchKey(p)] = p
		}
	}

	// Build tier-2 index: closed/resolved previous issues.
	closedIdx := map[string]*domain.Issue{}
	for _, p := range previous {
		if p.Status == domain.StatusClosed || p.Status == domain.StatusReopened {
			closedIdx[matchKey(p)] = p
		}
	}

	matchedOpenKeys := map[string]bool{}

	for _, cur := range current {
		key := matchKey(cur)

		// Tier 1: open match
		if prev, ok := openIdx[key]; ok {
			result.Unchanged = append(result.Unchanged, IssuePair{Current: cur, Previous: prev})
			matchedOpenKeys[key] = true
			continue
		}

		// Tier 2: closed match → reopened
		if prev, ok := closedIdx[key]; ok {
			result.Reopened = append(result.Reopened, IssuePair{Current: cur, Previous: prev})
			continue
		}

		// No match → new
		result.New = append(result.New, cur)
	}

	// Unmatched open issues → closed
	for _, p := range previous {
		if (p.Status == domain.StatusOpen || p.Status == domain.StatusConfirmed) &&
			!matchedOpenKeys[matchKey(p)] {
			result.Closed = append(result.Closed, p)
		}
	}

	return result
}

// matchKey builds the matching key for an issue.
// If LineHash is set, uses {rule_key, line_hash} for location-stable matching.
// Falls back to {rule_key, component_path, line} when LineHash is empty.
func matchKey(iss *domain.Issue) string {
	if iss.LineHash != "" {
		return fmt.Sprintf("%s\x00%s", iss.RuleKey, iss.LineHash)
	}
	return fmt.Sprintf("%s\x00%s\x00%d", iss.RuleKey, iss.ComponentPath, iss.Line)
}
