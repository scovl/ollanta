package customrules

import (
	"strings"
	"testing"

	"github.com/scovl/ollanta/domain/model"
)

func TestDecodeDocumentRejectsUnknownField(t *testing.T) {
	_, err := DecodeDocument([]byte(`{"version":1,"pack":{"name":"Team"},"rules":[],"script":"bad"}`))
	if err == nil {
		t.Fatal("DecodeDocument() error = nil, want unknown field error")
	}
}

func TestValidateDocumentRejectsBundledKeyAndUnsupportedVersion(t *testing.T) {
	doc, result := ValidateDocument(model.CustomRulePackDocument{
		Version: 99,
		Pack:    model.CustomRulePack{Name: "Team", Namespace: "team"},
		Rules: []model.CustomRuleDefinition{{
			RuleKey:         "go:no-large-functions",
			Name:            "Collision",
			Language:        model.LangGo,
			Type:            model.TypeCodeSmell,
			DefaultSeverity: model.SeverityMajor,
			Engine:          model.CustomRuleEngineText,
			EngineConfig:    map[string]string{"pattern": "debug"},
			Examples:        []model.CustomRuleExample{{Name: "bad", Code: "debug", Compliant: false}},
		}},
	}, ValidationContext{})

	if doc.Version != 99 {
		t.Fatalf("doc version changed unexpectedly: %d", doc.Version)
	}
	if result.Status != model.CustomRuleValidationFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !containsDiagnostic(result.Diagnostics, "unsupported_schema_version") || !containsDiagnostic(result.Diagnostics, "bundled_key_collision") {
		t.Fatalf("diagnostics = %+v, want version and key collision", result.Diagnostics)
	}
}

func TestValidateDefinitionEvaluatesExamples(t *testing.T) {
	rule := model.CustomRuleDefinition{
		RuleKey:         "team:no-debug",
		Name:            "No Debug",
		Language:        model.LangGo,
		Type:            model.TypeCodeSmell,
		DefaultSeverity: model.SeverityMajor,
		Engine:          model.CustomRuleEngineText,
		EngineConfig:    map[string]string{"pattern": "debug"},
		Examples: []model.CustomRuleExample{
			{Name: "bad", Code: "debug()", Compliant: false},
			{Name: "good", Code: "trace()", Compliant: true},
		},
	}
	result := ValidateDefinition(rule, ValidationContext{})
	if result.Status != model.CustomRuleValidationPassed {
		t.Fatalf("status = %q diagnostics=%+v, want passed", result.Status, result.Diagnostics)
	}
}

func TestValidateTreeSitterRequiresRuntime(t *testing.T) {
	rule := model.CustomRuleDefinition{
		RuleKey:         "team:query",
		Name:            "Query",
		Language:        model.LangGo,
		Type:            model.TypeCodeSmell,
		DefaultSeverity: model.SeverityMajor,
		Engine:          model.CustomRuleEngineTreeSitter,
		EngineConfig:    map[string]string{"query": "(identifier) @id"},
		Examples:        []model.CustomRuleExample{{Name: "bad", Code: "package main", Compliant: false}},
	}
	result := ValidateDefinition(rule, ValidationContext{})
	if result.Status != model.CustomRuleValidationRequiresRuntime {
		t.Fatalf("status = %q diagnostics=%+v, want requires_runtime", result.Status, result.Diagnostics)
	}
}

func TestGoASTForbiddenCall(t *testing.T) {
	rule := model.CustomRuleDefinition{
		RuleKey:         "team:no-println",
		Name:            "No Println",
		Language:        model.LangGo,
		Type:            model.TypeCodeSmell,
		DefaultSeverity: model.SeverityMajor,
		Engine:          model.CustomRuleEngineGoAST,
		EngineConfig:    map[string]string{"pattern": "forbidden_call", "target": "fmt.Println"},
	}
	matches, diagnostics := Evaluate(rule, "main.go", []byte("package main\nfunc main(){ fmt.Println(1) }"))
	if len(diagnostics) != 0 || len(matches) != 1 {
		t.Fatalf("matches=%+v diagnostics=%+v, want one match", matches, diagnostics)
	}
}

func containsDiagnostic(diagnostics []model.CustomRuleDiagnostic, code string) bool {
	for _, diagnostic := range diagnostics {
		if strings.Contains(diagnostic.Code, code) || diagnostic.Code == code {
			return true
		}
	}
	return false
}
