package customrules

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/scovl/ollanta/domain/model"
	"github.com/scovl/ollanta/ollantacore/rulecatalog"
	"gopkg.in/yaml.v3"
)

const (
	fieldEngineConfigPattern = "engine_config.pattern"
	fieldEngineConfigTarget  = "engine_config.target"
	fieldEngineConfigQuery   = "engine_config.query"
)

type ValidationContext struct {
	ExistingRuleKeys       map[string]bool
	AllowExistingRuleKey   string
	EngineBackedValidation bool
}

func EngineCapabilities() []model.CustomRuleEngineCapability {
	return []model.CustomRuleEngineCapability{
		{
			Engine:                model.CustomRuleEngineText,
			Name:                  "Text pattern",
			Languages:             supportedLanguageKeys(),
			RequiredFields:        []string{fieldEngineConfigPattern},
			SupportedPatterns:     []string{"regexp"},
			SupportsExampleTests:  true,
			SupportsSourcePreview: true,
			DefaultLimits:         defaultRuleLimits(),
		},
		{
			Engine:                model.CustomRuleEngineGoAST,
			Name:                  "Go AST pattern",
			Languages:             []string{model.LangGo},
			RequiredFields:        []string{fieldEngineConfigPattern, fieldEngineConfigTarget},
			SupportedPatterns:     []string{"forbidden_call", "forbidden_import"},
			SupportsExampleTests:  true,
			SupportsSourcePreview: true,
			DefaultLimits:         defaultRuleLimits(),
		},
		{
			Engine:                model.CustomRuleEngineTreeSitter,
			Name:                  "Tree-sitter query",
			Languages:             supportedLanguageKeys(),
			RequiredFields:        []string{fieldEngineConfigQuery},
			SupportedPatterns:     []string{"query"},
			RequiresRuntime:       true,
			SupportsExampleTests:  true,
			SupportsSourcePreview: true,
			DefaultLimits:         defaultRuleLimits(),
		},
	}
}

func DecodeDocument(data []byte) (model.CustomRulePackDocument, error) {
	var doc model.CustomRulePackDocument
	jsonDecoder := json.NewDecoder(bytes.NewReader(data))
	jsonDecoder.DisallowUnknownFields()
	if err := jsonDecoder.Decode(&doc); err == nil {
		return normalizeDocument(doc), nil
	}
	yamlDecoder := yaml.NewDecoder(bytes.NewReader(data))
	yamlDecoder.KnownFields(true)
	if err := yamlDecoder.Decode(&doc); err != nil {
		return model.CustomRulePackDocument{}, err
	}
	return normalizeDocument(doc), nil
}

func EncodeDocument(doc model.CustomRulePackDocument) ([]byte, error) {
	doc = normalizeDocument(doc)
	return json.MarshalIndent(doc, "", "  ")
}

func ValidateDocument(doc model.CustomRulePackDocument, ctx ValidationContext) (model.CustomRulePackDocument, model.CustomRuleValidationResult) {
	doc = normalizeDocument(doc)
	result := model.CustomRuleValidationResult{Status: model.CustomRuleValidationPassed, CheckedAt: time.Now().UTC(), ValidatorCapabilities: validatorCapabilities(ctx.EngineBackedValidation)}
	if doc.Version != model.CustomRulePackSchemaVersion {
		result.Diagnostics = append(result.Diagnostics, diagnostic("error", "unsupported_schema_version", "version", fmt.Sprintf("unsupported custom rule pack schema version %d", doc.Version)))
	}
	if strings.TrimSpace(doc.Pack.Name) == "" {
		result.Diagnostics = append(result.Diagnostics, diagnostic("error", "pack_name_required", "pack.name", "rule pack name is required"))
	}
	if len(doc.Rules) == 0 {
		result.Diagnostics = append(result.Diagnostics, diagnostic("error", "rules_required", "rules", "at least one rule is required"))
	}
	seen := map[string]bool{}
	for index, rule := range doc.Rules {
		fieldPrefix := "rules[" + strconv.Itoa(index) + "]"
		ruleResult := ValidateDefinition(rule, ctx)
		for _, item := range ruleResult.Diagnostics {
			item.Field = qualifyField(fieldPrefix, item.Field)
			result.Diagnostics = append(result.Diagnostics, item)
		}
		if seen[rule.RuleKey] {
			result.Diagnostics = append(result.Diagnostics, diagnostic("error", "duplicate_rule_key", fieldPrefix+".key", "duplicate rule key in document"))
		}
		seen[rule.RuleKey] = true
	}
	result.Status = statusFromDiagnostics(result.Diagnostics)
	return doc, result
}

