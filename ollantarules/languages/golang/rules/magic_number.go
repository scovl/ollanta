package rules

import (
	"fmt"
	"go/ast"
	"go/token"

	"github.com/scovl/ollanta/ollantacore/domain"
	ollantarules "github.com/scovl/ollanta/ollantarules"
)

// MagicNumber flags numeric literals used directly in code outside of constant
// declarations, variable initialisations, and a small set of common neutral values.
// Using named constants instead improves readability and maintainability.
// SonarQube equivalent: squid:S109.
type MagicNumber struct{}

func (r *MagicNumber) Key() string                      { return "go:magic-number" }
func (r *MagicNumber) Name() string                     { return "Magic Number" }
func (r *MagicNumber) Language() string                 { return "go" }
func (r *MagicNumber) Type() domain.IssueType           { return domain.TypeCodeSmell }
func (r *MagicNumber) DefaultSeverity() domain.Severity { return domain.SeverityMinor }
func (r *MagicNumber) Tags() []string                   { return []string{"readability", "convention"} }
func (r *MagicNumber) Description() string {
	return "Numeric literals used directly in expressions should be replaced with named constants to improve readability."
}
func (r *MagicNumber) Params() []domain.ParamDef {
	return []domain.ParamDef{
		{Key: "authorized_numbers", Description: "Comma-separated list of allowed literal values", DefaultValue: "0,1,2,-1", Type: "string"},
	}
}

// allowed are the numeric literals considered neutral and not flagged by default.
var defaultAuthorized = map[string]bool{"0": true, "1": true, "2": true, "-1": true}

func (r *MagicNumber) Check(ctx *ollantarules.AnalysisContext) []*domain.Issue {
	var issues []*domain.Issue

	// Walk all nodes; track whether we're inside a const/var decl.
	ast.Inspect(ctx.AST, func(n ast.Node) bool {
		switch decl := n.(type) {
		case *ast.GenDecl:
			if decl.Tok == token.CONST || decl.Tok == token.VAR {
				return false // skip — literal in const/var is fine
			}
		case *ast.BasicLit:
			if decl.Kind != token.INT && decl.Kind != token.FLOAT {
				return true
			}
			val := decl.Value
			if defaultAuthorized[val] {
				return true
			}
			line := ctx.FileSet.Position(decl.Pos()).Line
			issue := domain.NewIssue(r.Key(), ctx.Path, line)
			issue.Severity = r.DefaultSeverity()
			issue.Type = r.Type()
			issue.Message = fmt.Sprintf("Magic number %s; extract to a named constant", val)
			issues = append(issues, issue)
		}
		return true
	})
	return issues
}
