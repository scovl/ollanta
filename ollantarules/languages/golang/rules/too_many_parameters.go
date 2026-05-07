package rules

import (
	"fmt"
	"go/ast"

	"github.com/scovl/ollanta/ollantacore/domain"
	ollantarules "github.com/scovl/ollanta/ollantarules"
)

// TooManyParameters flags Go functions whose parameter count exceeds a configurable limit.
// Functions with many parameters are harder to call correctly, test, and understand.
// SonarQube equivalent: squid:S00107.
var TooManyParameters = ollantarules.Rule{
	MetaKey: "go:too-many-parameters",
	Check: func(ctx *ollantarules.AnalysisContext) []*domain.Issue {
		maxVal := ollantarules.ParamInt(ctx.Params, "max_params", 5)
		var issues []*domain.Issue

		ast.Inspect(ctx.AST, func(n ast.Node) bool {
			fn, ok := n.(*ast.FuncDecl)
			if !ok || fn.Type.Params == nil {
				return true
			}
			count := countFields(fn.Type.Params)
			if count > maxVal {
				line := ctx.FileSet.Position(fn.Pos()).Line
				issue := domain.NewIssue("go:too-many-parameters", ctx.Path, line)
				issue.Severity = domain.SeverityMajor
				issue.Type = domain.TypeCodeSmell
				issue.Message = fmt.Sprintf(
					"Function '%s' has %d parameters (max: %d)",
					fn.Name.Name, count, maxVal,
				)
				issues = append(issues, issue)
			}
			return true
		})
		return issues
	},
}

// countFields counts the total number of individual parameters in a FieldList.
// A field like (a, b int) counts as 2.
func countFields(fl *ast.FieldList) int {
	if fl == nil {
		return 0
	}
	count := 0
	for _, field := range fl.List {
		if len(field.Names) == 0 {
			count++ // anonymous parameter
		} else {
			count += len(field.Names)
		}
	}
	return count
}