func ValidateDefinition(rule model.CustomRuleDefinition, ctx ValidationContext) model.CustomRuleValidationResult {
	rule = model.NormalizeCustomRuleDefinition(rule)
	rule.VersionHash = model.HashCustomRuleDefinition(rule)
	result := model.CustomRuleValidationResult{
		RuleKey:               rule.RuleKey,
		VersionHash:           rule.VersionHash,
		Status:                model.CustomRuleValidationPassed,
		CheckedAt:             time.Now().UTC(),
		ValidatorCapabilities: validatorCapabilities(ctx.EngineBackedValidation),
	}
	diagnostics := validateRuleMetadata(rule, ctx)
	diagnostics = append(diagnostics, validateRuleEngine(rule, ctx)...)
	if len(diagnostics) == 0 {
		diagnostics = append(diagnostics, validateExamples(rule)...)
	}
	result.Diagnostics = diagnostics
	result.Status = statusFromDiagnostics(diagnostics)
	return result
}

func Evaluate(rule model.CustomRuleDefinition, filePath string, source []byte) ([]model.CustomRulePreviewMatch, []model.CustomRuleDiagnostic) {
	rule = model.NormalizeCustomRuleDefinition(rule)
	if exceededFileLimit(rule, source) {
		return nil, []model.CustomRuleDiagnostic{diagnostic("warning", "file_too_large", "limits.max_file_bytes", "file skipped because it exceeds the custom rule file size limit")}
	}
	switch effectiveEngine(rule) {
	case model.CustomRuleEngineText:
		return evaluateTextRule(rule, filePath, source)
	case model.CustomRuleEngineGoAST:
		return evaluateGoASTRule(rule, filePath, source)
	default:
		return nil, []model.CustomRuleDiagnostic{diagnostic("error", "unsupported_engine_runtime", "engine", "engine requires scanner-specific runtime support")}
	}
}

func ToIssue(rule model.CustomRuleDefinition, match model.CustomRulePreviewMatch) *model.Issue {
	message := rule.Message
	if message == "" {
		message = rule.Name
	}
	issue := model.NewIssue(rule.RuleKey, match.FilePath, match.Line)
	issue.Column = match.Column
	issue.Message = message
	issue.Type = rule.Type
	issue.Severity = rule.DefaultSeverity
	issue.Tags = append([]string(nil), rule.Tags...)
	issue.EngineID = "custom:" + string(rule.Engine)
	issue.QualityDomain = model.DeriveIssueQualityDomain(issue.Type, issue.Tags)
	return issue
}

func normalizeDocument(doc model.CustomRulePackDocument) model.CustomRulePackDocument {
	if doc.Version == 0 {
		doc.Version = model.CustomRulePackSchemaVersion
	}
	doc.Pack.Namespace = strings.TrimSpace(strings.ToLower(doc.Pack.Namespace))
	for index := range doc.Rules {
		doc.Rules[index].RuleKey = model.NormalizeCustomRuleKey(doc.Pack.Namespace, doc.Rules[index].RuleKey)
		doc.Rules[index] = model.NormalizeCustomRuleDefinition(doc.Rules[index])
		doc.Rules[index].VersionHash = model.HashCustomRuleDefinition(doc.Rules[index])
	}
	sort.Slice(doc.Rules, func(left, right int) bool { return doc.Rules[left].RuleKey < doc.Rules[right].RuleKey })
	doc.Pack.SourceHash = model.HashCustomRuleCatalog(doc.Rules)
	return doc
}

