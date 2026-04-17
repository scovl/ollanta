package languages

import (
	python "github.com/smacker/go-tree-sitter/python"
	ollantaparser "github.com/scovl/ollanta/ollantaparser"
)

// Python is the tree-sitter grammar for .py files.
var Python = ollantaparser.NewLanguage(
	"python",
	[]string{".py"},
	python.GetLanguage(),
)
