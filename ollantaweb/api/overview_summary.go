package api

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/scovl/ollanta/domain/model"
	"github.com/scovl/ollanta/ollantastore/postgres"
)

const (
	summaryMustFixLimit       = 5
	summaryImpactedFilesLimit = 5
)

var summaryNewCodeMetricKeys = []string{
	"new_bugs",
	"new_vulnerabilities",
	"new_code_smells",
	"new_coverage",
	"new_duplications",
}

type overviewSummary struct {
	Review        *overviewSummaryReview     `json:"review,omitempty"`
	NewCode       *overviewSummaryNewCode    `json:"new_code,omitempty"`
	MustFixNow    []*overviewSummaryIssue    `json:"must_fix_now"`
	ImpactedFiles []*overviewSummaryFile     `json:"impacted_files"`
	Coverage      *overviewCoverageSummary   `json:"coverage,omitempty"`
	OverallCode   *overviewSummaryOverall    `json:"overall_code,omitempty"`
	EmptyState    *overviewSummaryEmptyState `json:"empty_state,omitempty"`
}

type overviewSummaryReview struct {
	Status     string                   `json:"status"`
	GateStatus string                   `json:"gate_status,omitempty"`
	Headline   string                   `json:"headline"`
	Reasons    []*overviewSummaryReason `json:"reasons,omitempty"`
}

type overviewSummaryReason struct {
	Metric    string  `json:"metric"`
	Label     string  `json:"label"`
	Actual    float64 `json:"actual"`
	Threshold float64 `json:"threshold"`
	Operator  string  `json:"operator"`
	OnNewCode bool    `json:"on_new_code,omitempty"`
}

type overviewSummaryNewCode struct {
	Baseline *overviewSummaryBaseline `json:"baseline,omitempty"`
	Metrics  map[string]float64       `json:"metrics"`
}

type overviewSummaryBaseline struct {
	Strategy string `json:"strategy"`
	Value    string `json:"value,omitempty"`
	Scope    string `json:"scope,omitempty"`
	Label    string `json:"label"`
}

type overviewSummaryIssue struct {
	IssueID       int64  `json:"issue_id"`
	RuleKey       string `json:"rule_key"`
	Type          string `json:"type"`
	Severity      string `json:"severity"`
	TrackingState string `json:"tracking_state,omitempty"`
	ComponentPath string `json:"component_path"`
	Line          int    `json:"line,omitempty"`
	Message       string `json:"message"`
	WhySelected   string `json:"why_selected,omitempty"`
}

type overviewSummaryFile struct {
	ComponentPath          string   `json:"component_path"`
	IssueCount             int      `json:"issue_count"`
	Coverage               *float64 `json:"coverage,omitempty"`
	DuplicatedLinesDensity *float64 `json:"duplicated_lines_density,omitempty"`
}

type overviewSummaryOverall struct {
	Metrics map[string]float64 `json:"metrics"`
}

type overviewCoverageSummary struct {
	Coverage     *float64                `json:"coverage,omitempty"`
	Files        []*overviewCoverageFile `json:"files"`
	FilesCovered int                     `json:"files_covered,omitempty"`
	FilesMissing int                     `json:"files_missing,omitempty"`
}

type overviewCoverageFile struct {
	ComponentPath        string  `json:"component_path"`
	Coverage             float64 `json:"coverage"`
	LinesToCover         int     `json:"lines_to_cover,omitempty"`
	CoveredLines         int     `json:"covered_lines,omitempty"`
	UncoveredLines       int     `json:"uncovered_lines,omitempty"`
	CoveredLineNumbers   []int   `json:"covered_line_numbers,omitempty"`
	UncoveredLineNumbers []int   `json:"uncovered_line_numbers,omitempty"`
}

type overviewSummaryEmptyState struct {
	HasScans          bool   `json:"has_scans"`
	Headline          string `json:"headline"`
	RecommendedAction string `json:"recommended_action,omitempty"`
}

