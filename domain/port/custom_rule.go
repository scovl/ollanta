package port

import (
	"context"

	"github.com/scovl/ollanta/domain/model"
)

type ICustomRuleRepo interface {
	List(ctx context.Context) ([]model.CustomRuleDefinition, error)
	Get(ctx context.Context, id int64) (*model.CustomRuleDefinition, error)
	CreateDraft(ctx context.Context, pack model.CustomRulePack, rule model.CustomRuleDefinition) (*model.CustomRuleDefinition, error)
	UpdateDraft(ctx context.Context, id int64, rule model.CustomRuleDefinition) (*model.CustomRuleDefinition, error)
	StoreValidation(ctx context.Context, id int64, result model.CustomRuleValidationResult) (*model.CustomRuleDefinition, error)
	Publish(ctx context.Context, id int64) (*model.CustomRuleDefinition, error)
	Disable(ctx context.Context, id int64) (*model.CustomRuleDefinition, error)
	Deprecate(ctx context.Context, id int64) (*model.CustomRuleDefinition, error)
	ImportDocument(ctx context.Context, doc model.CustomRulePackDocument) ([]model.CustomRuleDefinition, error)
	ExportDocument(ctx context.Context, id int64) (*model.CustomRulePackDocument, error)
	Audit(ctx context.Context, id int64, limit, offset int) ([]model.CustomRuleAuditEntry, int, error)
	PublishedCatalogSnapshot(ctx context.Context) (*model.CustomRuleCatalogSnapshot, error)
}

type ICustomRuleValidator interface {
	Capabilities(ctx context.Context) ([]model.CustomRuleEngineCapability, error)
	Validate(ctx context.Context, rule model.CustomRuleDefinition) model.CustomRuleValidationResult
}

type ICustomRulePreviewer interface {
	Preview(ctx context.Context, rule model.CustomRuleDefinition, files []string) (*model.CustomRulePreviewResult, error)
}
