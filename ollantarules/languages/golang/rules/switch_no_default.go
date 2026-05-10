package rules

import (
	"go/ast"

	"github.com/scovl/ollanta/ollantacore/domain"
	ollantarules "github.com/scovl/ollanta/ollantarules"
)

// SwitchNoDefault flags switch statements without a default clause.
// Missing defaults can lead to unhandled cases and silent bugs.
// SonarQube equivalent: squid:S131 / CWE-478.
var SwitchNoDefault = ollantarules.Rule{
	MetaKey: "go:switch-no-default",
	Check: func(ctx *ollantarules.AnalysisContext) []*domain.Issue {
		var issues []*domain.Issue
		ast.Inspect(ctx.AST, func(n ast.Node) bool {
			sw, ok := n.(*ast.SwitchStmt)
			if !ok || sw.Body == nil {
				return true
			}
			if len(sw.Body.List) == 0 {
				return true // empty switch is intentional
			}
			for _, stmt := range sw.Body.List {
				if clause, ok := stmt.(*ast.CaseClause); ok && clause.List == nil {
					// default clause found — no issue
					return true
				}
			}
			line := ctx.FileSet.Position(sw.Pos()).Line
			issue := domain.NewIssue("go:switch-no-default", ctx.Path, line)
			issue.Severity = domain.SeverityMajor
			issue.Type = domain.TypeBug
			issue.Message = "Add a default clause to this switch statement"
			issues = append(issues, issue)
			return true
		})
		return issues
	},
}
