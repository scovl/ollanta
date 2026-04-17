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
type CognitiveComplexity struct{}

func (r *CognitiveComplexity) Key() string                      { return "go:cognitive-complexity" }
func (r *CognitiveComplexity) Name() string                     { return "Cognitive Complexity" }
func (r *CognitiveComplexity) Language() string                 { return "go" }
func (r *CognitiveComplexity) Type() domain.IssueType           { return domain.TypeCodeSmell }
func (r *CognitiveComplexity) DefaultSeverity() domain.Severity { return domain.SeverityCritical }
func (r *CognitiveComplexity) Tags() []string                   { return []string{"complexity", "readability"} }
func (r *CognitiveComplexity) Description() string {
	return "Functions with high cognitive complexity are hard to understand, test and maintain."
}
func (r *CognitiveComplexity) Params() []domain.ParamDef {
	return []domain.ParamDef{
		{Key: "max_complexity", Description: "Maximum allowed cognitive complexity", DefaultValue: "15", Type: "int"},
	}
}

func (r *CognitiveComplexity) Check(ctx *ollantarules.AnalysisContext) []*domain.Issue {
	maxVal := paramInt(ctx.Params, "max_complexity", 15)
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
			issue := domain.NewIssue(r.Key(), ctx.Path, start)
			issue.EndLine = end
			issue.Severity = r.DefaultSeverity()
			issue.Type = r.Type()
			issue.Message = fmt.Sprintf(
				"Function '%s' has cognitive complexity of %d (max: %d)",
				fn.Name.Name, score, maxVal,
			)
			issues = append(issues, issue)
		}
		return true
	})
	return issues
}

// cognitiveScore computes the cognitive complexity score for a block recursively.
func cognitiveScore(node ast.Node, depth int) int {
	score := 0
	ast.Inspect(node, func(n ast.Node) bool {
		if n == node {
			return true // don't double-count the root
		}
		switch v := n.(type) {
		case *ast.IfStmt:
			score += 1 + depth
			score += cognitiveScore(v.Body, depth+1)
			if v.Else != nil {
				score++ // else or else-if
				score += cognitiveScore(v.Else, depth+1)
			}
			return false // already recursed
		case *ast.ForStmt, *ast.RangeStmt:
			score += 1 + depth
			if fs, ok := n.(*ast.ForStmt); ok {
				score += cognitiveScore(fs.Body, depth+1)
			} else if rs, ok := n.(*ast.RangeStmt); ok {
				score += cognitiveScore(rs.Body, depth+1)
			}
			return false
		case *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
			score += 1 + depth
			return true // let ast.Inspect recurse into case clauses
		case *ast.CaseClause, *ast.CommClause:
			score += 1 + depth
			return true
		case *ast.BinaryExpr:
			if v.Op.String() == "&&" || v.Op.String() == "||" {
				score++
			}
		case *ast.FuncLit:
			// nested function literal: recurse with increased depth
			score += cognitiveScore(v.Body, depth+1)
			return false
		}
		return true
	})
	return score
}
