package scan

import (
	"context"
	goast "go/ast"
	"go/token"

	"github.com/scovl/ollanta/domain/model"
	"github.com/scovl/ollanta/domain/port"
	coredomain "github.com/scovl/ollanta/ollantacore/domain"
	"github.com/scovl/ollanta/ollantaparser"
	parserlanguages "github.com/scovl/ollanta/ollantaparser/languages"
	"github.com/scovl/ollanta/ollantarules"
	"github.com/scovl/ollanta/ollantarules/defaults"
)

type parsedSource struct {
	file    *ollantaparser.ParsedFile
	grammar ollantaparser.Language
}

type parserBridge struct {
	registry *ollantaparser.LanguageRegistry
}

var _ port.IParser = (*parserBridge)(nil)
var _ port.IAnalyzer = (*analyzerBridge)(nil)

func newParserBridge() *parserBridge {
	return &parserBridge{registry: parserlanguages.DefaultRegistry()}
}

func (p *parserBridge) ParseFile(path, language string) (any, error) {
	grammar, ok := p.registry.ForFile(path)
	if !ok && language != "" {
		grammar, ok = p.registry.ForName(language)
	}
	if !ok {
		return nil, nil
	}
	parsed, err := ollantaparser.ParseFile(path, grammar)
	if err != nil {
		return nil, err
	}
	return &parsedSource{file: parsed, grammar: grammar}, nil
}

func (p *parserBridge) ParseSource(path string, src []byte, language string) (any, error) {
	grammar, ok := p.registry.ForName(language)
	if !ok {
		grammar, ok = p.registry.ForFile(path)
	}
	if !ok {
		return nil, nil
	}
	parsed, err := ollantaparser.Parse(path, src, grammar)
	if err != nil {
		return nil, err
	}
	return &parsedSource{file: parsed, grammar: grammar}, nil
}

type analyzerBridge struct {
	inner ollantarules.Rule
}

func newAnalyzerBridges() []port.IAnalyzer {
	rules := defaults.NewRegistry().All()
	out := make([]port.IAnalyzer, len(rules))
	for i, rule := range rules {
		out[i] = &analyzerBridge{inner: rule}
	}
	return out
}

func (b *analyzerBridge) Key() string {
	return b.inner.Meta.Key
}

func (b *analyzerBridge) Name() string {
	return b.inner.Meta.Name
}

func (b *analyzerBridge) Description() string {
	return b.inner.Meta.Description
}

func (b *analyzerBridge) Language() string {
	return b.inner.Meta.Language
}

func (b *analyzerBridge) Type() model.IssueType {
	return model.IssueType(b.inner.Meta.Type)
}

func (b *analyzerBridge) DefaultSeverity() model.Severity {
	return model.Severity(b.inner.Meta.DefaultSeverity)
}

func (b *analyzerBridge) Tags() []string {
	return b.inner.Meta.Tags
}

func (b *analyzerBridge) Params() map[string]model.ParamDef {
	out := make(map[string]model.ParamDef, len(b.inner.Meta.Params))
	for _, param := range b.inner.Meta.Params {
		out[param.Key] = model.ParamDef{
			Key:          param.Key,
			Description:  param.Description,
			DefaultValue: param.DefaultValue,
			Type:         param.Type,
		}
	}
	return out
}

func (b *analyzerBridge) Check(ctx context.Context, ac port.AnalysisContext, issues *[]*model.Issue) error {
	ruleCtx := &ollantarules.AnalysisContext{
		Path:     ac.Path,
		Source:   ac.Source,
		Language: ac.Language,
		Params:   map[string]string{},
		AST:      (*goast.File)(nil),
		FileSet:  (*token.FileSet)(nil),
		Query:    ollantaparser.NewQueryRunner(),
	}

	if ac.GoFile != nil {
		ruleCtx.AST = ac.GoFile
		ruleCtx.FileSet = ac.GoFileSet
	}

	if parsed, ok := ac.ParsedFile.(*parsedSource); ok {
		ruleCtx.ParsedFile = parsed.file
		ruleCtx.Grammar = parsed.grammar
	} else if parsed, ok := ac.ParsedFile.(*ollantaparser.ParsedFile); ok {
		ruleCtx.ParsedFile = parsed
	}

	for _, issue := range b.inner.Check(ruleCtx) {
		*issues = append(*issues, toDomainIssue(issue))
	}
	return ctx.Err()
}

func toDomainIssue(issue *coredomain.Issue) *model.Issue {
	if issue == nil {
		return nil
	}
	secondary := make([]model.SecondaryLocation, len(issue.SecondaryLocations))
	for i, location := range issue.SecondaryLocations {
		secondary[i] = model.SecondaryLocation{
			FilePath:    location.FilePath,
			Message:     location.Message,
			StartLine:   location.StartLine,
			StartColumn: location.StartColumn,
			EndLine:     location.EndLine,
			EndColumn:   location.EndColumn,
		}
	}
	return &model.Issue{
		RuleKey:            issue.RuleKey,
		ComponentPath:      issue.ComponentPath,
		Line:               issue.Line,
		Column:             issue.Column,
		EndLine:            issue.EndLine,
		EndColumn:          issue.EndColumn,
		Message:            issue.Message,
		Type:               model.IssueType(issue.Type),
		Severity:           model.Severity(issue.Severity),
		Status:             model.Status(issue.Status),
		Resolution:         issue.Resolution,
		EffortMinutes:      issue.EffortMinutes,
		EngineID:           issue.EngineID,
		LineHash:           issue.LineHash,
		Tags:               issue.Tags,
		SecondaryLocations: secondary,
	}
}
