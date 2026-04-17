// Package rules provides Go-specific static analysis rules for use with GoSensor.
package rules

import (
	"fmt"
	"go/ast"
	"strings"
	"unicode"

	"github.com/scovl/ollanta/ollantacore/domain"
	ollantarules "github.com/scovl/ollanta/ollantarules"
)

// NamingConventions detects Go identifiers that violate standard naming conventions:
// exported names must be MixedCaps; unexported names must not contain underscores
// (except for test functions and blank identifiers).
type NamingConventions struct{}

func (r *NamingConventions) Key() string                      { return "go:naming-conventions" }
func (r *NamingConventions) Name() string                     { return "Naming Conventions" }
func (r *NamingConventions) Language() string                 { return "go" }
func (r *NamingConventions) Type() domain.IssueType           { return domain.TypeCodeSmell }
func (r *NamingConventions) DefaultSeverity() domain.Severity { return domain.SeverityMinor }
func (r *NamingConventions) Tags() []string                   { return []string{"convention", "readability"} }
func (r *NamingConventions) Description() string {
	return "Go naming conventions: exported identifiers must use MixedCaps; underscores in names violate Effective Go guidelines."
}
func (r *NamingConventions) Params() []domain.ParamDef { return nil }

func (r *NamingConventions) Check(ctx *ollantarules.AnalysisContext) []*domain.Issue {
	var issues []*domain.Issue

	ast.Inspect(ctx.AST, func(n ast.Node) bool {
		switch decl := n.(type) {
		case *ast.FuncDecl:
			name := decl.Name.Name
			if name == "_" || strings.HasPrefix(name, "Test") || strings.HasPrefix(name, "Benchmark") || strings.HasPrefix(name, "Example") {
				return true
			}
			if violation := checkName(name); violation != "" {
				line := lineOf(ctx.FileSet, decl.Name.Pos())
				issue := domain.NewIssue(r.Key(), ctx.Path, line)
				issue.Severity = r.DefaultSeverity()
				issue.Type = r.Type()
				issue.Message = fmt.Sprintf("Function '%s' violates naming convention: %s", name, violation)
				issues = append(issues, issue)
			}
		case *ast.TypeSpec:
			name := decl.Name.Name
			if violation := checkName(name); violation != "" {
				line := lineOf(ctx.FileSet, decl.Name.Pos())
				issue := domain.NewIssue(r.Key(), ctx.Path, line)
				issue.Severity = r.DefaultSeverity()
				issue.Type = r.Type()
				issue.Message = fmt.Sprintf("Type '%s' violates naming convention: %s", name, violation)
				issues = append(issues, issue)
			}
		}
		return true
	})
	return issues
}

// checkName returns a non-empty violation string if the name breaks Go conventions.
func checkName(name string) string {
	if name == "_" {
		return ""
	}
	// Any name with an underscore (except leading _) is a violation.
	// Acronyms like HTTP, URL, ID are fine (all caps, no underscore).
	if strings.Contains(name, "_") {
		return "name contains underscore; use MixedCaps instead"
	}
	// Exported names starting with a lowercase letter.
	if len(name) > 0 && unicode.IsUpper(rune(name[0])) {
		// ALL_CAPS pattern (has uppercase after underscore if we stripped it, or purely uppercase with length > 1 non-acronym)
		// Simplified: if all chars are uppercase or digit — it's ALL_CAPS style without underscore caught above.
		allUpper := true
		for _, c := range name {
			if !unicode.IsUpper(c) && !unicode.IsDigit(c) {
				allUpper = false
				break
			}
		}
		if allUpper && len(name) > 1 {
			return "use MixedCaps instead of ALL_CAPS"
		}
	}
	return ""
}
