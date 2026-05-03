package rulecatalog

import "testing"

func TestCatalogCounts(t *testing.T) {
	if got := len(Rules()); got != 17 {
		t.Fatalf("Rules() count = %d, want 17", got)
	}

	wantCounts := map[string]int{"go": 8, "javascript": 4, "python": 5, "typescript": 0, "rust": 0}
	for _, language := range SupportedLanguages() {
		if got := len(ByLanguage(language.Key)); got != wantCounts[language.Key] {
			t.Fatalf("ByLanguage(%q) count = %d, want %d", language.Key, got, wantCounts[language.Key])
		}
		if language.ParserOnly && language.HasRules {
			t.Fatalf("language %q cannot be parser-only and have rules", language.Key)
		}
	}
}

func TestCatalogDefensiveCopies(t *testing.T) {
	rule, ok := ByKey("go:no-large-functions")
	if !ok {
		t.Fatal("go:no-large-functions not found")
	}
	rule.ParamsSchema["max_lines"] = param("max_lines", "changed", "1", "int")

	again, ok := ByKey("go:no-large-functions")
	if !ok {
		t.Fatal("go:no-large-functions not found on second read")
	}
	if got := again.ParamsSchema["max_lines"].DefaultValue; got != "40" {
		t.Fatalf("catalog mutation leaked, default = %q", got)
	}
}
