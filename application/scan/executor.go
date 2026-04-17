// Package scan provides a dependency-injected executor that dispatches
// discovered source files to IAnalyzer implementations in parallel.
package scan

import (
	"context"
	goast "go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"runtime"
	"sync"

	"github.com/scovl/ollanta/domain/model"
	"github.com/scovl/ollanta/domain/port"
)

// Executor routes files to matching analyzers in parallel using a bounded worker pool.
// It accepts a port.IParser for tree-sitter backed languages and a set of port.IAnalyzer
// implementations injected at construction time.
type Executor struct {
	parser    port.IParser
	analyzers []port.IAnalyzer
	workers   int
}

// NewExecutor creates an Executor backed by the provided parser and analyzers.
// Worker count defaults to runtime.NumCPU()*2, minimum 1.
func NewExecutor(p port.IParser, analyzers []port.IAnalyzer) *Executor {
	w := runtime.NumCPU() * 2
	if w < 1 {
		w = 1
	}
	return &Executor{parser: p, analyzers: analyzers, workers: w}
}

// Run analyses all files in parallel and returns the aggregated issues.
// Individual file failures are isolated: a crash or parse error produces an
// empty result for that file rather than aborting the whole run.
func (e *Executor) Run(ctx context.Context, files []DiscoveredFile) ([]*model.Issue, error) {
	if len(files) == 0 {
		return []*model.Issue{}, nil
	}

	type result struct{ issues []*model.Issue }

	sem := make(chan struct{}, e.workers)
	out := make(chan result, len(files))

	var wg sync.WaitGroup
	for _, f := range files {
		f := f
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			// Fault isolation: recover from any panic in analyser code.
			defer func() {
				if r := recover(); r != nil {
					log.Printf("ollanta: panic analyzing %s: %v", f.Path, r)
					out <- result{}
				}
			}()

			select {
			case <-ctx.Done():
				out <- result{}
				return
			default:
			}

			src, err := os.ReadFile(f.Path)
			if err != nil {
				out <- result{}
				return
			}

			// Parse via the injected IParser (tree-sitter / language server).
			var parsedFile any
			if e.parser != nil {
				parsedFile, _ = e.parser.ParseSource(f.Path, src, f.Language)
			}

			// For Go files, also produce a stdlib AST for rules that prefer it.
			var goFile *goast.File
			var goFset *token.FileSet
			if f.Language == model.LangGo {
				fset := token.NewFileSet()
				af, err := parser.ParseFile(fset, f.Path, src, parser.AllErrors)
				if err == nil {
					goFile = af
					goFset = fset
				}
			}

			ac := port.AnalysisContext{
				Path:       f.Path,
				Language:   f.Language,
				Source:     src,
				ParsedFile: parsedFile,
				GoFile:     goFile,
				GoFileSet:  goFset,
			}

			var issues []*model.Issue
			for _, a := range e.analyzers {
				if a.Language() != f.Language && a.Language() != "*" {
					continue
				}
				if err := a.Check(ctx, ac, &issues); err != nil {
					log.Printf("ollanta: analyser %s error on %s: %v", a.Key(), f.Path, err)
				}
			}
			out <- result{issues: issues}
		}()
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	var all []*model.Issue
	for r := range out {
		all = append(all, r.issues...)
	}
	return all, nil
}
