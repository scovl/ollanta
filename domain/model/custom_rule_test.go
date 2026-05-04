package model

import "testing"

func TestHashCustomRuleDefinitionStable(t *testing.T) {
	rule := CustomRuleDefinition{
		RuleKey:         "custom:no-debug",
		Name:            "No Debug",
		Language:        LangGo,
		Type:            TypeCodeSmell,
		DefaultSeverity: SeverityMajor,
		Tags:            []string{"debug", "team"},
		Engine:          CustomRuleEngineText,
		EngineConfig:    map[string]string{"pattern": "debug"},
		Examples:        []CustomRuleExample{{Name: "bad", Code: "debug", Compliant: false}},
	}
	reordered := rule
	reordered.Tags = []string{"team", "debug"}

	if HashCustomRuleDefinition(rule) != HashCustomRuleDefinition(reordered) {
		t.Fatal("hash changed after normalizing equivalent rule content")
	}
}

func TestCanPublishCustomRuleRequiresCurrentValidationHash(t *testing.T) {
	rule := NormalizeCustomRuleDefinition(CustomRuleDefinition{
		RuleKey:         "custom:no-debug",
		Name:            "No Debug",
		Language:        LangGo,
		Type:            TypeCodeSmell,
		DefaultSeverity: SeverityMajor,
		Engine:          CustomRuleEngineText,
		EngineConfig:    map[string]string{"pattern": "debug"},
		Examples:        []CustomRuleExample{{Name: "bad", Code: "debug", Compliant: false}},
	})
	rule.ValidationStatus = CustomRuleValidationPassed
	rule.ValidationHash = HashCustomRuleDefinition(rule)
	rule.Lifecycle = CustomRuleValid
	if !CanPublishCustomRule(rule) {
		t.Fatal("validated rule should remain publishable after lifecycle transition")
	}

	rule.EngineConfig["pattern"] = "trace"
	if CanPublishCustomRule(rule) {
		t.Fatal("changed rule should require revalidation before publish")
	}
}

func TestNormalizeCustomRuleKey(t *testing.T) {
	if got := NormalizeCustomRuleKey("Team", "No-Debug"); got != "team:no-debug" {
		t.Fatalf("NormalizeCustomRuleKey() = %q, want team:no-debug", got)
	}
	if got := NormalizeCustomRuleKey("team", "custom:no-debug"); got != "custom:no-debug" {
		t.Fatalf("NormalizeCustomRuleKey() = %q, want existing namespace preserved", got)
	}
}
