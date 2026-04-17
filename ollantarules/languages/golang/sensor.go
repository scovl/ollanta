// Package golang provides the GoSensor, which dispatches Go source files
// through registered Analyzer rules using the go/ast front-end.
package golang

import (
	"fmt"
	"go/parser"
	"go/token"
	"sync"

	"github.com/scovl/ollanta/ollantacore/constants"
	"github.com/scovl/ollanta/ollantacore/domain"
	ollantarules "github.com/scovl/ollanta/ollantarules"
)

// GoSensor parses Go source files with go/parser and dispatches them to all
// registered Analyzer rules whose Language() == "go".
type GoSensor struct {
	registry *ollantarules.Registry
}

// NewGoSensor creates a GoSensor backed by the given rule registry.
func NewGoSensor(registry *ollantarules.Registry) *GoSensor {
	return &GoSensor{registry: registry}
}

// Language returns the language this sensor handles.
func (s *GoSensor) Language() string { return constants.LangGo }

// Analyze parses source and runs all applicable rules in parallel.
// activeRules limits which rules are executed; nil means all rules are active.
func (s *GoSensor) Analyze(path string, source []byte, activeRules map[string]bool) ([]*domain.Issue, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, source, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("gosensor: parse %s: %w", path, err)
	}

	analyzers := s.registry.FindByLanguage(constants.LangGo)

	var (
		mu     sync.Mutex
		issues []*domain.Issue
		wg     sync.WaitGroup
	)

	for _, a := range analyzers {
		if activeRules != nil && !activeRules[a.Key()] {
			continue
		}
		a := a
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					_ = r // isolate panicking rules so one bad rule doesn't abort the sensor
				}
			}()
			ctx := &ollantarules.AnalysisContext{
				Path:     path,
				Source:   source,
				Language: constants.LangGo,
				Params:   map[string]string{},
				AST:      file,
				FileSet:  fset,
			}
			found := a.Check(ctx)
			if len(found) > 0 {
				mu.Lock()
				issues = append(issues, found...)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return issues, nil
}
