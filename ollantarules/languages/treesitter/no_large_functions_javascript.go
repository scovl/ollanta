package treesitter

import (
	"fmt"
	"strconv"

	"github.com/scovl/ollanta/ollantacore/domain"
	ollantarules "github.com/scovl/ollanta/ollantarules"
)

// NoLargeFunctionsJS detects JavaScript function declarations that exceed a
// configurable line count threshold using a tree-sitter S-expression query.
type NoLargeFunctionsJS struct{}

func (r *NoLargeFunctionsJS) Key() string                      { return "js:no-large-functions" }
func (r *NoLargeFunctionsJS) Name() string                     { return "No Large Functions (JS)" }
func (r *NoLargeFunctionsJS) Language() string                 { return "javascript" }
func (r *NoLargeFunctionsJS) Type() domain.IssueType           { return domain.TypeCodeSmell }
func (r *NoLargeFunctionsJS) DefaultSeverity() domain.Severity { return domain.SeverityMajor }
func (r *NoLargeFunctionsJS) Tags() []string                   { return []string{"size", "complexity"} }
func (r *NoLargeFunctionsJS) Description() string {
	return "JavaScript functions exceeding the configured line limit are harder to read and maintain."
}
func (r *NoLargeFunctionsJS) Params() []domain.ParamDef {
	return []domain.ParamDef{
		{Key: "max_lines", Description: "Maximum allowed lines per function", DefaultValue: "40", Type: "int"},
	}
}

func (r *NoLargeFunctionsJS) Check(ctx *ollantarules.AnalysisContext) []*domain.Issue {
	maxLines := tsParamInt(ctx.Params, "max_lines", 40)
	query := `(function_declaration name: (identifier) @fn.name) @fn`
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

func tsParamInt(params map[string]string, key string, defaultVal int) int {
	if v, ok := params[key]; ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return defaultVal
}
