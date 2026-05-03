// Package rulecatalog exposes the bundled Ollanta rule catalog without loading
// analyzer implementations or tree-sitter bindings.
package rulecatalog

import (
	"sort"

	"github.com/scovl/ollanta/ollantacore/domain"
)

// Language describes a scanner language and whether Ollanta ships bundled rules for it.
type Language struct {
	Key        string `json:"key"`
	Name       string `json:"name"`
	HasParser  bool   `json:"has_parser"`
	HasRules   bool   `json:"has_rules"`
	ParserOnly bool   `json:"parser_only"`
}

var supportedLanguages = []Language{
	{Key: "go", Name: "Go", HasParser: true, HasRules: true},
	{Key: "javascript", Name: "JavaScript", HasParser: true, HasRules: true},
	{Key: "typescript", Name: "TypeScript", HasParser: true, ParserOnly: true},
	{Key: "python", Name: "Python", HasParser: true, HasRules: true},
	{Key: "rust", Name: "Rust", HasParser: true, ParserOnly: true},
}

var builtinRules = []*domain.Rule{
	rule("go:cognitive-complexity", "Cognitive Complexity", "go", domain.TypeCodeSmell, domain.SeverityCritical, []string{"complexity", "readability"}, param("max_complexity", "Maximum allowed cognitive complexity", "15", "int")),
	rule("go:function-nesting-depth", "Function Nesting Depth", "go", domain.TypeCodeSmell, domain.SeverityMajor, []string{"complexity", "readability"}, param("max_depth", "Maximum allowed nesting depth", "4", "int")),
	rule("go:magic-number", "Magic Number", "go", domain.TypeCodeSmell, domain.SeverityMinor, []string{"readability", "convention"}, param("authorized_numbers", "Comma-separated list of allowed literal values", "0,1,2,-1", "string")),
	rule("go:naming-conventions", "Naming Conventions", "go", domain.TypeCodeSmell, domain.SeverityMinor, []string{"convention", "readability"}),
	rule("go:no-large-functions", "No Large Functions", "go", domain.TypeCodeSmell, domain.SeverityMajor, []string{"size", "complexity"}, param("max_lines", "Maximum allowed lines per function", "40", "int")),
	rule("go:no-naked-returns", "No Naked Returns", "go", domain.TypeBug, domain.SeverityCritical, []string{"correctness", "readability"}, param("min_lines", "Minimum function length to flag naked returns", "5", "int")),
	rule("go:todo-comment", "TODO Comment", "go", domain.TypeCodeSmell, domain.SeverityInfo, []string{"convention"}),
	rule("go:too-many-parameters", "Too Many Parameters", "go", domain.TypeCodeSmell, domain.SeverityMajor, []string{"design", "readability"}, param("max_params", "Maximum allowed parameter count", "5", "int")),
	rule("js:eqeqeq", "Strict Equality (JavaScript)", "javascript", domain.TypeBug, domain.SeverityMajor, []string{"correctness", "pitfall"}),
	rule("js:no-console-log", "No console.log (JavaScript)", "javascript", domain.TypeCodeSmell, domain.SeverityMinor, []string{"convention", "debug"}),
	rule("js:no-large-functions", "No Large Functions (JS)", "javascript", domain.TypeCodeSmell, domain.SeverityMajor, []string{"size", "complexity"}, param("max_lines", "Maximum allowed lines per function", "40", "int")),
	rule("js:too-many-parameters", "Too Many Parameters (JavaScript)", "javascript", domain.TypeCodeSmell, domain.SeverityMajor, []string{"design", "readability"}, param("max_params", "Maximum allowed parameter count", "5", "int")),
	rule("py:broad-except", "Broad Exception Catch (Python)", "python", domain.TypeBug, domain.SeverityMajor, []string{"error-handling", "correctness"}),
	rule("py:comparison-to-none", "Comparison to None (Python)", "python", domain.TypeCodeSmell, domain.SeverityMinor, []string{"convention", "correctness"}),
	rule("py:mutable-default-argument", "Mutable Default Argument (Python)", "python", domain.TypeBug, domain.SeverityMajor, []string{"bug", "pitfall"}),
	rule("py:no-large-functions", "No Large Functions (Python)", "python", domain.TypeCodeSmell, domain.SeverityMajor, []string{"size", "complexity"}, param("max_lines", "Maximum allowed lines per function", "40", "int")),
	rule("py:too-many-parameters", "Too Many Parameters (Python)", "python", domain.TypeCodeSmell, domain.SeverityMajor, []string{"design", "readability"}, param("max_params", "Maximum allowed parameter count (excluding self/cls)", "5", "int")),
}

// SupportedLanguages returns all languages known to the scanner.
func SupportedLanguages() []Language {
	out := make([]Language, len(supportedLanguages))
	copy(out, supportedLanguages)
	return out
}

// Rules returns all bundled rule metadata as defensive copies.
func Rules() []*domain.Rule {
	out := make([]*domain.Rule, 0, len(builtinRules))
	for _, rule := range builtinRules {
		out = append(out, cloneRule(rule))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

// ByKey returns a bundled rule by key.
func ByKey(key string) (*domain.Rule, bool) {
	for _, rule := range builtinRules {
		if rule.Key == key {
			return cloneRule(rule), true
		}
	}
	return nil, false
}

// ByLanguage returns bundled rules for the given language.
func ByLanguage(language string) []*domain.Rule {
	out := []*domain.Rule{}
	for _, rule := range builtinRules {
		if rule.Language == language || rule.Language == "*" {
			out = append(out, cloneRule(rule))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

// LanguageByKey returns metadata for a supported language.
func LanguageByKey(key string) (Language, bool) {
	for _, language := range supportedLanguages {
		if language.Key == key {
			return language, true
		}
	}
	return Language{}, false
}

// LanguageHasRules reports whether the language has at least one bundled rule.
func LanguageHasRules(key string) bool {
	language, ok := LanguageByKey(key)
	return ok && language.HasRules
}

// LanguageIsParserOnly reports whether Ollanta can parse the language but ships no rules yet.
func LanguageIsParserOnly(key string) bool {
	language, ok := LanguageByKey(key)
	return ok && language.ParserOnly
}

// DefaultParams returns rule parameter defaults keyed by parameter name.
func DefaultParams(rule *domain.Rule) map[string]string {
	out := make(map[string]string, len(rule.ParamsSchema))
	for key, param := range rule.ParamsSchema {
		out[key] = param.DefaultValue
	}
	return out
}

func rule(key, name, language string, issueType domain.IssueType, severity domain.Severity, tags []string, params ...domain.ParamDef) *domain.Rule {
	schema := make(map[string]domain.ParamDef, len(params))
	for _, p := range params {
		schema[p.Key] = p
	}
	return &domain.Rule{
		Key:             key,
		Name:            name,
		Language:        language,
		Type:            issueType,
		DefaultSeverity: severity,
		Tags:            append([]string(nil), tags...),
		ParamsSchema:    schema,
	}
}

func param(key, description, defaultValue, paramType string) domain.ParamDef {
	return domain.ParamDef{Key: key, Description: description, DefaultValue: defaultValue, Type: paramType}
}

func cloneRule(rule *domain.Rule) *domain.Rule {
	clone := *rule
	clone.Tags = append([]string(nil), rule.Tags...)
	clone.ParamsSchema = make(map[string]domain.ParamDef, len(rule.ParamsSchema))
	for key, param := range rule.ParamsSchema {
		clone.ParamsSchema[key] = param
	}
	return &clone
}
