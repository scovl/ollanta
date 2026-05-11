package model

import "time"

// Scan is the canonical scan record.
type Scan struct {
	ID                   int64       `json:"id"`
	ProjectID            int64       `json:"project_id"`
	Version              string      `json:"version"`
	ScopeType            string      `json:"scope_type"`
	Branch               string      `json:"branch"`
	CommitSHA            string      `json:"commit_sha"`
	PullRequestKey       string      `json:"pull_request_key"`
	PullRequestBase      string      `json:"pull_request_base"`
	Status               string      `json:"status"`
	ElapsedMs            int64       `json:"elapsed_ms"`
	GateStatus           string      `json:"gate_status"`
	GateResult           *GateResult `json:"gate_result,omitempty"`
	AnalysisDate         time.Time   `json:"analysis_date"`
	CreatedAt            time.Time   `json:"created_at"`
	TotalFiles           int         `json:"total_files"`
	TotalLines           int         `json:"total_lines"`
	TotalNcloc           int         `json:"total_ncloc"`
	TotalComments        int         `json:"total_comments"`
	TotalIssues          int         `json:"total_issues"`
	TotalBugs            int         `json:"total_bugs"`
	TotalCodeSmells      int         `json:"total_code_smells"`
	TotalVulnerabilities int         `json:"total_vulnerabilities"`
	NewIssues            int         `json:"new_issues"`
	ClosedIssues         int         `json:"closed_issues"`
}
