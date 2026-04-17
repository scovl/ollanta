package treesitter

import (
	"fmt"

	"github.com/scovl/ollanta/ollantacore/domain"
	ollantarules "github.com/scovl/ollanta/ollantarules"
)

// NoConsoleLogJS detects calls to console.log, console.warn, console.error, etc.
// left in production JavaScript code. SonarQube equivalent: javascript:S2228.
type NoConsoleLogJS struct{}

func (r *NoConsoleLogJS) Key() string                      { return "js:no-console-log" }
func (r *NoConsoleLogJS) Name() string                     { return "No console.log (JavaScript)" }
func (r *NoConsoleLogJS) Language() string                 { return "javascript" }
func (r *NoConsoleLogJS) Type() domain.IssueType           { return domain.TypeCodeSmell }
func (r *NoConsoleLogJS) DefaultSeverity() domain.Severity { return domain.SeverityMinor }
func (r *NoConsoleLogJS) Tags() []string                   { return []string{"convention", "debug"} }
func (r *NoConsoleLogJS) Description() string {
	return "console.log and friends should not be left in production code. Use a proper logging library."
}
func (r *NoConsoleLogJS) Params() []domain.ParamDef { return nil }

func (r *NoConsoleLogJS) Check(ctx *ollantarules.AnalysisContext) []*domain.Issue {
	query := `(call_expression
	  function: (member_expression
	    object: (identifier) @obj
	    property: (property_identifier) @method)
	) @call`

	matches, err := ctx.Query.Run(ctx.ParsedFile, query, ctx.Grammar)
	if err != nil {
		return nil
	}

	consoleMethods := map[string]bool{
		"log": true, "warn": true, "error": true, "info": true,
		"debug": true, "trace": true, "dir": true,
	}

	var issues []*domain.Issue
	for _, m := range matches {
		obj := m.Captures["obj"]
		method := m.Captures["method"]
		call := m.Captures["call"]
		if obj == nil || method == nil || call == nil {
			continue
		}
		if ctx.Query.Text(ctx.ParsedFile, obj) != "console" {
			continue
		}
		methodName := ctx.Query.Text(ctx.ParsedFile, method)
		if !consoleMethods[methodName] {
			continue
		}
		line, _, _, _ := ctx.Query.Position(call)
		issue := domain.NewIssue(r.Key(), ctx.Path, line)
		issue.Severity = r.DefaultSeverity()
		issue.Type = r.Type()
		issue.Message = fmt.Sprintf(
			"Remove this 'console.%s' call from production code", methodName,
		)
		issues = append(issues, issue)
	}
	return issues
}

// EqEqEqJS detects use of == and != instead of === and !==.
// SonarQube equivalent: javascript:S1440.
type EqEqEqJS struct{}

func (r *EqEqEqJS) Key() string                      { return "js:eqeqeq" }
func (r *EqEqEqJS) Name() string                     { return "Strict Equality (JavaScript)" }
func (r *EqEqEqJS) Language() string                 { return "javascript" }
func (r *EqEqEqJS) Type() domain.IssueType           { return domain.TypeBug }
func (r *EqEqEqJS) DefaultSeverity() domain.Severity { return domain.SeverityMajor }
func (r *EqEqEqJS) Tags() []string                   { return []string{"correctness", "pitfall"} }
func (r *EqEqEqJS) Description() string {
	return "Use === and !== instead of == and != to avoid type coercion bugs in JavaScript."
}
func (r *EqEqEqJS) Params() []domain.ParamDef { return nil }

func (r *EqEqEqJS) Check(ctx *ollantarules.AnalysisContext) []*domain.Issue {
	query := `(binary_expression
	  operator: _ @op
	) @expr`

	matches, err := ctx.Query.Run(ctx.ParsedFile, query, ctx.Grammar)
	if err != nil {
		return nil
	}

	var issues []*domain.Issue
	for _, m := range matches {
		op := m.Captures["op"]
		expr := m.Captures["expr"]
		if op == nil || expr == nil {
			continue
		}
		opText := ctx.Query.Text(ctx.ParsedFile, op)
		if opText != "==" && opText != "!=" {
			continue
		}
		better := "==="
		if opText == "!=" {
			better = "!=="
		}
		line, _, _, _ := ctx.Query.Position(expr)
		issue := domain.NewIssue(r.Key(), ctx.Path, line)
		issue.Severity = r.DefaultSeverity()
		issue.Type = r.Type()
		issue.Message = fmt.Sprintf("Use '%s' instead of '%s' to avoid type coercion", better, opText)
		issues = append(issues, issue)
	}
	return issues
}

// TooManyParametersJS flags JavaScript functions with too many parameters.
// SonarQube equivalent: javascript:S107.
type TooManyParametersJS struct{}

func (r *TooManyParametersJS) Key() string                      { return "js:too-many-parameters" }
func (r *TooManyParametersJS) Name() string                     { return "Too Many Parameters (JavaScript)" }
func (r *TooManyParametersJS) Language() string                 { return "javascript" }
func (r *TooManyParametersJS) Type() domain.IssueType           { return domain.TypeCodeSmell }
func (r *TooManyParametersJS) DefaultSeverity() domain.Severity { return domain.SeverityMajor }
func (r *TooManyParametersJS) Tags() []string                   { return []string{"design", "readability"} }
func (r *TooManyParametersJS) Description() string {
	return "Functions with too many parameters are hard to call correctly and signal a missing abstraction."
}
func (r *TooManyParametersJS) Params() []domain.ParamDef {
	return []domain.ParamDef{
		{Key: "max_params", Description: "Maximum allowed parameter count", DefaultValue: "5", Type: "int"},
	}
}

func (r *TooManyParametersJS) Check(ctx *ollantarules.AnalysisContext) []*domain.Issue {
	maxVal := tsParamInt(ctx.Params, "max_params", 5)
	query := `[
	  (function_declaration
	    name: (identifier) @fn.name
	    parameters: (formal_parameters) @params) @fn
	  (function_expression
	    parameters: (formal_parameters) @params) @fn
	  (arrow_function
	    parameters: (formal_parameters) @params) @fn
	]`

	matches, err := ctx.Query.Run(ctx.ParsedFile, query, ctx.Grammar)
	if err != nil {
		return nil
	}

	var issues []*domain.Issue
	for _, m := range matches {
		params := m.Captures["params"]
		fn := m.Captures["fn"]
		if params == nil || fn == nil {
			continue
		}
		// Count top-level parameters by counting commas + 1 inside formal_parameters
		paramText := ctx.Query.Text(ctx.ParsedFile, params)
		count := countJSParams(paramText)
		if count > maxVal {
			line, _, _, _ := ctx.Query.Position(fn)
			name := "anonymous"
			if fnName := m.Captures["fn.name"]; fnName != nil {
				name = ctx.Query.Text(ctx.ParsedFile, fnName)
			}
			issue := domain.NewIssue(r.Key(), ctx.Path, line)
			issue.Severity = r.DefaultSeverity()
			issue.Type = r.Type()
			issue.Message = fmt.Sprintf("Function '%s' has %d parameters (max: %d)", name, count, maxVal)
			issues = append(issues, issue)
		}
	}
	return issues
}

// countJSParams counts top-level parameters in a JS formal_parameters string like "(a, b, c)".
func countJSParams(s string) int {
	parts := splitParams(s)
	count := 0
	for _, p := range parts {
		t := trimParamName(p)
		if t != "" {
			count++
		}
	}
	return count
}
