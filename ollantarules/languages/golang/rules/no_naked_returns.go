package rules

import (
	"fmt"
	"go/ast"

	"github.com/scovl/ollanta/ollantacore/domain"
	ollantarules "github.com/scovl/ollanta/ollantarules"
)

// NoNakedReturns detects naked return statements (return without values) in
// functions that use named return values and exceed a minimum line count.
// In long functions, naked returns reduce readability and are a source of bugs.
var NoNakedReturns = ollantarules.Rule{
	MetaKey: "go:no-naked-returns",
	Check: func(ctx *ollantarules.AnalysisContext) []*domain.Issue {
		minLines := ollantarules.ParamInt(ctx.Params, "min_lines", 5)
		var issues []*domain.Issue

		ast.Inspect(ctx.AST, func(n ast.Node) bool {
			fn, ok := n.(*ast.FuncDecl)
			if !ok || fn.Body == nil || fn.Type.Results == nil {
				return true
			}
			if !hasNamedReturns(fn.Type.Results) {
				return true
			}
			start := lineOf(ctx.FileSet, fn.Pos())
			end := lineOf(ctx.FileSet, fn.Body.End())
			if end-start+1 <= minLines {
				return true
			}
			ast.Inspect(fn.Body, func(inner ast.Node) bool {
				ret, ok := inner.(*ast.ReturnStmt)
				if !ok {
					return true
				}
				if len(ret.Results) == 0 {
					line := lineOf(ctx.FileSet, ret.Pos())
					issue := domain.NewIssue("go:no-naked-returns", ctx.Path, line)
					issue.Severity = domain.SeverityCritical
					issue.Type = domain.TypeBug
					issue.Message = fmt.Sprintf("Naked return in function '%s' (line %d); use explicit return values", fn.Name.Name, line)
					issues = append(issues, issue)
				}
				return true
			})
			return true
		})
		return issues
	},
}

func hasNamedReturns(results *ast.FieldList) bool {
	for _, field := range results.List {
		if len(field.Names) > 0 {
			return true
		}
	}
	return false
}
