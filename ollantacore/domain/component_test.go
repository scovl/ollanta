package domain_test

import (
	"sync"
	"testing"

	"github.com/scovl/ollanta/ollantacore/domain"
)

func TestComponent_DefaultValues(t *testing.T) {
	c := &domain.Component{
		Key:  "project:ollanta",
		Name: "ollanta",
		Type: domain.ComponentProject,
	}
	if c.Key != "project:ollanta" {
		t.Errorf("Key: got %q", c.Key)
	}
	if c.Language != "" {
		t.Errorf("expected empty Language for project node, got %q", c.Language)
	}
	if len(c.Children) != 0 {
		t.Errorf("expected no children initially, got %d", len(c.Children))
	}
}

func TestComponent_AddChild(t *testing.T) {
	parent := &domain.Component{Key: "pkg", Type: domain.ComponentPackage}
	child1 := &domain.Component{Key: "file1.go", Type: domain.ComponentFile, Language: "go"}
	child2 := &domain.Component{Key: "file2.go", Type: domain.ComponentFile, Language: "go"}
	parent.AddChild(child1)
	parent.AddChild(child2)

	if len(parent.Children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(parent.Children))
	}
	if parent.Children[0].Key != "file1.go" {
		t.Errorf("first child Key: got %q", parent.Children[0].Key)
	}
	if parent.Children[1].Key != "file2.go" {
		t.Errorf("second child Key: got %q", parent.Children[1].Key)
	}
}

func TestComponent_Walk_AllNodes(t *testing.T) {
	root := &domain.Component{Key: "project", Type: domain.ComponentProject}
	pkg := &domain.Component{Key: "pkg", Type: domain.ComponentPackage}
	file1 := &domain.Component{Key: "file1.go", Type: domain.ComponentFile}
	file2 := &domain.Component{Key: "file2.go", Type: domain.ComponentFile}
	pkg.AddChild(file1)
	pkg.AddChild(file2)
	root.AddChild(pkg)

	var visited []string
	root.Walk(func(c *domain.Component) bool {
		visited = append(visited, c.Key)
		return true
	})

	if len(visited) != 4 {
		t.Fatalf("expected 4 nodes visited, got %d: %v", len(visited), visited)
	}
	if visited[0] != "project" {
		t.Errorf("first visited should be root, got %q", visited[0])
	}
}

func TestComponent_Walk_StopsOnFalse(t *testing.T) {
	root := &domain.Component{Key: "project", Type: domain.ComponentProject}
	pkg := &domain.Component{Key: "pkg", Type: domain.ComponentPackage}
	file := &domain.Component{Key: "file.go", Type: domain.ComponentFile}
	pkg.AddChild(file)
	root.AddChild(pkg)

	var visited []string
	root.Walk(func(c *domain.Component) bool {
		visited = append(visited, c.Key)
		return c.Key != "pkg" // stop descending into pkg subtree
	})

	// project and pkg visited, but file.go under pkg is skipped
	if len(visited) != 2 {
		t.Fatalf("expected 2 nodes, got %d: %v", len(visited), visited)
	}
}

func TestComponent_FindByPath_Found(t *testing.T) {
	root := &domain.Component{Key: "project", Path: "/proj"}
	pkg := &domain.Component{Key: "pkg", Path: "/proj/pkg"}
	file := &domain.Component{Key: "file.go", Path: "/proj/pkg/file.go"}
	pkg.AddChild(file)
	root.AddChild(pkg)

	found := root.FindByPath("/proj/pkg/file.go")
	if found == nil {
		t.Fatal("expected to find component at /proj/pkg/file.go")
	}
	if found.Key != "file.go" {
		t.Errorf("Key: got %q", found.Key)
	}
}

func TestComponent_FindByPath_NotFound(t *testing.T) {
	root := &domain.Component{Key: "project", Path: "/proj"}
	if root.FindByPath("/nonexistent") != nil {
		t.Error("expected nil for nonexistent path")
	}
}

func TestComponent_MetricsInline(t *testing.T) {
	file := &domain.Component{
		Key:  "file.go",
		Type: domain.ComponentFile,
		Metrics: map[string]float64{
			"ncloc":      150,
			"complexity": 12,
		},
	}
	if file.Metrics["ncloc"] != 150 {
		t.Errorf("ncloc: got %f", file.Metrics["ncloc"])
	}
	if file.Metrics["complexity"] != 12 {
		t.Errorf("complexity: got %f", file.Metrics["complexity"])
	}
}

// T9: verify AddChild is safe when caller synchronizes externally.
func TestComponent_AddChild_WithExternalSync(t *testing.T) {
	parent := &domain.Component{Key: "root", Type: domain.ComponentProject}
	const n = 100
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mu.Lock()
			parent.AddChild(&domain.Component{Key: "child", Type: domain.ComponentFile})
			mu.Unlock()
		}()
	}
	wg.Wait()
	if len(parent.Children) != n {
		t.Errorf("expected %d children, got %d", n, len(parent.Children))
	}
}
