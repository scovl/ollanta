package ollantarules_test

import (
	"testing"

	"github.com/scovl/ollanta/ollantarules"
	"github.com/scovl/ollanta/ollantarules/defaults"
)

func TestRegistry_All(t *testing.T) {
	r := defaults.NewRegistry()
	all := r.All()
	if len(all) != 17 {
		t.Errorf("expected 17 rules, got %d", len(all))
	}
}

func TestRegistry_FindByKey_Found(t *testing.T) {
	r := defaults.NewRegistry()
	a := r.FindByKey("go:no-large-functions")
	if a == nil {
		t.Fatal("expected to find go:no-large-functions")
	}
	if a.Key() != "go:no-large-functions" {
		t.Errorf("Key: got %q", a.Key())
	}
}

func TestRegistry_FindByKey_NotFound(t *testing.T) {
	r := defaults.NewRegistry()
	if r.FindByKey("nonexistent") != nil {
		t.Error("expected nil for unknown key")
	}
}

func TestRegistry_FindByLanguage_Go(t *testing.T) {
	r := defaults.NewRegistry()
	goRules := r.FindByLanguage("go")
	if len(goRules) != 8 {
		t.Errorf("expected 8 Go rules, got %d", len(goRules))
	}
}

func TestRegistry_FindByLanguage_JS(t *testing.T) {
	r := defaults.NewRegistry()
	jsRules := r.FindByLanguage("javascript")
	if len(jsRules) != 4 {
		t.Errorf("expected 4 JS rules, got %d", len(jsRules))
	}
}

func TestRegistry_Rules_Metadata(t *testing.T) {
	r := defaults.NewRegistry()
	rules := r.Rules()
	if len(rules) != 17 {
		t.Errorf("expected 17 domain rules, got %d", len(rules))
	}
	for _, rule := range rules {
		if rule.Key == "" {
			t.Error("rule has empty Key")
		}
		if rule.Name == "" {
			t.Error("rule has empty Name")
		}
		if rule.Language == "" {
			t.Error("rule has empty Language")
		}
	}
}

func TestRegistry_Empty(t *testing.T) {
	r := ollantarules.NewRegistry()
	if len(r.All()) != 0 {
		t.Error("new registry should be empty")
	}
}
