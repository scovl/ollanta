// Package defaults provides a pre-configured Registry with all built-in
// Ollanta rules registered. Import this package to get a ready-to-use
// registry without manually registering each rule.
package defaults

import (
	ollantarules "github.com/scovl/ollanta/ollantarules"
	gorules "github.com/scovl/ollanta/ollantarules/languages/golang/rules"
	tsrules "github.com/scovl/ollanta/ollantarules/languages/treesitter"
)

// NewRegistry returns a Registry pre-loaded with all built-in rules.
func NewRegistry() *ollantarules.Registry {
	r := ollantarules.NewRegistry()

	// Go rules
	r.Register(&gorules.NoLargeFunctions{})
	r.Register(&gorules.NamingConventions{})
	r.Register(&gorules.NoNakedReturns{})
	r.Register(&gorules.CognitiveComplexity{})
	r.Register(&gorules.TooManyParameters{})
	r.Register(&gorules.FunctionNestingDepth{})
	r.Register(&gorules.MagicNumber{})
	r.Register(&gorules.TodoComment{})

	// Python rules
	r.Register(&tsrules.NoLargeFunctionsPY{})
	r.Register(&tsrules.BroadExceptPY{})
	r.Register(&tsrules.MutableDefaultArgumentPY{})
	r.Register(&tsrules.ComparisonToNonePY{})
	r.Register(&tsrules.TooManyParametersPY{})

	// JavaScript rules
	r.Register(&tsrules.NoLargeFunctionsJS{})
	r.Register(&tsrules.NoConsoleLogJS{})
	r.Register(&tsrules.EqEqEqJS{})
	r.Register(&tsrules.TooManyParametersJS{})

	return r
}
