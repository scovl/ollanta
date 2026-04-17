package summarizer_test

import (
	"testing"

	"github.com/scovl/ollanta/ollantacore/domain"
	"github.com/scovl/ollanta/ollantaengine/summarizer"
)

// ── helpers ────────────────────────────────────────────────────────────────

func fileNode(path string, metrics map[string]float64) *domain.Component {
	return &domain.Component{
		Key:     path,
		Path:    path,
		Type:    domain.ComponentFile,
		Metrics: metrics,
	}
}

func pkgNode(name string, children ...*domain.Component) *domain.Component {
	c := &domain.Component{Key: name, Name: name, Type: domain.ComponentPackage}
	for _, ch := range children {
		c.AddChild(ch)
	}
	return c
}

func projectNode(children ...*domain.Component) *domain.Component {
	c := &domain.Component{Key: "root", Name: "root", Type: domain.ComponentProject}
	for _, ch := range children {
		c.AddChild(ch)
	}
	return c
}

// ── CumSum ────────────────────────────────────────────────────────────────

func TestCumSum_SimpleLeafToRoot(t *testing.T) {
	root := projectNode(
		fileNode("a.go", map[string]float64{"lines": 100}),
		fileNode("b.go", map[string]float64{"lines": 50}),
	)
	summarizer.CumSum(root, []string{"lines"})
	if root.Metrics["lines"] != 150 {
		t.Errorf("root lines: got %v, want 150", root.Metrics["lines"])
	}
}

func TestCumSum_ThreeLevels(t *testing.T) {
	root := projectNode(
		pkgNode("pkg_a",
			fileNode("pkg_a/f1.go", map[string]float64{"lines": 100, "ncloc": 80}),
			fileNode("pkg_a/f2.go", map[string]float64{"lines": 50, "ncloc": 40}),
		),
		pkgNode("pkg_b",
			fileNode("pkg_b/f3.go", map[string]float64{"lines": 200, "ncloc": 160}),
		),
	)
	summarizer.CumSum(root, []string{"lines", "ncloc"})

	pkgA := root.Children[0]
	if pkgA.Metrics["lines"] != 150 {
		t.Errorf("pkg_a lines: got %v, want 150", pkgA.Metrics["lines"])
	}
	if pkgA.Metrics["ncloc"] != 120 {
		t.Errorf("pkg_a ncloc: got %v, want 120", pkgA.Metrics["ncloc"])
	}
	if root.Metrics["lines"] != 350 {
		t.Errorf("root lines: got %v, want 350", root.Metrics["lines"])
	}
	if root.Metrics["ncloc"] != 280 {
		t.Errorf("root ncloc: got %v, want 280", root.Metrics["ncloc"])
	}
}

func TestCumSum_LeafValuesUnchanged(t *testing.T) {
	leaf := fileNode("f.go", map[string]float64{"lines": 42})
	root := projectNode(leaf)
	summarizer.CumSum(root, []string{"lines"})
	if leaf.Metrics["lines"] != 42 {
		t.Errorf("leaf lines should not change, got %v", leaf.Metrics["lines"])
	}
}

func TestCumSum_SingleNode(t *testing.T) {
	root := &domain.Component{
		Key:     "root",
		Type:    domain.ComponentProject,
		Metrics: map[string]float64{"lines": 100},
	}
	summarizer.CumSum(root, []string{"lines"})
	// No children — value unchanged
	if root.Metrics["lines"] != 100 {
		t.Errorf("single node: lines should be 100, got %v", root.Metrics["lines"])
	}
}

func TestCumSum_MultipleMetrics(t *testing.T) {
	root := projectNode(
		fileNode("a.go", map[string]float64{"lines": 10, "ncloc": 8, "complexity": 3}),
		fileNode("b.go", map[string]float64{"lines": 20, "ncloc": 16, "complexity": 5}),
	)
	summarizer.CumSum(root, []string{"lines", "ncloc", "complexity"})
	if root.Metrics["lines"] != 30 {
		t.Errorf("lines: got %v", root.Metrics["lines"])
	}
	if root.Metrics["ncloc"] != 24 {
		t.Errorf("ncloc: got %v", root.Metrics["ncloc"])
	}
	if root.Metrics["complexity"] != 8 {
		t.Errorf("complexity: got %v", root.Metrics["complexity"])
	}
}

// ── CumAvg ────────────────────────────────────────────────────────────────

func TestCumAvg_Correct(t *testing.T) {
	root := projectNode(
		fileNode("a.go", map[string]float64{"complexity": 6, "functions": 2}),
		fileNode("b.go", map[string]float64{"complexity": 9, "functions": 3}),
	)
	summarizer.CumSum(root, []string{"complexity", "functions"})
	summarizer.CumAvg(root, "complexity", "functions", "avg_complexity")

	want := 15.0 / 5.0 // (6+9) / (2+3)
	if root.Metrics["avg_complexity"] != want {
		t.Errorf("avg_complexity: got %v, want %v", root.Metrics["avg_complexity"], want)
	}
}

func TestCumSum_NilRoot(_ *testing.T) {
	// Should not panic.
	summarizer.CumSum(nil, []string{"ncloc"})
}

func TestCumAvg_NilRoot(_ *testing.T) {
	// Should not panic.
	summarizer.CumAvg(nil, "total", "count", "avg")
}

func TestCumAvg_ZeroCount(_ *testing.T) {
	root := projectNode(
		pkgNode("pkg",
			fileNode("a.go", map[string]float64{"total": 10, "count": 0}),
		),
	)
	// Should not panic or produce NaN/Inf.
	summarizer.CumAvg(root, "total", "count", "avg")
}
