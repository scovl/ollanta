// Package executor dispatches discovered source files to the appropriate sensor
// in parallel and aggregates the resulting issues.
package executor

import (
	"context"
	"log"
	"os"
	"runtime"
	"sync"

	"github.com/scovl/ollanta/ollantacore/domain"
	gosensor "github.com/scovl/ollanta/ollantarules/languages/golang"
	tssensor "github.com/scovl/ollanta/ollantarules/languages/treesitter"
	"github.com/scovl/ollanta/ollantascanner/discovery"
)

// Executor routes files to GoSensor or TreeSitterSensor based on language and
// runs them concurrently with a bounded worker pool.
type Executor struct {
	goSensor *gosensor.GoSensor
	tsSensor *tssensor.TreeSitterSensor
	workers  int
}

// New creates an Executor backed by the provided sensors.
// Worker count defaults to runtime.NumCPU()*2, minimum 1.
func New(goSensor *gosensor.GoSensor, ts *tssensor.TreeSitterSensor) *Executor {
	w := runtime.NumCPU() * 2
	if w < 1 {
		w = 1
	}
	return &Executor{goSensor: goSensor, tsSensor: ts, workers: w}
}

// Run analyzes all files in parallel and returns the aggregated issues.
// Individual file failures are isolated: a crash or parse error produces an
// empty result for that file rather than aborting the whole run.
func (e *Executor) Run(ctx context.Context, files []discovery.DiscoveredFile) ([]*domain.Issue, error) {
	if len(files) == 0 {
		return []*domain.Issue{}, nil
	}

	type result struct{ issues []*domain.Issue }

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

			// Fault isolation: recover from any panic in sensor code.
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

			var issues []*domain.Issue
			var analyzeErr error
			if f.Language == "go" {
				issues, analyzeErr = e.goSensor.Analyze(f.Path, src, nil)
			} else {
				issues, analyzeErr = e.tsSensor.Analyze(f.Path, src, f.Language, nil)
			}
			if analyzeErr != nil {
				log.Printf("ollanta: error analyzing %s: %v", f.Path, analyzeErr)
			}
			out <- result{issues: issues}
		}()
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	var all []*domain.Issue
	for r := range out {
		all = append(all, r.issues...)
	}
	return all, nil
}
