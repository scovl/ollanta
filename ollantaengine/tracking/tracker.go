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
		result.Closed = filterByStatus(previous, isOpenOrConfirmed)
		return result
	}

	openIdx, closedIdx := buildIndices(previous)
	matchedOpenKeys := matchCurrent(current, openIdx, closedIdx, result)
	result.Closed = findUnmatchedOpen(previous, matchedOpenKeys)

	return result
}

func isOpenOrConfirmed(s domain.Status) bool {
	return s == domain.StatusOpen || s == domain.StatusConfirmed
}

func filterByStatus(issues []*domain.Issue, predicate func(domain.Status) bool) []*domain.Issue {
	out := make([]*domain.Issue, 0, len(issues))
	for _, iss := range issues {
		if predicate(iss.Status) {
			out = append(out, iss)
		}
	}
	return out
}

func buildIndices(previous []*domain.Issue) (open, closed map[string]*domain.Issue) {
	open = make(map[string]*domain.Issue, len(previous))
	closed = make(map[string]*domain.Issue)
	for _, p := range previous {
		key := matchKey(p)
		switch p.Status {
		case domain.StatusOpen, domain.StatusConfirmed:
			open[key] = p
		case domain.StatusClosed, domain.StatusReopened:
			closed[key] = p
		}
	}
	return open, closed
}

func matchCurrent(current []*domain.Issue, openIdx, closedIdx map[string]*domain.Issue, result *TrackingResult) map[string]bool {
	matched := make(map[string]bool, len(current))
	for _, cur := range current {
		key := matchKey(cur)
		if prev, ok := openIdx[key]; ok {
			result.Unchanged = append(result.Unchanged, IssuePair{Current: cur, Previous: prev})
			matched[key] = true
			continue
		}
		if prev, ok := closedIdx[key]; ok {
			result.Reopened = append(result.Reopened, IssuePair{Current: cur, Previous: prev})
			continue
		}
		result.New = append(result.New, cur)
	}
	return matched
}

func findUnmatchedOpen(previous []*domain.Issue, matchedOpenKeys map[string]bool) []*domain.Issue {
	out := make([]*domain.Issue, 0, len(previous))
	for _, p := range previous {
		if isOpenOrConfirmed(p.Status) && !matchedOpenKeys[matchKey(p)] {
			out = append(out, p)
		}
	}
	return out
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
