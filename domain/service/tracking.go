// Package service contains pure domain logic with zero external dependencies.
// Issue tracking is inspired by the genealogy/clone tracking from OpenStaticAnalyzer
// and the "leak period" concept from SonarQube.
package service

import (
	"fmt"

	"github.com/scovl/ollanta/domain/model"
)

// IssuePair links a current issue to its matched previous counterpart.
type IssuePair struct {
	Current  *model.Issue `json:"current"`
	Previous *model.Issue `json:"previous"`
}

// TrackingResult holds the categorised outcome of a Track() call.
type TrackingResult struct {
	New       []*model.Issue `json:"new"`
	Unchanged []IssuePair    `json:"unchanged"`
	Closed    []*model.Issue `json:"closed"`
	Reopened  []IssuePair    `json:"reopened"`
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
func Track(current, previous []*model.Issue) *TrackingResult {
	result := &TrackingResult{}

	if len(previous) == 0 {
		result.New = append(result.New, current...)
		return result
	}
	if len(current) == 0 {
		result.Closed = closeAllOpen(previous)
		return result
	}

	openIdx, closedIdx := buildIssueIndex(previous)
	matchedOpenKeys := map[string]bool{}

	for _, cur := range current {
		key := matchKey(cur)

		if prev, ok := openIdx[key]; ok {
			result.Unchanged = append(result.Unchanged, IssuePair{Current: cur, Previous: prev})
			matchedOpenKeys[key] = true
			continue
		}

		if prev, ok := closedIdx[key]; ok {
			result.Reopened = append(result.Reopened, IssuePair{Current: cur, Previous: prev})
			continue
		}

		result.New = append(result.New, cur)
	}

	result.Closed = unmatchedOpenIssues(previous, matchedOpenKeys)
	return result
}

func closeAllOpen(previous []*model.Issue) []*model.Issue {
	var closed []*model.Issue
	for _, p := range previous {
		if p.Status == model.StatusOpen || p.Status == model.StatusConfirmed {
			closed = append(closed, p)
		}
	}
	return closed
}

func buildIssueIndex(previous []*model.Issue) (openIdx, closedIdx map[string]*model.Issue) {
	openIdx = map[string]*model.Issue{}
	closedIdx = map[string]*model.Issue{}
	for _, p := range previous {
		key := matchKey(p)
		switch p.Status {
		case model.StatusOpen, model.StatusConfirmed:
			openIdx[key] = p
		case model.StatusClosed, model.StatusReopened:
			closedIdx[key] = p
		}
	}
	return openIdx, closedIdx
}

func unmatchedOpenIssues(previous []*model.Issue, matchedKeys map[string]bool) []*model.Issue {
	var closed []*model.Issue
	for _, p := range previous {
		if (p.Status == model.StatusOpen || p.Status == model.StatusConfirmed) &&
			!matchedKeys[matchKey(p)] {
			closed = append(closed, p)
		}
	}
	return closed
}

// matchKey builds the matching key for an issue.
// If LineHash is set, uses {rule_key, line_hash} for location-stable matching.
// Falls back to {rule_key, component_path, line} when LineHash is empty.
func matchKey(iss *model.Issue) string {
	if iss.LineHash != "" {
		return fmt.Sprintf("%s\x00%s", iss.RuleKey, iss.LineHash)
	}
	return fmt.Sprintf("%s\x00%s\x00%d", iss.RuleKey, iss.ComponentPath, iss.Line)
}
