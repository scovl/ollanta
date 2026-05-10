package rules

import (
	"fmt"
	"go/ast"
	"go/token"

	"github.com/scovl/ollanta/ollantacore/domain"
	ollantarules "github.com/scovl/ollanta/ollantarules"
)

// NoLargeFunctions detects Go functions and methods that exceed a configurable
// line count threshold. Long functions are harder to read, test and maintain.
var NoLargeFunctions = ollantarules.Rule{
	MetaKey: "go:no-large-functions",
	Check: func(ctx *ollantarules.AnalysisContext) []*domain.Issue {
		maxLines := ollantarules.ParamInt(ctx.Params, "max_lines", 60)
		var issues []*domain.Issue

		ast.Inspect(ctx.AST, func(n ast.Node) bool {
			fn, ok := n.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				return true
			}
			start := ctx.FileSet.Position(fn.Pos()).Line
			end := ctx.FileSet.Position(fn.Body.End()).Line
			lines := end - start + 1
			if lines > maxLines {
				name := fn.Name.Name
				issue := domain.NewIssue("go:no-large-functions", ctx.Path, start)
				issue.EndLine = end
				issue.Severity = domain.SeverityMajor
				issue.Type = domain.TypeCodeSmell
				issue.Message = fmt.Sprintf("Function '%s' has %d lines (max: %d)", name, lines, maxLines)
				issues = append(issues, issue)
			}
			return true
	})
	return issues
},
}

// lineOf returns the 1-based line number of a token.Pos.
func lineOf(fset *token.FileSet, pos token.Pos) int {
	return fset.Position(pos).Line
}
