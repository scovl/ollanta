package rules

import (
	"embed"

	ollantarules "github.com/scovl/ollanta/ollantarules"
)

// MetaFS holds the JSON metadata files for all Go rules in this package.
//
//go:embed *.json
var MetaFS embed.FS

func init() {
	ollantarules.MustRegister(MetaFS, "*.json",
		CognitiveComplexity,
		FunctionNestingDepth,
		MagicNumber,
		NamingConventions,
		NoLargeFunctions,
		NoNakedReturns,
		TodoComment,
		TooManyParameters,
	)
}
