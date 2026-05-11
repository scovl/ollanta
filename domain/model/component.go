// Package model defines the language-agnostic data model used across all Ollanta modules.
package model

// ComponentType represents the level of a Component in the project hierarchy.
type ComponentType string

const (
	// ComponentProject is the root node of the hierarchy.
	ComponentProject ComponentType = "project"
	// ComponentModule groups packages within a monorepo or multi-module project.
	ComponentModule ComponentType = "module"
	// ComponentPackage groups files sharing the same namespace or directory.
	ComponentPackage ComponentType = "package"
	// ComponentFile is a single source file leaf node.
	ComponentFile ComponentType = "file"
)

// Component is a node in the project's hierarchical representation
// (Project → Module → Package → File). Inspired by the LIM Scope node from
// OpenStaticAnalyzer, each Component carries inline Metrics to avoid re-traversal.
// The Language field identifies the source language of file-level components and is
// used by the Executor to route each file to the correct sensor.
type Component struct {
	Key  string        `json:"key"`
	Name string        `json:"name"`
	Type ComponentType `json:"type"`
	// Language holds the canonical language identifier ("go", "javascript", "python", etc.)
	// for file-level components. Empty for project/module/package nodes.
	Language string       `json:"language,omitempty"`
	Path     string       `json:"path,omitempty"`
	Lines    int          `json:"lines,omitempty"`
	Children []*Component `json:"children,omitempty"`
	// Metrics stores pre-computed metric values keyed by metric key (e.g. "ncloc", "complexity").
	// Follows the LIM pattern where Scope nodes carry metrics directly.
	Metrics map[string]float64 `json:"metrics,omitempty"`
}

// AddChild appends a child Component to this node's Children slice.
func (c *Component) AddChild(child *Component) {
	c.Children = append(c.Children, child)
}

// Walk performs a depth-first traversal of the Component tree, calling fn for each node.
// Traversal stops descending into a subtree when fn returns false.
func (c *Component) Walk(fn func(*Component) bool) {
	if !fn(c) {
		return
	}
	for _, child := range c.Children {
		child.Walk(fn)
	}
}

// FindByPath returns the first Component whose Path matches the given value, or nil if not found.
func (c *Component) FindByPath(path string) *Component {
	if c.Path == path {
		return c
	}
	for _, child := range c.Children {
		if found := child.FindByPath(path); found != nil {
			return found
		}
	}
	return nil
}
