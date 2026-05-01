package languages

import (
	ollantaparser "github.com/scovl/ollanta/ollantaparser"
	python "github.com/smacker/go-tree-sitter/python"
)

// Python is the tree-sitter grammar for .py files.
var Python = ollantaparser.NewLanguage(
	"python",
	[]string{".py"},
	python.GetLanguage(),
)
