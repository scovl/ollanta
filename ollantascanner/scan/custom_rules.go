package scan

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	appcustom "github.com/scovl/ollanta/application/customrules"
	appscan "github.com/scovl/ollanta/application/scan"
	"github.com/scovl/ollanta/domain/model"
	"github.com/scovl/ollanta/domain/port"
	"github.com/scovl/ollanta/ollantacore/rulecatalog"
	"github.com/scovl/ollanta/ollantaparser"
	sitter "github.com/smacker/go-tree-sitter"
)

type customAnalyzerBridge struct {
	rule model.CustomRuleDefinition
}

var _ port.IAnalyzer = (*customAnalyzerBridge)(nil)

func loadCustomAnalyzerBridges(ctx context.Context, opts *ScanOptions) ([]port.IAnalyzer, error) {
	rules, sources, err := loadCustomRuleDefinitions(ctx, opts)
	if err != nil {
		return nil, err
	}
	if opts != nil {
		opts.CustomRules = appscan.CustomRuleOptions{
			CatalogHash: model.HashCustomRuleCatalog(rules),
			Rules:       rules,
			Sources:     sources,
		}
	}
	analyzers := make([]port.IAnalyzer, 0, len(rules))
	for _, rule := range rules {
		analyzers = append(analyzers, &customAnalyzerBridge{rule: rule})
	}
	return analyzers, nil
}

func loadCustomRuleDefinitions(ctx context.Context, opts *ScanOptions) ([]model.CustomRuleDefinition, []string, error) {
	if opts == nil {
		return nil, nil, nil
	}
	seen := bundledRuleKeys()
	rules := []model.CustomRuleDefinition{}
	sources := []string{}
	serverRules, err := fetchServerCustomCatalog(ctx, opts)
	if err != nil && shouldRequireServerCustomCatalog(opts) {
		return nil, nil, err
	}
	for _, rule := range serverRules {
		if seen[rule.RuleKey] {
			return nil, nil, fmt.Errorf("duplicate custom rule key %q", rule.RuleKey)
		}
		seen[rule.RuleKey] = true
		rules = append(rules, rule)
	}
	if len(serverRules) > 0 {
		sources = append(sources, "server")
	}
	localRules, localSources, err := loadLocalRulePacks(opts.ProjectDir, seen)
	if err != nil {
		return nil, nil, err
	}
	rules = append(rules, localRules...)
	sources = append(sources, localSources...)
	sort.Slice(rules, func(left, right int) bool { return rules[left].RuleKey < rules[right].RuleKey })
	return rules, sources, nil
}

func shouldRequireServerCustomCatalog(opts *ScanOptions) bool {
	if opts == nil || opts.Server == "" {
		return false
	}
	return opts.Profiles.Strict || strings.EqualFold(opts.Profiles.Source, appscan.ProfileSourceServer)
}

func bundledRuleKeys() map[string]bool {
	seen := map[string]bool{}
	for _, rule := range rulecatalog.Rules() {
		seen[rule.Key] = true
	}
	return seen
}

func fetchServerCustomCatalog(ctx context.Context, opts *ScanOptions) ([]model.CustomRuleDefinition, error) {
	if opts.Server == "" {
		return nil, nil
	}
	timeout := opts.Profiles.FetchTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	endpoint := strings.TrimRight(opts.Server, "/") + "/api/v1/custom-rules/catalog"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build custom rule catalog request: %w", err)
	}
	if opts.ServerToken != "" {
		req.Header.Set("Authorization", "Bearer "+opts.ServerToken)
	}
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch custom rule catalog: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("fetch custom rule catalog returned %d", resp.StatusCode)
	}
	var snapshot model.CustomRuleCatalogSnapshot
	if err := json.NewDecoder(resp.Body).Decode(&snapshot); err != nil {
		return nil, fmt.Errorf("decode custom rule catalog: %w", err)
	}
	return snapshot.Rules, nil
}

