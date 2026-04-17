package treesitter

import (
	"fmt"

	"github.com/scovl/ollanta/ollantacore/domain"
	ollantarules "github.com/scovl/ollanta/ollantarules"
)

// BroadExceptPY detects Python except clauses that catch Exception (or any base
// exception type) without narrowing to a specific error, silently swallowing bugs.
// SonarQube equivalent: python:S5754 / pylint: W0703.
type BroadExceptPY struct{}

func (r *BroadExceptPY) Key() string                      { return "py:broad-except" }
func (r *BroadExceptPY) Name() string                     { return "Broad Exception Catch (Python)" }
func (r *BroadExceptPY) Language() string                 { return "python" }
func (r *BroadExceptPY) Type() domain.IssueType           { return domain.TypeBug }
func (r *BroadExceptPY) DefaultSeverity() domain.Severity { return domain.SeverityMajor }
func (r *BroadExceptPY) Tags() []string                   { return []string{"error-handling", "correctness"} }
func (r *BroadExceptPY) Description() string {
	return "Catching broad exceptions like 'Exception' or 'BaseException' hides bugs. Catch specific exceptions instead."
}
func (r *BroadExceptPY) Params() []domain.ParamDef { return nil }

func (r *BroadExceptPY) Check(ctx *ollantarules.AnalysisContext) []*domain.Issue {
	// Match bare `except:` and `except Exception/BaseException/...:`
	query := `[
	  (except_clause) @bare
	  (except_clause
	    (as_pattern
	      (attribute
	        object: (identifier) @cls)))
	  (except_clause
	    (as_pattern
	      (identifier) @cls))
	  (except_clause
	    (attribute
	      object: (identifier) @cls))
	  (except_clause
	    (identifier) @cls)
	] @clause`

	matches, err := ctx.Query.Run(ctx.ParsedFile, query, ctx.Grammar)
	if err != nil {
		return nil
	}

	broadTypes := map[string]bool{
		"Exception":     true,
		"BaseException": true,
	}

	var issues []*domain.Issue
	seen := map[int]bool{}
	for _, m := range matches {
		clause := m.Captures["clause"]
		if clause == nil {
			continue
		}
		startLine, _, _, _ := ctx.Query.Position(clause)
		if seen[startLine] {
			continue
		}

		cls := m.Captures["cls"]
		isBroad := cls == nil // bare except:
		if cls != nil {
			name := ctx.Query.Text(ctx.ParsedFile, cls)
			isBroad = broadTypes[name]
		}

		if isBroad {
			seen[startLine] = true
			issue := domain.NewIssue(r.Key(), ctx.Path, startLine)
			issue.Severity = r.DefaultSeverity()
			issue.Type = r.Type()
			if cls == nil {
				issue.Message = "Bare 'except:' clause catches all exceptions; catch specific exceptions instead"
			} else {
				issue.Message = fmt.Sprintf(
					"Catching broad exception type '%s' hides bugs; catch specific exceptions instead",
					ctx.Query.Text(ctx.ParsedFile, cls),
				)
			}
			issues = append(issues, issue)
		}
	}
	return issues
}