func validateRuleMetadata(rule model.CustomRuleDefinition, ctx ValidationContext) []model.CustomRuleDiagnostic {
	diagnostics := []model.CustomRuleDiagnostic{}
	if rule.RuleKey == "" {
		diagnostics = append(diagnostics, diagnostic("error", "key_required", "key", "rule key is required"))
	}
	if _, ok := rulecatalog.ByKey(rule.RuleKey); ok {
		diagnostics = append(diagnostics, diagnostic("error", "bundled_key_collision", "key", "custom rule key conflicts with a bundled rule"))
	}
	if ctx.ExistingRuleKeys[rule.RuleKey] && rule.RuleKey != ctx.AllowExistingRuleKey {
		diagnostics = append(diagnostics, diagnostic("error", "custom_key_collision", "key", "custom rule key is already published"))
	}
	if strings.TrimSpace(rule.Name) == "" {
		diagnostics = append(diagnostics, diagnostic("error", "name_required", "name", "rule name is required"))
	}
	if _, ok := rulecatalog.LanguageByKey(rule.Language); !ok {
		diagnostics = append(diagnostics, diagnostic("error", "unsupported_language", "language", "rule language is not supported by Ollanta"))
	}
	if !validIssueType(rule.Type) {
		diagnostics = append(diagnostics, diagnostic("error", "invalid_type", "type", "rule type must be bug, vulnerability, security_hotspot, or code_smell"))
	}
	if !validSeverity(rule.DefaultSeverity) {
		diagnostics = append(diagnostics, diagnostic("error", "invalid_severity", "severity", "severity must be blocker, critical, major, minor, or info"))
	}
	for key, param := range rule.ParamsSchema {
		if key == "" || param.Key == "" || key != param.Key {
			diagnostics = append(diagnostics, diagnostic("error", "invalid_param_key", "params_schema."+key, "parameter map key and parameter key must match"))
		}
		if !validParamType(param.Type) {
			diagnostics = append(diagnostics, diagnostic("error", "invalid_param_type", "params_schema."+key+".type", "parameter type must be int, float, bool, or string"))
		}
	}
	return diagnostics
}

func validateRuleEngine(rule model.CustomRuleDefinition, ctx ValidationContext) []model.CustomRuleDiagnostic {
	diagnostics := rejectExecutableFields(rule.EngineConfig)
	switch effectiveEngine(rule) {
	case model.CustomRuleEngineText:
		diagnostics = append(diagnostics, validateTextEngine(rule)...)
	case model.CustomRuleEngineGoAST:
		diagnostics = append(diagnostics, validateGoASTEngine(rule)...)
	case model.CustomRuleEngineTreeSitter:
		diagnostics = append(diagnostics, validateTreeSitterEngine(rule, ctx)...)
	default:
		diagnostics = append(diagnostics, diagnostic("error", "unsupported_engine", "engine", "custom rule engine is not supported"))
	}
	return diagnostics
}

func validateTextEngine(rule model.CustomRuleDefinition) []model.CustomRuleDiagnostic {
	pattern := strings.TrimSpace(rule.EngineConfig["pattern"])
	if pattern == "" {
		return []model.CustomRuleDiagnostic{diagnostic("error", "pattern_required", fieldEngineConfigPattern, "text rules require a regexp pattern")}
	}
	if _, err := regexp.Compile(pattern); err != nil {
		return []model.CustomRuleDiagnostic{diagnostic("error", "invalid_regexp", fieldEngineConfigPattern, err.Error())}
	}
	return nil
}

