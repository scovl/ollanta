package model

import "testing"

func TestDeriveIssueQualityDomain(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		issueType IssueType
		tags      []string
		want      IssueQualityDomain
	}{
		{name: "vulnerability", issueType: TypeVulnerability, want: QualitySecurity},
		{name: "security hotspot", issueType: TypeSecurityHotspot, want: QualitySecurity},
		{name: "bug", issueType: TypeBug, want: QualityReliability},
		{name: "code smell", issueType: TypeCodeSmell, want: QualityMaintainability},
		{name: "testability tag wins", issueType: TypeCodeSmell, tags: []string{"survived-mutant"}, want: QualityTestability},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := DeriveIssueQualityDomain(tt.issueType, tt.tags)
			if got != tt.want {
				t.Fatalf("quality = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSecurityCategories(t *testing.T) {
	t.Parallel()

	categories := SecurityCategories([]string{"OWASP-A03", "cwe-89", "complexity", "injection", "cwe-89"})
	want := []string{"owasp-a03", "cwe-89", "injection"}
	if len(categories) != len(want) {
		t.Fatalf("categories = %#v, want %#v", categories, want)
	}
	for i := range want {
		if categories[i] != want[i] {
			t.Fatalf("categories = %#v, want %#v", categories, want)
		}
	}
}

func TestLanguageFromPath(t *testing.T) {
	t.Parallel()

	if got := LanguageFromPath("src/app.tsx"); got != LangTypeScript {
		t.Fatalf("language = %q, want %q", got, LangTypeScript)
	}
	if got := LanguageFromPath("README"); got != LangUnknown {
		t.Fatalf("language = %q, want %q", got, LangUnknown)
	}
}
