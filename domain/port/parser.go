// Package port defines the inbound and outbound interfaces (Ports) of the domain layer.
package port

// IParser is the outbound port for source file parsing.
// The concrete implementation (tree-sitter + Go AST) lives in adapter/secondary/parser/.
// Returning `any` keeps the domain module free of CGo dependencies.
type IParser interface {
	// ParseFile parses the file at path and returns an opaque ParsedFile handle.
	// The language parameter is a canonical language identifier (e.g. "go").
	// Callers in adapter/ may type-assert to the concrete *parser.ParsedFile.
	ParseFile(path, language string) (any, error)
	// ParseSource parses src bytes as if they came from path.
	// Useful for in-memory analysis (e.g. pre-commit hooks).
	ParseSource(path string, src []byte, language string) (any, error)
}
