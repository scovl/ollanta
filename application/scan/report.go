// Package scan assembles scan results into a structured Report and writes
// JSON and SARIF output files to the .ollanta/ directory under the project root.
package scan

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/scovl/ollanta/domain/model"
)

const Version = "0.1.0"

const (
	DefaultCodeSnapshotMaxFileBytes  = 128 * 1024
	DefaultCodeSnapshotMaxTotalBytes = 4 * 1024 * 1024
)

// Measures holds basic size metrics and issue type counts aggregated across all scanned files.
type Measures struct {
	Files           int            `json:"files"`
	Lines           int            `json:"lines"`
	Ncloc           int            `json:"ncloc"`
	Comments        int            `json:"comments"`
	Bugs            int            `json:"bugs"`
	CodeSmells      int            `json:"code_smells"`
	Vulnerabilities int            `json:"vulnerabilities"`
	Coverage        *float64       `json:"coverage,omitempty"`
	Tests           int            `json:"tests,omitempty"`
	TestFailures    int            `json:"test_failures,omitempty"`
	TestErrors      int            `json:"test_errors,omitempty"`
	TestSkipped     int            `json:"test_skipped,omitempty"`
	TestDurationMs  int64          `json:"test_duration_ms,omitempty"`
	MutationScore   *float64       `json:"mutation_score,omitempty"`
	MutantsTotal    int            `json:"mutants_total,omitempty"`
	MutantsKilled   int            `json:"mutants_killed,omitempty"`
	MutantsSurvived int            `json:"mutants_survived,omitempty"`
	MutantsTimeout  int            `json:"mutants_timeout,omitempty"`
	MutantsError    int            `json:"mutants_error,omitempty"`
	ByLang          map[string]int `json:"by_language"` // file count per language
}

// Metadata describes the scan run context.
type Metadata struct {
	ProjectKey      string `json:"project_key"`
	AnalysisDate    string `json:"analysis_date"` // RFC 3339
	Version         string `json:"version"`
	ElapsedMs       int64  `json:"elapsed_ms"`
	ScopeType       string `json:"scope_type,omitempty"`
	Branch          string `json:"branch,omitempty"`
	CommitSHA       string `json:"commit_sha,omitempty"`
	PullRequestKey  string `json:"pull_request_key,omitempty"`
	PullRequestBase string `json:"pull_request_base,omitempty"`
}

// Report is the complete output of a scan run.
type Report struct {
	Metadata     Metadata            `json:"metadata"`
	Measures     Measures            `json:"measures"`
	Issues       []*model.Issue      `json:"issues"`
	CodeSnapshot *model.CodeSnapshot `json:"code_snapshot,omitempty"`
}

// Build assembles a Report from the discovered files, analysis results, and elapsed time.
func Build(projectKey, projectDir string, files []DiscoveredFile, issues []*model.Issue, elapsed time.Duration, metadata Metadata) *Report {
	m := computeMeasures(files)
	enrichIssues(projectDir, files, issues, &m)
	if metadata.ProjectKey == "" {
		metadata.ProjectKey = projectKey
	}
	if metadata.AnalysisDate == "" {
		metadata.AnalysisDate = time.Now().UTC().Format(time.RFC3339)
	}
	if metadata.Version == "" {
		metadata.Version = Version
	}
	if metadata.ElapsedMs == 0 {
		metadata.ElapsedMs = elapsed.Milliseconds()
	}
	if metadata.ScopeType == "" {
		metadata.ScopeType = model.ScopeTypeBranch
	}
	return &Report{
		Metadata:     metadata,
		Measures:     m,
		Issues:       issues,
		CodeSnapshot: buildCodeSnapshot(projectDir, files),
	}
}

func enrichIssues(projectDir string, files []DiscoveredFile, issues []*model.Issue, measures *Measures) {
	languageByPath := buildLanguageLookup(projectDir, files)
	for _, issue := range issues {
		if issue == nil {
			continue
		}
		enrichIssue(issue, languageByPath)
		incrementIssueTypeCount(issue, measures)
	}
}

func buildLanguageLookup(projectDir string, files []DiscoveredFile) map[string]string {
	languageByPath := make(map[string]string, len(files))
	for _, file := range files {
		languageByPath[filepath.ToSlash(file.Path)] = file.Language
		if rel, err := filepath.Rel(projectDir, file.Path); err == nil {
			languageByPath[filepath.ToSlash(rel)] = file.Language
		}
	}
	return languageByPath
}

