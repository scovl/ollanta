package scan

import (
	"testing"

	coredomain "github.com/scovl/ollanta/ollantacore/domain"
	"github.com/scovl/ollanta/ollantacore/rulecatalog"
	"github.com/scovl/ollanta/ollantarules/defaults"
)

type registryRuleSnapshot struct {
	language string
	severity string
	params   map[string]string
}

func TestRuleCatalogMatchesDefaultRegistry(t *testing.T) {
	registryByKey := registryRuleSnapshots()
	catalogRules := rulecatalog.Rules()
	assertCatalogSize(t, catalogRules, registryByKey)
	assertCatalogRulesMatchRegistry(t, catalogRules, registryByKey)
}

func registryRuleSnapshots() map[string]registryRuleSnapshot {
	registryByKey := map[string]registryRuleSnapshot{}
	for _, rule := range defaults.NewRegistry().All() {
		params := map[string]string{}
		for _, param := range rule.Meta.Params {
			params[param.Key] = param.DefaultValue
		}
		registryByKey[rule.Meta.Key] = registryRuleSnapshot{language: rule.Meta.Language, severity: string(rule.Meta.DefaultSeverity), params: params}
	}
	return registryByKey
}

func assertCatalogSize(t *testing.T, catalogRules []*coredomain.Rule, registryByKey map[string]registryRuleSnapshot) {
	t.Helper()
	if len(registryByKey) != len(catalogRules) {
		t.Logf("catalog rule count = %d, registry rule count = %d", len(catalogRules), len(registryByKey))
	}
}

func assertCatalogRulesMatchRegistry(t *testing.T, catalogRules []*coredomain.Rule, registryByKey map[string]registryRuleSnapshot) {
	t.Helper()
	for _, rule := range catalogRules {
		registryRule, ok := registryByKey[rule.Key]
		if !ok {
			t.Fatalf("catalog rule %q is missing from default registry", rule.Key)
		}
		assertCatalogRuleMatchesRegistry(t, rule, registryRule)
	}
}

func assertCatalogRuleMatchesRegistry(t *testing.T, rule *coredomain.Rule, registryRule registryRuleSnapshot) {
	t.Helper()
	if rule.Language != registryRule.language {
		t.Fatalf("rule %q language = %q, want %q", rule.Key, rule.Language, registryRule.language)
	}
	if string(rule.DefaultSeverity) != registryRule.severity {
		t.Fatalf("rule %q severity = %q, want %q", rule.Key, rule.DefaultSeverity, registryRule.severity)
	}
	for key, param := range rule.ParamsSchema {
		if registryRule.params[key] != param.DefaultValue {
			t.Fatalf("rule %q param %q default = %q, want %q", rule.Key, key, param.DefaultValue, registryRule.params[key])
		}
	}
}