func loadLocalRulePacks(projectDir string, seen map[string]bool) ([]model.CustomRuleDefinition, []string, error) {
	rulesDir := filepath.Join(projectDir, ".ollanta", "rules")
	patterns := []string{filepath.Join(rulesDir, "*.yaml"), filepath.Join(rulesDir, "*.yml"), filepath.Join(rulesDir, "*.json")}
	rules := []model.CustomRuleDefinition{}
	sources := []string{}
	for _, pattern := range patterns {
		paths, err := filepath.Glob(pattern)
		if err != nil {
			return nil, nil, fmt.Errorf("load local custom rule packs: %w", err)
		}
		for _, path := range paths {
			loaded, err := loadLocalRulePack(path, seen)
			if err != nil {
				return nil, nil, err
			}
			rules = append(rules, loaded...)
			sources = append(sources, path)
		}
	}
	return rules, sources, nil
}

func loadLocalRulePack(path string, seen map[string]bool) ([]model.CustomRuleDefinition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read custom rule pack %s: %w", path, err)
	}
	doc, err := appcustom.DecodeDocument(data)
	if err != nil {
		return nil, fmt.Errorf("parse custom rule pack %s: %w", path, err)
	}
	existing := make(map[string]bool, len(seen))
	for key, value := range seen {
		existing[key] = value
	}
	doc, result := appcustom.ValidateDocument(doc, appcustom.ValidationContext{EngineBackedValidation: true, ExistingRuleKeys: existing})
	if result.Status == model.CustomRuleValidationFailed {
		return nil, fmt.Errorf("validate custom rule pack %s: %s", path, firstCustomRuleDiagnostic(result.Diagnostics))
	}
	out := make([]model.CustomRuleDefinition, 0, len(doc.Rules))
	for _, rule := range doc.Rules {
		if seen[rule.RuleKey] {
			return nil, fmt.Errorf("duplicate custom rule key %q in %s", rule.RuleKey, path)
		}
		if customRuntimeEngine(rule) == model.CustomRuleEngineTreeSitter {
			if err := validateTreeSitterExamples(rule, path); err != nil {
				return nil, err
			}
		}
		seen[rule.RuleKey] = true
		rule.Lifecycle = model.CustomRulePublished
		rule.ValidationStatus = model.CustomRuleValidationPassed
		rule.ValidationHash = rule.VersionHash
		out = append(out, rule)
	}
	return out, nil
}

func validateTreeSitterExamples(rule model.CustomRuleDefinition, packPath string) error {
	query := strings.TrimSpace(rule.EngineConfig["query"])
	if query == "" {
		return nil
	}
	parser := newParserBridge()
	runner := ollantaparser.NewQueryRunner()
	for index, example := range rule.Examples {
		parsedAny, err := parser.ParseSource(rule.RuleKey+".example", []byte(example.Code), rule.Language)
		if err != nil {
			return fmt.Errorf("validate custom rule pack %s: parse example %d for %s: %w", packPath, index, rule.RuleKey, err)
		}
		parsed, ok := parsedAny.(*parsedSource)
		if !ok || parsed == nil || parsed.file == nil {
			continue
		}
		matches, err := runner.Run(parsed.file, query, parsed.grammar)
		if err != nil {
			return fmt.Errorf("validate custom rule pack %s: compile query for %s: %w", packPath, rule.RuleKey, err)
		}
		if !example.Compliant && len(matches) == 0 {
			return fmt.Errorf("validate custom rule pack %s: noncompliant example for %s did not match", packPath, rule.RuleKey)
		}
		if example.Compliant && len(matches) > 0 {
			return fmt.Errorf("validate custom rule pack %s: compliant example for %s matched", packPath, rule.RuleKey)
		}
	}
	return nil
}

func firstCustomRuleDiagnostic(diagnostics []model.CustomRuleDiagnostic) string {
	if len(diagnostics) == 0 {
		return "validation failed"
	}
	return diagnostics[0].Message
}

func (b *customAnalyzerBridge) Key() string {
	return b.rule.RuleKey
}

func (b *customAnalyzerBridge) Name() string {
	return b.rule.Name
}

func (b *customAnalyzerBridge) Description() string {
	return b.rule.Description
}

func (b *customAnalyzerBridge) Language() string {
	return b.rule.Language
}

func (b *customAnalyzerBridge) Type() model.IssueType {
	return b.rule.Type
}

func (b *customAnalyzerBridge) DefaultSeverity() model.Severity {
	return b.rule.DefaultSeverity
}

func (b *customAnalyzerBridge) Tags() []string {
	return append([]string(nil), b.rule.Tags...)
}

