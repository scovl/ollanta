package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/scovl/ollanta/application/customrules"
	"github.com/scovl/ollanta/domain/model"
	"github.com/scovl/ollanta/domain/port"
	coredomain "github.com/scovl/ollanta/ollantacore/domain"
	"github.com/scovl/ollanta/ollantacore/rulecatalog"
)

type CustomRuleRepository struct {
	db *DB
}

var _ port.ICustomRuleRepo = (*CustomRuleRepository)(nil)

func NewCustomRuleRepository(db *DB) *CustomRuleRepository {
	return &CustomRuleRepository{db: db}
}

func (r *CustomRuleRepository) List(ctx context.Context) ([]model.CustomRuleDefinition, error) {
	rows, err := r.db.Pool.Query(ctx, customRuleSelectSQL()+" ORDER BY crv.rule_key, crv.version DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCustomRules(rows)
}

func (r *CustomRuleRepository) Get(ctx context.Context, id int64) (*model.CustomRuleDefinition, error) {
	rule, err := r.get(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return rule, err
}

func (r *CustomRuleRepository) CreateDraft(ctx context.Context, pack model.CustomRulePack, rule model.CustomRuleDefinition) (*model.CustomRuleDefinition, error) {
	packID, err := r.ensurePack(ctx, pack)
	if err != nil {
		return nil, err
	}
	rule.PackID = packID
	rule.PackName = pack.Name
	rule.Lifecycle = model.CustomRuleDraft
	rule.ValidationStatus = model.CustomRuleValidationNone
	rule.ValidationHash = ""
	version, err := r.nextVersion(ctx, rule.RuleKey)
	if err != nil {
		return nil, err
	}
	rule.Version = version
	rule.VersionHash = model.HashCustomRuleDefinition(rule)
	created, err := r.insertRule(ctx, rule)
	if err != nil {
		return nil, err
	}
	if err := r.recordAudit(ctx, *created, "create", "", created.Lifecycle); err != nil {
		return nil, err
	}
	return created, nil
}

func (r *CustomRuleRepository) UpdateDraft(ctx context.Context, id int64, rule model.CustomRuleDefinition) (*model.CustomRuleDefinition, error) {
	existing, err := r.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	rule.RuleKey = existing.RuleKey
	rule.PackID = existing.PackID
	rule.PackName = existing.PackName
	rule.Lifecycle = model.CustomRuleDraft
	rule.ValidationStatus = model.CustomRuleValidationNone
	rule.ValidationHash = ""
	if existing.Lifecycle == model.CustomRulePublished {
		version, err := r.nextVersion(ctx, existing.RuleKey)
		if err != nil {
			return nil, err
		}
		rule.Version = version
		rule.VersionHash = model.HashCustomRuleDefinition(rule)
		created, err := r.insertRule(ctx, rule)
		if err != nil {
			return nil, err
		}
		if err := r.recordAudit(ctx, *created, "edit_published", existing.Lifecycle, created.Lifecycle); err != nil {
			return nil, err
		}
		return created, nil
	}
	rule.ID = existing.ID
	rule.Version = existing.Version
	rule.VersionHash = model.HashCustomRuleDefinition(rule)
	updated, err := r.updateRule(ctx, rule)
	if err != nil {
		return nil, err
	}
	if err := r.recordAudit(ctx, *updated, "update", existing.Lifecycle, updated.Lifecycle); err != nil {
		return nil, err
	}
	return updated, nil
}

func (r *CustomRuleRepository) StoreValidation(ctx context.Context, id int64, result model.CustomRuleValidationResult) (*model.CustomRuleDefinition, error) {
	rule, err := r.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	oldState := rule.Lifecycle
	rule.ValidationStatus = result.Status
	rule.ValidationHash = result.VersionHash
	rule.ValidationTimestamp = result.CheckedAt
	rule.ValidationResult = &result
	switch result.Status {
	case model.CustomRuleValidationPassed:
		rule.Lifecycle = model.CustomRuleValid
	case model.CustomRuleValidationFailed:
		rule.Lifecycle = model.CustomRuleInvalid
	case model.CustomRuleValidationRequiresRuntime:
		rule.Lifecycle = model.CustomRuleDraft
	}
	updated, err := r.updateValidation(ctx, *rule, result)
	if err != nil {
		return nil, err
	}
	if err := r.recordAudit(ctx, *updated, "validate", oldState, updated.Lifecycle); err != nil {
		return nil, err
	}
	return updated, nil
}

func (r *CustomRuleRepository) Publish(ctx context.Context, id int64) (*model.CustomRuleDefinition, error) {
	rule, err := r.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if !model.CanPublishCustomRule(*rule) {
		return nil, fmt.Errorf("custom rule %q must pass current validation before publishing", rule.RuleKey)
	}
	if err := r.ensureNoPublishedConflict(ctx, rule.RuleKey, rule.PackID, rule.ID); err != nil {
		return nil, err
	}
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin publish custom rule: %w", err)
	}
	defer rollbackCustomRuleTx(ctx, tx)
	if _, err := tx.Exec(ctx, `UPDATE custom_rule_versions SET lifecycle = 'deprecated', updated_at = now() WHERE rule_key = $1 AND lifecycle = 'published' AND id <> $2`, rule.RuleKey, rule.ID); err != nil {
		return nil, fmt.Errorf("deprecate previous custom rule versions: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE custom_rule_versions SET lifecycle = 'published', published_at = now(), updated_at = now() WHERE id = $1`, rule.ID); err != nil {
		return nil, fmt.Errorf("publish custom rule: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit publish custom rule: %w", err)
	}
	published, err := r.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := r.recordAudit(ctx, *published, "publish", rule.Lifecycle, published.Lifecycle); err != nil {
		return nil, err
	}
	return published, nil
}

func (r *CustomRuleRepository) Disable(ctx context.Context, id int64) (*model.CustomRuleDefinition, error) {
	return r.transition(ctx, id, model.CustomRuleDisabled, "disable")
}

func (r *CustomRuleRepository) Deprecate(ctx context.Context, id int64) (*model.CustomRuleDefinition, error) {
	return r.transition(ctx, id, model.CustomRuleDeprecated, "deprecate")
}

func (r *CustomRuleRepository) ImportDocument(ctx context.Context, doc model.CustomRulePackDocument) ([]model.CustomRuleDefinition, error) {
	doc, result := customrules.ValidateDocument(doc, customrules.ValidationContext{})
	if result.Status == model.CustomRuleValidationFailed {
		return nil, fmt.Errorf("invalid custom rule pack: %s", firstCustomRuleDiagnostic(result.Diagnostics))
	}
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin custom rule import: %w", err)
	}
	defer rollbackCustomRuleTx(ctx, tx)
	packID, err := ensurePackTx(ctx, tx, doc.Pack)
	if err != nil {
		return nil, err
	}
	created := make([]model.CustomRuleDefinition, 0, len(doc.Rules))
	for _, rule := range doc.Rules {
		rule.PackID = packID
		rule.PackName = doc.Pack.Name
		rule.Lifecycle = model.CustomRuleDraft
		rule.ValidationStatus = model.CustomRuleValidationNone
		rule.ValidationHash = ""
		version, err := r.nextVersion(ctx, rule.RuleKey)
		if err != nil {
			return nil, err
		}
		rule.Version = version
		rule.VersionHash = model.HashCustomRuleDefinition(rule)
		inserted, err := insertRuleTx(ctx, tx, rule)
		if err != nil {
			return nil, err
		}
		created = append(created, *inserted)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit custom rule import: %w", err)
	}
	for _, rule := range created {
		if err := r.recordAudit(ctx, rule, "import", "", rule.Lifecycle); err != nil {
			return nil, err
		}
	}
	return created, nil
}

func (r *CustomRuleRepository) ExportDocument(ctx context.Context, id int64) (*model.CustomRulePackDocument, error) {
	rule, err := r.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	doc := &model.CustomRulePackDocument{
		Version: model.CustomRulePackSchemaVersion,
		Pack:    model.CustomRulePack{ID: rule.PackID, Name: rule.PackName, SourceHash: rule.VersionHash},
		Rules:   []model.CustomRuleDefinition{*rule},
	}
	return doc, nil
}

func (r *CustomRuleRepository) Audit(ctx context.Context, id int64, limit, offset int) ([]model.CustomRuleAuditEntry, int, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	var total int
	if err := r.db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM custom_rule_audit WHERE rule_id = $1`, id).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, COALESCE(pack_id, 0), COALESCE(rule_id, 0), rule_key, version_hash, action, old_state, new_state,
		       COALESCE(actor_user_id, 0), created_at
		FROM custom_rule_audit
		WHERE rule_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2 OFFSET $3`, id, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	entries := []model.CustomRuleAuditEntry{}
	for rows.Next() {
		var entry model.CustomRuleAuditEntry
		var oldState, newState string
		if err := rows.Scan(&entry.ID, &entry.PackID, &entry.RuleID, &entry.RuleKey, &entry.VersionHash, &entry.Action, &oldState, &newState, &entry.ActorUserID, &entry.CreatedAt); err != nil {
			return nil, 0, err
		}
		entry.OldState = model.CustomRuleLifecycle(oldState)
		entry.NewState = model.CustomRuleLifecycle(newState)
		entries = append(entries, entry)
	}
	return entries, total, rows.Err()
}

func (r *CustomRuleRepository) PublishedCatalogSnapshot(ctx context.Context) (*model.CustomRuleCatalogSnapshot, error) {
	rules, err := r.publishedRules(ctx)
	if err != nil {
		return nil, err
	}
	packIDs := map[int64]bool{}
	for index := range rules {
		packIDs[rules[index].PackID] = true
	}
	outPackIDs := make([]int64, 0, len(packIDs))
	for packID := range packIDs {
		outPackIDs = append(outPackIDs, packID)
	}
	sort.Slice(outPackIDs, func(left, right int) bool { return outPackIDs[left] < outPackIDs[right] })
	return &model.CustomRuleCatalogSnapshot{
		Source:     "server",
		Hash:       model.HashCustomRuleCatalog(rules),
		RuleCount:  len(rules),
		PackIDs:    outPackIDs,
		Rules:      rules,
		ResolvedAt: time.Now().UTC(),
	}, nil
}

func (r *CustomRuleRepository) PublishedRuleByKey(ctx context.Context, key string) (*model.CustomRuleDefinition, error) {
	row := r.db.Pool.QueryRow(ctx, customRuleSelectSQL()+" WHERE crv.rule_key = $1 AND crv.lifecycle = 'published' ORDER BY crv.version DESC LIMIT 1", key)
	rule, err := scanCustomRule(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return rule, err
}

func (r *CustomRuleRepository) PublishedCoreRuleByKey(ctx context.Context, key string) (*coredomain.Rule, string, error) {
	rule, err := r.PublishedRuleByKey(ctx, key)
	if err != nil {
		return nil, "", err
	}
	return CustomRuleToCoreRule(*rule), rule.VersionHash, nil
}

func CustomRuleToCoreRule(rule model.CustomRuleDefinition) *coredomain.Rule {
	params := make(map[string]coredomain.ParamDef, len(rule.ParamsSchema))
	for key, param := range rule.ParamsSchema {
		params[key] = coredomain.ParamDef{Key: param.Key, Description: param.Description, DefaultValue: param.DefaultValue, Type: param.Type}
	}
	return &coredomain.Rule{
		Key:             rule.RuleKey,
		Name:            rule.Name,
		Description:     rule.Description,
		Language:        rule.Language,
		Type:            coredomain.IssueType(rule.Type),
		DefaultSeverity: coredomain.Severity(rule.DefaultSeverity),
		Tags:            append([]string(nil), rule.Tags...),
		ParamsSchema:    params,
	}
}

func (r *CustomRuleRepository) transition(ctx context.Context, id int64, next model.CustomRuleLifecycle, action string) (*model.CustomRuleDefinition, error) {
	rule, err := r.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	oldState := rule.Lifecycle
	_, err = r.db.Pool.Exec(ctx, `UPDATE custom_rule_versions SET lifecycle = $1, updated_at = now() WHERE id = $2`, string(next), id)
	if err != nil {
		return nil, err
	}
	updated, err := r.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := r.recordAudit(ctx, *updated, action, oldState, next); err != nil {
		return nil, err
	}
	return updated, nil
}

func (r *CustomRuleRepository) get(ctx context.Context, id int64) (*model.CustomRuleDefinition, error) {
	row := r.db.Pool.QueryRow(ctx, customRuleSelectSQL()+" WHERE crv.id = $1", id)
	return scanCustomRule(row)
}

func (r *CustomRuleRepository) publishedRules(ctx context.Context) ([]model.CustomRuleDefinition, error) {
	rows, err := r.db.Pool.Query(ctx, customRuleSelectSQL()+" WHERE crv.lifecycle = 'published' ORDER BY crv.rule_key")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCustomRules(rows)
}

func (r *CustomRuleRepository) ensurePack(ctx context.Context, pack model.CustomRulePack) (int64, error) {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer rollbackCustomRuleTx(ctx, tx)
	packID, err := ensurePackTx(ctx, tx, pack)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return packID, nil
}

func ensurePackTx(ctx context.Context, tx pgx.Tx, pack model.CustomRulePack) (int64, error) {
	if pack.Name == "" {
		pack.Name = "Custom Rules"
	}
	if pack.Namespace == "" {
		pack.Namespace = "custom"
	}
	var packID int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO custom_rule_packs (name, namespace, description, source_hash)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (name, namespace) DO UPDATE
		  SET description = EXCLUDED.description, source_hash = EXCLUDED.source_hash, updated_at = now()
		RETURNING id`, pack.Name, pack.Namespace, pack.Description, pack.SourceHash).Scan(&packID); err != nil {
		return 0, fmt.Errorf("upsert custom rule pack: %w", err)
	}
	return packID, nil
}

func (r *CustomRuleRepository) insertRule(ctx context.Context, rule model.CustomRuleDefinition) (*model.CustomRuleDefinition, error) {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer rollbackCustomRuleTx(ctx, tx)
	inserted, err := insertRuleTx(ctx, tx, rule)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return inserted, nil
}

func insertRuleTx(ctx context.Context, tx pgx.Tx, rule model.CustomRuleDefinition) (*model.CustomRuleDefinition, error) {
	rule = model.NormalizeCustomRuleDefinition(rule)
	if rule.VersionHash == "" {
		rule.VersionHash = model.HashCustomRuleDefinition(rule)
	}
	paramsSchema, examples, limits, engineConfig, err := marshalCustomRuleJSON(rule)
	if err != nil {
		return nil, err
	}
	var id int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO custom_rule_versions (
			pack_id, rule_key, version, lifecycle, name, description, language, type, severity, tags,
			params_schema, engine, engine_config, message, examples, limits, version_hash, validation_status, validation_hash
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)
		RETURNING id`,
		rule.PackID, rule.RuleKey, rule.Version, string(rule.Lifecycle), rule.Name, rule.Description, rule.Language,
		string(rule.Type), string(rule.DefaultSeverity), rule.Tags, paramsSchema, string(rule.Engine), engineConfig, rule.Message,
		examples, limits, rule.VersionHash, string(rule.ValidationStatus), rule.ValidationHash).Scan(&id); err != nil {
		return nil, fmt.Errorf("insert custom rule version: %w", err)
	}
	return scanCustomRule(tx.QueryRow(ctx, customRuleSelectSQL()+" WHERE crv.id = $1", id))
}

func (r *CustomRuleRepository) updateRule(ctx context.Context, rule model.CustomRuleDefinition) (*model.CustomRuleDefinition, error) {
	paramsSchema, examples, limits, engineConfig, err := marshalCustomRuleJSON(rule)
	if err != nil {
		return nil, err
	}
	if _, err := r.db.Pool.Exec(ctx, `
		UPDATE custom_rule_versions
		SET lifecycle = $1, name = $2, description = $3, language = $4, type = $5, severity = $6, tags = $7,
		    params_schema = $8, engine = $9, engine_config = $10, message = $11, examples = $12, limits = $13,
		    version_hash = $14, validation_status = $15, validation_hash = $16, validation_diagnostics = '[]',
		    validator_capabilities = '{}', validation_timestamp = NULL, updated_at = now()
		WHERE id = $17`,
		string(rule.Lifecycle), rule.Name, rule.Description, rule.Language, string(rule.Type), string(rule.DefaultSeverity), rule.Tags,
		paramsSchema, string(rule.Engine), engineConfig, rule.Message, examples, limits, rule.VersionHash,
		string(rule.ValidationStatus), rule.ValidationHash, rule.ID); err != nil {
		return nil, fmt.Errorf("update custom rule: %w", err)
	}
	return r.Get(ctx, rule.ID)
}

func (r *CustomRuleRepository) updateValidation(ctx context.Context, rule model.CustomRuleDefinition, result model.CustomRuleValidationResult) (*model.CustomRuleDefinition, error) {
	diagnostics, err := json.Marshal(result.Diagnostics)
	if err != nil {
		return nil, err
	}
	if _, err := r.db.Pool.Exec(ctx, `
		UPDATE custom_rule_versions
		SET lifecycle = $1, validation_status = $2, validation_hash = $3, validation_diagnostics = $4,
		    validator_capabilities = $5, validation_timestamp = $6, updated_at = now()
		WHERE id = $7`, string(rule.Lifecycle), string(result.Status), result.VersionHash, diagnostics, result.ValidatorCapabilities, result.CheckedAt, rule.ID); err != nil {
		return nil, fmt.Errorf("store custom rule validation: %w", err)
	}
	return r.Get(ctx, rule.ID)
}

func marshalCustomRuleJSON(rule model.CustomRuleDefinition) ([]byte, []byte, []byte, []byte, error) {
	paramsSchema, err := json.Marshal(rule.ParamsSchema)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	examples, err := json.Marshal(rule.Examples)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	limits, err := json.Marshal(rule.Limits)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	engineConfig, err := json.Marshal(rule.EngineConfig)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	return paramsSchema, examples, limits, engineConfig, nil
}

func (r *CustomRuleRepository) nextVersion(ctx context.Context, ruleKey string) (int, error) {
	var version int
	if err := r.db.Pool.QueryRow(ctx, `SELECT COALESCE(MAX(version), 0) + 1 FROM custom_rule_versions WHERE rule_key = $1`, ruleKey).Scan(&version); err != nil {
		return 0, err
	}
	if version <= 0 {
		return 1, nil
	}
	return version, nil
}

func (r *CustomRuleRepository) ensureNoPublishedConflict(ctx context.Context, ruleKey string, packID, id int64) error {
	if _, ok := rulecatalog.ByKey(ruleKey); ok {
		return fmt.Errorf("custom rule key %q conflicts with a bundled rule", ruleKey)
	}
	var exists bool
	if err := r.db.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM custom_rule_versions WHERE rule_key = $1 AND lifecycle = 'published' AND pack_id <> $2 AND id <> $3)`, ruleKey, packID, id).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("custom rule key %q is already published", ruleKey)
	}
	return nil
}

func (r *CustomRuleRepository) recordAudit(ctx context.Context, rule model.CustomRuleDefinition, action string, oldState, newState model.CustomRuleLifecycle) error {
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO custom_rule_audit (pack_id, rule_id, rule_key, version_hash, action, old_state, new_state)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`, rule.PackID, rule.ID, rule.RuleKey, rule.VersionHash, action, string(oldState), string(newState))
	return err
}

