package model

import "strings"

const (
	ScopeTypeBranch      = "branch"
	ScopeTypePullRequest = "pull_request"
)

// AnalysisScope identifies the logical branch or pull request a scan belongs to.
type AnalysisScope struct {
	Type            string `json:"type"`
	Branch          string `json:"branch,omitempty"`
	PullRequestKey  string `json:"pull_request_key,omitempty"`
	PullRequestBase string `json:"pull_request_base,omitempty"`
}

// Normalize applies the default branch scope and trims user-provided values.
func (s AnalysisScope) Normalize() AnalysisScope {
	s.Type = strings.TrimSpace(s.Type)
	s.Branch = strings.TrimSpace(s.Branch)
	s.PullRequestKey = strings.TrimSpace(s.PullRequestKey)
	s.PullRequestBase = strings.TrimSpace(s.PullRequestBase)

	if s.Type == "" {
		if s.PullRequestKey != "" {
			s.Type = ScopeTypePullRequest
		} else {
			s.Type = ScopeTypeBranch
		}
	}

	if s.Type != ScopeTypePullRequest {
		s.Type = ScopeTypeBranch
	}

	return s
}

// Key returns the stable lookup key for the scope.
func (s AnalysisScope) Key() string {
	s = s.Normalize()
	if s.Type == ScopeTypePullRequest {
		return s.PullRequestKey
	}
	return s.Branch
}

// ScopeFromScan derives the logical scope from a persisted scan.
func ScopeFromScan(scan *Scan) AnalysisScope {
	if scan == nil {
		return AnalysisScope{Type: ScopeTypeBranch}
	}
	return AnalysisScope{
		Type:            scan.ScopeType,
		Branch:          scan.Branch,
		PullRequestKey:  scan.PullRequestKey,
		PullRequestBase: scan.PullRequestBase,
	}.Normalize()
}