func (b *customAnalyzerBridge) Params() map[string]model.ParamDef {
	out := make(map[string]model.ParamDef, len(b.rule.ParamsSchema))
	for key, param := range b.rule.ParamsSchema {
		out[key] = param
	}
	return out
}

func (b *customAnalyzerBridge) Check(ctx context.Context, ac port.AnalysisContext, issues *[]*model.Issue) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	rule := b.rule
	if len(ac.Params) > 0 {
		rule.EngineConfig = applyCustomRuleParams(rule.EngineConfig, ac.Params)
	}
	if customRuntimeEngine(rule) == model.CustomRuleEngineTreeSitter {
		return b.checkTreeSitter(ctx, rule, ac, issues)
	}
	matches, diagnostics := appcustom.Evaluate(rule, ac.Path, ac.Source)
	if hasCustomRuleError(diagnostics) {
		return fmt.Errorf("%s", firstCustomRuleDiagnostic(diagnostics))
	}
	for _, match := range matches {
		issue := appcustom.ToIssue(rule, match)
		if issue != nil {
			*issues = append(*issues, issue)
		}
	}
	return nil
}

func (b *customAnalyzerBridge) checkTreeSitter(ctx context.Context, rule model.CustomRuleDefinition, ac port.AnalysisContext, issues *[]*model.Issue) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	parsed, ok := ac.ParsedFile.(*parsedSource)
	if !ok || parsed == nil || parsed.file == nil {
		return nil
	}
	query := strings.TrimSpace(rule.EngineConfig["query"])
	if query == "" {
		return nil
	}
	runner := ollantaparser.NewQueryRunner()
	matches, err := runner.Run(parsed.file, query, parsed.grammar)
	if err != nil {
		return err
	}
	limit := rule.Limits.MaxMatches
	if limit <= 0 {
		limit = 200
	}
	for index, match := range matches {
		if index >= limit {
			break
		}
		node := treeSitterIssueNode(match)
		if node == nil {
			continue
		}
		line, column, endLine, endColumn := runner.Position(node)
		issue := model.NewIssue(rule.RuleKey, ac.Path, line)
		issue.Column = column
		issue.EndLine = endLine
		issue.EndColumn = endColumn
		issue.Message = customRuleMessage(rule)
		issue.Type = rule.Type
		issue.Severity = rule.DefaultSeverity
		issue.Tags = append([]string(nil), rule.Tags...)
		issue.EngineID = "custom:" + string(customRuntimeEngine(rule))
		issue.QualityDomain = model.DeriveIssueQualityDomain(issue.Type, issue.Tags)
		*issues = append(*issues, issue)
	}
	return nil
}

func treeSitterIssueNode(match ollantaparser.QueryMatch) *sitter.Node {
	if node := match.Captures["issue"]; node != nil {
		return node
	}
	if node := match.Captures["node"]; node != nil {
		return node
	}
	keys := make([]string, 0, len(match.Captures))
	for key := range match.Captures {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if match.Captures[key] != nil {
			return match.Captures[key]
		}
	}
	return nil
}

func applyCustomRuleParams(config map[string]string, params map[string]string) map[string]string {
	out := make(map[string]string, len(config)+len(params))
	for key, value := range config {
		out[key] = value
	}
	for key, value := range params {
		out[key] = value
	}
	return out
}

func hasCustomRuleError(diagnostics []model.CustomRuleDiagnostic) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Level == "error" {
			return true
		}
	}
	return false
}

func customRuleMessage(rule model.CustomRuleDefinition) string {
	if rule.Message != "" {
		return rule.Message
	}
	if rule.Name != "" {
		return rule.Name
	}
	return rule.RuleKey
}

func customRuntimeEngine(rule model.CustomRuleDefinition) model.CustomRuleEngine {
	if rule.Engine != "" && rule.Engine != model.CustomRuleEngineAuto {
		return rule.Engine
	}
	if strings.TrimSpace(rule.EngineConfig["query"]) != "" {
		return model.CustomRuleEngineTreeSitter
	}
	if strings.TrimSpace(rule.EngineConfig["pattern"]) != "" {
		if rule.Language == model.LangGo && strings.TrimSpace(rule.EngineConfig["target"]) != "" {
			return model.CustomRuleEngineGoAST
		}
		return model.CustomRuleEngineText
	}
	return model.CustomRuleEngineText
}
