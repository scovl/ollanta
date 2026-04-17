package treesitter

import (
	"fmt"

	"github.com/scovl/ollanta/ollantacore/domain"
	ollantarules "github.com/scovl/ollanta/ollantarules"
)

// ComparisonToNonePY detects Python comparisons to None using == or != instead of
// the identity operators `is` and `is not`. PEP 8 explicitly requires identity
// comparison for None. SonarQube equivalent: python:S5727.
type ComparisonToNonePY struct{}

func (r *ComparisonToNonePY) Key() string                      { return "py:comparison-to-none" }
func (r *ComparisonToNonePY) Name() string                     { return "Comparison to None (Python)" }
func (r *ComparisonToNonePY) Language() string                 { return "python" }
func (r *ComparisonToNonePY) Type() domain.IssueType           { return domain.TypeCodeSmell }
func (r *ComparisonToNonePY) DefaultSeverity() domain.Severity { return domain.SeverityMinor }
func (r *ComparisonToNonePY) Tags() []string                   { return []string{"convention", "correctness"} }
func (r *ComparisonToNonePY) Description() string {
	return "Compare to None using 'is' or 'is not', not '==' or '!='. PEP 8 / SonarQube python:S5727."
}
func (r *ComparisonToNonePY) Params() []domain.ParamDef { return nil }

func (r *ComparisonToNonePY) Check(ctx *ollantarules.AnalysisContext) []*domain.Issue {
	// Match: x == None  or  None == x  or  x != None  or  None != x
	query := `(comparison_operator
	  [
	    (none) @none
	    (_ (none) @none)
	  ]
	  operators: _ @op
	) @cmp`

	matches, err := ctx.Query.Run(ctx.ParsedFile, query, ctx.Grammar)
	if err != nil {
		return nil
	}

	var issues []*domain.Issue
	seen := map[int]bool{}
	for _, m := range matches {
		cmp := m.Captures["cmp"]
		op := m.Captures["op"]
		if cmp == nil || op == nil {
			continue
		}
		opText := ctx.Query.Text(ctx.ParsedFile, op)
		if opText != "==" && opText != "!=" {
			continue
		}
		line, _, _, _ := ctx.Query.Position(cmp)
		if seen[line] {
			continue
		}
		seen[line] = true

		better := "is"
		if opText == "!=" {
			better = "is not"
		}
		issue := domain.NewIssue(r.Key(), ctx.Path, line)
		issue.Severity = r.DefaultSeverity()
		issue.Type = r.Type()
		issue.Message = fmt.Sprintf(
			"Use '%s None' instead of '%s None' (PEP 8)", better, opText,
		)
		issues = append(issues, issue)
	}
	return issues
}
