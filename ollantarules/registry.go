package ollantarules

import "github.com/scovl/ollanta/ollantacore/domain"

// Registry holds the set of registered Analyzer rules and provides lookup methods.
type Registry struct {
	analyzers []Analyzer
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{}
}

// Register adds an Analyzer to the registry.
func (r *Registry) Register(a Analyzer) {
	r.analyzers = append(r.analyzers, a)
}

// All returns all registered analyzers.
func (r *Registry) All() []Analyzer {
	out := make([]Analyzer, len(r.analyzers))
	copy(out, r.analyzers)
	return out
}

// FindByKey returns the analyzer with the given key, or nil if not found.
func (r *Registry) FindByKey(key string) Analyzer {
	for _, a := range r.analyzers {
		if a.Key() == key {
			return a
		}
	}
	return nil
}

// FindByLanguage returns all analyzers targeting the given language,
// including cross-language rules (Language == "*").
func (r *Registry) FindByLanguage(lang string) []Analyzer {
	var out []Analyzer
	for _, a := range r.analyzers {
		if a.Language() == lang || a.Language() == "*" {
			out = append(out, a)
		}
	}
	return out
}

// Rules converts all registered analyzers to domain.Rule structs.
func (r *Registry) Rules() []*domain.Rule {
	rules := make([]*domain.Rule, len(r.analyzers))
	for i, a := range r.analyzers {
		schema := make(map[string]domain.ParamDef, len(a.Params()))
		for _, p := range a.Params() {
			schema[p.Key] = p
		}
		rules[i] = &domain.Rule{
			Key:             a.Key(),
			Name:            a.Name(),
			Description:     a.Description(),
			Language:        a.Language(),
			Type:            a.Type(),
			DefaultSeverity: a.DefaultSeverity(),
			Tags:            a.Tags(),
			ParamsSchema:    schema,
		}
	}
	return rules
}

// DefaultRegistry returns a Registry pre-loaded with all built-in rules.
// Prefer using defaults.NewRegistry() from the defaults sub-package instead,
// which includes all language-specific rule implementations.
func DefaultRegistry() *Registry {
	return NewRegistry()
}
