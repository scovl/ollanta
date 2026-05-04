package postgres

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/scovl/ollanta/domain/model"
	"github.com/scovl/ollanta/domain/port"
	coredomain "github.com/scovl/ollanta/ollantacore/domain"
	"github.com/scovl/ollanta/ollantacore/rulecatalog"
)

type QualityProfile = model.QualityProfile
type ProfileRule = model.ProfileRule
type EffectiveRule = model.EffectiveRule
type ProfileYAMLEntry = model.ProfileYAMLEntry

// ProfileRepository provides CRUD access to quality_profiles and related tables.
type ProfileRepository struct {
	db *DB
}

var _ port.IProfileRepo = (*ProfileRepository)(nil)

// NewProfileRepository creates a ProfileRepository backed by db.
func NewProfileRepository(db *DB) *ProfileRepository {
	return &ProfileRepository{db: db}
}

// SyncBuiltInProfiles synchronizes bundled Ollanta Way profiles from the CGo-free catalog.
func (r *ProfileRepository) SyncBuiltInProfiles(ctx context.Context) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin built-in profile sync: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for _, language := range rulecatalog.SupportedLanguages() {
		var profileID int64
		if err := tx.QueryRow(ctx, `
			INSERT INTO quality_profiles (name, language, is_default, is_builtin)
			VALUES ('Ollanta Way', $1, TRUE, TRUE)
			ON CONFLICT (name, language) DO UPDATE
			  SET is_builtin = TRUE, updated_at = now()
			RETURNING id`, language.Key).Scan(&profileID); err != nil {
			return fmt.Errorf("sync built-in profile %s: %w", language.Key, err)
		}

		if _, err := tx.Exec(ctx, `
			UPDATE quality_profiles qp
			SET is_default = TRUE, updated_at = now()
			WHERE qp.id = $1
			  AND NOT EXISTS (
			    SELECT 1 FROM quality_profiles existing
			    WHERE existing.language = qp.language AND existing.is_default = TRUE
			  )`, profileID); err != nil {
			return fmt.Errorf("ensure default profile %s: %w", language.Key, err)
		}

		for _, rule := range rulecatalog.ByLanguage(language.Key) {
			if _, err := tx.Exec(ctx, `
				INSERT INTO quality_profile_rules (profile_id, rule_key, severity, params)
				VALUES ($1, $2, $3, $4)
				ON CONFLICT (profile_id, rule_key) DO UPDATE
				  SET severity = EXCLUDED.severity, params = EXCLUDED.params`,
				profileID, rule.Key, string(rule.DefaultSeverity), rulecatalog.DefaultParams(rule)); err != nil {
				return fmt.Errorf("sync built-in rule %s: %w", rule.Key, err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit built-in profile sync: %w", err)
	}
	return nil
}

// List returns all profiles, optionally filtered by language.
func (r *ProfileRepository) List(ctx context.Context, language string) ([]*QualityProfile, error) {
	q := `SELECT qp.id, qp.name, qp.language, qp.parent_id, qp.is_default, qp.is_builtin,
	             COALESCE(COUNT(qpr.id) FILTER (WHERE qpr.severity <> 'OFF'), 0)::int AS rule_count,
	             qp.created_at, qp.updated_at
	      FROM quality_profiles qp
	      LEFT JOIN quality_profile_rules qpr ON qpr.profile_id = qp.id`
	args := []any{}
	if language != "" {
		q += " WHERE qp.language = $1"
		args = append(args, language)
	}
	q += " GROUP BY qp.id, qp.name, qp.language, qp.parent_id, qp.is_default, qp.is_builtin, qp.created_at, qp.updated_at ORDER BY qp.language, qp.name"

	rows, err := r.db.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanProfiles(rows)
}

// GetByID returns a profile by its ID.
func (r *ProfileRepository) GetByID(ctx context.Context, id int64) (*QualityProfile, error) {
	p := &QualityProfile{}
	err := r.db.Pool.QueryRow(ctx, `
		SELECT qp.id, qp.name, qp.language, qp.parent_id, qp.is_default, qp.is_builtin,
		       COALESCE(COUNT(qpr.id) FILTER (WHERE qpr.severity <> 'OFF'), 0)::int AS rule_count,
		       qp.created_at, qp.updated_at
		FROM quality_profiles qp
		LEFT JOIN quality_profile_rules qpr ON qpr.profile_id = qp.id
		WHERE qp.id = $1
		GROUP BY qp.id, qp.name, qp.language, qp.parent_id, qp.is_default, qp.is_builtin, qp.created_at, qp.updated_at`, id,
	).Scan(&p.ID, &p.Name, &p.Language, &p.ParentID, &p.IsDefault, &p.IsBuiltin, &p.RuleCount, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	enrichProfile(p)
	return p, nil
}

// Create inserts a new quality profile. Validates inheritance depth (max 3).
func (r *ProfileRepository) Create(ctx context.Context, p *QualityProfile) error {
	if p.ParentID != nil {
		depth, err := r.inheritanceDepth(ctx, *p.ParentID)
		if err != nil {
			return fmt.Errorf("check inheritance depth: %w", err)
		}
		if depth >= 3 {
			return fmt.Errorf("inheritance chain exceeds maximum of 3 levels")
		}
	}
	if err := r.db.Pool.QueryRow(ctx, `
		INSERT INTO quality_profiles (name, language, parent_id, is_default, is_builtin)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at`,
		p.Name, p.Language, p.ParentID, p.IsDefault, p.IsBuiltin,
	).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return err
	}
	return r.recordProfileChangelog(ctx, model.ProfileChangelogEntry{ProfileID: p.ID, Language: p.Language, Action: "create", NewValue: p.Name})
}

// Update updates name, parent_id and is_default of a profile. Validates depth.
func (r *ProfileRepository) Update(ctx context.Context, p *QualityProfile) error {
	if p.IsBuiltin {
		return fmt.Errorf("cannot update builtin profile")
	}
	if p.ParentID != nil {
		depth, err := r.inheritanceDepth(ctx, *p.ParentID)
		if err != nil {
			return fmt.Errorf("check inheritance depth: %w", err)
		}
		if depth >= 3 {
			return fmt.Errorf("inheritance chain exceeds maximum of 3 levels")
		}
	}
	_, err := r.db.Pool.Exec(ctx, `
		UPDATE quality_profiles
		SET name = $1, parent_id = $2, is_default = $3, updated_at = now()
		WHERE id = $4`,
		p.Name, p.ParentID, p.IsDefault, p.ID)
	if err != nil {
		return err
	}
	return r.recordProfileChangelog(ctx, model.ProfileChangelogEntry{ProfileID: p.ID, Language: p.Language, Action: "update", NewValue: p.Name})
}

// Delete removes a non-builtin profile.
func (r *ProfileRepository) Delete(ctx context.Context, id int64) error {
	profile, err := r.GetByID(ctx, id)
	if err != nil {
		return err
	}
	tag, err := r.db.Pool.Exec(ctx,
		`DELETE FROM quality_profiles WHERE id = $1 AND is_builtin = FALSE`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("profile not found or is builtin")
	}
	return r.recordProfileChangelog(ctx, model.ProfileChangelogEntry{ProfileID: id, Language: profile.Language, Action: "delete", OldValue: profile.Name})
}

// Copy duplicates a profile with a new name, including all its rules.
func (r *ProfileRepository) Copy(ctx context.Context, sourceID int64, newName string) (*QualityProfile, error) {
	src, err := r.GetByID(ctx, sourceID)
	if err != nil {
		return nil, err
	}
	newProfile := &QualityProfile{
		Name:     newName,
		Language: src.Language,
		ParentID: src.ParentID,
	}
	if err := r.Create(ctx, newProfile); err != nil {
		return nil, fmt.Errorf("create copy: %w", err)
	}
	rules, err := r.listRules(ctx, sourceID)
	if err != nil {
		return nil, fmt.Errorf("read rules: %w", err)
	}
	for _, rule := range rules {
		if err := r.ActivateRule(ctx, newProfile.ID, rule.RuleKey, rule.Severity, rule.Params); err != nil {
			return nil, fmt.Errorf("copy rule: %w", err)
		}
	}
	_ = r.recordProfileChangelog(ctx, model.ProfileChangelogEntry{ProfileID: newProfile.ID, Language: newProfile.Language, Action: "copy", OldValue: src.Name, NewValue: newName})
	return newProfile, nil
}

// SetDefault atomically sets a profile as default for its language and clears other defaults.
func (r *ProfileRepository) SetDefault(ctx context.Context, id int64) error {
	p, err := r.GetByID(ctx, id)
	if err != nil {
		return err
	}
	_, err = r.db.Pool.Exec(ctx,
		`UPDATE quality_profiles SET is_default = (id = $1), updated_at = now()
		 WHERE language = $2 AND (is_default = TRUE OR id = $1)`, id, p.Language)
	if err != nil {
		return err
	}
	return r.recordProfileChangelog(ctx, model.ProfileChangelogEntry{ProfileID: id, Language: p.Language, Action: "set_default", NewValue: p.Name})
}

// ActivateRule adds or updates an active rule in a profile.
// severity='OFF' deactivates the rule (stored explicitly for inheritance resolution).
func (r *ProfileRepository) ActivateRule(ctx context.Context, profileID int64, ruleKey, severity string, params map[string]string) error {
	profile, err := r.GetByID(ctx, profileID)
	if err != nil {
		return err
	}
	rule, _, err := r.validateRuleActivation(ctx, profile.Language, ruleKey, severity, params)
	if err != nil {
		return err
	}
	if severity == "" {
		severity = string(rule.DefaultSeverity)
	}
	if params == nil {
		params = rulecatalog.DefaultParams(rule)
	}
	_, err = r.db.Pool.Exec(ctx, `
		INSERT INTO quality_profile_rules (profile_id, rule_key, severity, params)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (profile_id, rule_key) DO UPDATE
		  SET severity = EXCLUDED.severity, params = EXCLUDED.params`,
		profileID, ruleKey, severity, params)
	if err != nil {
		return err
	}
	action := "activate_rule"
	if strings.EqualFold(severity, "OFF") {
		action = "deactivate_rule"
	}
	return r.recordProfileChangelog(ctx, model.ProfileChangelogEntry{ProfileID: profileID, Language: profile.Language, Action: action, RuleKey: ruleKey, NewValue: severity})
}

// DeactivateRule removes a rule from a profile (sets severity='OFF' for inheritance tracking).
func (r *ProfileRepository) DeactivateRule(ctx context.Context, profileID int64, ruleKey string) error {
	return r.ActivateRule(ctx, profileID, ruleKey, "OFF", nil)
}

// AssignToProject sets the active profile for a project+language combination.
func (r *ProfileRepository) AssignToProject(ctx context.Context, projectID int64, language string, profileID int64) error {
	profile, err := r.GetByID(ctx, profileID)
	if err != nil {
		return err
	}
	if profile.Language != language {
		return fmt.Errorf("profile language %q does not match assignment language %q", profile.Language, language)
	}
	_, err = r.db.Pool.Exec(ctx, `
		INSERT INTO project_profiles (project_id, language, profile_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (project_id, language) DO UPDATE SET profile_id = EXCLUDED.profile_id`,
		projectID, language, profileID)
	if err != nil {
		return err
	}
	return r.recordProfileChangelog(ctx, model.ProfileChangelogEntry{ProfileID: profileID, ProjectID: projectID, Language: language, Action: "assign_project", NewValue: profile.Name})
}

// ByProjectAndLanguage returns the active profile for a project+language, falling back to the default.
func (r *ProfileRepository) ByProjectAndLanguage(ctx context.Context, projectID int64, language string) (*QualityProfile, error) {
	// Try project-specific assignment first.
	var profileID int64
	err := r.db.Pool.QueryRow(ctx, `
		SELECT profile_id FROM project_profiles WHERE project_id = $1 AND language = $2`,
		projectID, language,
	).Scan(&profileID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if errors.Is(err, pgx.ErrNoRows) {
		// Fall back to language default.
		err = r.db.Pool.QueryRow(ctx, `
			SELECT id FROM quality_profiles WHERE language = $1 AND is_default = TRUE LIMIT 1`,
			language,
		).Scan(&profileID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		if err != nil {
			return nil, err
		}
	}
	return r.GetByID(ctx, profileID)
}

// ResolveEffectiveRules walks the inheritance chain and returns the union of active rules.
// Rules with severity='OFF' in any child profile are excluded from the result.
func (r *ProfileRepository) ResolveEffectiveRules(ctx context.Context, profileID int64) ([]*EffectiveRule, error) {
	chain, err := r.buildInheritanceChain(ctx, profileID)
	if err != nil {
		return nil, err
	}

	// Walk from root to leaf — child entries override parent entries.
	leafID := chain[len(chain)-1]
	merged := map[string]*EffectiveRule{}
	for _, pid := range chain {
		if err := r.mergeProfileRules(ctx, pid, leafID, merged); err != nil {
			return nil, err
		}
	}

	out := make([]*EffectiveRule, 0, len(merged))
	for _, rule := range merged {
		out = append(out, rule)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RuleKey < out[j].RuleKey })
	return out, nil
}

// ProjectProfiles returns the profile assignment/default state for each supported language.
func (r *ProfileRepository) ProjectProfiles(ctx context.Context, projectID int64) ([]*model.ProjectQualityProfile, error) {
	out := make([]*model.ProjectQualityProfile, 0, len(rulecatalog.SupportedLanguages()))
	for _, language := range rulecatalog.SupportedLanguages() {
		profile, source, err := r.projectProfile(ctx, projectID, language.Key)
		if errors.Is(err, ErrNotFound) {
			out = append(out, &model.ProjectQualityProfile{Language: language.Key, Source: model.ProfileSourceDefault})
			continue
		}
		if err != nil {
			return nil, err
		}
		out = append(out, &model.ProjectQualityProfile{Language: language.Key, Profile: profile, Source: source})
	}
	return out, nil
}

// ProjectEffectiveProfiles returns the effective rule policy for each supported language.
func (r *ProfileRepository) ProjectEffectiveProfiles(ctx context.Context, projectID int64) ([]*model.EffectiveQualityProfile, error) {
	out := make([]*model.EffectiveQualityProfile, 0, len(rulecatalog.SupportedLanguages()))
	customCatalogHash := ""
	if snapshot, err := NewCustomRuleRepository(r.db).PublishedCatalogSnapshot(ctx); err == nil {
		customCatalogHash = snapshot.Hash
	}
	for _, language := range rulecatalog.SupportedLanguages() {
		profile, source, err := r.projectProfile(ctx, projectID, language.Key)
		if errors.Is(err, ErrNotFound) {
			out = append(out, &model.EffectiveQualityProfile{
				Language:          language.Key,
				Source:            model.ProfileSourceDefault,
				Rules:             []*model.EffectiveRule{},
				RulesHash:         model.HashEffectiveRules(nil),
				CustomCatalogHash: customCatalogHash,
				HasRules:          language.HasRules,
				ParserOnly:        language.ParserOnly,
				Diagnostics:       []model.ProfileDiagnostic{{Level: "warning", Code: "no_default_profile", Message: "no default profile configured", Language: language.Key}},
			})
			continue
		}
		if err != nil {
			return nil, err
		}
		rules, err := r.ResolveEffectiveRules(ctx, profile.ID)
		if err != nil {
			return nil, err
		}
		diagnostics := []model.ProfileDiagnostic{}
		if language.ParserOnly {
			diagnostics = append(diagnostics, model.ProfileDiagnostic{Level: "info", Code: "parser_only_language", Message: "language is parsed but has no bundled rules", Language: language.Key})
		}
		out = append(out, &model.EffectiveQualityProfile{
			Language:          language.Key,
			ProfileID:         profile.ID,
			ProfileName:       profile.Name,
			Source:            source,
			Rules:             rules,
			ActiveRuleCount:   activeRuleCount(rules),
			RulesHash:         model.HashEffectiveRules(rules),
			CustomCatalogHash: customCatalogHash,
			HasRules:          language.HasRules,
			ParserOnly:        language.ParserOnly,
			Diagnostics:       diagnostics,
		})
	}
	return out, nil
}

// ProfileChangelog returns profile changelog entries newest first.
func (r *ProfileRepository) ProfileChangelog(ctx context.Context, profileID int64, limit, offset int) ([]model.ProfileChangelogEntry, int, error) {
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
	if err := r.db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM quality_profile_changelog WHERE profile_id = $1`, profileID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, COALESCE(profile_id, 0), COALESCE(project_id, 0), language, action, rule_key, old_value, new_value,
		       COALESCE(actor_user_id, 0), created_at
		FROM quality_profile_changelog
		WHERE profile_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2 OFFSET $3`, profileID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	entries := []model.ProfileChangelogEntry{}
	for rows.Next() {
		var entry model.ProfileChangelogEntry
		if err := rows.Scan(&entry.ID, &entry.ProfileID, &entry.ProjectID, &entry.Language, &entry.Action, &entry.RuleKey, &entry.OldValue, &entry.NewValue, &entry.ActorUserID, &entry.CreatedAt); err != nil {
			return nil, 0, err
		}
		entries = append(entries, entry)
	}
	return entries, total, rows.Err()
}

// ApplyProfileYAML applies a profile-as-code YAML payload transactionally.
// The payload contains activate/deactivate rule lists.
func (r *ProfileRepository) ApplyProfileYAML(ctx context.Context, projectID int64, language string, entries []ProfileYAMLEntry) error {
	profile, err := r.ByProjectAndLanguage(ctx, projectID, language)
	if err != nil {
		return fmt.Errorf("resolve profile for project %d language %s: %w", projectID, language, err)
	}
	return r.ApplyProfileRules(ctx, profile.ID, entries)
}

// ApplyProfileRules applies a profile-as-code payload transactionally to a profile.
func (r *ProfileRepository) ApplyProfileRules(ctx context.Context, profileID int64, entries []ProfileYAMLEntry) error {
	profile, err := r.GetByID(ctx, profileID)
	if err != nil {
		return err
	}
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin profile rule import: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for _, e := range entries {
		if err := r.applyProfileRuleEntry(ctx, tx, profile, e); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit profile rule import: %w", err)
	}
	_ = r.recordProfileChangelog(ctx, model.ProfileChangelogEntry{ProfileID: profileID, Language: profile.Language, Action: "import", NewValue: fmt.Sprintf("%d rules", len(entries))})
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func (r *ProfileRepository) buildInheritanceChain(ctx context.Context, profileID int64) ([]int64, error) {
	chain := []int64{}
	visited := map[int64]bool{}
	id := profileID
	for {
		if visited[id] {
			return nil, fmt.Errorf("cycle detected in profile inheritance at id=%d", id)
		}
		visited[id] = true
		chain = append([]int64{id}, chain...) // prepend so root is first
		var parentID *int64
		err := r.db.Pool.QueryRow(ctx, `SELECT parent_id FROM quality_profiles WHERE id = $1`, id).Scan(&parentID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		if err != nil {
			return nil, err
		}
		if parentID == nil {
			break
		}
		id = *parentID
	}
	return chain, nil
}

func (r *ProfileRepository) inheritanceDepth(ctx context.Context, profileID int64) (int, error) {
	chain, err := r.buildInheritanceChain(ctx, profileID)
	if err != nil {
		return 0, err
	}
	return len(chain), nil
}

func (r *ProfileRepository) mergeProfileRules(ctx context.Context, profileID, leafID int64, merged map[string]*EffectiveRule) error {
	profile, err := r.GetByID(ctx, profileID)
	if err != nil {
		return err
	}
	rules, err := r.listRules(ctx, profileID)
	if err != nil {
		return fmt.Errorf("list rules for profile %d: %w", profileID, err)
	}
	for _, rule := range rules {
		r.mergeEffectiveRule(ctx, merged, profile, rule, profileID, leafID)
	}
	return nil
}

func (r *ProfileRepository) customRuleVersionHash(ctx context.Context, ruleKey string) string {
	_, versionHash, err := NewCustomRuleRepository(r.db).PublishedCoreRuleByKey(ctx, ruleKey)
	if err != nil {
		return ""
	}
	return versionHash
}

func (r *ProfileRepository) mergeEffectiveRule(ctx context.Context, merged map[string]*EffectiveRule, profile *QualityProfile, rule *ProfileRule, profileID, leafID int64) {
	versionHash := r.customRuleVersionHash(ctx, rule.RuleKey)
	if rule.Severity == "OFF" {
		merged[rule.RuleKey] = &EffectiveRule{
			RuleKey:           rule.RuleKey,
			Severity:          rule.Severity,
			Params:            rule.Params,
			RuleVersionHash:   versionHash,
			Origin:            model.RuleOriginDisabled,
			Disabled:          true,
			SourceProfileID:   profile.ID,
			SourceProfileName: profile.Name,
		}
		return
	}
	origin := model.RuleOriginLocal
	if profileID != leafID {
		origin = model.RuleOriginInherited
	} else if _, existed := merged[rule.RuleKey]; existed {
		origin = model.RuleOriginOverridden
	}
	merged[rule.RuleKey] = &EffectiveRule{
		RuleKey:           rule.RuleKey,
		Severity:          rule.Severity,
		Params:            rule.Params,
		RuleVersionHash:   versionHash,
		Origin:            origin,
		SourceProfileID:   profile.ID,
		SourceProfileName: profile.Name,
	}
}

func (r *ProfileRepository) applyProfileRuleEntry(ctx context.Context, tx pgx.Tx, profile *QualityProfile, entry ProfileYAMLEntry) error {
	ruleKey := profileRuleEntryKey(entry)
	if entry.Activate {
		return r.applyActiveProfileRuleEntry(ctx, tx, profile, ruleKey, entry)
	}
	return r.applyDisabledProfileRuleEntry(ctx, tx, profile, ruleKey)
}

func profileRuleEntryKey(entry ProfileYAMLEntry) string {
	if entry.RuleKey != "" {
		return entry.RuleKey
	}
	return entry.Rule
}

func (r *ProfileRepository) applyActiveProfileRuleEntry(ctx context.Context, tx pgx.Tx, profile *QualityProfile, ruleKey string, entry ProfileYAMLEntry) error {
	severity := entry.Severity
	if severity == "" {
		severity = "major"
	}
	rule, _, err := r.validateRuleActivation(ctx, profile.Language, ruleKey, severity, entry.Params)
	if err != nil {
		return fmt.Errorf("activate rule %s: %w", ruleKey, err)
	}
	if entry.Severity == "" {
		severity = string(rule.DefaultSeverity)
	}
	params := entry.Params
	if params == nil {
		params = rulecatalog.DefaultParams(rule)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO quality_profile_rules (profile_id, rule_key, severity, params)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (profile_id, rule_key) DO UPDATE
		  SET severity = EXCLUDED.severity, params = EXCLUDED.params`,
		profile.ID, ruleKey, severity, params); err != nil {
		return fmt.Errorf("activate rule %s: %w", ruleKey, err)
	}
	return nil
}

func (r *ProfileRepository) applyDisabledProfileRuleEntry(ctx context.Context, tx pgx.Tx, profile *QualityProfile, ruleKey string) error {
	if _, _, err := r.validateRuleActivation(ctx, profile.Language, ruleKey, "OFF", nil); err != nil {
		return fmt.Errorf("deactivate rule %s: %w", ruleKey, err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO quality_profile_rules (profile_id, rule_key, severity, params)
		VALUES ($1, $2, 'OFF', '{}')
		ON CONFLICT (profile_id, rule_key) DO UPDATE
		  SET severity = 'OFF', params = '{}'`, profile.ID, ruleKey); err != nil {
		return fmt.Errorf("deactivate rule %s: %w", ruleKey, err)
	}
	return nil
}

func (r *ProfileRepository) listRules(ctx context.Context, profileID int64) ([]*ProfileRule, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, profile_id, rule_key, severity, params
		FROM quality_profile_rules WHERE profile_id = $1`, profileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*ProfileRule
	for rows.Next() {
		rule := &ProfileRule{}
		if err := rows.Scan(&rule.ID, &rule.ProfileID, &rule.RuleKey, &rule.Severity, &rule.Params); err != nil {
			return nil, err
		}
		out = append(out, rule)
	}
	return out, rows.Err()
}

func (r *ProfileRepository) projectProfile(ctx context.Context, projectID int64, language string) (*QualityProfile, model.ProfileSource, error) {
	var profileID int64
	err := r.db.Pool.QueryRow(ctx, `
		SELECT profile_id FROM project_profiles WHERE project_id = $1 AND language = $2`,
		projectID, language,
	).Scan(&profileID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, "", err
	}
	if err == nil {
		profile, err := r.GetByID(ctx, profileID)
		return profile, model.ProfileSourceAssigned, err
	}

	err = r.db.Pool.QueryRow(ctx, `
		SELECT id FROM quality_profiles WHERE language = $1 AND is_default = TRUE ORDER BY is_builtin DESC, id LIMIT 1`,
		language,
	).Scan(&profileID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", ErrNotFound
	}
	if err != nil {
		return nil, "", err
	}
	profile, err := r.GetByID(ctx, profileID)
	return profile, model.ProfileSourceDefault, err
}

func (r *ProfileRepository) recordProfileChangelog(ctx context.Context, entry model.ProfileChangelogEntry) error {
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO quality_profile_changelog (profile_id, project_id, language, action, rule_key, old_value, new_value, actor_user_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		nullZeroInt64(entry.ProfileID), nullZeroInt64(entry.ProjectID), entry.Language, entry.Action, entry.RuleKey, entry.OldValue, entry.NewValue, nullZeroInt64(entry.ActorUserID))
	return err
}

func scanProfiles(rows pgx.Rows) ([]*QualityProfile, error) {
	var out []*QualityProfile
	for rows.Next() {
		p := &QualityProfile{}
		if err := rows.Scan(&p.ID, &p.Name, &p.Language, &p.ParentID,
			&p.IsDefault, &p.IsBuiltin, &p.RuleCount, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		enrichProfile(p)
		out = append(out, p)
	}
	return out, rows.Err()
}

func enrichProfile(profile *QualityProfile) {
	profile.HasRules = rulecatalog.LanguageHasRules(profile.Language)
	profile.ParserOnly = rulecatalog.LanguageIsParserOnly(profile.Language)
}

func activeRuleCount(rules []*model.EffectiveRule) int {
	count := 0
	for _, rule := range rules {
		if rule != nil && !rule.Disabled && !strings.EqualFold(rule.Severity, "OFF") {
			count++
		}
	}
	return count
}

func (r *ProfileRepository) validateRuleActivation(ctx context.Context, language, ruleKey, severity string, params map[string]string) (*coredomain.Rule, string, error) {
	rule, versionHash, err := r.lookupActivationRule(ctx, language, ruleKey, severity)
	if err != nil {
		return nil, "", err
	}
	if rule.Language != language && rule.Language != "*" {
		return nil, "", fmt.Errorf("rule %q belongs to language %q, not %q", ruleKey, rule.Language, language)
	}
	if severity != "" && !validSeverity(severity) {
		return nil, "", fmt.Errorf("invalid severity %q", severity)
	}
	if err := validateActivationParams(rule, ruleKey, params); err != nil {
		return nil, "", err
	}
	return rule, versionHash, nil
}

func (r *ProfileRepository) lookupActivationRule(ctx context.Context, language, ruleKey, severity string) (*coredomain.Rule, string, error) {
	if rule, ok := rulecatalog.ByKey(ruleKey); ok {
		return rule, "", nil
	}
	customRule, hash, err := NewCustomRuleRepository(r.db).PublishedCoreRuleByKey(ctx, ruleKey)
	if errors.Is(err, ErrNotFound) {
		if strings.EqualFold(severity, "OFF") {
			return &coredomain.Rule{Key: ruleKey, Language: language, DefaultSeverity: coredomain.SeverityMajor, ParamsSchema: map[string]coredomain.ParamDef{}}, "", nil
		}
		return nil, "", fmt.Errorf("unknown rule %q", ruleKey)
	}
	if err != nil {
		return nil, "", err
	}
	return customRule, hash, nil
}

func validateActivationParams(rule *coredomain.Rule, ruleKey string, params map[string]string) error {
	for key, value := range params {
		param, ok := rule.ParamsSchema[key]
		if !ok {
			return fmt.Errorf("unknown parameter %q for rule %q", key, ruleKey)
		}
		if err := validateParamValue(param.Type, value); err != nil {
			return fmt.Errorf("invalid parameter %q for rule %q: %w", key, ruleKey, err)
		}
	}
	return nil
}

func validSeverity(severity string) bool {
	switch strings.ToLower(severity) {
	case "blocker", "critical", "major", "minor", "info", "off":
		return true
	default:
		return false
	}
}

func validateParamValue(paramType, value string) error {
	switch paramType {
	case "int":
		_, err := strconv.Atoi(value)
		return err
	case "float":
		_, err := strconv.ParseFloat(value, 64)
		return err
	case "bool":
		_, err := strconv.ParseBool(value)
		return err
	case "string", "":
		return nil
	default:
		return fmt.Errorf("unsupported parameter type %q", paramType)
	}
}
