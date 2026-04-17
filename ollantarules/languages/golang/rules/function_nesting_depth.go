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
type FunctionNestingDepth struct{}

func (r *FunctionNestingDepth) Key() string                      { return "go:function-nesting-depth" }
func (r *FunctionNestingDepth) Name() string                     { return "Function Nesting Depth" }
func (r *FunctionNestingDepth) Language() string                 { return "go" }
func (r *FunctionNestingDepth) Type() domain.IssueType           { return domain.TypeCodeSmell }
func (r *FunctionNestingDepth) DefaultSeverity() domain.Severity { return domain.SeverityMajor }
func (r *FunctionNestingDepth) Tags() []string                   { return []string{"complexity", "readability"} }
func (r *FunctionNestingDepth) Description() string {
	return "Code nested too deeply is hard to read and maintain. Extract inner logic into separate functions."
}
func (r *FunctionNestingDepth) Params() []domain.ParamDef {
	return []domain.ParamDef{
		{Key: "max_depth", Description: "Maximum allowed nesting depth", DefaultValue: "4", Type: "int"},
	}
}

func (r *FunctionNestingDepth) Check(ctx *ollantarules.AnalysisContext) []*domain.Issue {
	maxVal := paramInt(ctx.Params, "max_depth", 4)
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
			issue := domain.NewIssue(r.Key(), ctx.Path, fnLine)
			issue.EndLine = deepLine
			issue.Severity = r.DefaultSeverity()
			issue.Type = r.Type()
			issue.Message = fmt.Sprintf(
				"Function '%s' has a nesting depth of %d (max: %d) at line %d",
				fn.Name.Name, depth, maxVal, deepLine,
			)
			issues = append(issues, issue)
		}
		return true
	})
	return issues
}

// maxNestingDepth returns the maximum nesting depth and the position of the deepest point.
func maxNestingDepth(node ast.Node, currentDepth int) (int, token.Pos) {
	maxDepth := currentDepth
	deepestPos := node.Pos()

	ast.Inspect(node, func(n ast.Node) bool {
		if n == node {
			return true
		}
		var body ast.Node
		switch v := n.(type) {
		case *ast.IfStmt:
			body = v.Body
		case *ast.ForStmt:
			body = v.Body
		case *ast.RangeStmt:
			body = v.Body
		case *ast.SwitchStmt:
			body = v.Body
		case *ast.TypeSwitchStmt:
			body = v.Body
		case *ast.SelectStmt:
			body = v.Body
		case *ast.FuncLit:
			// nested func literal resets depth context
			return false
		default:
			return true
		}
		if body != nil {
			d, pos := maxNestingDepth(body, currentDepth+1)
			if d > maxDepth {
				maxDepth = d
				deepestPos = pos
			}
		}
		return false // already recursed
	})
	return maxDepth, deepestPos
}
