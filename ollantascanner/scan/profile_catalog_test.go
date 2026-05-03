package scan

import (
	"testing"

	"github.com/scovl/ollanta/ollantacore/rulecatalog"
	"github.com/scovl/ollanta/ollantarules/defaults"
)

func TestRuleCatalogMatchesDefaultRegistry(t *testing.T) {
	registryRules := defaults.NewRegistry().All()
	catalogRules := rulecatalog.Rules()
	if len(registryRules) != len(catalogRules) {
		t.Fatalf("catalog rule count = %d, registry rule count = %d", len(catalogRules), len(registryRules))
	}

	registryByKey := map[string]struct {
		language string
		severity string
		params   map[string]string
	}{}
	for _, rule := range registryRules {
		params := map[string]string{}
		for _, param := range rule.Meta.Params {
			params[param.Key] = param.DefaultValue
		}
		registryByKey[rule.Meta.Key] = struct {
			language string
			severity string
			params   map[string]string
		}{language: rule.Meta.Language, severity: string(rule.Meta.DefaultSeverity), params: params}
	}

	for _, rule := range catalogRules {
		registryRule, ok := registryByKey[rule.Key]
		if !ok {
			t.Fatalf("catalog rule %q is missing from default registry", rule.Key)
		}
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
}
