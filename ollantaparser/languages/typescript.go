package languages

import (
	typescript "github.com/smacker/go-tree-sitter/typescript/typescript"
	ollantaparser "github.com/scovl/ollanta/ollantaparser"
)

// TypeScript is the tree-sitter grammar for .ts / .tsx files.
var TypeScript = ollantaparser.NewLanguage(
	"typescript",
	[]string{".ts", ".tsx"},
	typescript.GetLanguage(),
)
