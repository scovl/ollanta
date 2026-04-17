package languages

import (
	rust "github.com/smacker/go-tree-sitter/rust"
	ollantaparser "github.com/scovl/ollanta/ollantaparser"
)

// Rust is the tree-sitter grammar for .rs files.
var Rust = ollantaparser.NewLanguage(
	"rust",
	[]string{".rs"},
	rust.GetLanguage(),
)