func customRuleSelectSQL() string {
	return `SELECT crv.id, crv.pack_id, crp.name, crv.rule_key, crv.version, crv.lifecycle, crv.name, crv.description,
	       crv.language, crv.type, crv.severity, crv.tags, crv.params_schema, crv.engine, crv.engine_config,
	       crv.message, crv.examples, crv.limits, crv.version_hash, crv.validation_status, crv.validation_hash,
	       crv.validation_diagnostics, crv.validator_capabilities, crv.validation_timestamp, crv.published_at,
	       crv.created_at, crv.updated_at
	FROM custom_rule_versions crv
	JOIN custom_rule_packs crp ON crp.id = crv.pack_id`
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanCustomRules(rows pgx.Rows) ([]model.CustomRuleDefinition, error) {
	rules := []model.CustomRuleDefinition{}
	for rows.Next() {
		rule, err := scanCustomRule(rows)
		if err != nil {
			return nil, err
		}
		rules = append(rules, *rule)
	}
	return rules, rows.Err()
}

func scanCustomRule(row rowScanner) (*model.CustomRuleDefinition, error) {
	var rule model.CustomRuleDefinition
	var lifecycle, issueType, severity, engine, validationStatus string
	var paramsSchemaRaw, engineConfigRaw, examplesRaw, limitsRaw, diagnosticsRaw []byte
	var validatorCapabilities []string
	var validationTimestamp *time.Time
	if err := row.Scan(&rule.ID, &rule.PackID, &rule.PackName, &rule.RuleKey, &rule.Version, &lifecycle, &rule.Name, &rule.Description,
		&rule.Language, &issueType, &severity, &rule.Tags, &paramsSchemaRaw, &engine, &engineConfigRaw,
		&rule.Message, &examplesRaw, &limitsRaw, &rule.VersionHash, &validationStatus, &rule.ValidationHash,
		&diagnosticsRaw, &validatorCapabilities, &validationTimestamp, &rule.PublishedAt, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
		return nil, err
	}
	rule.Lifecycle = model.CustomRuleLifecycle(lifecycle)
	rule.Type = model.IssueType(issueType)
	rule.DefaultSeverity = model.Severity(severity)
	rule.Engine = model.CustomRuleEngine(engine)
	rule.ValidationStatus = model.CustomRuleValidationStatus(validationStatus)
	if err := json.Unmarshal(paramsSchemaRaw, &rule.ParamsSchema); err != nil {
		return nil, fmt.Errorf("decode custom rule params schema: %w", err)
	}
	if err := json.Unmarshal(engineConfigRaw, &rule.EngineConfig); err != nil {
		return nil, fmt.Errorf("decode custom rule engine config: %w", err)
	}
	if err := json.Unmarshal(examplesRaw, &rule.Examples); err != nil {
		return nil, fmt.Errorf("decode custom rule examples: %w", err)
	}
	if err := json.Unmarshal(limitsRaw, &rule.Limits); err != nil {
		return nil, fmt.Errorf("decode custom rule limits: %w", err)
	}
	diagnostics := []model.CustomRuleDiagnostic{}
	if err := json.Unmarshal(diagnosticsRaw, &diagnostics); err != nil {
		return nil, fmt.Errorf("decode custom rule diagnostics: %w", err)
	}
	if rule.ValidationStatus != model.CustomRuleValidationNone || len(diagnostics) > 0 {
		checkedAt := time.Time{}
		if validationTimestamp != nil {
			checkedAt = *validationTimestamp
		}
		rule.ValidationTimestamp = checkedAt
		rule.ValidationResult = &model.CustomRuleValidationResult{
			Status:                rule.ValidationStatus,
			RuleKey:               rule.RuleKey,
			VersionHash:           rule.ValidationHash,
			ValidatorCapabilities: validatorCapabilities,
			Diagnostics:           diagnostics,
			CheckedAt:             checkedAt,
		}
	}
	return &rule, nil
}

func firstCustomRuleDiagnostic(diagnostics []model.CustomRuleDiagnostic) string {
	if len(diagnostics) == 0 {
		return "validation failed"
	}
	return diagnostics[0].Message
}

func rollbackCustomRuleTx(ctx context.Context, tx pgx.Tx) {
	if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		slog.Debug("rollback custom rule transaction", "error", err)
	}
}
