package rules

import (
	"fmt"
	"go/ast"
	"go/token"
	"strconv"

	"github.com/scovl/ollanta/ollantacore/domain"
	ollantarules "github.com/scovl/ollanta/ollantarules"
)

// NoLargeFunctions detects Go functions and methods that exceed a configurable
// line count threshold. Long functions are harder to read, test and maintain.
type NoLargeFunctions struct{}

func (r *NoLargeFunctions) Key() string                      { return "go:no-large-functions" }
func (r *NoLargeFunctions) Name() string                     { return "No Large Functions" }
func (r *NoLargeFunctions) Language() string                 { return "go" }
func (r *NoLargeFunctions) Type() domain.IssueType           { return domain.TypeCodeSmell }
func (r *NoLargeFunctions) DefaultSeverity() domain.Severity { return domain.SeverityMajor }
func (r *NoLargeFunctions) Tags() []string                   { return []string{"size", "complexity"} }
func (r *NoLargeFunctions) Description() string {
	return "Functions exceeding the configured line limit are harder to read and maintain."
}
func (r *NoLargeFunctions) Params() []domain.ParamDef {
	return []domain.ParamDef{
		{Key: "max_lines", Description: "Maximum allowed lines per function", DefaultValue: "40", Type: "int"},
	}
}

func (r *NoLargeFunctions) Check(ctx *ollantarules.AnalysisContext) []*domain.Issue {
	maxLines := paramInt(ctx.Params, "max_lines", 40)
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
			issue := domain.NewIssue(r.Key(), ctx.Path, start)
			issue.EndLine = end
			issue.Severity = r.DefaultSeverity()
			issue.Type = r.Type()
			issue.Message = fmt.Sprintf("Function '%s' has %d lines (max: %d)", name, lines, maxLines)
			issues = append(issues, issue)
		}
		return true
	})
	return issues
}

// paramInt reads an int param from ctx.Params, falling back to defaultVal.
func paramInt(params map[string]string, key string, defaultVal int) int {
	if v, ok := params[key]; ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return defaultVal
}

// lineOf returns the 1-based line number of a token.Pos.
func lineOf(fset *token.FileSet, pos token.Pos) int {
	return fset.Position(pos).Line
}
