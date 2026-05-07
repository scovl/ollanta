// Package scan assembles scan results into a structured Report and writes
// JSON and SARIF output files to the .ollanta/ directory under the project root.
package scan

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/scovl/ollanta/domain/model"
	"github.com/scovl/ollanta/ollantacore/constants"
)

const Version = constants.Version

const (
	DefaultCodeSnapshotMaxFileBytes  = 128 * 1024
	DefaultCodeSnapshotMaxTotalBytes = 4 * 1024 * 1024
)

// Measures holds basic size metrics and issue type counts aggregated across all scanned files.
type Measures struct {
	Files                  int            `json:"files"`
	Lines                  int            `json:"lines"`
	Ncloc                  int            `json:"ncloc"`
	Comments               int            `json:"comments"`
	Bugs                   int            `json:"bugs"`
	CodeSmells             int            `json:"code_smells"`
	Vulnerabilities        int            `json:"vulnerabilities"`
	Coverage               *float64       `json:"coverage,omitempty"`
	Tests                  int            `json:"tests,omitempty"`
	TestFailures           int            `json:"test_failures,omitempty"`
	TestErrors             int            `json:"test_errors,omitempty"`
	TestSkipped            int            `json:"test_skipped,omitempty"`
	TestDurationMs         int64          `json:"test_duration_ms,omitempty"`
	MutationScore          *float64       `json:"mutation_score,omitempty"`
	MutantsTotal           int            `json:"mutants_total,omitempty"`
	MutantsKilled          int            `json:"mutants_killed,omitempty"`
	MutantsSurvived        int            `json:"mutants_survived,omitempty"`
	MutantsTimeout         int            `json:"mutants_timeout,omitempty"`
	MutantsSkipped         int            `json:"mutants_skipped,omitempty"`
	MutantsError           int            `json:"mutants_error,omitempty"`
	ChangedMutationScore   *float64       `json:"changed_mutation_score,omitempty"`
	ChangedMutantsTotal    int            `json:"changed_mutants_total,omitempty"`
	ChangedMutantsKilled   int            `json:"changed_mutants_killed,omitempty"`
	ChangedMutantsSurvived int            `json:"changed_mutants_survived,omitempty"`
	ByLang                 map[string]int `json:"by_language"` // file count per language
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

// ScannerOptions describes the scanner parameters used to produce a report.
type ScannerOptions struct {
	ConfigPath        string                   `json:"config_path,omitempty"`
	ProjectDir        string                   `json:"project_dir,omitempty"`
	Sources           []string                 `json:"sources,omitempty"`
	Exclusions        []string                 `json:"exclusions,omitempty"`
	ProjectKey        string                   `json:"project_key,omitempty"`
	Branch            string                   `json:"branch,omitempty"`
	CommitSHA         string                   `json:"commit_sha,omitempty"`
	PullRequestKey    string                   `json:"pull_request_key,omitempty"`
	PullRequestBranch string                   `json:"pull_request_branch,omitempty"`
	PullRequestBase   string                   `json:"pull_request_base,omitempty"`
	Format            string                   `json:"format,omitempty"`
	Debug             bool                     `json:"debug,omitempty"`
	LocalUI           bool                     `json:"local_ui,omitempty"`
	Port              int                      `json:"port,omitempty"`
	Bind              string                   `json:"bind,omitempty"`
	Server            string                   `json:"server,omitempty"`
	ServerWait        bool                     `json:"server_wait,omitempty"`
	WaitTimeout       string                   `json:"server_wait_timeout,omitempty"`
	WaitPoll          string                   `json:"server_wait_poll,omitempty"`
	Profiles          ScannerProfileOptions    `json:"profiles,omitempty"`
	CustomRules       ScannerCustomRuleOptions `json:"custom_rules,omitempty"`
	Tests             ScannerTestOptions       `json:"tests,omitempty"`
	Mutations         ScannerMutationOptions   `json:"mutations,omitempty"`
}

type ScannerCustomRuleOptions struct {
	CatalogHash string   `json:"catalog_hash,omitempty"`
	RuleCount   int      `json:"rule_count,omitempty"`
	Sources     []string `json:"sources,omitempty"`
}

// ScannerProfileOptions describes quality profile loading options without secrets.
type ScannerProfileOptions struct {
	Source       string `json:"source,omitempty"`
	FilePath     string `json:"file_path,omitempty"`
	Strict       bool   `json:"strict,omitempty"`
	FetchTimeout string `json:"fetch_timeout,omitempty"`
}

// ScannerTestOptions describes test-signal scanner parameters without secrets.
type ScannerTestOptions struct {
	Enabled                bool                       `json:"enabled,omitempty"`
	Mode                   string                     `json:"mode,omitempty"`
	Discover               bool                       `json:"discover,omitempty"`
	Run                    bool                       `json:"run,omitempty"`
	MaxRuntime             string                     `json:"max_runtime,omitempty"`
	FailOnTimeout          bool                       `json:"fail_on_timeout,omitempty"`
	MaxReportAge           string                     `json:"max_report_age,omitempty"`
	Exclusions             []string                   `json:"exclusions,omitempty"`
	MaxDepth               int                        `json:"max_depth,omitempty"`
	MaxCandidates          int                        `json:"max_candidates,omitempty"`
	MaxReportBytes         int64                      `json:"max_report_bytes,omitempty"`
	CommandPolicy          string                     `json:"command_policy,omitempty"`
	AllowExternalArtifacts bool                       `json:"allow_external_artifacts,omitempty"`
	PathMappings           []TestPathMapping          `json:"path_mappings,omitempty"`
	Modules                []ScannerTestModuleOptions `json:"modules,omitempty"`
}

// ScannerTestModuleOptions describes one configured test module.
type ScannerTestModuleOptions struct {
	Name                   string   `json:"name,omitempty"`
	Root                   string   `json:"root,omitempty"`
	Language               string   `json:"language,omitempty"`
	ArchitectureRole       string   `json:"architecture_role,omitempty"`
	TestPolicy             string   `json:"test_policy,omitempty"`
	IgnoreReason           string   `json:"ignore_reason,omitempty"`
	SuiteKind              string   `json:"suite_kind,omitempty"`
	EvidenceConfidence     string   `json:"evidence_confidence,omitempty"`
	Command                string   `json:"command,omitempty"`
	ArtifactRoot           string   `json:"artifact_root,omitempty"`
	ReportRoot             string   `json:"report_root,omitempty"`
	AllowExternalArtifacts *bool    `json:"allow_external_artifacts,omitempty"`
	CoverageReports        []string `json:"coverage_reports,omitempty"`
	TestReports            []string `json:"test_reports,omitempty"`
	MutationReports        []string `json:"mutation_reports,omitempty"`
	NativeReports          []string `json:"native_reports,omitempty"`
	CoverageThreshold      *float64 `json:"coverage_threshold,omitempty"`
	NewCoverageThreshold   *float64 `json:"new_coverage_threshold,omitempty"`
	MutationThreshold      *float64 `json:"mutation_threshold,omitempty"`
	Owner                  string   `json:"owner,omitempty"`
	Team                   string   `json:"team,omitempty"`
	IntegrationRequired    bool     `json:"integration_required,omitempty"`
}

// ScannerMutationOptions describes mutation-signal scanner parameters without secrets.
type ScannerMutationOptions struct {
	Enabled                bool                           `json:"enabled,omitempty"`
	Mode                   string                         `json:"mode,omitempty"`
	Discover               bool                           `json:"discover,omitempty"`
	Run                    bool                           `json:"run,omitempty"`
	ChangedOnly            bool                           `json:"changed_only,omitempty"`
	MaxRuntime             string                         `json:"max_runtime,omitempty"`
	MaxMutants             int                            `json:"max_mutants,omitempty"`
	Exclusions             []string                       `json:"exclusions,omitempty"`
	MaxReportAge           string                         `json:"max_report_age,omitempty"`
	MaxDepth               int                            `json:"max_depth,omitempty"`
	MaxCandidates          int                            `json:"max_candidates,omitempty"`
	MaxReportBytes         int64                          `json:"max_report_bytes,omitempty"`
	CommandPolicy          string                         `json:"command_policy,omitempty"`
	FailOnTimeout          bool                           `json:"fail_on_timeout,omitempty"`
	AllowExternalArtifacts bool                           `json:"allow_external_artifacts,omitempty"`
	PathMappings           []TestPathMapping              `json:"path_mappings,omitempty"`
	Modules                []ScannerMutationModuleOptions `json:"modules,omitempty"`
}

// ScannerMutationModuleOptions describes one configured mutation module.
type ScannerMutationModuleOptions struct {
	Name                   string            `json:"name,omitempty"`
	Root                   string            `json:"root,omitempty"`
	Language               string            `json:"language,omitempty"`
	ArchitectureRole       string            `json:"architecture_role,omitempty"`
	Tool                   string            `json:"tool,omitempty"`
	Command                string            `json:"command,omitempty"`
	SuiteKind              string            `json:"suite_kind,omitempty"`
	EvidenceConfidence     string            `json:"evidence_confidence,omitempty"`
	ArtifactRoot           string            `json:"artifact_root,omitempty"`
	ReportRoot             string            `json:"report_root,omitempty"`
	AllowExternalArtifacts *bool             `json:"allow_external_artifacts,omitempty"`
	ReportPaths            []string          `json:"report_paths,omitempty"`
	NativeReportPaths      []string          `json:"native_report_paths,omitempty"`
	PathMappings           []TestPathMapping `json:"path_mappings,omitempty"`
	Threshold              *float64          `json:"threshold,omitempty"`
	ChangedCodeThreshold   *float64          `json:"changed_code_threshold,omitempty"`
	Owner                  string            `json:"owner,omitempty"`
	Team                   string            `json:"team,omitempty"`
	MutationPolicy         string            `json:"mutation_policy,omitempty"`
	IgnoreReason           string            `json:"ignore_reason,omitempty"`
	ChangedOnly            *bool             `json:"changed_only,omitempty"`
	MaxRuntime             string            `json:"max_runtime,omitempty"`
	MaxMutants             int               `json:"max_mutants,omitempty"`
	Exclusions             []string          `json:"exclusions,omitempty"`
	FailOnTimeout          *bool             `json:"fail_on_timeout,omitempty"`
}

// Report is the complete output of a scan run.
type Report struct {
	Metadata        Metadata                `json:"metadata"`
	ScannerOptions  ScannerOptions          `json:"scanner_options,omitempty"`
	Measures        Measures                `json:"measures"`
	Issues          []*model.Issue          `json:"issues"`
	QualityProfiles []model.ProfileSnapshot `json:"quality_profiles,omitempty"`
	CodeSnapshot    *model.CodeSnapshot     `json:"code_snapshot,omitempty"`
	TestSignals     *TestSignalReport       `json:"test_signals,omitempty"`
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
		slog.Warn("cannot read file for metrics", "path", path, "error", err)
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
