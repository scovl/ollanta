package rules

import (
	"fmt"
	"strings"

	"github.com/scovl/ollanta/ollantacore/domain"
	ollantarules "github.com/scovl/ollanta/ollantarules"
)

// TodoComment flags TODO, FIXME, HACK, and XXX comment markers left in production code.
// These markers indicate incomplete or problematic code that should be tracked in an
// issue tracker rather than silently left in source. SonarQube equivalent: squid:S1135.
type TodoComment struct{}

func (r *TodoComment) Key() string                      { return "go:todo-comment" }
func (r *TodoComment) Name() string                     { return "TODO Comment" }
func (r *TodoComment) Language() string                 { return "go" }
func (r *TodoComment) Type() domain.IssueType           { return domain.TypeCodeSmell }
func (r *TodoComment) DefaultSeverity() domain.Severity { return domain.SeverityInfo }
func (r *TodoComment) Tags() []string                   { return []string{"convention"} }
func (r *TodoComment) Description() string {
	return "TODO, FIXME, HACK and XXX comments signal incomplete or problematic code. Track them as issues instead."
}
func (r *TodoComment) Params() []domain.ParamDef { return nil }

var todoMarkers = []string{"TODO", "FIXME", "HACK", "XXX"}

func (r *TodoComment) Check(ctx *ollantarules.AnalysisContext) []*domain.Issue {
	var issues []*domain.Issue
	for _, cg := range ctx.AST.Comments {
		for _, c := range cg.List {
			text := strings.ToUpper(c.Text)
			for _, marker := range todoMarkers {
				if strings.Contains(text, marker) {
					line := ctx.FileSet.Position(c.Slash).Line
					issue := domain.NewIssue(r.Key(), ctx.Path, line)
					issue.Severity = r.DefaultSeverity()
					issue.Type = r.Type()
					issue.Message = fmt.Sprintf("Complete the task associated with this %q comment", marker)
					issues = append(issues, issue)
					break
				}
			}
		}
	}
	return issues
}