func validateGoASTEngine(rule model.CustomRuleDefinition) []model.CustomRuleDiagnostic {
	diagnostics := []model.CustomRuleDiagnostic{}
	if rule.Language != model.LangGo {
		diagnostics = append(diagnostics, diagnostic("error", "go_ast_language", "language", "go-ast rules require language go"))
	}
	pattern := rule.EngineConfig["pattern"]
	if pattern != "forbidden_call" && pattern != "forbidden_import" {
		diagnostics = append(diagnostics, diagnostic("error", "unsupported_go_ast_pattern", fieldEngineConfigPattern, "go-ast pattern must be forbidden_call or forbidden_import"))
	}
	if strings.TrimSpace(rule.EngineConfig["target"]) == "" {
		diagnostics = append(diagnostics, diagnostic("error", "target_required", fieldEngineConfigTarget, "go-ast rules require a target"))
	}
	return diagnostics
}

func validateTreeSitterEngine(rule model.CustomRuleDefinition, ctx ValidationContext) []model.CustomRuleDiagnostic {
	diagnostics := []model.CustomRuleDiagnostic{}
	if strings.TrimSpace(rule.EngineConfig["query"]) == "" {
		diagnostics = append(diagnostics, diagnostic("error", "query_required", fieldEngineConfigQuery, "tree-sitter rules require a query"))
	}
	if !ctx.EngineBackedValidation {
		diagnostics = append(diagnostics, diagnostic("error", "runtime_validation_required", "engine", "tree-sitter query validation must run in a scanner-capable runtime"))
	}
	return diagnostics
}

func validateExamples(rule model.CustomRuleDefinition) []model.CustomRuleDiagnostic {
	if len(rule.Examples) == 0 {
		return []model.CustomRuleDiagnostic{diagnostic("error", "examples_required", "examples", "at least one example is required")}
	}
	diagnostics := []model.CustomRuleDiagnostic{}
	for index, example := range rule.Examples {
		field := exampleField(index)
		if example.Code == "" {
			diagnostics = append(diagnostics, diagnostic("error", "example_code_required", field+".code", "example code is required"))
			continue
		}
		if effectiveEngine(rule) == model.CustomRuleEngineTreeSitter {
			continue
		}
		matches, evalDiagnostics := Evaluate(rule, example.Name, []byte(example.Code))
		for _, evalDiagnostic := range evalDiagnostics {
			evalDiagnostic.Field = field
			diagnostics = append(diagnostics, evalDiagnostic)
		}
		if !example.Compliant && len(matches) == 0 {
			diagnostics = append(diagnostics, diagnostic("error", "noncompliant_example_missed", field, "noncompliant example did not produce an issue"))
		}
		if example.Compliant && len(matches) > 0 {
			diagnostics = append(diagnostics, diagnostic("error", "compliant_example_matched", field, "compliant example produced an issue"))
		}
	}
	return diagnostics
}

func exampleField(index int) string {
	return "examples[" + strconv.Itoa(index) + "]"
}

func evaluateTextRule(rule model.CustomRuleDefinition, filePath string, source []byte) ([]model.CustomRulePreviewMatch, []model.CustomRuleDiagnostic) {
	compiled, err := regexp.Compile(rule.EngineConfig["pattern"])
	if err != nil {
		return nil, []model.CustomRuleDiagnostic{diagnostic("error", "invalid_regexp", fieldEngineConfigPattern, err.Error())}
	}
	limits := effectiveLimits(rule.Limits)
	matches := []model.CustomRulePreviewMatch{}
	for _, location := range compiled.FindAllIndex(source, limits.MaxMatches) {
		line, column := offsetToLineColumn(source, location[0])
		matches = append(matches, previewMatch(rule, filePath, line, column, lineSnippet(source, line)))
	}
	return matches, nil
}

