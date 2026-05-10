package rules

import (
	"go/ast"
	"go/token"

	"github.com/scovl/ollanta/ollantacore/domain"
	ollantarules "github.com/scovl/ollanta/ollantarules"
)

// UselessEqEq detects self-comparisons like x == x or x != x which are
// always deterministic and indicate a bug or dead code.
var UselessEqEq = ollantarules.Rule{
	MetaKey: "go:useless-eqeq",
	Check: func(ctx *ollantarules.AnalysisContext) []*domain.Issue {
		var issues []*domain.Issue
		ast.Inspect(ctx.AST, func(n ast.Node) bool {
			be, ok := n.(*ast.BinaryExpr)
			if !ok {
				return true
			}
			if be.Op != token.EQL && be.Op != token.NEQ {
				return true
			}
			if exprEqual(be.X, be.Y) {
				line := ctx.FileSet.Position(be.Pos()).Line
				issue := domain.NewIssue("go:useless-eqeq", ctx.Path, line)
				issue.Severity = domain.SeverityMinor
				issue.Type = domain.TypeBug
				issue.Message = "Useless self-comparison: this expression is always true or always false"
				issues = append(issues, issue)
			}
			return true
		})
		return issues
	},
}

// exprEqual reports whether two expressions are syntactically identical
// for the purpose of self-comparison detection.
func exprEqual(a, b ast.Expr) bool {
	if a == nil || b == nil {
		return a == b
	}
	switch xa := a.(type) {
	case *ast.Ident:
		xb, ok := b.(*ast.Ident)
		return ok && xa.Name == xb.Name
	case *ast.BasicLit:
		xb, ok := b.(*ast.BasicLit)
		return ok && xa.Kind == xb.Kind && xa.Value == xb.Value
	case *ast.SelectorExpr:
		xb, ok := b.(*ast.SelectorExpr)
		return ok && exprEqual(xa.X, xb.X) && xa.Sel.Name == xb.Sel.Name
	case *ast.IndexExpr:
		xb, ok := b.(*ast.IndexExpr)
		return ok && exprEqual(xa.X, xb.X) && exprEqual(xa.Index, xb.Index)
	case *ast.CallExpr:
		return equalCallExpr(xa, b)
	default:
		return false
	}
}

func equalCallExpr(a *ast.CallExpr, b ast.Expr) bool {
	xb, ok := b.(*ast.CallExpr)
	if !ok || len(a.Args) != len(xb.Args) {
		return false
	}
	if !exprEqual(a.Fun, xb.Fun) {
		return false
	}
	for i := range a.Args {
		if !exprEqual(a.Args[i], xb.Args[i]) {
			return false
		}
	}
	return true
}
