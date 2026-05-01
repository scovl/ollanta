package languages

import (
	ollantaparser "github.com/scovl/ollanta/ollantaparser"
	typescript "github.com/smacker/go-tree-sitter/typescript/typescript"
)

// TypeScript is the tree-sitter grammar for .ts / .tsx files.
var TypeScript = ollantaparser.NewLanguage(
	"typescript",
	[]string{".ts", ".tsx"},
	typescript.GetLanguage(),
)