func (h *OverviewHandler) loadOverviewSummary(ctx context.Context, resolved *resolvedProjectScope, scan *postgres.Scan, resp *overviewResponse) (*overviewSummary, error) {
	if scan == nil {
		return buildEmptyOverviewSummary(), nil
	}

	issues, err := h.loadSummaryIssues(ctx, scan)
	if err != nil {
		return nil, err
	}

	newMetrics, err := h.loadSummaryNewCodeMetrics(ctx, scan)
	if err != nil {
		return nil, err
	}

	baseline, err := h.loadSummaryBaseline(ctx, resolved, scan)
	if err != nil {
		return nil, err
	}

	summary := buildOverviewSummary(scan, resp.QualityGate, resp.Facets, resp.Measures, newMetrics, baseline, issues)
	if err := h.fillSummaryFileMetrics(ctx, scan.ID, summary.ImpactedFiles); err != nil {
		return nil, err
	}
	coverage, err := h.loadCoverageSummary(ctx, scan, resp.Measures)
	if err != nil {
		return nil, err
	}
	summary.Coverage = coverage
	return summary, nil
}

func (h *OverviewHandler) loadSummaryIssues(ctx context.Context, scan *postgres.Scan) ([]*postgres.IssueRow, error) {
	if h.issues == nil || scan == nil {
		return []*postgres.IssueRow{}, nil
	}
	projectID := scan.ProjectID
	scanID := scan.ID
	issues, _, err := h.issues.Query(ctx, postgres.IssueFilter{
		ProjectID: &projectID,
		ScanID:    &scanID,
		Limit:     1000,
	})
	if err != nil {
		return nil, err
	}
	return issues, nil
}

func (h *OverviewHandler) loadSummaryNewCodeMetrics(ctx context.Context, scan *postgres.Scan) (map[string]float64, error) {
	metrics := map[string]float64{
		"new_issues":    float64(scan.NewIssues),
		"closed_issues": float64(scan.ClosedIssues),
	}
	if h.measures == nil || scan == nil {
		return metrics, nil
	}
	for _, metricKey := range summaryNewCodeMetricKeys {
		measure, err := h.measures.GetForScan(ctx, scan.ID, metricKey)
		if err == nil && measure != nil {
			metrics[metricKey] = measure.Value
			continue
		}
		if err != nil && !isMissingMeasureErr(err) {
			return nil, err
		}
	}
	return metrics, nil
}

