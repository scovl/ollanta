// Package treesitter provides the TreeSitterSensor, which dispatches source files
// for all non-Go languages through registered Analyzer rules using the
// ollantaparser (tree-sitter) front-end.
package treesitter

import (
	"fmt"

	"github.com/scovl/ollanta/ollantacore/domain"
	"github.com/scovl/ollanta/ollantaparser"
	ollantarules "github.com/scovl/ollanta/ollantarules"
)

// TreeSitterSensor parses source files with ollantaparser (tree-sitter) and
// dispatches them to Analyzer rules matching the file's language.
type TreeSitterSensor struct {
	registry       *ollantarules.Registry
	parserRegistry *ollantaparser.LanguageRegistry
	queryRunner    *ollantaparser.QueryRunner
}

// NewTreeSitterSensor creates a TreeSitterSensor backed by the given registries.
func NewTreeSitterSensor(registry *ollantarules.Registry, parserRegistry *ollantaparser.LanguageRegistry) *TreeSitterSensor {
	return &TreeSitterSensor{
		registry:       registry,
		parserRegistry: parserRegistry,
		queryRunner:    ollantaparser.NewQueryRunner(),
	}
}

// SupportedLanguages returns the list of languages this sensor can handle,
// derived from the grammars registered in the parser registry.
func (s *TreeSitterSensor) SupportedLanguages() []string {
	langs := []string{"javascript", "typescript", "python", "rust"}
	return langs
}

// Analyze parses source in the given language and runs all matching rules in parallel.
// activeRules limits which rules are executed; nil means all rules are active.
func (s *TreeSitterSensor) Analyze(path string, source []byte, lang string, activeRules map[string]bool) ([]*domain.Issue, error) {
	grammar, ok := s.parserRegistry.ForName(lang)
	if !ok {
		return nil, fmt.Errorf("treesittersensor: no grammar registered for language %q", lang)
	}

	pf, err := ollantaparser.Parse(path, source, grammar)
	if err != nil {
		return nil, fmt.Errorf("treesittersensor: parse %s: %w", path, err)
	}
	defer pf.Close()

	analyzers := s.registry.FindByLanguage(lang)

	var issues []*domain.Issue

	for _, a := range analyzers {
		if activeRules != nil && !activeRules[a.Key()] {
			continue
		}
		func(a ollantarules.Analyzer) {
			defer func() {
				if r := recover(); r != nil {
					_ = r // isolate panicking rules so one bad rule doesn't abort the sensor
				}
			}()
			ctx := &ollantarules.AnalysisContext{
				Path:       path,
				Source:     source,
				Language:   lang,
				Params:     map[string]string{},
				ParsedFile: pf,
				Query:      s.queryRunner,
				Grammar:    grammar,
			}
			found := a.Check(ctx)
			issues = append(issues, found...)
		}(a)
	}
	return issues, nil
}
