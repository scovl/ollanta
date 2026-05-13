package languages

import (
	"github.com/scovl/ollanta/adapter/secondary/parser"
	"github.com/scovl/ollanta/ollantaparser"
	javascript "github.com/smacker/go-tree-sitter/javascript"
	python "github.com/smacker/go-tree-sitter/python"
	rust "github.com/smacker/go-tree-sitter/rust"
	typescript "github.com/smacker/go-tree-sitter/typescript/typescript"
)

var JavaScript = ollantaparser.NewLanguage("javascript", []string{".js", ".mjs"}, javascript.GetLanguage())

var Python = ollantaparser.NewLanguage("python", []string{".py"}, python.GetLanguage())

var TypeScript = ollantaparser.NewLanguage("typescript", []string{".ts", ".tsx"}, typescript.GetLanguage())

var Rust = ollantaparser.NewLanguage("rust", []string{".rs"}, rust.GetLanguage())

func DefaultRegistry() *parser.LanguageRegistry {
	r := parser.NewRegistry()
	r.Register(JavaScript)
	r.Register(Python)
	r.Register(TypeScript)
	r.Register(Rust)
	return r
}
