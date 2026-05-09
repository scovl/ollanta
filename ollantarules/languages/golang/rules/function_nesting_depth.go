package rules

import (
	"fmt"
	"go/ast"
	"go/token"

	"github.com/scovl/ollanta/ollantacore/domain"
	ollantarules "github.com/scovl/ollanta/ollantarules"
)

// FunctionNestingDepth flags Go functions where control flow is nested deeper than
// a configurable limit. Deep nesting makes code hard to read and is a strong signal
// that the function should be decomposed. SonarQube equivalent: squid:S134.
var FunctionNestingDepth = ollantarules.Rule{
	MetaKey: "go:function-nesting-depth",
	Check: func(ctx *ollantarules.AnalysisContext) []*domain.Issue {
		maxVal := ollantarules.ParamInt(ctx.Params, "max_depth", 4)
		var issues []*domain.Issue

		ast.Inspect(ctx.AST, func(n ast.Node) bool {
			fn, ok := n.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				return true
			}
			depth, deepestPos := maxNestingDepth(fn.Body, 0)
			if depth > maxVal {
				fnLine := ctx.FileSet.Position(fn.Pos()).Line
				deepLine := ctx.FileSet.Position(deepestPos).Line
				issue := domain.NewIssue("go:function-nesting-depth", ctx.Path, fnLine)
				issue.EndLine = deepLine
				issue.Severity = domain.SeverityMajor
				issue.Type = domain.TypeCodeSmell
				issue.Message = fmt.Sprintf(
					"Function '%s' has a nesting depth of %d (max: %d) at line %d",
					fn.Name.Name, depth, maxVal, deepLine,
				)
				issues = append(issues, issue)
			}
			return true
		})
		return issues
	},
}

// maxNestingDepth returns the maximum nesting depth and the position of the deepest point.
func maxNestingDepth(node ast.Node, currentDepth int) (int, token.Pos) {
	maxDepth := currentDepth
	deepestPos := node.Pos()

	ast.Inspect(node, func(n ast.Node) bool {
		if n == node {
			return true
		}
		body, walkChildren := nestingConstructBody(n)
		if body != nil {
			d, pos := maxNestingDepth(body, currentDepth+1)
			if d > maxDepth {
				maxDepth = d
				deepestPos = pos
			}
			return false
		}
		return walkChildren
	})
	return maxDepth, deepestPos
}

// nestingConstructBody returns the body of a nesting construct (if/for/range/switch/select).
// Returns (nil, false) for FuncLit (depth reset) and (nil, true) for other nodes.
func nestingConstructBody(n ast.Node) (body ast.Node, walkChildren bool) {
	switch v := n.(type) {
	case *ast.IfStmt:
		return v.Body, false
	case *ast.ForStmt:
		return v.Body, false
	case *ast.RangeStmt:
		return v.Body, false
	case *ast.SwitchStmt:
		return v.Body, false
	case *ast.TypeSwitchStmt:
		return v.Body, false
	case *ast.SelectStmt:
		return v.Body, false
	case *ast.FuncLit:
		return nil, false
	}
	return nil, true
}
