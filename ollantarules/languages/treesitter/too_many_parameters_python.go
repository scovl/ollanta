package treesitter

import (
	"fmt"

	"github.com/scovl/ollanta/ollantacore/domain"
	ollantarules "github.com/scovl/ollanta/ollantarules"
)

// TooManyParametersPY flags Python functions with too many parameters.
// SonarQube equivalent: python:S107.
type TooManyParametersPY struct{}

func (r *TooManyParametersPY) Key() string                      { return "py:too-many-parameters" }
func (r *TooManyParametersPY) Name() string                     { return "Too Many Parameters (Python)" }
func (r *TooManyParametersPY) Language() string                 { return "python" }
func (r *TooManyParametersPY) Type() domain.IssueType           { return domain.TypeCodeSmell }
func (r *TooManyParametersPY) DefaultSeverity() domain.Severity { return domain.SeverityMajor }
func (r *TooManyParametersPY) Tags() []string                   { return []string{"design", "readability"} }
func (r *TooManyParametersPY) Description() string {
	return "Functions with too many parameters are hard to call correctly and signal a missing abstraction."
}
func (r *TooManyParametersPY) Params() []domain.ParamDef {
	return []domain.ParamDef{
		{Key: "max_params", Description: "Maximum allowed parameter count (excluding self/cls)", DefaultValue: "5", Type: "int"},
	}
}

func (r *TooManyParametersPY) Check(ctx *ollantarules.AnalysisContext) []*domain.Issue {
	maxVal := tsParamInt(ctx.Params, "max_params", 5)
	query := `(function_definition
	  name: (identifier) @fn.name
	  parameters: (parameters) @params
	) @fn`

	matches, err := ctx.Query.Run(ctx.ParsedFile, query, ctx.Grammar)
	if err != nil {
		return nil
	}

	var issues []*domain.Issue
	for _, m := range matches {
		params := m.Captures["params"]
		fn := m.Captures["fn"]
		fnName := m.Captures["fn.name"]
		if params == nil || fn == nil {
			continue
		}

		// Count parameters, skipping self and cls
		paramText := ctx.Query.Text(ctx.ParsedFile, params)
		count := countPythonParams(paramText)
		if count > maxVal {
			line, _, _, _ := ctx.Query.Position(fn)
			issue := domain.NewIssue(r.Key(), ctx.Path, line)
			issue.Severity = r.DefaultSeverity()
			issue.Type = r.Type()
			name := ""
			if fnName != nil {
				name = ctx.Query.Text(ctx.ParsedFile, fnName)
			}
			issue.Message = fmt.Sprintf(
				"Function '%s' has %d parameters (max: %d)",
				name, count, maxVal,
			)
			issues = append(issues, issue)
		}
	}
	return issues
}

// countPythonParams counts the number of parameters in a Python parameter list
// text, excluding self and cls.
func countPythonParams(paramText string) int {
	if paramText == "()" {
		return 0
	}
	// Strip surrounding parentheses
	inner := paramText
	if len(inner) >= 2 && inner[0] == '(' && inner[len(inner)-1] == ')' {
		inner = inner[1 : len(inner)-1]
	}
	if inner == "" {
		return 0
	}
	count := 0
	for _, p := range splitParams(inner) {
		name := trimParamName(p)
		if name == "self" || name == "cls" {
			continue
		}
		count++
	}
	return count
}

// splitParams splits a comma-separated parameter list, respecting brackets.
func splitParams(s string) []string {
	var parts []string
	depth := 0
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	if start < len(s) {
		parts = append(parts, s[start:])
	}
	return parts
}

// trimParamName extracts the bare parameter name from a parameter entry like
// "name: type = default" or "*args" or "**kwargs".
func trimParamName(p string) string {
	// Trim whitespace
	s := ""
	for i := 0; i < len(p); i++ {
		if p[i] != ' ' && p[i] != '\t' && p[i] != '\n' {
			s = p[i:]
			break
		}
	}
	// Strip leading * or **
	for len(s) > 0 && s[0] == '*' {
		s = s[1:]
	}
	// Take up to first non-identifier char
	end := 0
	for end < len(s) {
		c := s[end]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' {
			end++
		} else {
			break
		}
	}
	return s[:end]
}
