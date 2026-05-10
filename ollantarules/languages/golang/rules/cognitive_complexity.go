package rules

import (
	"fmt"
	"go/ast"

	"github.com/scovl/ollanta/ollantacore/domain"
	ollantarules "github.com/scovl/ollanta/ollantarules"
)

// CognitiveComplexity measures the cognitive complexity of each Go function.
// Cognitive complexity (as defined by SonarSource) counts decision points weighted
// by nesting depth, giving a better approximation of "how hard is this to understand"
// than raw cyclomatic complexity.
//
// Scoring:
//   - +1 for each if / else if / else / for / switch / select / case
//   - +1 per additional nesting level for the above
//   - +1 for each && or || sequence break in a boolean expression
var CognitiveComplexity = ollantarules.Rule{
	MetaKey: "go:cognitive-complexity",
	Check: func(ctx *ollantarules.AnalysisContext) []*domain.Issue {
		maxVal := ollantarules.ParamInt(ctx.Params, "max_complexity", 15)
		var issues []*domain.Issue

		ast.Inspect(ctx.AST, func(n ast.Node) bool {
			fn, ok := n.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				return true
			}
			score := cognitiveScore(fn.Body, 0)
			if score > maxVal {
				start := ctx.FileSet.Position(fn.Pos()).Line
				end := ctx.FileSet.Position(fn.Body.End()).Line
				issue := domain.NewIssue("go:cognitive-complexity", ctx.Path, start)
				issue.EndLine = end
				issue.Severity = domain.SeverityCritical
				issue.Type = domain.TypeCodeSmell
				issue.Message = fmt.Sprintf(
					"Function '%s' has cognitive complexity of %d (max: %d)",
					fn.Name.Name, score, maxVal,
				)
				issues = append(issues, issue)
			}
			return true
		})
		return issues
	},
}

func cognitiveScore(node ast.Node, depth int) int {
	score := 0
	ast.Inspect(node, func(n ast.Node) bool {
		if n == node {
			return true
		}
		score += nodeScore(n, depth)
		return descendInto(n)
	})
	return score
}

func nodeScore(n ast.Node, depth int) int {
	switch v := n.(type) {
	case *ast.IfStmt:
		s := 1 + depth
		s += cognitiveScore(v.Body, depth+1)
		if v.Else != nil {
			s++
			s += cognitiveScore(v.Else, depth+1)
		}
		return s
	case *ast.ForStmt:
		return 1 + depth + cognitiveScore(v.Body, depth+1)
	case *ast.RangeStmt:
		return 1 + depth + cognitiveScore(v.Body, depth+1)
	case *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
		return 1 + depth
	case *ast.CaseClause, *ast.CommClause:
		return 1 + depth
	case *ast.BinaryExpr:
		if v.Op.String() == "&&" || v.Op.String() == "||" {
			return 1
		}
	case *ast.FuncLit:
		return cognitiveScore(v.Body, depth+1)
	}
	return 0
}

func descendInto(n ast.Node) bool {
	switch n.(type) {
	case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.FuncLit:
		return false
	}
	return true
}
