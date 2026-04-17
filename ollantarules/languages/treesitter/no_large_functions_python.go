package treesitter

import (
	"fmt"

	"github.com/scovl/ollanta/ollantacore/domain"
	ollantarules "github.com/scovl/ollanta/ollantarules"
)

// NoLargeFunctionsPY detects Python function definitions that exceed a
// configurable line count threshold using a tree-sitter S-expression query.
type NoLargeFunctionsPY struct{}

func (r *NoLargeFunctionsPY) Key() string                      { return "py:no-large-functions" }
func (r *NoLargeFunctionsPY) Name() string                     { return "No Large Functions (Python)" }
func (r *NoLargeFunctionsPY) Language() string                 { return "python" }
func (r *NoLargeFunctionsPY) Type() domain.IssueType           { return domain.TypeCodeSmell }
func (r *NoLargeFunctionsPY) DefaultSeverity() domain.Severity { return domain.SeverityMajor }
func (r *NoLargeFunctionsPY) Tags() []string                   { return []string{"size", "complexity"} }
func (r *NoLargeFunctionsPY) Description() string {
	return "Python functions exceeding the configured line limit are harder to read and maintain."
}
func (r *NoLargeFunctionsPY) Params() []domain.ParamDef {
	return []domain.ParamDef{
		{Key: "max_lines", Description: "Maximum allowed lines per function", DefaultValue: "40", Type: "int"},
	}
}

func (r *NoLargeFunctionsPY) Check(ctx *ollantarules.AnalysisContext) []*domain.Issue {
	maxLines := tsParamInt(ctx.Params, "max_lines", 40)
	query := `(function_definition name: (identifier) @fn.name) @fn`
	matches, err := ctx.Query.Run(ctx.ParsedFile, query, ctx.Grammar)
	if err != nil {
		return nil
	}

	var issues []*domain.Issue
	for _, m := range matches {
		fn := m.Captures["fn"]
		if fn == nil {
			continue
		}
		startLine, _, endLine, _ := ctx.Query.Position(fn)
		if endLine-startLine+1 > maxLines {
			nameNode := m.Captures["fn.name"]
			fnName := ctx.Query.Text(ctx.ParsedFile, nameNode)
			issue := domain.NewIssue(r.Key(), ctx.Path, startLine)
			issue.EndLine = endLine
			issue.Severity = r.DefaultSeverity()
			issue.Type = r.Type()
			issue.Message = fmt.Sprintf("Function '%s' has %d lines (max: %d)", fnName, endLine-startLine+1, maxLines)
			issues = append(issues, issue)
		}
	}
	return issues
}