func enrichIssue(issue *model.Issue, languageByPath map[string]string) {
	if issue.Language == "" {
		issue.Language = languageByPath[filepath.ToSlash(issue.ComponentPath)]
		if issue.Language == "" {
			issue.Language = model.LanguageFromPath(issue.ComponentPath)
		}
	}
	if issue.QualityDomain == "" {
		issue.QualityDomain = model.DeriveIssueQualityDomain(issue.Type, issue.Tags)
	}
}

func incrementIssueTypeCount(issue *model.Issue, measures *Measures) {
	switch issue.Type {
	case model.TypeBug:
		measures.Bugs++
	case model.TypeCodeSmell:
		measures.CodeSmells++
	case model.TypeVulnerability:
		measures.Vulnerabilities++
	}
}

// SaveJSON writes the report as pretty-printed JSON to <baseDir>/.ollanta/report.json.
// Returns the path of the file written.
func (r *Report) SaveJSON(baseDir string) (string, error) {
	dir := filepath.Join(baseDir, ".ollanta")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create .ollanta dir: %w", err)
	}
	path := filepath.Join(dir, "report.json")
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return path, enc.Encode(r)
}

// computeMeasures reads each file to tally line counts and aggregates by language.
func computeMeasures(files []DiscoveredFile) Measures {
	m := Measures{
		Files:  len(files),
		ByLang: map[string]int{},
	}
	for _, f := range files {
		m.ByLang[f.Language]++
		total, ncloc, comments := countLines(f.Path)
		m.Lines += total
		m.Ncloc += ncloc
		m.Comments += comments
	}
	return m
}

func buildCodeSnapshot(baseDir string, files []DiscoveredFile) *model.CodeSnapshot {
	snapshot := &model.CodeSnapshot{
		Files:         make([]model.CodeSnapshotFile, 0, len(files)),
		TotalFiles:    len(files),
		MaxFileBytes:  DefaultCodeSnapshotMaxFileBytes,
		MaxTotalBytes: DefaultCodeSnapshotMaxTotalBytes,
	}

	for _, file := range files {
		path := file.Path
		if rel, err := filepath.Rel(baseDir, file.Path); err == nil {
			path = rel
		}
		path = filepath.ToSlash(path)

		entry := model.CodeSnapshotFile{
			Path:     path,
			Language: file.Language,
		}

		src, err := os.ReadFile(file.Path)
		if err != nil {
			entry.IsOmitted = true
			entry.OmittedReason = "read_error"
			snapshot.OmittedFiles++
			snapshot.Files = append(snapshot.Files, entry)
			continue
		}

		entry.SizeBytes = len(src)
		entry.LineCount = countContentLines(src)

		remaining := snapshot.MaxTotalBytes - snapshot.StoredBytes
		if remaining <= 0 {
			entry.IsOmitted = true
			entry.OmittedReason = "snapshot_limit"
			snapshot.OmittedFiles++
			snapshot.Files = append(snapshot.Files, entry)
			continue
		}

		limit := len(src)
		if limit > snapshot.MaxFileBytes {
			limit = snapshot.MaxFileBytes
			entry.IsTruncated = true
		}
		if limit > remaining {
			limit = remaining
			entry.IsTruncated = true
		}
		if limit <= 0 {
			entry.IsOmitted = true
			entry.OmittedReason = "snapshot_limit"
			snapshot.OmittedFiles++
			snapshot.Files = append(snapshot.Files, entry)
			continue
		}

		entry.Content = string(src[:limit])
		snapshot.StoredFiles++
		snapshot.StoredBytes += limit
		if entry.IsTruncated {
			snapshot.TruncatedFiles++
		}
		snapshot.Files = append(snapshot.Files, entry)
	}

	return snapshot
}

func countContentLines(src []byte) int {
	if len(src) == 0 {
		return 0
	}
	return bytes.Count(src, []byte{'\n'}) + 1
}

// countLines returns (total lines, ncloc, comment lines) for a file.
// Supports line comments (//, #) and block comments (/* ... */).
func countLines(path string) (total, ncloc, comments int) {
	f, err := os.Open(path)
	if err != nil {
		log.Printf("ollanta: cannot read %s for metrics: %v", path, err)
		return 0, 0, 0
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	inBlock := false
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		total++
		switch {
		case inBlock:
			comments++
			if strings.Contains(line, "*/") {
				inBlock = false
			}
		case strings.HasPrefix(line, "/*"):
			inBlock = true
			comments++
			if strings.Contains(line[2:], "*/") {
				inBlock = false
			}
		case strings.HasPrefix(line, "//"), strings.HasPrefix(line, "#"):
			comments++
		case line == "":
			// blank line — not counted in ncloc or comments
		default:
			ncloc++
		}
	}
	return
}
