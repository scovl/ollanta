// Package rules bridges the ollantarules.Rule type (which uses ollantacore/domain
// and ollantaparser types) to the domain/port.IAnalyzer interface consumed by the
// application layer.
package rules

import (
	"context"

	coredomain "github.com/scovl/ollanta/ollantacore/domain"
	"github.com/scovl/ollanta/ollantarules"
	"github.com/scovl/ollanta/ollantaparser"
	"github.com/scovl/ollanta/domain/model"
	"github.com/scovl/ollanta/domain/port"
)

// AnalyzerBridge wraps an ollantarules.Rule to implement port.IAnalyzer.
type AnalyzerBridge struct {
	inner ollantarules.Rule
}

// compile-time check
var _ port.IAnalyzer = (*AnalyzerBridge)(nil)

// Wrap creates an AnalyzerBridge for the given rule.
func Wrap(a ollantarules.Rule) *AnalyzerBridge {
	return &AnalyzerBridge{inner: a}
}

func (b *AnalyzerBridge) Key() string             { return b.inner.Meta.Key }
func (b *AnalyzerBridge) Name() string            { return b.inner.Meta.Name }
func (b *AnalyzerBridge) Description() string     { return b.inner.Meta.Description }
func (b *AnalyzerBridge) Language() string        { return b.inner.Meta.Language }
func (b *AnalyzerBridge) Type() model.IssueType            { return model.IssueType(b.inner.Meta.Type) }
func (b *AnalyzerBridge) DefaultSeverity() model.Severity  { return model.Severity(b.inner.Meta.DefaultSeverity) }
func (b *AnalyzerBridge) Tags() []string                   { return b.inner.Meta.Tags }

func (b *AnalyzerBridge) Params() map[string]model.ParamDef {
	src := b.inner.Meta.Params
	out := make(map[string]model.ParamDef, len(src))
	for _, p := range src {
		out[p.Key] = model.ParamDef{
			Key:          p.Key,
			Description:  p.Description,
			DefaultValue: p.DefaultValue,
			Type:         p.Type,
		}
	}
	return out
}

// Check converts the port.AnalysisContext to ollantarules.AnalysisContext, runs the
// wrapped rule's CheckFunc, and converts resulting issues back to *model.Issue.
func (b *AnalyzerBridge) Check(ctx context.Context, ac port.AnalysisContext, issues *[]*model.Issue) error {
	ruleCtx := &ollantarules.AnalysisContext{
		Path:     ac.Path,
		Source:   ac.Source,
		Language: ac.Language,
		Params:   ac.Params,
	}

	if ac.GoFile != nil {
		ruleCtx.AST = ac.GoFile
		ruleCtx.FileSet = ac.GoFileSet
	}

	if ac.ParsedFile != nil {
		if pf, ok := ac.ParsedFile.(*ollantaparser.ParsedFile); ok {
			ruleCtx.ParsedFile = pf
		}
	}

	if ac.Grammar != nil {
		if g, ok := ac.Grammar.(ollantaparser.Language); ok {
			ruleCtx.Grammar = g
		}
	}

	ruleCtx.Query = ollantaparser.NewQueryRunner()

	ruleTags := b.inner.Meta.Tags
	for _, ci := range b.inner.Check(ruleCtx) {
		di := convertIssue(ci)
		if di == nil {
			continue
		}
		if di.Language == "" {
			di.Language = ac.Language
		}
		if len(di.Tags) == 0 && len(ruleTags) > 0 {
			di.Tags = append([]string(nil), ruleTags...)
		}
		if di.QualityDomain == "" {
			di.QualityDomain = model.DeriveIssueQualityDomain(di.Type, di.Tags)
		}
		*issues = append(*issues, di)
	}
	return ctx.Err()
}

func convertIssue(ci *coredomain.Issue) *model.Issue {
	if ci == nil {
		return nil
	}
	secondary := make([]model.SecondaryLocation, len(ci.SecondaryLocations))
	for i, loc := range ci.SecondaryLocations {
		secondary[i] = model.SecondaryLocation{
			FilePath:    loc.FilePath,
			Message:     loc.Message,
			StartLine:   loc.StartLine,
			StartColumn: loc.StartColumn,
			EndLine:     loc.EndLine,
			EndColumn:   loc.EndColumn,
		}
	}
	return &model.Issue{
		RuleKey:            ci.RuleKey,
		ComponentPath:      ci.ComponentPath,
		Line:               ci.Line,
		Column:             ci.Column,
		EndLine:            ci.EndLine,
		EndColumn:          ci.EndColumn,
		Message:            ci.Message,
		Severity:           model.Severity(ci.Severity),
		Type:               model.IssueType(ci.Type),
		Status:             model.Status(ci.Status),
		Resolution:         ci.Resolution,
		EffortMinutes:      ci.EffortMinutes,
		EngineID:           ci.EngineID,
		LineHash:           ci.LineHash,
		Tags:               ci.Tags,
		SecondaryLocations: secondary,
	}
}
