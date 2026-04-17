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
type NoNakedReturns struct{}

func (r *NoNakedReturns) Key() string                      { return "go:no-naked-returns" }
func (r *NoNakedReturns) Name() string                     { return "No Naked Returns" }
func (r *NoNakedReturns) Language() string                 { return "go" }
func (r *NoNakedReturns) Type() domain.IssueType           { return domain.TypeBug }
func (r *NoNakedReturns) DefaultSeverity() domain.Severity { return domain.SeverityCritical }
func (r *NoNakedReturns) Tags() []string                   { return []string{"correctness", "readability"} }
func (r *NoNakedReturns) Description() string {
	return "Naked returns in long functions with named return values make code harder to reason about and are a frequent source of bugs."
}
func (r *NoNakedReturns) Params() []domain.ParamDef {
	return []domain.ParamDef{
		{Key: "min_lines", Description: "Minimum function length to flag naked returns", DefaultValue: "5", Type: "int"},
	}
}

func (r *NoNakedReturns) Check(ctx *ollantarules.AnalysisContext) []*domain.Issue {
	minLines := paramInt(ctx.Params, "min_lines", 5)
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
		// Walk the body looking for bare returns.
		ast.Inspect(fn.Body, func(inner ast.Node) bool {
			ret, ok := inner.(*ast.ReturnStmt)
			if !ok {
				return true
			}
			if len(ret.Results) == 0 {
				line := lineOf(ctx.FileSet, ret.Pos())
				issue := domain.NewIssue(r.Key(), ctx.Path, line)
				issue.Severity = r.DefaultSeverity()
				issue.Type = r.Type()
				issue.Message = fmt.Sprintf("Naked return in function '%s' (line %d); use explicit return values", fn.Name.Name, line)
				issues = append(issues, issue)
			}
			return true
		})
		return true
	})
	return issues
}

func hasNamedReturns(results *ast.FieldList) bool {
	for _, field := range results.List {
		if len(field.Names) > 0 {
			return true
		}
	}
	return false
}
