// Package ollantarules defines the Analyzer interface — the extension point for all
// static analysis rules in Ollanta. Rules are dispatched by the GoSensor (go/ast)
// or the TreeSitterSensor (tree-sitter) depending on the language of the file
// being analysed.
package ollantarules

import (
	"go/ast"
	"go/token"

	"github.com/scovl/ollanta/ollantacore/domain"
	"github.com/scovl/ollanta/ollantaparser"
)

// Analyzer is the plugin interface that every analysis rule must satisfy.
// Implementations are registered in a Registry and dispatched by a sensor.
type Analyzer interface {
	// Key returns the unique rule identifier, e.g. "go:no-large-functions".
	Key() string
	// Name returns a human-readable rule name.
	Name() string
	// Description explains what the rule detects and why it matters.
	Description() string
	// Language returns the target language, e.g. "go", "javascript", or "*" for cross-language.
	Language() string
	// Type returns the issue category produced by this rule.
	Type() domain.IssueType
	// DefaultSeverity returns the default severity for issues found by this rule.
	DefaultSeverity() domain.Severity
	// Tags returns categorisation tags.
	Tags() []string
	// Params returns the list of configurable parameters with defaults.
	Params() []domain.ParamDef
	// Check runs the rule against the given context and returns any issues found.
	Check(ctx *AnalysisContext) []*domain.Issue
}

// AnalysisContext carries the per-file context passed to each Analyzer.Check call.
// Exactly one of AST or ParsedFile will be non-nil, depending on which sensor
// is invoking the rule.
type AnalysisContext struct {
	// Path is the relative path of the file being analysed.
	Path string
	// Source is the raw file content.
	Source []byte
	// Language is the canonical language identifier, e.g. "go", "javascript".
	Language string
	// Params holds the configured parameter values for this rule invocation.
	Params map[string]string

	// AST and FileSet are populated by GoSensor (Language == "go" only).
	AST     *ast.File
	FileSet *token.FileSet

	// ParsedFile and Query are populated by TreeSitterSensor (all non-Go languages).
	ParsedFile *ollantaparser.ParsedFile
	Query      *ollantaparser.QueryRunner
	// Grammar is the tree-sitter Language grammar for the file being analysed.
	// Required by Query.Run to compile S-expression queries.
	Grammar ollantaparser.Language
}
