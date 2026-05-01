package model

import "strings"

var testabilityTags = map[string]bool{
	"testability":      true,
	"unit-test":        true,
	"coverage-gap":     true,
	"mutation":         true,
	"survived-mutant":  true,
	"failing-test":     true,
	"flaky-test":       true,
	"mutation-testing": true,
}

var securityCategoryTags = map[string]bool{
	"injection": true,
	"auth":      true,
	"crypto":    true,
	"secrets":   true,
	"xss":       true,
	"csrf":      true,
	"ssrf":      true,
}

func DeriveIssueQualityDomain(issueType IssueType, tags []string) IssueQualityDomain {
	for _, tag := range tags {
		if testabilityTags[normalizeTag(tag)] {
			return QualityTestability
		}
	}

	switch issueType {
	case TypeVulnerability, TypeSecurityHotspot:
		return QualitySecurity
	case TypeBug:
		return QualityReliability
	default:
		return QualityMaintainability
	}
}

func SecurityCategories(tags []string) []string {
	seen := map[string]bool{}
	categories := make([]string, 0)
	for _, tag := range tags {
		normalized := normalizeTag(tag)
		if normalized == "" {
			continue
		}
		isSecurityTag := strings.HasPrefix(normalized, "owasp-") || strings.HasPrefix(normalized, "cwe-") || securityCategoryTags[normalized]
		if isSecurityTag && !seen[normalized] {
			seen[normalized] = true
			categories = append(categories, normalized)
		}
	}
	return categories
}

func LanguageFromPath(path string) string {
	lower := strings.ToLower(strings.ReplaceAll(path, "\\", "/"))
	for ext, language := range ExtensionToLanguage {
		if strings.HasSuffix(lower, ext) {
			return language
		}
	}
	return LangUnknown
}

func normalizeTag(tag string) string {
	return strings.ToLower(strings.TrimSpace(tag))
}
