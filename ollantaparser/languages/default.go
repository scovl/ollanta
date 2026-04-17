package languages

import "github.com/scovl/ollanta/ollantaparser"

// DefaultRegistry returns a LanguageRegistry with all four built-in grammars registered:
// JavaScript, Python, TypeScript, and Rust.
func DefaultRegistry() *ollantaparser.LanguageRegistry {
	r := ollantaparser.NewRegistry()
	r.Register(JavaScript)
	r.Register(Python)
	r.Register(TypeScript)
	r.Register(Rust)
	return r
}
