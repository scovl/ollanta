package api

import (
	"strings"
	"testing"

	"github.com/scovl/ollanta/domain/model"
	"github.com/scovl/ollanta/ollantastore/postgres"
)

func TestBuildOverviewSummaryReviewBlockedWithReasons(t *testing.T) {
	t.Parallel()

	review := buildOverviewSummaryReview(&overviewGate{
		Status: "ERROR",
		Conditions: []*postgres.GateCondition{
			{Metric: "new_vulnerabilities", Operator: "GT", Threshold: 0, OnNewCode: true},
			{Metric: "coverage", Operator: "LT", Threshold: 80},
		},
	}, map[string]float64{"coverage": 72}, map[string]float64{"new_vulnerabilities": 1})

	if review.Status != "blocked" {
		t.Fatalf("review status = %q, want blocked", review.Status)
	}
	if review.GateStatus != "ERROR" {
		t.Fatalf("gate status = %q, want ERROR", review.GateStatus)
	}
	if len(review.Reasons) != 2 {
		t.Fatalf("reason count = %d, want 2", len(review.Reasons))
	}
	if review.Reasons[0].Metric != "new_vulnerabilities" {
		t.Fatalf("first reason metric = %q, want new_vulnerabilities", review.Reasons[0].Metric)
	}
	if !strings.Contains(review.Headline, "Review blocked") {
		t.Fatalf("headline = %q, want blocked review wording", review.Headline)
	}
}

func TestBuildOverviewMustFixNowPrioritizesFailingGateCategory(t *testing.T) {
	t.Parallel()

	issues := []*postgres.IssueRow{
		{ID: 1, RuleKey: "go:nil-deref", Type: "bug", Severity: "blocker", TrackingState: string(model.IssueTrackingStateNew), ComponentPath: "internal/bug.go", Line: 41, Message: "nil dereference", Status: "open"},
		{ID: 2, RuleKey: "go:sql-injection", Type: "vulnerability", Severity: "major", TrackingState: string(model.IssueTrackingStateNew), ComponentPath: "internal/auth.go", Line: 12, Message: "sql injection", Status: "open"},
	}
	shortlist := buildOverviewMustFixNow(issues, []*overviewSummaryReason{{Metric: "new_vulnerabilities", Label: "New vulnerabilities"}})

	if len(shortlist) != 2 {
		t.Fatalf("shortlist length = %d, want 2", len(shortlist))
	}
	if shortlist[0].IssueID != 2 {
		t.Fatalf("first issue id = %d, want vulnerability prioritized", shortlist[0].IssueID)
	}
	if shortlist[0].WhySelected != "new issue failing quality gate" {
		t.Fatalf("why_selected = %q, want gate explanation", shortlist[0].WhySelected)
	}
}

func TestBuildOverviewMustFixNowPrioritizesFreshIssuesAheadOfUnchangedOnes(t *testing.T) {
	t.Parallel()

	issues := []*postgres.IssueRow{
		{ID: 1, RuleKey: "go:legacy-vuln", Type: "vulnerability", Severity: "blocker", TrackingState: string(model.IssueTrackingStateUnchanged), ComponentPath: "internal/legacy.go", Line: 20, Message: "legacy vulnerability", Status: "open"},
		{ID: 2, RuleKey: "go:new-bug", Type: "bug", Severity: "major", TrackingState: string(model.IssueTrackingStateNew), ComponentPath: "internal/new.go", Line: 10, Message: "new bug", Status: "open"},
	}

	shortlist := buildOverviewMustFixNow(issues, nil)
	if len(shortlist) != 2 {
		t.Fatalf("shortlist length = %d, want 2", len(shortlist))
	}
	if shortlist[0].IssueID != 2 {
		t.Fatalf("first issue id = %d, want fresh issue first", shortlist[0].IssueID)
	}
	if shortlist[0].WhySelected != "new issue in current scope" {
		t.Fatalf("why_selected = %q, want fresh issue explanation", shortlist[0].WhySelected)
	}
}

func TestBuildOverviewImpactedFilesUsesFacetRanking(t *testing.T) {
	t.Parallel()

	files := buildOverviewImpactedFiles(&postgres.IssueFacets{
		ByFile: map[string]int{
			"internal/a.go": 3,
			"internal/b.go": 1,
		},
	}, nil)

	if len(files) != 2 {
		t.Fatalf("impacted files length = %d, want 2", len(files))
	}
	if files[0].ComponentPath != "internal/a.go" {
		t.Fatalf("first impacted file = %q, want internal/a.go", files[0].ComponentPath)
	}
	if files[0].IssueCount != 3 {
		t.Fatalf("issue count = %d, want 3", files[0].IssueCount)
	}
}

func TestBuildEmptyOverviewSummaryProvidesStableContract(t *testing.T) {
	t.Parallel()

	summary := buildEmptyOverviewSummary()
	if summary.EmptyState == nil || summary.EmptyState.HasScans {
		t.Fatalf("empty state = %#v, want has_scans=false", summary.EmptyState)
	}
	if summary.NewCode == nil {
		t.Fatal("new_code should be present")
	}
	if summary.NewCode.Metrics == nil {
		t.Fatal("new_code.metrics should be initialized")
	}
	if len(summary.MustFixNow) != 0 {
		t.Fatalf("must_fix_now length = %d, want 0", len(summary.MustFixNow))
	}
	if len(summary.ImpactedFiles) != 0 {
		t.Fatalf("impacted_files length = %d, want 0", len(summary.ImpactedFiles))
	}
	if summary.OverallCode == nil || summary.OverallCode.Metrics == nil {
		t.Fatal("overall_code.metrics should be initialized")
	}
	if summary.Review == nil || summary.Review.Status != "empty" {
		t.Fatalf("review = %#v, want empty status", summary.Review)
	}
}

func TestFormatSummaryBaselineLabel(t *testing.T) {
	t.Parallel()

	label := formatSummaryBaselineLabel(&postgres.NewCodePeriod{Strategy: "reference_branch", Value: "main"})
	if label != "Reference branch: main" {
		t.Fatalf("baseline label = %q, want reference branch label", label)
	}
}
