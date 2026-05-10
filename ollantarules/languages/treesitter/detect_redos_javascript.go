package treesitter

import (
	"github.com/scovl/ollanta/ollantacore/domain"
	ollantarules "github.com/scovl/ollanta/ollantarules"
)

// DetectRedosJS detects regex literals or RegExp constructors with patterns
// that contain nested quantifiers, a common ReDoS vector.
var DetectRedosJS = ollantarules.Rule{
	MetaKey: "js:detect-redos",
	Check: func(ctx *ollantarules.AnalysisContext) []*domain.Issue {
		// Match regex literals /.../
		query := `[
  (regex
    (regex_pattern) @pattern) @regex
  (new_expression
    constructor: (identifier) @ctor
    arguments: (arguments
      (string
        (string_fragment) @pattern))
    (#eq? @ctor "RegExp"))
    @regex
]`
		matches, err := ctx.Query.Run(ctx.ParsedFile, query, ctx.Grammar)
		if err != nil {
			return nil
		}
		var issues []*domain.Issue
		seen := map[int]bool{}
		for _, m := range matches {
			regexNode := m.Captures["regex"]
			patNode := m.Captures["pattern"]
			if regexNode == nil || patNode == nil {
				continue
			}
			pattern := ctx.Query.Text(ctx.ParsedFile, patNode)
			if !hasNestedQuantifiersJS(pattern) {
				continue
			}
			line, _, _, _ := ctx.Query.Position(regexNode)
			if seen[line] {
				continue
			}
			seen[line] = true
			issue := domain.NewIssue("js:detect-redos", ctx.Path, line)
			issue.Severity = domain.SeverityMajor
			issue.Type = domain.TypeVulnerability
			issue.Message = "Regex pattern may be vulnerable to ReDoS due to nested quantifiers"
			issues = append(issues, issue)
		}
		return issues
	},
}

func hasNestedQuantifiersJS(pattern string) bool {
	return hasNestedQuantifiers(pattern)
}
