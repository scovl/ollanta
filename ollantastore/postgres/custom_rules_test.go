package postgres

import (
	"context"
	"strings"
	"testing"

	"github.com/scovl/ollanta/application/customrules"
	"github.com/scovl/ollanta/domain/model"
)

func TestCustomRuleRepository_ImportPublishActivateWithOmittedOptionalFields(t *testing.T) {
	db, ctx, prefix := openJobRepositoryTestDB(t)
	packName := prefix + "-custom-rules"
	profileName := prefix + "-profile"
	projectKey := prefix + "-project"

	t.Cleanup(func() {
		if _, err := db.Pool.Exec(context.Background(), "DELETE FROM quality_profiles WHERE name = $1 AND is_builtin = FALSE", profileName); err != nil {
			t.Logf("cleanup quality profile: %v", err)
		}
		if _, err := db.Pool.Exec(context.Background(), "DELETE FROM custom_rule_packs WHERE name = $1", packName); err != nil {
			t.Logf("cleanup custom rule pack: %v", err)
		}
	})

	repo := NewCustomRuleRepository(db)
	created := importCustomRuleForRegression(t, ctx, repo, packName, prefix)
	published := validateAndPublishCustomRule(t, ctx, repo, created)
	assertPublishedCatalogContains(t, ctx, repo, published.RuleKey)
	profile := createProfileWithCustomRule(t, ctx, db, profileName, published)
	assertProjectEffectiveProfileContainsRule(t, ctx, db, projectKey, profile.ID, published.RuleKey)
}

func importCustomRuleForRegression(t *testing.T, ctx context.Context, repo *CustomRuleRepository, packName, namespace string) model.CustomRuleDefinition {
	t.Helper()
	doc := model.CustomRulePackDocument{
		Version: model.CustomRulePackSchemaVersion,
		Pack: model.CustomRulePack{
			Name:      packName,
			Namespace: namespace,
		},
		Rules: []model.CustomRuleDefinition{{
			RuleKey:         "no-debug-marker",
			Name:            "No debug marker",
			Language:        model.LangGo,
			Type:            model.TypeCodeSmell,
			DefaultSeverity: model.SeverityMajor,
			Engine:          model.CustomRuleEngineText,
			EngineConfig: map[string]string{
				"pattern": "CUSTOM_RULE_DEBUG_MARKER",
			},
			Message: "Remove the custom debug marker.",
		}},
	}

	created, err := repo.ImportDocument(ctx, doc)
	if err != nil {
		t.Fatalf("ImportDocument() error = %v", err)
	}
	if len(created) != 1 {
		t.Fatalf("ImportDocument() created %d rules, want 1", len(created))
	}
	if created[0].Tags == nil {
		t.Fatal("expected omitted tags to be normalized to an empty slice")
	}
	return created[0]
}

func validateAndPublishCustomRule(t *testing.T, ctx context.Context, repo *CustomRuleRepository, created model.CustomRuleDefinition) *model.CustomRuleDefinition {
	t.Helper()
	validation := customrules.Validate(ctx, created, customrules.ValidationContext{AllowExistingRuleKey: created.RuleKey})
	if validation.Status != model.CustomRuleValidationPassed {
		t.Fatalf("validation status = %q, diagnostics = %+v", validation.Status, validation.Diagnostics)
	}
	validated, err := repo.StoreValidation(ctx, created.ID, validation)
	if err != nil {
		t.Fatalf("StoreValidation() error = %v", err)
	}
	if !model.CanPublishCustomRule(*validated) {
		t.Fatalf("validated rule is not publishable: lifecycle=%q validation_hash=%q version_hash=%q", validated.Lifecycle, validated.ValidationHash, validated.VersionHash)
	}

	published, err := repo.Publish(ctx, validated.ID)
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if published.Lifecycle != model.CustomRulePublished {
		t.Fatalf("published lifecycle = %q, want %q", published.Lifecycle, model.CustomRulePublished)
	}
	return published
}

func assertPublishedCatalogContains(t *testing.T, ctx context.Context, repo *CustomRuleRepository, ruleKey string) {
	t.Helper()
	snapshot, err := repo.PublishedCatalogSnapshot(ctx)
	if err != nil {
		t.Fatalf("PublishedCatalogSnapshot() error = %v", err)
	}
	if !catalogContainsRule(snapshot, ruleKey) {
		t.Fatalf("published catalog does not contain %q", ruleKey)
	}
}

func createProfileWithCustomRule(t *testing.T, ctx context.Context, db *DB, profileName string, published *model.CustomRuleDefinition) *model.QualityProfile {
	t.Helper()
	profileRepo := NewProfileRepository(db)
	profile := &model.QualityProfile{Name: profileName, Language: model.LangGo}
	if err := profileRepo.Create(ctx, profile); err != nil {
		t.Fatalf("Create profile error = %v", err)
	}
	if err := profileRepo.ActivateRule(ctx, profile.ID, published.RuleKey, string(model.SeverityCritical), nil); err != nil {
		t.Fatalf("ActivateRule() error = %v", err)
	}
	effective, err := profileRepo.ResolveEffectiveRules(ctx, profile.ID)
	if err != nil {
		t.Fatalf("ResolveEffectiveRules() error = %v", err)
	}
	activeRule := findEffectiveRule(effective, published.RuleKey)
	if activeRule == nil {
		t.Fatalf("effective rules do not contain %q", published.RuleKey)
	}
	if activeRule.RuleVersionHash != published.VersionHash {
		t.Fatalf("rule version hash = %q, want %q", activeRule.RuleVersionHash, published.VersionHash)
	}
	return profile
}

func assertProjectEffectiveProfileContainsRule(t *testing.T, ctx context.Context, db *DB, projectKey string, profileID int64, ruleKey string) {
	t.Helper()
	profileRepo := NewProfileRepository(db)
	project := &Project{Key: projectKey, Name: projectKey, MainBranch: "main"}
	if err := NewProjectRepository(db).Create(ctx, project); err != nil {
		t.Fatalf("Create project error = %v", err)
	}
	if err := profileRepo.AssignToProject(ctx, project.ID, model.LangGo, profileID); err != nil {
		t.Fatalf("AssignToProject() error = %v", err)
	}
	projectProfiles, err := profileRepo.ProjectEffectiveProfiles(ctx, project.ID)
	if err != nil {
		t.Fatalf("ProjectEffectiveProfiles() error = %v", err)
	}
	goProfile := findEffectiveProfile(projectProfiles, model.LangGo)
	if goProfile == nil || goProfile.CustomCatalogHash == "" {
		t.Fatalf("go effective profile missing custom catalog hash: %+v", goProfile)
	}
	if findEffectiveRule(goProfile.Rules, ruleKey) == nil {
		t.Fatalf("project effective profile does not contain %q", ruleKey)
	}
}

func catalogContainsRule(snapshot *model.CustomRuleCatalogSnapshot, ruleKey string) bool {
	if snapshot == nil {
		return false
	}
	for _, rule := range snapshot.Rules {
		if strings.EqualFold(rule.RuleKey, ruleKey) {
			return true
		}
	}
	return false
}

func findEffectiveRule(rules []*model.EffectiveRule, ruleKey string) *model.EffectiveRule {
	for _, rule := range rules {
		if rule != nil && rule.RuleKey == ruleKey {
			return rule
		}
	}
	return nil
}

func findEffectiveProfile(profiles []*model.EffectiveQualityProfile, language string) *model.EffectiveQualityProfile {
	for _, profile := range profiles {
		if profile != nil && profile.Language == language {
			return profile
		}
	}
	return nil
}
