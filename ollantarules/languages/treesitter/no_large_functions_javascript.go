package treesitter

import (
	"fmt"

	"github.com/scovl/ollanta/ollantacore/domain"
	ollantarules "github.com/scovl/ollanta/ollantarules"
)

// NoLargeFunctionsJS detects JavaScript function declarations that exceed a
// configurable line count threshold using a tree-sitter S-expression query.
var NoLargeFunctionsJS = ollantarules.Rule{
	MetaKey: "js:no-large-functions",
	Check: func(ctx *ollantarules.AnalysisContext) []*domain.Issue {
		maxLines := ollantarules.ParamInt(ctx.Params, "max_lines", 40)
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
				issue := domain.NewIssue("js:no-large-functions", ctx.Path, startLine)
				issue.EndLine = endLine
				issue.Severity = domain.SeverityMajor
				issue.Type = domain.TypeCodeSmell
				issue.Message = fmt.Sprintf("Function '%s' has %d lines (max: %d)", fnName, endLine-startLine+1, maxLines)
				issues = append(issues, issue)
			}
		}
		return issues
	},
}