func evaluateGoASTRule(rule model.CustomRuleDefinition, filePath string, source []byte) ([]model.CustomRulePreviewMatch, []model.CustomRuleDiagnostic) {
	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, filePath, source, parser.AllErrors)
	if err != nil {
		return nil, []model.CustomRuleDiagnostic{diagnostic("error", "go_parse_error", "examples", err.Error())}
	}
	switch rule.EngineConfig["pattern"] {
	case "forbidden_import":
		return evaluateForbiddenImport(rule, fileSet, parsed, filePath, source), nil
	case "forbidden_call":
		return evaluateForbiddenCall(rule, fileSet, parsed, filePath, source), nil
	default:
		return nil, []model.CustomRuleDiagnostic{diagnostic("error", "unsupported_go_ast_pattern", fieldEngineConfigPattern, "go-ast pattern must be forbidden_call or forbidden_import")}
	}
}

func evaluateForbiddenImport(rule model.CustomRuleDefinition, fileSet *token.FileSet, parsed *ast.File, filePath string, source []byte) []model.CustomRulePreviewMatch {
	target := strings.Trim(rule.EngineConfig["target"], "\"")
	matches := []model.CustomRulePreviewMatch{}
	for _, importSpec := range parsed.Imports {
		path := strings.Trim(importSpec.Path.Value, "\"")
		if path != target {
			continue
		}
		position := fileSet.Position(importSpec.Pos())
		matches = append(matches, previewMatch(rule, filePath, position.Line, position.Column, lineSnippet(source, position.Line)))
	}
	return matches
}

func evaluateForbiddenCall(rule model.CustomRuleDefinition, fileSet *token.FileSet, parsed *ast.File, filePath string, source []byte) []model.CustomRulePreviewMatch {
	target := strings.TrimSpace(rule.EngineConfig["target"])
	matches := []model.CustomRulePreviewMatch{}
	ast.Inspect(parsed, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		if callName(call.Fun) != target {
			return true
		}
		position := fileSet.Position(call.Pos())
		matches = append(matches, previewMatch(rule, filePath, position.Line, position.Column, lineSnippet(source, position.Line)))
		return true
	})
	return matches
}

func callName(expr ast.Expr) string {
	switch value := expr.(type) {
	case *ast.Ident:
		return value.Name
	case *ast.SelectorExpr:
		prefix := callName(value.X)
		if prefix == "" {
			return value.Sel.Name
		}
		return prefix + "." + value.Sel.Name
	default:
		return ""
	}
}

func previewMatch(rule model.CustomRuleDefinition, filePath string, line, column int, snippet string) model.CustomRulePreviewMatch {
	message := rule.Message
	if message == "" {
		message = rule.Name
	}
	return model.CustomRulePreviewMatch{FilePath: filepath.ToSlash(filePath), Line: line, Column: column, Message: message, Snippet: snippet}
}

func effectiveEngine(rule model.CustomRuleDefinition) model.CustomRuleEngine {
	if rule.Engine == model.CustomRuleEngineAuto {
		if rule.Language == model.LangGo && (rule.EngineConfig["pattern"] == "forbidden_call" || rule.EngineConfig["pattern"] == "forbidden_import") {
			return model.CustomRuleEngineGoAST
		}
		return model.CustomRuleEngineText
	}
	return rule.Engine
}

func effectiveLimits(limits model.CustomRuleLimits) model.CustomRuleLimits {
	defaults := defaultRuleLimits()
	if limits.MaxFileBytes <= 0 {
		limits.MaxFileBytes = defaults.MaxFileBytes
	}
	if limits.MaxMatches <= 0 {
		limits.MaxMatches = defaults.MaxMatches
	}
	if limits.MaxFiles <= 0 {
		limits.MaxFiles = defaults.MaxFiles
	}
	if limits.TimeoutMs <= 0 {
		limits.TimeoutMs = defaults.TimeoutMs
	}
	return limits
}