func (h *OverviewHandler) loadSummaryBaseline(ctx context.Context, resolved *resolvedProjectScope, scan *postgres.Scan) (*overviewSummaryBaseline, error) {
	if h.periods == nil || scan == nil {
		return nil, nil
	}
	branch := summaryScopeBranch(resolved, scan)
	period, err := h.periods.Resolve(ctx, scan.ProjectID, branch)
	if err != nil {
		if errors.Is(err, postgres.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &overviewSummaryBaseline{
		Strategy: period.Strategy,
		Value:    period.Value,
		Scope:    period.Scope,
		Label:    formatSummaryBaselineLabel(period),
	}, nil
}

func (h *OverviewHandler) fillSummaryFileMetrics(ctx context.Context, scanID int64, files []*overviewSummaryFile) error {
	if h.measures == nil {
		return nil
	}
	for _, file := range files {
		if file == nil || file.ComponentPath == "" {
			continue
		}
		coverage, err := h.measures.GetForScanComponent(ctx, scanID, "coverage", file.ComponentPath)
		if err == nil && coverage != nil {
			value := coverage.Value
			file.Coverage = &value
		} else if err != nil && !isMissingMeasureErr(err) {
			return err
		}

		dup, err := h.measures.GetForScanComponent(ctx, scanID, "duplicated_lines_density", file.ComponentPath)
		if err == nil && dup != nil {
			value := dup.Value
			file.DuplicatedLinesDensity = &value
		} else if err != nil && !isMissingMeasureErr(err) {
			return err
		}
	}
	return nil
}

func (h *OverviewHandler) loadCoverageSummary(ctx context.Context, scan *postgres.Scan, measures map[string]float64) (*overviewCoverageSummary, error) {
	if h.measures == nil || scan == nil {
		return nil, nil
	}
	coverageRows, err := h.measures.ListForScanMetric(ctx, scan.ID, model.MetricCoverage, 20)
	if err != nil {
		return nil, err
	}
	if len(coverageRows) == 0 {
		return nil, nil
	}
	lineDetails := h.loadCoverageLineDetails(ctx, scan.ID)
	coverageSummary := &overviewCoverageSummary{Files: make([]*overviewCoverageFile, 0, len(coverageRows))}
	if coverage, ok := measures[model.MetricCoverage]; ok {
		coverageSummary.Coverage = &coverage
	}
	for _, row := range coverageRows {
		file := &overviewCoverageFile{ComponentPath: row.ComponentPath, Coverage: row.Value}
		fillCoverageFileMetric(ctx, h.measures, scan.ID, file, model.MetricLinesToCover, &file.LinesToCover)
		fillCoverageFileMetric(ctx, h.measures, scan.ID, file, model.MetricCoveredLines, &file.CoveredLines)
		fillCoverageFileMetric(ctx, h.measures, scan.ID, file, model.MetricUncoveredLines, &file.UncoveredLines)
		if details, ok := lineDetails[file.ComponentPath]; ok {
			file.CoveredLineNumbers = details.CoveredLineNumbers
			file.UncoveredLineNumbers = details.UncoveredLineNumbers
		}
		if file.UncoveredLines > 0 {
			coverageSummary.FilesMissing++
		} else {
			coverageSummary.FilesCovered++
		}
		coverageSummary.Files = append(coverageSummary.Files, file)
	}
	return coverageSummary, nil
}

type coverageLineDetails struct {
	CoveredLineNumbers   []int
	UncoveredLineNumbers []int
}

func (h *OverviewHandler) loadCoverageLineDetails(ctx context.Context, scanID int64) map[string]coverageLineDetails {
	if h.scanJobs == nil {
		return nil
	}
	job, err := h.scanJobs.GetByScanID(ctx, scanID)
	if err != nil || job == nil || len(job.Payload) == 0 {
		return nil
	}
	var payload struct {
		TestSignals struct {
			Modules []struct {
				Files []struct {
					Path               string `json:"path"`
					CoveredLineNumbers []int  `json:"covered_line_numbers"`
					UncoveredLines     []int  `json:"uncovered_lines"`
				} `json:"files"`
			} `json:"modules"`
		} `json:"test_signals"`
	}
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return nil
	}
	details := map[string]coverageLineDetails{}
	for _, module := range payload.TestSignals.Modules {
		for _, file := range module.Files {
			if file.Path == "" {
				continue
			}
			details[file.Path] = coverageLineDetails{
				CoveredLineNumbers:   append([]int(nil), file.CoveredLineNumbers...),
				UncoveredLineNumbers: append([]int(nil), file.UncoveredLines...),
			}
		}
	}
	return details
}

func fillCoverageFileMetric(ctx context.Context, measures *postgres.MeasureRepository, scanID int64, file *overviewCoverageFile, metricKey string, target *int) {
	measure, err := measures.GetForScanComponent(ctx, scanID, metricKey, file.ComponentPath)
	if err == nil && measure != nil {
		*target = int(measure.Value)
	}
}

func buildOverviewSummary(
	scan *postgres.Scan,
	gate *overviewGate,
	facets *postgres.IssueFacets,
	totalMeasures map[string]float64,
	newMetrics map[string]float64,
	baseline *overviewSummaryBaseline,
	issues []*postgres.IssueRow,
) *overviewSummary {
	review := buildOverviewSummaryReview(gate, totalMeasures, newMetrics)
	return &overviewSummary{
		Review:        review,
		NewCode:       &overviewSummaryNewCode{Baseline: baseline, Metrics: cloneFloatMap(newMetrics)},
		MustFixNow:    buildOverviewMustFixNow(issues, review.Reasons),
		ImpactedFiles: buildOverviewImpactedFiles(facets, issues),
		OverallCode:   buildOverviewOverallCode(scan, totalMeasures, facets),
	}
}

func buildEmptyOverviewSummary() *overviewSummary {
	return &overviewSummary{
		Review: &overviewSummaryReview{
			Status:   "empty",
			Headline: "No completed scans for this scope yet",
		},
		NewCode:       &overviewSummaryNewCode{Metrics: map[string]float64{}},
		MustFixNow:    []*overviewSummaryIssue{},
		ImpactedFiles: []*overviewSummaryFile{},
		Coverage:      &overviewCoverageSummary{Files: []*overviewCoverageFile{}},
		OverallCode:   &overviewSummaryOverall{Metrics: map[string]float64{}},
		EmptyState: &overviewSummaryEmptyState{
			HasScans:          false,
			Headline:          "No completed scans for this scope yet",
			RecommendedAction: "Submit a scan to populate Review Summary for this scope",
		},
	}
}

func buildOverviewSummaryReview(gate *overviewGate, totalMeasures, newMeasures map[string]float64) *overviewSummaryReview {
	if gate == nil || gate.Status == "" || gate.Status == "NONE" {
		return &overviewSummaryReview{
			Status:     "needs_setup",
			GateStatus: "NONE",
			Headline:   "Latest scan available, but no quality gate is configured",
		}
	}

	reasons := collectOverviewSummaryReasons(gate.Conditions, totalMeasures, newMeasures)
	return &overviewSummaryReview{
		Status:     reviewStatusFromGate(gate.Status),
		GateStatus: gate.Status,
		Headline:   buildOverviewReviewHeadline(gate.Status, reasons),
		Reasons:    reasons,
	}
}

func collectOverviewSummaryReasons(conds []*postgres.GateCondition, totalMeasures, newMeasures map[string]float64) []*overviewSummaryReason {
	if len(conds) == 0 {
		return nil
	}
	reasons := make([]*overviewSummaryReason, 0, len(conds))
	for _, cond := range conds {
		if cond == nil {
			continue
		}
		measureSet := totalMeasures
		if cond.OnNewCode {
			measureSet = newMeasures
		}
		actual, ok := measureSet[cond.Metric]
		if !ok || !violatesOverviewCondition(cond.Operator, actual, cond.Threshold) {
			continue
		}
		reasons = append(reasons, &overviewSummaryReason{
			Metric:    cond.Metric,
			Label:     overviewMetricLabel(cond.Metric),
			Actual:    actual,
			Threshold: cond.Threshold,
			Operator:  cond.Operator,
			OnNewCode: cond.OnNewCode,
		})
	}
	return reasons
}

func reviewStatusFromGate(status string) string {
	switch status {
	case "OK":
		return "ready"
	case "WARN":
		return "attention"
	default:
		return "blocked"
	}
}

func buildOverviewReviewHeadline(status string, reasons []*overviewSummaryReason) string {
	switch reviewStatusFromGate(status) {
	case "ready":
		return "Review ready: no active quality gate conditions are failing"
	case "attention":
		if len(reasons) == 0 {
			return "Review needs attention from the active quality gate"
		}
		return fmt.Sprintf("Review needs attention because %s", summaryReasonHeadline(reasons[0]))
	default:
		if len(reasons) == 0 {
			return "Review blocked by the active quality gate"
		}
		if len(reasons) == 1 {
			return fmt.Sprintf("Review blocked by %s", summaryReasonHeadline(reasons[0]))
		}
		return fmt.Sprintf("Review blocked by %d failing quality gate conditions", len(reasons))
	}
}

func summaryReasonHeadline(reason *overviewSummaryReason) string {
	if reason == nil {
		return "the active quality gate"
	}
	switch reason.Metric {
	case "bugs":
		return "bugs in current code"
	case "vulnerabilities":
		return "vulnerabilities in current code"
	case "code_smells":
		return "code smells in current code"
	case "new_bugs":
		return "new bugs"
	case "new_vulnerabilities":
		return "new vulnerabilities"
	case "new_code_smells":
		return "new code smells"
	case "coverage":
		return "coverage below threshold"
	case "new_coverage":
		return "new coverage below threshold"
	case "duplicated_lines_density":
		return "duplication above threshold"
	case "new_duplications":
		return "new-code duplication above threshold"
	default:
		return strings.ToLower(reason.Label)
	}
}

func buildOverviewMustFixNow(issues []*postgres.IssueRow, reasons []*overviewSummaryReason) []*overviewSummaryIssue {
	active := activeSummaryIssues(issues)
	if len(active) == 0 {
		return []*overviewSummaryIssue{}
	}
	typePriority := overviewFailingIssueTypePriority(reasons)
	slices.SortFunc(active, func(left, right *postgres.IssueRow) int {
		if diff := cmp.Compare(overviewIssueBucket(left, typePriority), overviewIssueBucket(right, typePriority)); diff != 0 {
			return diff
		}
		if diff := cmp.Compare(overviewSeverityRank(left.Severity), overviewSeverityRank(right.Severity)); diff != 0 {
			return diff
		}
		if diff := cmp.Compare(overviewTypeRank(left.Type), overviewTypeRank(right.Type)); diff != 0 {
			return diff
		}
		if diff := strings.Compare(left.ComponentPath, right.ComponentPath); diff != 0 {
			return diff
		}
		if diff := cmp.Compare(left.Line, right.Line); diff != 0 {
			return diff
		}
		return cmp.Compare(left.ID, right.ID)
	})

	limit := minInt(summaryMustFixLimit, len(active))
	out := make([]*overviewSummaryIssue, 0, limit)
	for _, issue := range active[:limit] {
		out = append(out, &overviewSummaryIssue{
			IssueID:       issue.ID,
			RuleKey:       issue.RuleKey,
			Type:          issue.Type,
			Severity:      issue.Severity,
			TrackingState: issue.TrackingState,
			ComponentPath: issue.ComponentPath,
			Line:          issue.Line,
			Message:       issue.Message,
			WhySelected:   overviewWhySelected(issue, typePriority),
		})
	}
	return out
}

func buildOverviewImpactedFiles(facets *postgres.IssueFacets, issues []*postgres.IssueRow) []*overviewSummaryFile {
	type fileCount struct {
		path  string
		count int
	}
	rows := make([]fileCount, 0)
	if facets != nil && len(facets.ByFile) > 0 {
		for path, count := range facets.ByFile {
			rows = append(rows, fileCount{path: path, count: count})
		}
	} else {
		counts := make(map[string]int)
		for _, issue := range activeSummaryIssues(issues) {
			counts[issue.ComponentPath]++
		}
		for path, count := range counts {
			rows = append(rows, fileCount{path: path, count: count})
		}
	}

	slices.SortFunc(rows, func(left, right fileCount) int {
		if left.count != right.count {
			return cmp.Compare(right.count, left.count)
		}
		return strings.Compare(left.path, right.path)
	})

	limit := minInt(summaryImpactedFilesLimit, len(rows))
	out := make([]*overviewSummaryFile, 0, limit)
	for _, row := range rows[:limit] {
		out = append(out, &overviewSummaryFile{ComponentPath: row.path, IssueCount: row.count})
	}
	return out
}

func buildOverviewOverallCode(scan *postgres.Scan, totalMeasures map[string]float64, facets *postgres.IssueFacets) *overviewSummaryOverall {
	metrics := cloneFloatMap(totalMeasures)
	if metrics == nil {
		metrics = make(map[string]float64)
	}
	if scan != nil {
		metrics["total_issues"] = float64(scan.TotalIssues)
		metrics["bugs"] = float64(scan.TotalBugs)
		metrics["code_smells"] = float64(scan.TotalCodeSmells)
		metrics["vulnerabilities"] = float64(scan.TotalVulnerabilities)
		metrics["files"] = float64(scan.TotalFiles)
		metrics["lines"] = float64(scan.TotalLines)
		metrics["ncloc"] = float64(scan.TotalNcloc)
	}
	if facets != nil && facets.ByType != nil {
		if hotspots, ok := facets.ByType["security_hotspot"]; ok {
			metrics["security_hotspots"] = float64(hotspots)
		}
	}
	return &overviewSummaryOverall{Metrics: metrics}
}

func summaryScopeBranch(resolved *resolvedProjectScope, scan *postgres.Scan) string {
	if scan != nil && scan.Branch != "" {
		return scan.Branch
	}
	if resolved != nil && resolved.Scope.Branch != "" {
		return resolved.Scope.Branch
	}
	if resolved != nil {
		return resolved.DefaultBranch
	}
	return ""
}

func formatSummaryBaselineLabel(period *postgres.NewCodePeriod) string {
	if period == nil {
		return ""
	}
	switch period.Strategy {
	case "reference_branch":
		if period.Value != "" {
			return "Reference branch: " + period.Value
		}
		return "Reference branch"
	case "previous_version":
		return "Previous version"
	case "number_of_days":
		if period.Value != "" {
			return "Previous " + period.Value + " days"
		}
		return "Previous days"
	case "specific_analysis":
		if period.Value != "" {
			return "Specific analysis: " + period.Value
		}
		return "Specific analysis"
	default:
		return "Automatic baseline"
	}
}

func violatesOverviewCondition(operator string, actual, threshold float64) bool {
	switch strings.ToUpper(operator) {
	case "GT":
		return actual > threshold
	case "LT":
		return actual < threshold
	case "GTE":
		return actual >= threshold
	case "LTE":
		return actual <= threshold
	case "EQ":
		return actual == threshold
	case "NE":
		return actual != threshold
	default:
		return false
	}
}

func overviewMetricLabel(metric string) string {
	labels := map[string]string{
		"bugs":                     "Bugs",
		"vulnerabilities":          "Vulnerabilities",
		"code_smells":              "Code smells",
		"coverage":                 "Coverage",
		"duplicated_lines_density": "Duplication",
		"new_bugs":                 "New bugs",
		"new_vulnerabilities":      "New vulnerabilities",
		"new_code_smells":          "New code smells",
		"new_coverage":             "New coverage",
		"new_duplications":         "New duplication",
	}
	if label, ok := labels[metric]; ok {
		return label
	}
	return strings.ReplaceAll(metric, "_", " ")
}

func overviewFailingIssueTypePriority(reasons []*overviewSummaryReason) map[string]int {
	priority := make(map[string]int)
	for idx, reason := range reasons {
		issueType := overviewIssueTypeForMetric(reason.Metric)
		if issueType == "" {
			continue
		}
		if _, exists := priority[issueType]; !exists {
			priority[issueType] = idx
		}
	}
	return priority
}

func overviewReasonPriority(priority map[string]int, issueType string) int {
	if len(priority) == 0 {
		return 100
	}
	if rank, ok := priority[issueType]; ok {
		return rank
	}
	return 100
}

func overviewIssueBucket(issue *postgres.IssueRow, priority map[string]int) int {
	if issue == nil {
		return 4
	}
	isGateRelated := overviewReasonPriority(priority, issue.Type) < 100
	isFresh := issue.TrackingState == string(model.IssueTrackingStateNew) || issue.TrackingState == string(model.IssueTrackingStateReopened)
	switch {
	case isFresh && isGateRelated:
		return 0
	case isFresh:
		return 1
	case isGateRelated:
		return 2
	default:
		return 3
	}
}

func overviewWhySelected(issue *postgres.IssueRow, priority map[string]int) string {
	if issue == nil {
		return ""
	}
	isGateRelated := overviewReasonPriority(priority, issue.Type) < 100
	switch issue.TrackingState {
	case string(model.IssueTrackingStateNew):
		if isGateRelated {
			return "new issue failing quality gate"
		}
		return "new issue in current scope"
	case string(model.IssueTrackingStateReopened):
		if isGateRelated {
			return "reopened issue failing quality gate"
		}
		return "reopened issue in current scope"
	case string(model.IssueTrackingStateUnchanged):
		if isGateRelated {
			return "existing issue still failing quality gate"
		}
	}
	if isGateRelated {
		return "matches failing quality gate"
	}
	if overviewSeverityRank(issue.Severity) <= 1 {
		return "highest-severity issue in current scope"
	}
	return "current-scope issue prioritized for triage"
}

func overviewIssueTypeForMetric(metric string) string {
	switch metric {
	case "bugs", "new_bugs":
		return "bug"
	case "vulnerabilities", "new_vulnerabilities":
		return "vulnerability"
	case "code_smells", "new_code_smells":
		return "code_smell"
	case "security_hotspots", "new_security_hotspots":
		return "security_hotspot"
	default:
		return ""
	}
}

func overviewSeverityRank(severity string) int {
	switch severity {
	case "blocker":
		return 0
	case "critical":
		return 1
	case "major":
		return 2
	case "minor":
		return 3
	case "info":
		return 4
	default:
		return 5
	}
}

func overviewTypeRank(issueType string) int {
	switch issueType {
	case "vulnerability":
		return 0
	case "bug":
		return 1
	case "security_hotspot":
		return 2
	case "code_smell":
		return 3
	default:
		return 4
	}
}

func activeSummaryIssues(issues []*postgres.IssueRow) []*postgres.IssueRow {
	out := make([]*postgres.IssueRow, 0, len(issues))
	for _, issue := range issues {
		if issue == nil || issue.Status == "closed" {
			continue
		}
		out = append(out, issue)
	}
	return out
}

func cloneFloatMap(src map[string]float64) map[string]float64 {
	if src == nil {
		return map[string]float64{}
	}
	dst := make(map[string]float64, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func isMissingMeasureErr(err error) bool {
	return errors.Is(err, postgres.ErrNotFound) || errors.Is(err, pgx.ErrNoRows)
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}
