package postgres

import (
	"strings"
	"testing"

	"github.com/scovl/ollanta/domain/model"
)

func TestBuildIssueFilter_CombinedQualityTagAndDirectoryFilters(t *testing.T) {
	t.Parallel()

	projectID := int64(1)
	quality := string(model.QualityTestability)
	tag := "survived-mutant"
	directory := "internal/service"
	language := model.LangGo
	filter := IssueFilter{
		ProjectID:     &projectID,
		QualityDomain: &quality,
		Language:      &language,
		Tag:           &tag,
		Directory:     &directory,
	}

	conditions, args := buildIssueFilter(filter)
	joined := strings.Join(conditions, " AND ")

	if !strings.Contains(joined, "project_id = $1") {
		t.Fatalf("conditions = %v, missing project filter", conditions)
	}
	if !strings.Contains(joined, "tags && $2::text[]") {
		t.Fatalf("conditions = %v, missing testability quality filter", conditions)
	}
	if !strings.Contains(joined, "LOWER(REPLACE(component_path") || !strings.Contains(joined, "%.go") {
		t.Fatalf("conditions = %v, missing language filter", conditions)
	}
	if !strings.Contains(joined, "$3 = ANY(tags)") {
		t.Fatalf("conditions = %v, missing tag filter", conditions)
	}
	if !strings.Contains(joined, "component_path = $4") || !strings.Contains(joined, "component_path LIKE $5") {
		t.Fatalf("conditions = %v, missing directory filter", conditions)
	}
	if len(args) != 5 {
		t.Fatalf("args = %v, want 5 args", args)
	}
	if args[2] != tag || args[3] != directory || args[4] != directory+"/%" {
		t.Fatalf("args = %v, unexpected tag/directory args", args)
	}
}

func TestEnrichIssueRow_DerivesFacetFields(t *testing.T) {
	t.Parallel()

	issue := &IssueRow{
		Type:          string(model.TypeVulnerability),
		ComponentPath: "src/auth.ts",
		Tags:          []string{"owasp-a01", "auth"},
	}
	enrichIssueRow(issue)

	if issue.QualityDomain != string(model.QualitySecurity) {
		t.Fatalf("quality_domain = %q, want security", issue.QualityDomain)
	}
	if issue.Language != model.LangTypeScript {
		t.Fatalf("language = %q, want typescript", issue.Language)
	}
	if categories := model.SecurityCategories(issue.Tags); len(categories) != 2 {
		t.Fatalf("security categories = %v, want 2", categories)
	}
}
