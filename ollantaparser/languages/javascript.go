// Package languages registers tree-sitter grammars for ollantaparser.
// Import this package with a blank identifier to activate all built-in languages:
//
//	import _ "github.com/scovl/ollanta/ollantaparser/languages"
package languages

import (
	javascript "github.com/smacker/go-tree-sitter/javascript"
	ollantaparser "github.com/scovl/ollanta/ollantaparser"
)

// JavaScript is the tree-sitter grammar for .js / .mjs files.
var JavaScript = ollantaparser.NewLanguage(
	"javascript",
	[]string{".js", ".mjs"},
	javascript.GetLanguage(),
)
