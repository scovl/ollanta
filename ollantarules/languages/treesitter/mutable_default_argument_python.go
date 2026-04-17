package treesitter

import (
	"fmt"

	"github.com/scovl/ollanta/ollantacore/domain"
	ollantarules "github.com/scovl/ollanta/ollantarules"
)

// MutableDefaultArgumentPY detects Python function definitions that use a mutable
// object (list, dict, or set) as a default parameter value. Because Python evaluates
// default values once at function definition time, mutations persist across calls,
// causing subtle and hard-to-debug bugs.
// SonarQube equivalent: python:S5717 / pylint: W0102.
type MutableDefaultArgumentPY struct{}

func (r *MutableDefaultArgumentPY) Key() string                      { return "py:mutable-default-argument" }
func (r *MutableDefaultArgumentPY) Name() string                     { return "Mutable Default Argument (Python)" }
func (r *MutableDefaultArgumentPY) Language() string                 { return "python" }
func (r *MutableDefaultArgumentPY) Type() domain.IssueType           { return domain.TypeBug }
func (r *MutableDefaultArgumentPY) DefaultSeverity() domain.Severity { return domain.SeverityMajor }
func (r *MutableDefaultArgumentPY) Tags() []string                   { return []string{"bug", "pitfall"} }
func (r *MutableDefaultArgumentPY) Description() string {
	return "Using mutable objects (list, dict, set) as default argument values is a common Python pitfall. The same object is shared across all calls."
}
func (r *MutableDefaultArgumentPY) Params() []domain.ParamDef { return nil }

func (r *MutableDefaultArgumentPY) Check(ctx *ollantarules.AnalysisContext) []*domain.Issue {
	// Match default parameter values that are list, dict, or set literals.
	query := `(default_parameter
	  value: [
	    (list)       @mut
	    (dictionary) @mut
	    (set)        @mut
	  ]) @param`

	matches, err := ctx.Query.Run(ctx.ParsedFile, query, ctx.Grammar)
	if err != nil {
		return nil
	}

	var issues []*domain.Issue
	for _, m := range matches {
		mut := m.Captures["mut"]
		if mut == nil {
			continue
		}
		line, _, _, _ := ctx.Query.Position(mut)
		issue := domain.NewIssue(r.Key(), ctx.Path, line)
		issue.Severity = r.DefaultSeverity()
		issue.Type = r.Type()
		issue.Message = fmt.Sprintf(
			"Mutable default argument at line %d; use None and initialise inside the function instead",
			line,
		)
		issues = append(issues, issue)
	}
	return issues
}
