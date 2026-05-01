package treesitter

import (
	"embed"

	ollantarules "github.com/scovl/ollanta/ollantarules"
)

// MetaFS holds the JSON metadata files for all tree-sitter rules in this package.
//
//go:embed *.json
var MetaFS embed.FS

func init() {
	ollantarules.MustRegister(MetaFS, "*.json",
		BroadExceptPY,
		ComparisonToNonePY,
		EqEqEqJS,
		MutableDefaultArgumentPY,
		NoConsoleLogJS,
		NoLargeFunctionsJS,
		NoLargeFunctionsPY,
		TooManyParametersJS,
		TooManyParametersPY,
	)
}
