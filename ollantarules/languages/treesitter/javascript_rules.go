package treesitter

import (
	"fmt"

	"github.com/scovl/ollanta/ollantacore/domain"
	ollantarules "github.com/scovl/ollanta/ollantarules"
)

// NoConsoleLogJS detects calls to console.log, console.warn, console.error, etc.
// left in production JavaScript code. SonarQube equivalent: javascript:S2228.
var NoConsoleLogJS = ollantarules.Rule{
	MetaKey: "js:no-console-log",
	Check: func(ctx *ollantarules.AnalysisContext) []*domain.Issue {
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
			issue := domain.NewIssue("js:no-console-log", ctx.Path, line)
			issue.Severity = domain.SeverityMinor
			issue.Type = domain.TypeCodeSmell
			issue.Message = fmt.Sprintf(
				"Remove this 'console.%s' call from production code", methodName,
			)
			issues = append(issues, issue)
		}
		return issues
	},
}

// EqEqEqJS detects use of == and != instead of === and !==.
// SonarQube equivalent: javascript:S1440.
var EqEqEqJS = ollantarules.Rule{
	MetaKey: "js:eqeqeq",
	Check: func(ctx *ollantarules.AnalysisContext) []*domain.Issue {
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
			issue := domain.NewIssue("js:eqeqeq", ctx.Path, line)
			issue.Severity = domain.SeverityMajor
			issue.Type = domain.TypeBug
			issue.Message = fmt.Sprintf("Use '%s' instead of '%s' to avoid type coercion", better, opText)
			issues = append(issues, issue)
		}
		return issues
	},
}

// TooManyParametersJS flags JavaScript functions with too many parameters.
// SonarQube equivalent: javascript:S107.
var TooManyParametersJS = ollantarules.Rule{
	MetaKey: "js:too-many-parameters",
	Check: func(ctx *ollantarules.AnalysisContext) []*domain.Issue {
		maxVal := ollantarules.ParamInt(ctx.Params, "max_params", 5)
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
			paramText := ctx.Query.Text(ctx.ParsedFile, params)
			count := countJSParams(paramText)
			if count > maxVal {
				line, _, _, _ := ctx.Query.Position(fn)
				name := "anonymous"
				if fnName := m.Captures["fn.name"]; fnName != nil {
					name = ctx.Query.Text(ctx.ParsedFile, fnName)
				}
				issue := domain.NewIssue("js:too-many-parameters", ctx.Path, line)
				issue.Severity = domain.SeverityMajor
				issue.Type = domain.TypeCodeSmell
				issue.Message = fmt.Sprintf("Function '%s' has %d parameters (max: %d)", name, count, maxVal)
				issues = append(issues, issue)
			}
		}
		return issues
	},
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
