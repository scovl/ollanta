package scan_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	appscan "github.com/scovl/ollanta/application/scan"
	"github.com/scovl/ollanta/domain/model"
	scanner "github.com/scovl/ollanta/ollantascanner/scan"
)

func TestRun_UsesServerCustomCatalogAndEffectiveProfile(t *testing.T) {
	t.Parallel()

	rule := model.NormalizeCustomRuleDefinition(model.CustomRuleDefinition{
		RuleKey:         "server:no-debug-marker",
		Name:            "No debug marker",
		Language:        model.LangGo,
		Type:            model.TypeCodeSmell,
		DefaultSeverity: model.SeverityMajor,
		Engine:          model.CustomRuleEngineText,
		EngineConfig: map[string]string{
			"pattern": "SERVER_CUSTOM_RULE_MARKER",
		},
		Message:   "Remove the server custom marker.",
		Lifecycle: model.CustomRulePublished,
	})
	rule.VersionHash = model.HashCustomRuleDefinition(rule)
	catalogHash := model.HashCustomRuleCatalog([]model.CustomRuleDefinition{rule})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer token" {
			t.Fatalf("authorization = %q, want Bearer token", got)
		}
		switch r.URL.Path {
		case "/api/v1/custom-rules/catalog":
			writeJSON(t, w, model.CustomRuleCatalogSnapshot{Hash: catalogHash, RuleCount: 1, Rules: []model.CustomRuleDefinition{rule}})
		case "/api/v1/projects/demo/profiles/effective":
			profiles := []*model.EffectiveQualityProfile{{
				Language:          model.LangGo,
				ProfileName:       "Remote custom",
				Source:            model.ProfileSourceAssigned,
				CustomCatalogHash: catalogHash,
				Rules: []*model.EffectiveRule{{
					RuleKey:         rule.RuleKey,
					Severity:        string(model.SeverityCritical),
					RuleVersionHash: rule.VersionHash,
				}},
			}}
			writeJSON(t, w, profiles)
		default:
			t.Fatalf("unexpected request path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	projectDir := t.TempDir()
	writeFile(t, filepath.Join(projectDir, "main.go"), "package main\n\nfunc main() {\n\tprintln(\"SERVER_CUSTOM_RULE_MARKER\")\n}\n")

	report, err := scanner.Run(context.Background(), &scanner.ScanOptions{
		ProjectDir:  projectDir,
		ProjectKey:  "demo",
		Sources:     []string{"./..."},
		Format:      "summary",
		Server:      server.URL,
		ServerToken: "token",
		Profiles: appscan.ProfileOptions{
			Source:       appscan.ProfileSourceServer,
			Strict:       true,
			FetchTimeout: time.Second,
		},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.ScannerOptions.CustomRules.CatalogHash != catalogHash {
		t.Fatalf("catalog hash = %q, want %q", report.ScannerOptions.CustomRules.CatalogHash, catalogHash)
	}
	if len(report.Issues) != 1 {
		t.Fatalf("issues = %d, want 1", len(report.Issues))
	}
	issue := report.Issues[0]
	if issue.RuleKey != rule.RuleKey {
		t.Fatalf("issue rule key = %q, want %q", issue.RuleKey, rule.RuleKey)
	}
	if issue.Severity != model.SeverityCritical {
		t.Fatalf("issue severity = %q, want %q", issue.Severity, model.SeverityCritical)
	}
	if len(report.QualityProfiles) != 1 || report.QualityProfiles[0].CustomCatalogHash != catalogHash {
		t.Fatalf("quality profile snapshots = %+v", report.QualityProfiles)
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