func exceededFileLimit(rule model.CustomRuleDefinition, source []byte) bool {
	return len(source) > effectiveLimits(rule.Limits).MaxFileBytes
}

func defaultRuleLimits() model.CustomRuleLimits {
	return model.CustomRuleLimits{MaxFileBytes: 512 * 1024, MaxMatches: 100, MaxFiles: 500, TimeoutMs: 1000}
}

func offsetToLineColumn(source []byte, offset int) (int, int) {
	line := 1
	column := 1
	for index := 0; index < len(source) && index < offset; index++ {
		if source[index] == '\n' {
			line++
			column = 1
			continue
		}
		column++
	}
	return line, column
}

func lineSnippet(source []byte, line int) string {
	if line <= 0 {
		return ""
	}
	lines := strings.Split(string(source), "\n")
	if line > len(lines) {
		return ""
	}
	return strings.TrimSpace(lines[line-1])
}

func statusFromDiagnostics(diagnostics []model.CustomRuleDiagnostic) model.CustomRuleValidationStatus {
	requiresRuntime := false
	for _, item := range diagnostics {
		if item.Code == "runtime_validation_required" {
			requiresRuntime = true
			continue
		}
		if item.Level == "error" {
			return model.CustomRuleValidationFailed
		}
	}
	if requiresRuntime {
		return model.CustomRuleValidationRequiresRuntime
	}
	return model.CustomRuleValidationPassed
}

func validatorCapabilities(engineBacked bool) []string {
	capabilities := []string{"schema", "metadata", "text", "go-ast", "examples"}
	if engineBacked {
		capabilities = append(capabilities, "tree-sitter")
	}
	return capabilities
}

func rejectExecutableFields(fields map[string]string) []model.CustomRuleDiagnostic {
	diagnostics := []model.CustomRuleDiagnostic{}
	for key := range fields {
		lower := strings.ToLower(key)
		if lower == "script" || lower == "command" || lower == "shell" || lower == "plugin" || lower == "wasm" || lower == "executable" {
			diagnostics = append(diagnostics, diagnostic("error", "executable_payload", "engine_config."+key, "custom rules cannot provide executable payloads"))
		}
	}
	return diagnostics
}

func supportedLanguageKeys() []string {
	languages := rulecatalog.SupportedLanguages()
	out := make([]string, 0, len(languages))
	for _, language := range languages {
		out = append(out, language.Key)
	}
	sort.Strings(out)
	return out
}

func validIssueType(issueType model.IssueType) bool {
	switch issueType {
	case model.TypeBug, model.TypeVulnerability, model.TypeCodeSmell, model.TypeSecurityHotspot:
		return true
	default:
		return false
	}
}

func validSeverity(severity model.Severity) bool {
	switch severity {
	case model.SeverityBlocker, model.SeverityCritical, model.SeverityMajor, model.SeverityMinor, model.SeverityInfo:
		return true
	default:
		return false
	}
}

func validParamType(paramType string) bool {
	switch paramType {
	case "", "int", "float", "bool", "string":
		return true
	default:
		return false
	}
}

func diagnostic(level, code, field, message string) model.CustomRuleDiagnostic {
	return model.CustomRuleDiagnostic{Level: level, Code: code, Field: field, Message: message}
}

func qualifyField(prefix, field string) string {
	if field == "" {
		return prefix
	}
	return prefix + "." + field
}

func Validate(ctx context.Context, rule model.CustomRuleDefinition, validationContext ValidationContext) model.CustomRuleValidationResult {
	select {
	case <-ctx.Done():
		return model.CustomRuleValidationResult{RuleKey: rule.RuleKey, Status: model.CustomRuleValidationFailed, Diagnostics: []model.CustomRuleDiagnostic{diagnostic("error", "validation_cancelled", "", ctx.Err().Error())}}
	default:
		return ValidateDefinition(rule, validationContext)
	}
}
