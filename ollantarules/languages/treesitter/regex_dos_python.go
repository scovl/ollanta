package treesitter

import (
	"strings"

	"github.com/scovl/ollanta/ollantacore/domain"
	ollantarules "github.com/scovl/ollanta/ollantarules"
)

// RegexDosPY detects regex patterns that contain nested quantifiers,
// which are a common source of Regular Expression Denial of Service (ReDoS).
// This is a conservative heuristic and may miss some complex cases.
var RegexDosPY = ollantarules.Rule{
	MetaKey: "py:regex-dos",
	Check: func(ctx *ollantarules.AnalysisContext) []*domain.Issue {
		query := `(call
  function: (attribute
    object: (identifier) @mod
    attribute: (identifier) @func)
  arguments: (argument_list
    (string
      (string_content) @pattern))
  (#eq? @mod "re")
  (#match? @func "^(compile|match|search|findall|split|sub|subn|fullmatch)$")
) @call`
		matches, err := ctx.Query.Run(ctx.ParsedFile, query, ctx.Grammar)
		if err != nil {
			return nil
		}
		var issues []*domain.Issue
		seen := map[int]bool{}
		for _, m := range matches {
			call := m.Captures["call"]
			patNode := m.Captures["pattern"]
			if call == nil || patNode == nil {
				continue
			}
			pattern := ctx.Query.Text(ctx.ParsedFile, patNode)
			if !hasNestedQuantifiers(pattern) {
				continue
			}
			line, _, _, _ := ctx.Query.Position(call)
			if seen[line] {
				continue
			}
			seen[line] = true
			issue := domain.NewIssue("py:regex-dos", ctx.Path, line)
			issue.Severity = domain.SeverityMajor
			issue.Type = domain.TypeVulnerability
			issue.Message = "Regex pattern may be vulnerable to ReDoS due to nested quantifiers"
			issues = append(issues, issue)
		}
		return issues
	},
}

func hasNestedQuantifiers(pattern string) bool {
	depth := 0
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		switch c {
		case '(':
			depth++
		case ')':
			if isGroupWithQuantifier(pattern, depth, i) {
				return true
			}
			depth--
		}
	}
	return false
}

func isGroupWithQuantifier(pattern string, depth int, i int) bool {
	if depth <= 0 || i+1 >= len(pattern) {
		return false
	}
	next := pattern[i+1]
	if next != '*' && next != '+' && next != '?' && next != '{' {
		return false
	}
	groupStart := findMatchingOpen(pattern, i)
	return groupStart >= 0 && strings.ContainsAny(pattern[groupStart:i], "*+?{")
}

func findMatchingOpen(pattern string, closeIdx int) int {
	depth := 1
	for i := closeIdx - 1; i >= 0; i-- {
		switch pattern[i] {
		case ')':
			depth++
		case '(':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}
